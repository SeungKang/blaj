package teleporter

import (
	"context"
	"fmt"
	"github.com/Andoryuuta/kiwi"
	"github.com/SeungKang/speedometer/internal/appconfig"
	"github.com/stephen-fox/user32util"
	"log"
	"math"
	"time"
)

type GameRoutine struct {
	Game   *appconfig.Game
	User32 *user32util.User32DLL
	ticker *time.Ticker
	//proc   kiwi.Process
	//xCoord float32
	//yCoord float32
	//zCoord float32
	//telset bool
	//kbEvnt chan user32util.LowLevelKeyboardEvent
	done   chan struct{}
	err    error
}

func (o *GameRoutine) Done() <-chan struct{} {
	return o.done
}

func (o *GameRoutine) Err() error {
	return o.err
}

func (o *GameRoutine) Start(ctx context.Context) error {
	o.done = make(chan struct{})
	o.ticker = time.NewTicker(5 * time.Second)

	go o.loop(ctx)
}

func (o *GameRoutine) loop(ctx context.Context) {
	var cancelFn func()
	ctx, cancelFn = context.WithCancel(ctx)
	defer cancelFn()

	o.err = o.loopWithError(ctx)
	close(o.done)
}

func (o *GameRoutine) loopWithError(ctx context.Context) error {
	defer o.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-o.ticker.C:
			err := o.handleGameStartup(ctx)
			if err != nil {
				return fmt.Errorf("failed to handle game startup - %w", err)
			}
		case:
			o.handleKeyboardEvent(o.User32)
			// read keyboard inputs
			// execute keyboard action code
		}
	}
}

func (o *GameRoutine) handleGameStartup(ctx context.Context) error {
	// TODO: first check if process exists
	proc, err := kiwi.GetProcessByFileName(o.Game.ExeName)
	if err != nil {
		log.Printf("failed to get process by exe name - %s", err)
		return nil
	}

	gameStates := make(map[string]*gameState)
	for _, pointer := range o.Game.Pointers {
		gameStates[pointer.Name] = &gameState{
			pointer:    pointer,
		}
	}

	runningGame := newRunningGameRoutine(o.Game, proc, gameStates)

	listener, err := user32util.NewLowLevelKeyboardListener(runningGame.handleKeyboardEvent, o.User32)
	if err != nil {
		log.Fatalf("failed to create listener - %s", err.Error())
	}
	defer listener.Release()

	o.ticker.Stop()

	return nil
}

func newRunningGameRoutine(game *appconfig.Game, proc kiwi.Process, state map[string]*gameState) *runningGameRoutine {
	return &runningGameRoutine{
		game:   game,
		proc:   proc,
		states: state,
		done:   make(chan struct{}),
	}
}

type runningGameRoutine struct {
	game *appconfig.Game
	proc   kiwi.Process
	states map[string]*gameState
	kbEvnt chan user32util.LowLevelKeyboardEvent
	done   chan struct{}
	err    error
}

func (o *runningGameRoutine) handleKeyboardEvent(event user32util.LowLevelKeyboardEvent) {
	if o.err != nil {
		return
	}

	err := o.handleKeyboardEventWithError(event)
	if err != nil {
		o.err = err
		close(o.done)
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

func getFloat(proc kiwi.Process, start uint32, offsets ...uint32) (float32, error) {
	addr, err := getAddr(proc, start, offsets...)
	if err != nil {
		return 0, err
	}

	chunk, err := proc.ReadUint32(uintptr(addr))
	if err != nil {
		return 0, fmt.Errorf("failed to read memory at 0x%x - %w", addr, err)
	}

	return math.Float32frombits(chunk), nil
}

type gameState struct {
	pointer appconfig.Pointer
	stateSet bool
	savedState uint32 // TODO: use uintpointer
}
