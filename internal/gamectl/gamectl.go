package gamectl

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
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
	gameExitedNormallyErr = errors.New("game exited without error")
)

type Notifier interface {
	GameStarted(exename string)
	GameStopped(exename string, err error)
}

type Routine struct {
	Game    *appconfig.Game
	User32  *user32util.User32DLL
	Notif   Notifier
	ticker  *time.Ticker
	current *runningGameRoutine
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
	o.ticker = time.NewTicker(5 * time.Second)

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
		o.ticker.Stop()
		if o.current != nil {
			o.current.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-o.ticker.C:
			err := o.checkGameRunning()
			if err != nil {
				return fmt.Errorf("failed to handle game startup for %s - %w", o.Game.ExeName, err)
			}
		case <-o.current.Done():
			log.Printf("%s routine exited - %s", o.Game.ExeName, o.current.Err())
			o.ticker.Reset(5 * time.Second)

			if o.Notif != nil {
				if errors.Is(o.current.Err(), gameExitedNormallyErr) {
					o.Notif.GameStopped(o.Game.ExeName, nil)
				} else {
					o.Notif.GameStopped(o.Game.ExeName, o.Err())
				}
			}

			o.current = nil
		}
	}
}

func (o *Routine) checkGameRunning() error {
	// TODO: logger to make prefix with exename
	log.Printf("checking for game running with exe name: %s", o.Game.ExeName)

	processes, err := ps.Processes()
	if err != nil {
		return fmt.Errorf("failed to get active processes - %w", err)
	}

	possiblePID := -1
	for _, process := range processes {
		if process.Executable() == o.Game.ExeName {
			possiblePID = process.Pid()
			break
		}
	}

	if possiblePID == -1 {
		return nil
	}

	proc, err := kiwi.GetProcessByPID(possiblePID)
	if err != nil {
		return fmt.Errorf("failed to get process by PID - %w", err)
	}

	runningGame, err := newRunningGameRoutine(o.Game, proc, o.User32)
	if err != nil {
		return fmt.Errorf("failed to create new running game routine - %w", err)
	}

	o.current = runningGame
	o.ticker.Stop()
	if o.Notif != nil {
		o.Notif.GameStarted(o.Game.ExeName)
	}

	return nil
}

func newRunningGameRoutine(game *appconfig.Game, proc kiwi.Process, dll *user32util.User32DLL) (*runningGameRoutine, error) {
	gameStates := make(map[string]*gameState)
	for _, pointer := range game.Pointers {
		gameStates[pointer.Name] = &gameState{
			pointer: pointer,
		}
	}

	runningGame := &runningGameRoutine{
		game:   game,
		proc:   proc,
		states: gameStates,
		done:   make(chan struct{}),
	}

	baseAddr, err := kernel32.ModuleBaseAddr(syscall.Handle(proc.Handle), game.ExeName)
	if err != nil {
		return nil, fmt.Errorf("failed to get module base address - %w", err)
	}
	runningGame.base = baseAddr

	listener, err := user32util.NewLowLevelKeyboardListener(runningGame.handleKeyboardEvent, dll)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener - %s", err.Error())
	}
	runningGame.ln = listener

	process, err := os.FindProcess(int(proc.PID))
	if err != nil {
		runningGame.Stop()
		return nil, fmt.Errorf("failed to find process with PID: %d - %w", proc.PID, err)
	}

	go func() {
		_, err := process.Wait()
		if err == nil {
			err = gameExitedNormallyErr
		}

		runningGame.exited(err)
	}()

	go func() {
		err := <-listener.OnDone()
		if err == nil {
			err = errors.New("listener exited without error")
		}

		runningGame.exited(err)
	}()

	return runningGame, nil
}

type runningGameRoutine struct {
	game   *appconfig.Game
	base   uintptr
	proc   kiwi.Process
	states map[string]*gameState
	once   sync.Once
	ln     *user32util.LowLevelKeyboardEventListener
	done   chan struct{}
	err    error
}

func (o *runningGameRoutine) Stop() {
	o.exited(errors.New("stopped"))
}

func (o *runningGameRoutine) Done() <-chan struct{} {
	if o == nil {
		return nil
	}

	return o.done
}

func (o *runningGameRoutine) Err() error {
	return o.err
}

func (o *runningGameRoutine) exited(err error) {
	o.once.Do(func() {
		o.ln.Release()
		o.err = err
		close(o.done)
	})
}

func (o *runningGameRoutine) handleKeyboardEvent(event user32util.LowLevelKeyboardEvent) {
	err := o.handleKeyboardEventWithError(event)
	if err != nil {
		o.exited(err)
	}
}

func (o *runningGameRoutine) handleKeyboardEventWithError(event user32util.LowLevelKeyboardEvent) error {
	if event.KeyboardButtonAction() != user32util.WMKeyDown {
		return nil
	}

	switch event.Struct.VirtualKeyCode() {
	case o.game.SaveState:
		for name, state := range o.states {
			log.Printf("saving state %s at %+v", name, state.pointer)

			// TODO: refactor function to just take a single slice
			addr, err := getAddr(o.proc, o.base, state.pointer.Addrs[0], state.pointer.Addrs[1:]...)
			if err != nil {
				return err
			}

			savedState, err := o.proc.ReadUint32(uintptr(addr))
			if err != nil {
				// TODO update with INI name
				return fmt.Errorf("error while trying to read from %s at 0x%x - %w",
					name, addr, err)
			}

			log.Printf("saved state %s at %+v as 0x%x",
				name, state.pointer, savedState)

			state.savedState = savedState
			state.stateSet = true
		}
	case o.game.RestoreState:
		for name, state := range o.states {
			if !state.stateSet {
				continue
			}

			log.Printf("restoring state %s at %+v to 0x%x",
				name, state.pointer, state.savedState)

			addr, err := getAddr(o.proc, o.base, state.pointer.Addrs[0], state.pointer.Addrs[1:]...)
			if err != nil {
				return err
			}

			err = o.proc.WriteUint32(uintptr(addr), state.savedState)
			if err != nil {
				return fmt.Errorf("error while trying to write to %s at 0x%x - %w",
					name, addr, err)
			}
		}
	}
	return nil
}

func getAddr(proc kiwi.Process, base uintptr, start uint32, offsets ...uint32) (uint32, error) {
	addr, err := proc.ReadUint32(base + uintptr(start))
	if err != nil {
		return 0, fmt.Errorf("error while trying to read from target process at 0x%x - %w", addr, err)
	}

	if len(offsets) == 0 {
		return addr, nil
	}

	for _, offset := range offsets[:len(offsets)-1] {
		addr, err = proc.ReadUint32(uintptr(addr + offset))
		if err != nil {
			return 0, fmt.Errorf("error while trying to read from target process at 0x%x - %w", addr, err)
		}
	}
	addr += offsets[len(offsets)-1]
	return addr, nil
}

type gameState struct {
	pointer    appconfig.Pointer
	stateSet   bool
	savedState uint32 // TODO: use uintpointer
}
