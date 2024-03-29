package main

import (
	_ "embed"
	"fmt"
	"github.com/SeungKang/speedometer/internal/appconfig"
	"github.com/stephen-fox/user32util"
	_ "image/png"
	"log"
	"os"
	"os/signal"
	"syscall"
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

	fn := func(event user32util.LowLevelKeyboardEvent) {

	}

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
