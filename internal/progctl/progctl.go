package progctl

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Andoryuuta/kiwi"
	"github.com/SeungKang/blaj/internal/appconfig"
	"github.com/SeungKang/blaj/internal/kernel32"
	"github.com/mitchellh/go-ps"
	"github.com/stephen-fox/user32util"
)

var (
	programExitedNormallyErr = errors.New("program exited without error")
)

type Notifier interface {
	ProgramStarted(exename string)
	ProgramStopped(exename string, err error)
}

type Routine struct {
	Program *appconfig.ProgramConfig
	User32  *user32util.User32DLL
	Notif   Notifier
	timer   *time.Timer
	current *runningProgramRoutine
	done    chan struct{}
	err     error
}

func (o *Routine) Done() <-chan struct{} {
	return o.done
}

func (o *Routine) Err() error {
	return o.err
}

func (o *Routine) Start(ctx context.Context) {
	o.done = make(chan struct{})
	o.timer = time.NewTimer(time.Millisecond)

	go o.loop(ctx)
}

func (o *Routine) loop(ctx context.Context) {
	var cancelFn func()
	ctx, cancelFn = context.WithCancel(ctx)
	defer cancelFn()

	o.err = o.loopWithError(ctx)
	close(o.done)
}

func (o *Routine) loopWithError(ctx context.Context) error {
	defer func() {
		o.timer.Stop()
		if o.current != nil {
			o.current.Stop()
		}
	}()

	log.Printf("checking for program running with exe name: %s", o.Program.General.ExeName)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-o.timer.C:
			err := o.checkProgramRunning()
			if err != nil {
				return fmt.Errorf("failed to handle program startup for %s - %w", o.Program.General.ExeName, err)
			}
		case <-o.current.Done():
			log.Printf("%s routine exited - %s", o.Program.General.ExeName, o.current.Err())
			o.timer.Reset(5 * time.Second)

			if o.Notif != nil {
				if errors.Is(o.current.Err(), programExitedNormallyErr) {
					o.Notif.ProgramStopped(o.Program.General.ExeName, nil)
				} else {
					o.Notif.ProgramStopped(o.Program.General.ExeName, o.current.Err())
				}
			}

			o.current = nil
		}
	}
}

func (o *Routine) checkProgramRunning() error {
	// TODO: logger to make prefix with exename
	processes, err := ps.Processes()
	if err != nil {
		return fmt.Errorf("failed to get active processes - %w", err)
	}

	possiblePID := -1
	for _, process := range processes {
		if strings.ToLower(process.Executable()) == o.Program.General.ExeName {
			possiblePID = process.Pid()
			break
		}
	}

	if possiblePID == -1 {
		o.timer.Reset(5 * time.Second)
		return nil
	}

	runningProgram, err := newRunningProgramRoutine(o.Program, possiblePID, o.User32)
	if err != nil {
		return fmt.Errorf("failed to create new running program routine - %w", err)
	}

	o.current = runningProgram
	if o.Notif != nil {
		o.Notif.ProgramStarted(o.Program.General.ExeName)
	}

	return nil
}

// TODO: make source file for running program stuff
func newRunningProgramRoutine(program *appconfig.ProgramConfig, pid int, dll *user32util.User32DLL) (*runningProgramRoutine, error) {
	proc, err := kiwi.GetProcessByPID(pid)
	if err != nil {
		return nil, fmt.Errorf("failed to get process by PID - %w", err)
	}

	// TODO: changing to be map[*appconfig.pointer]*programState
	programStates := make(map[string]*programState)
	for _, saveRestore := range program.SaveRestores {
		for _, pointer := range saveRestore.Pointers {
			programStates[pointer.Name] = &programState{
				pointer: pointer,
			}
		}
	}

	runningProgram := &runningProgramRoutine{
		program: program,
		proc:    proc,
		states:  programStates,
		done:    make(chan struct{}),
	}

	modules, err := kernel32.ProcessModules(syscall.Handle(proc.Handle))
	if err != nil {
		runningProgram.Stop()
		return nil, fmt.Errorf("failed to get process modules - %w", err)
	}

	baseAddr, requiredModules, err := getRequiredModules(program, modules)
	if err != nil {
		runningProgram.Stop()
		return nil, fmt.Errorf("failed to get required modules - %w", err)
	}

	runningProgram.base = baseAddr
	runningProgram.mods = requiredModules

	is32Bit, err := kernel32.IsProcess32Bit(syscall.Handle(proc.Handle))
	if err != nil {
		runningProgram.Stop()
		return nil, fmt.Errorf("failed to determine if process is 32 bit - %w", err)
	}
	runningProgram.is32b = is32Bit

	if is32Bit {
		runningProgram.addrFn = func(u uintptr) (uintptr, error) {
			data, err := proc.ReadUint32(u)
			return uintptr(data), err
		}
	} else {
		runningProgram.addrFn = func(u uintptr) (uintptr, error) {
			data, err := proc.ReadUint64(u)
			return uintptr(data), err
		}
	}

	listener, err := user32util.NewLowLevelKeyboardListener(runningProgram.handleKeyboardEvent, dll)
	if err != nil {
		runningProgram.Stop()
		return nil, fmt.Errorf("failed to create listener - %s", err.Error())
	}
	runningProgram.ln = listener

	process, err := os.FindProcess(int(proc.PID))
	if err != nil {
		runningProgram.Stop()
		return nil, fmt.Errorf("failed to find process with PID: %d - %w", proc.PID, err)
	}

	go func() {
		_, err := process.Wait()
		if err == nil {
			err = programExitedNormallyErr
		}

		runningProgram.exited(err)
	}()

	go func() {
		err := <-listener.OnDone()
		if err == nil {
			err = errors.New("listener exited without error")
		}

		runningProgram.exited(err)
	}()

	return runningProgram, nil
}

func getRequiredModules(program *appconfig.ProgramConfig, modules []kernel32.Module) (uintptr, map[string]kernel32.Module, error) {
	needed := make(map[string]kernel32.Module)
	needed[program.General.ExeName] = kernel32.Module{}
	for _, saveRestore := range program.SaveRestores {
		for _, pointer := range saveRestore.Pointers {
			if pointer.OptModule != "" {
				needed[pointer.OptModule] = kernel32.Module{}
			}
		}
	}

	numNeeded := len(needed)
	for _, module := range modules {
		moduleLc := strings.ToLower(module.Filename)

		_, isRequired := needed[moduleLc]
		if isRequired {
			needed[moduleLc] = module

			numNeeded--
			if numNeeded == 0 {
				return needed[program.General.ExeName].BaseAddr, needed, nil
			}
		}
	}

	var missing []string
	for name, tmp := range needed {
		if tmp.BaseAddr == 0 {
			missing = append(missing, name)
		}
	}

	return 0, nil, fmt.Errorf("failed to find modules: %q", missing)
}

type runningProgramRoutine struct {
	program *appconfig.ProgramConfig
	base    uintptr
	is32b   bool
	mods    map[string]kernel32.Module
	addrFn  func(uintptr) (uintptr, error)
	proc    kiwi.Process
	states  map[string]*programState
	once    sync.Once
	ln      *user32util.LowLevelKeyboardEventListener
	done    chan struct{}
	err     error
}

func (o *runningProgramRoutine) Stop() {
	o.exited(errors.New("stopped"))
}

func (o *runningProgramRoutine) Done() <-chan struct{} {
	if o == nil {
		return nil
	}

	return o.done
}

func (o *runningProgramRoutine) Err() error {
	return o.err
}

func (o *runningProgramRoutine) exited(err error) {
	o.once.Do(func() {
		_ = syscall.CloseHandle(syscall.Handle(o.proc.Handle))
		if o.ln != nil {
			o.ln.Release()
		}
		o.err = err
		close(o.done)
	})
}

func (o *runningProgramRoutine) handleKeyboardEvent(event user32util.LowLevelKeyboardEvent) {
	err := o.handleKeyboardEventWithError(event)
	if err != nil {
		o.exited(err)
	}
}

func (o *runningProgramRoutine) handleKeyboardEventWithError(event user32util.LowLevelKeyboardEvent) error {
	if event.KeyboardButtonAction() != user32util.WMKeyDown {
		return nil
	}

	pressedKey := event.Struct.VirtualKeyCode()
	sections, hasKeybind := o.program.Keybinds[pressedKey]
	if !hasKeybind {
		return nil
	}

	for _, section := range sections {
		switch v := section.(type) {
		case *appconfig.SaveRestore:
			switch pressedKey {
			case v.SaveState:
				for _, pointer := range v.Pointers {
					state, hasIt := o.states[pointer.Name]
					if !hasIt {
						continue
					}
					err := o.saveState(pointer.Name, state)
					if err != nil {
						return fmt.Errorf("failed to get %s state at %+#v to 0x%x",
							pointer.Name, pointer, state.savedState)
					}
				}
			case v.RestoreState:
				for _, pointer := range v.Pointers {
					state, hasIt := o.states[pointer.Name]
					if !hasIt || !state.stateSet {
						continue
					}
					err := o.restoreState(pointer.Name, state)
					if err != nil {
						return fmt.Errorf("failed to restore %s state at %+#v to 0x%x",
							pointer.Name, state.pointer, state.savedState)
					}
				}
			}
		case *appconfig.Writer:
			for _, pointer := range v.Pointers {
				err := o.write(pointer)
				if err != nil {
					return fmt.Errorf("failed to write to %s - %w", pointer.Pointer.Name, err)
				}
			}
		}
	}

	return nil
}

func (o *runningProgramRoutine) saveState(name string, state *programState) error {
	baseAddr := o.base
	if state.pointer.OptModule != "" {
		module, hasIt := o.mods[state.pointer.OptModule]
		if !hasIt {
			return fmt.Errorf("unknown module %q", state.pointer.OptModule)
		}

		baseAddr = module.BaseAddr
	}

	stateAddr, err := lookupAddr(baseAddr, state.pointer, o.addrFn)
	if err != nil {
		return fmt.Errorf("failed to lookup address of state %s - %w",
			name, err)
	}

	savedState, err := o.proc.ReadBytes(stateAddr, state.pointer.NBytes)
	if err != nil {
		// TODO: update with INI name
		return fmt.Errorf("failed to read from %s at 0x%x - %w",
			name, stateAddr, err)
	}

	state.savedState = savedState
	state.stateSet = true
	log.Printf("saved %s state at 0x%x", name, stateAddr)

	return nil
}

func (o *runningProgramRoutine) restoreState(name string, state *programState) error {
	baseAddr := o.base
	if state.pointer.OptModule != "" {
		module, hasIt := o.mods[state.pointer.OptModule]
		if !hasIt {
			return fmt.Errorf("unknown module %q", state.pointer.OptModule)
		}

		baseAddr = module.BaseAddr
	}

	stateAddr, err := lookupAddr(baseAddr, state.pointer, o.addrFn)
	if err != nil {
		return fmt.Errorf("failed to get memory address of state %s - %w",
			name, err)
	}

	err = o.proc.WriteBytes(stateAddr, state.savedState)
	if err != nil {
		return fmt.Errorf("failed to write to %s at 0x%x - %w",
			name, stateAddr, err)
	}

	log.Printf("restored %s state at 0x%x", name, stateAddr)
	return nil
}

func lookupAddr(base uintptr, ptr appconfig.Pointer, addrFn func(uintptr) (uintptr, error)) (uintptr, error) {
	start := ptr.Addrs[0]
	if len(ptr.Addrs) == 1 {
		return base + start, nil
	}

	addr, err := addrFn(base + start)
	if err != nil {
		return 0, fmt.Errorf("failed to read from target process at 0x%x - %w",
			addr, err)
	}

	var offsets = ptr.Addrs[1:]
	for _, offset := range offsets[:len(offsets)-1] {
		addr, err = addrFn(addr + offset)
		if err != nil {
			return 0, fmt.Errorf("failed to read from target process at 0x%x - %w",
				addr, err)
		}
	}

	addr += offsets[len(offsets)-1]

	return addr, nil
}

func (o *runningProgramRoutine) write(pointer appconfig.WritePointer) error {
	baseAddr := o.base
	if pointer.Pointer.OptModule != "" {
		module, hasIt := o.mods[pointer.Pointer.OptModule]
		if !hasIt {
			return fmt.Errorf("unknown module %q", pointer.Pointer.OptModule)
		}

		baseAddr = module.BaseAddr
	}

	writeAddr, err := lookupAddr(baseAddr, pointer.Pointer, o.addrFn)
	if err != nil {
		return fmt.Errorf("failed to lookup write address %s - %w",
			pointer.Pointer.Name, err)
	}

	err = o.proc.WriteBytes(writeAddr, pointer.Data)
	if err != nil {
		// TODO: update with INI name
		return fmt.Errorf("failed to write bytes at %s (0x%x) - %w",
			pointer.Pointer.Name, writeAddr, err)
	}

	log.Printf("wrote bytes at %s (0x%x)", pointer.Pointer.Name, writeAddr)

	return nil
}

type programState struct {
	pointer    appconfig.Pointer
	stateSet   bool
	savedState []byte
}
