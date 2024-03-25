package main

import (
	_ "embed"
	"errors"
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
	return fmt.Errorf("%+v", config.Games[0])
	// Find the process from the executable name.
	proc, err := kiwi.GetProcessByFileName("MirrorsEdge.exe")
	if err != nil {
		return errors.New("failed to find process")
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
				xAddr, err := getAddr(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xE8)
				if err != nil {
					log.Println(err)
				}
				xCoord, err = getFloatAtAddr(proc, xAddr)
				if err != nil {
					log.Println(err)
				}

				// y coord
				yAddr, err := getAddr(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xEC)
				if err != nil {
					log.Println(err)
				}
				yCoord, err = getFloatAtAddr(proc, yAddr)
				if err != nil {
					log.Println(err)
				}

				// z coord
				zAddr, err := getAddr(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xF0)
				if err != nil {
					log.Println(err)
				}
				zCoord, err = getFloatAtAddr(proc, zAddr)
				if err != nil {
					log.Println(err)
				}

				teleportSet = true

				log.Printf("teleport set (%.2f, %.2f, %.2f)", xCoord, yCoord, zCoord)
			// Key 5
			case 53:
				if teleportSet == false {
					log.Println("teleport not set, press 4 to set teleport")
				}

				// x coord
				xAddr, err := getAddr(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xE8)
				if err != nil {
					log.Println(err)
				}

				// y coord
				yAddr, err := getAddr(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xEC)
				if err != nil {
					log.Println(err)
				}

				// z coord
				zAddr, err := getAddr(proc, 0x01C553D0, 0xCC, 0x1CC, 0x2F8, 0xF0)
				if err != nil {
					log.Println(err)
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

	log.Println("now listening for keyboard events - press Ctrl+C to stop")

	go scanMedgeLoop(proc)

	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-listener.OnDone():
		log.Fatalf("keyboard listener stopped unexpectedly - %v", err)
	case <-interrupts:
	}

	return nil
}

func scanMedgeLoop(proc kiwi.Process) {
	for {
		err := scanMedgeLoopWithError(proc)
		if err != nil {
			log.Printf("failed to open and/or read from medge process - %s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func scanMedgeLoopWithError(proc kiwi.Process) error {
	var lastCheckpoint string

	for {
		time.Sleep(50 * time.Millisecond)

		var exitStatus uint32
		err := syscall.GetExitCodeProcess(syscall.Handle(proc.Handle), &exitStatus)
		if err != nil {
			return fmt.Errorf("failed get exit code process - %s", err)
		}

		if exitStatus != 259 {
			// https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-getexitcodeprocess
			// 259 STILL_ACTIVE
			return fmt.Errorf("process exited with status: %d", exitStatus)
		}

		// get current checkpoint
		checkpointAddr, err := getAddr(proc, 0x01C55EA8, 0x74, 0x0, 0x3C, 0x0)
		if err != nil {
			log.Println(err)
			continue
		}

		currentCheckpoint, err := getStringAtAddr(proc, checkpointAddr)
		if err != nil {
			log.Println(err)
			continue
		}

		if currentCheckpoint == lastCheckpoint {
			continue
		}

		lastCheckpoint = currentCheckpoint

		log.Printf("current checkpoint: %q", currentCheckpoint)
	}
}

func getAddr(proc kiwi.Process, start uint32, offsets ...uint32) (uint32, error) {
	stringAddr, err := proc.ReadUint32(uintptr(start + 0x400000)) // 400000 the base address
	if err != nil {
		return 0, fmt.Errorf("error while trying to read from target process at 0x%x - %w", stringAddr, err)
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

func getStringAtAddr(proc kiwi.Process, addr uint32) (string, error) {
	buf := bytes.NewBuffer(nil)
	maxReads := 69
	term := []byte{0x00, 0x00, 0x00}
	chunkSlice := make([]byte, 4)

	for i := 0; i < maxReads; i++ {

		chunk, err := proc.ReadUint32(uintptr(addr))
		if err != nil {
			return "", fmt.Errorf("failed to read memory at 0x%x - %w", addr, err)
		}

		binary.LittleEndian.PutUint32(chunkSlice, chunk)
		buf.Write(chunkSlice)

		if j := bytes.Index(buf.Bytes(), term); j > -1 {
			chapterName := strings.ReplaceAll(string(buf.Bytes()[0:j]), "\x00", "")

			return chapterName, nil

		}

		addr += 0x4
	}

	return "", nil
}

func getFloatAtAddr(proc kiwi.Process, addr uint32) (float32, error) {
	chunk, err := proc.ReadUint32(uintptr(addr))
	if err != nil {
		return 0, fmt.Errorf("failed to read memory at 0x%x - %w", addr, err)
	}

	return math.Float32frombits(chunk), nil
}
