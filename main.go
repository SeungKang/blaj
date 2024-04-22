package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/SeungKang/blaj/internal/appconfig"
	"github.com/SeungKang/blaj/internal/gamectl"
	"github.com/getlantern/systray"
	"github.com/stephen-fox/user32util"
)

const appName = "blaj"

var (
	//go:embed icons/shark_blue.ico
	sharkBlue []byte

	//go:embed icons/shark_red.ico
	sharkRed []byte

	//go:embed icons/shark_red_white.ico
	gameErrorIcon []byte

	//go:embed icons/shark_blue_white.ico
	gameNotRunningIcon []byte

	//go:embed icons/shark_green_white.ico
	gameRunningIcon []byte
)

func main() {
	a := &app{}
	systray.Run(a.ready, a.exit)
}

type app struct {
	status      *systray.MenuItem
	statusChild *systray.MenuItem
	mu          sync.Mutex
	lastAppErr  error
	// TODO: use INI header as key
	games map[string]error
}

func (o *app) ready() {
	systray.SetTitle(appName)
	systray.SetIcon(sharkBlue)

	systray.AddMenuItem(appName, "").Disable()
	systray.AddSeparator()
	o.status = systray.AddMenuItem("Status", "Application status")
	o.statusChild = o.status.AddSubMenuItem("", "")
	o.statusChild.Hide()

	quit := systray.AddMenuItem("Quit", "Quit the application")
	systray.AddSeparator()

	ctx, cancelFn := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)

	go func() {
		select {
		case <-quit.ClickedCh:
		case <-ctx.Done():
		}

		cancelFn()
		o.exit()
	}()

	go o.loop(ctx)
}

func (o *app) loop(ctx context.Context) {
	for {
		gameCtx, cancelGameCtxFn := context.WithCancel(ctx)
		defer cancelGameCtxFn()

		gameUIs, gameErrors, err := startApp(gameCtx)
		if err != nil {
			goto onGameExit
		}

		o.status.SetTitle("Status: running")
		systray.SetIcon(sharkBlue)
		o.statusChild.Hide()

		select {
		case <-ctx.Done():
		case err = <-gameErrors:
		}

	onGameExit:
		log.Printf("app loop error - %v", err)

		cancelGameCtxFn()

		if err != nil {
			o.status.SetTitle("Status: error")
			systray.SetIcon(sharkRed)
			o.statusChild.SetTitle(err.Error())
			o.statusChild.Show()
		}

		select {
		case <-ctx.Done():
			log.Printf("app loop exited - %s", ctx.Err())
			return
		case <-time.After(5 * time.Second):
			for _, ui := range gameUIs {
				ui.hide()
			}

			continue
		}
	}
}

func (o *app) exit() {
	systray.Quit()
}

func (o *app) gameErroredStatus(exename string, err error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.games == nil {
		o.games = make(map[string]error)
	}

	o.games[exename] = err
	if o.lastAppErr == nil {
		o.status.SetTitle("Status: error")
		o.statusChild.Show()
		o.statusChild.SetTitle(err.Error())
	}

	systray.SetIcon(sharkRed)
}

func (o *app) gameOkStatus(exename string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.games == nil {
		o.games = make(map[string]error)
	}

	o.games[exename] = nil
	for _, err := range o.games {
		if err != nil {
			return
		}
	}

	systray.SetIcon(sharkBlue)
}

func (o *app) appRunningStatus() {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.lastAppErr = nil
	for _, err := range o.games {
		if err != nil {
			return
		}
	}

	o.status.SetTitle("Status: running")
	systray.SetIcon(sharkBlue)
	o.statusChild.Hide()
}

func (o *app) appErrorStatus(err error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	o.lastAppErr = err

	o.status.SetTitle("Status: error")
	systray.SetIcon(sharkRed)
	o.statusChild.SetTitle(err.Error())
	o.statusChild.Show()
}

func newProgramUI(program *appconfig.ProgramConfig) *programUI {
	gui := &programUI{
		// TODO: maybe use the INI header
		runningMenu: systray.AddMenuItem(program.General.ExeName, ""),
		errorMenu:   systray.AddMenuItem(program.General.ExeName, ":c"),
	}

	gui.runningMenu.SetIcon(gameNotRunningIcon)
	gui.errorSubMenu = gui.errorMenu.AddSubMenuItem("", "")
	gui.errorMenu.Hide()

	return gui
}

type programUI struct {
	runningMenu  *systray.MenuItem
	errorMenu    *systray.MenuItem
	errorSubMenu *systray.MenuItem
}

func (o *programUI) GameStarted(exename string) {
	log.Printf("connected to %s", exename)

	o.runningMenu.SetIcon(gameRunningIcon)
	o.runningMenu.Show()

	o.errorMenu.Hide()
}

func (o *programUI) GameStopped(exename string, err error) {
	log.Printf("disconnected from %s", exename)

	if err != nil {
		o.errorMenu.SetIcon(gameErrorIcon)
		o.errorMenu.Show()
		o.errorSubMenu.SetTitle(err.Error())

		o.runningMenu.Hide()
	} else {
		o.runningMenu.SetIcon(gameNotRunningIcon)
	}
}

func (o *programUI) hide() {
	o.runningMenu.Hide()
	o.errorMenu.Hide()
	o.errorSubMenu.Hide()
}

func startApp(ctx context.Context) ([]*programUI, <-chan error, error) {
	user32, err := user32util.LoadUser32DLL()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load user32.dll - %s", err.Error())
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user home dir - %w", err)
	}

	// TODO: support multiple config files
	configPath := filepath.Join(homeDir, "."+appName, "config.conf")
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open config file - %w", err)
	}
	defer configFile.Close()

	config, err := appconfig.Parse(configFile)
	configFile.Close()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse config - %w", err)
	}

	gameUIs := make([]*programUI, len(config.Programs))
	programRoutinesExited := make(chan error, len(config.Programs))

	for i, program := range config.Programs {
		program := program

		gameUIs[i] = newProgramUI(program)

		// TODO: write function that creates and starts program routine
		programRoutine := &gamectl.Routine{
			Program: program,
			User32:  user32,
			Notif:   gameUIs[i],
		}

		programRoutine.Start(ctx)

		go func() {
			<-programRoutine.Done()
			programRoutinesExited <- fmt.Errorf("%s exited - %w",
				program.General.ExeName, programRoutine.Err())
		}()
	}

	return gameUIs, programRoutinesExited, nil
}
