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
	proc   *kiwi.Process
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
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// kiwi stuff
			// once process found ticker.Stop
			// save process to proc field in gameroutine
		case:
			// read keyboard inputs
			// execute keyboard action code
		}
	}
}

func (o *GameRoutine) handleKeyboardEvent(event user32util.LowLevelKeyboardEvent) {
	if o.proc == nil {
		return
	}

	// TODO: invert if statement
	if event.KeyboardButtonAction() == user32util.WMKeyDown {
		switch event.Struct.VkCode {
		// Key 4
		case 52:
			//fmt.Printf("%q (%d) down\n", event.Struct.VirtualKeyCode(), event.Struct.VkCode)
			// x coord
			xCoord, err = getFloat(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xE8)
			if err != nil {
				log.Println(err)
				return
			}

			// y coord
			yCoord, err = getFloat(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xEC)
			if err != nil {
				log.Println(err)
				return
			}

			// z coord
			zCoord, err = getFloat(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xF0)
			if err != nil {
				log.Println(err)
				return
			}

			teleportSet = true

			log.Printf("teleport set  (%.2f, %.2f, %.2f)", xCoord, yCoord, zCoord)
		// Key 5
		case 53:
			if teleportSet == false {
				log.Println("teleport not set, press 4 to set teleport")
				return
			}

			// x coord
			xAddr, err := getAddr(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xE8)
			if err != nil {
				log.Println(err)
				return
			}

			// y coord
			yAddr, err := getAddr(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xEC)
			if err != nil {
				log.Println(err)
				return
			}

			// z coord
			zAddr, err := getAddr(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xF0)
			if err != nil {
				log.Println(err)
				return
			}

			err = proc.WriteFloat32(uintptr(xAddr), xCoord)
			err = proc.WriteFloat32(uintptr(yAddr), yCoord)
			err = proc.WriteFloat32(uintptr(zAddr), zCoord)
			log.Printf("teleported to (%.2f, %.2f, %.2f)", xCoord, yCoord, zCoord)
		}
	}
}

func getAddr(proc kiwi.Process, start uint32, offsets ...uint32) (uint32, error) {
	stringAddr, err := proc.ReadUint32(uintptr(start + 0x400000)) // 400000 the base address
	if err != nil {
		return 0, fmt.Errorf("error while trying to read from target process at 0x%x - %w", stringAddr, err)
	}

	if len(offsets) == 0 {
		return 0, fmt.Errorf("please specify 1 or more offset")
	}

	for _, offset := range offsets[:len(offsets)-1] {
		stringAddr, err = proc.ReadUint32(uintptr(stringAddr + offset))
		if err != nil {
			return 0, fmt.Errorf("error while trying to read from target process at 0x%x - %w", stringAddr, err)
		}
	}
	stringAddr += offsets[len(offsets)-1]
	return stringAddr, nil
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
