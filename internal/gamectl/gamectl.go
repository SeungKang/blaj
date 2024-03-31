package gamectl

import (
	"context"
	"errors"
	"fmt"
	"github.com/Andoryuuta/kiwi"
	"github.com/SeungKang/speedometer/internal/appconfig"
	"github.com/stephen-fox/user32util"
	"log"
	"sync"
	"time"
)

type Routine struct {
	Game    *appconfig.Game
	User32  *user32util.User32DLL
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
			err := o.handleGameStartup()
			if err != nil {
				return fmt.Errorf("failed to handle game startup for %s - %w", o.Game.ExeName, err)
			}
		case <-o.current.Done():
			log.Printf("%s routine exited - %s", o.Game.ExeName, o.current.Err())
			o.ticker.Reset(5 * time.Second)
			o.current = nil
		}
	}
}

func (o *Routine) handleGameStartup() error {
	// TODO: first check if process exists
	proc, err := kiwi.GetProcessByFileName(o.Game.ExeName)
	if err != nil {
		log.Printf("failed to get process by exe name - %s", err)
		return nil
	}

	runningGame, err := newRunningGameRoutine(o.Game, proc, o.User32)
	if err != nil {
		return fmt.Errorf("failed to create new running game routine - %w", err)
	}

	o.current = runningGame
	o.ticker.Stop()

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

	listener, err := user32util.NewLowLevelKeyboardListener(runningGame.handleKeyboardEvent, dll)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener - %s", err.Error())
	}

	go func() {
		err := <-listener.OnDone()
		if err == nil {
			err = errors.New("listener exited without error")
		}

		runningGame.exited(err)
	}()

	runningGame.ln = listener
	return runningGame, nil
}

type runningGameRoutine struct {
	game   *appconfig.Game
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

	switch event.Struct.VkCode {
	case o.game.SaveState:
		for name, state := range o.states {
			log.Printf("saving state %s at %+v", name, state.pointer)
			// TODO: refactor function to just take a single slice
			addr, err := getAddr(o.proc, state.pointer.Addrs[0], state.pointer.Addrs[1:]...)
			if err != nil {
				return err
			}

			savedState, err := o.proc.ReadUint32(uintptr(addr))
			if err != nil {
				return err
			}

			log.Printf("saved state %s at %+v as 0x%x", name, state.pointer, savedState)

			state.savedState = savedState
			state.stateSet = true
		}
	case o.game.RestoreState:
		for name, state := range o.states {
			if !state.stateSet {
				continue
			}

			log.Printf("restoring state %s at %+v to 0x%x", name, state.pointer, state.savedState)
			addr, err := getAddr(o.proc, state.pointer.Addrs[0], state.pointer.Addrs[1:]...)
			if err != nil {
				return err
			}

			err = o.proc.WriteUint32(uintptr(addr), state.savedState)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func getAddr(proc kiwi.Process, start uint32, offsets ...uint32) (uint32, error) {
	addr, err := proc.ReadUint32(uintptr(start + 0x400000)) // 400000 the base address
	if err != nil {
		return 0, fmt.Errorf("error while trying to read from target process at 0x%x - %w", addr, err)
	}

	if len(offsets) == 0 {
		return 0, fmt.Errorf("please specify 1 or more offset")
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
