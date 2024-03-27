package main

import (
	_ "embed"
	"fmt"
	"github.com/Andoryuuta/kiwi"
	"github.com/SeungKang/speedometer/internal/appconfig"
	"github.com/stephen-fox/user32util"
	_ "image/png"
	"log"
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
	user32, err := user32util.LoadUser32DLL()
	if err != nil {
		return fmt.Errorf("failed to load user32.dll - %s", err.Error())
	}

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

	fn := func(event user32util.LowLevelKeyboardEvent) {

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
