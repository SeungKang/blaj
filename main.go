package main

import (
	_ "embed"
	"fmt"
	"github.com/Andoryuuta/kiwi"
	"github.com/SeungKang/speedometer/internal/appconfig"
	"github.com/stephen-fox/user32util"
	_ "image/png"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	xCoord      float32
	yCoord      float32
	zCoord      float32
	teleportSet = false
)

func main() {
	log.SetFlags(0)

	err := mainWithError()
	if err != nil {
		log.Fatalln("error:", err)
	}
}

func mainWithError() error {
	config, err := appconfig.Parse(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to parse config - %w", err)
	}

	log.Println(config.Games[0].ExeName)

	var proc kiwi.Process
	for {
		proc, err = kiwi.GetProcessByFileName("MirrorsEdge.exe")
		if err != nil {
			log.Printf("failed to find process")
		} else {
			break
		}

		time.Sleep(5 * time.Second)
	}

	user32, err := user32util.LoadUser32DLL()
	if err != nil {
		log.Fatalf("failed to load user32.dll - %s", err.Error())
	}

	fn := func(event user32util.LowLevelKeyboardEvent) {
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

	listener, err := user32util.NewLowLevelKeyboardListener(fn, user32)
	if err != nil {
		log.Fatalf("failed to create listener - %s", err.Error())
	}
	defer listener.Release()

	log.Println("Speedometer started")

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-listener.OnDone():
		log.Fatalf("keyboard listener stopped unexpectedly - %v", err)
	case <-interrupts:
	}

	return nil
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
