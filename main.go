package main

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/SeungKang/speedometer/internal/appconfig"
	"github.com/SeungKang/speedometer/internal/gamectl"
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

	ctx, cancelFn := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelFn()

	gameRoutinesExited := make(chan error, len(config.Games))
	for _, game := range config.Games {
		game := game

		// TODO: write function that creates and starts game routine
		gameRoutine := &gamectl.Routine{
			Game:   game,
			User32: user32,
		}

		gameRoutine.Start(ctx)

		go func() {
			<-gameRoutine.Done()
			gameRoutinesExited <- fmt.Errorf("%s exited - %w",
				game.ExeName, gameRoutine.Err())
		}()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-gameRoutinesExited:
		return err
	}
}
