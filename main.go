package main

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/SeungKang/blaj/internal/appconfig"
	"github.com/SeungKang/blaj/internal/gamectl"
	"github.com/getlantern/systray"
	"github.com/stephen-fox/user32util"
)

const appName = "blaj"

// systray icon
// blue: if no games found
// green: if at least one game found
// red: if there is any errors

// game icon
// blue: if checking for game
// green: if game found
// red: if error

var (
	//go:embed icons/shark_red.ico
	systrayRedIco []byte

	//go:embed icons/shark_blue.ico
	systrayBlueIco []byte

	//go:embed icons/shark_green.ico
	systrayGreenIco []byte

	//go:embed icons/shark_red_white.ico
	statusErrorIcon []byte

	//go:embed icons/shark_blue_white.ico
	statusCheckingIcon []byte

	//go:embed icons/shark_green_white.ico
	statusRunningIcon []byte
)

func main() {
	a := &app{}
	systray.Run(a.ready, a.exit)
}

type app struct {
	errorLog *logUI
}

func (o *app) ready() {
	systray.SetTitle(appName)
	systray.SetIcon(systrayBlueIco)

	systray.AddMenuItem(appName, "").Disable()
	systray.AddSeparator()
	o.errorLog = newLogUI("Error Log")
	o.setChecking()

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

func (o *app) setChecking() {
	systray.SetIcon(systrayBlueIco)
}

func (o *app) setRunning() {
	systray.SetIcon(systrayGreenIco)
}

func (o *app) setError(err error) {
	systray.SetIcon(systrayRedIco)
}

func (o *app) loop(ctx context.Context) {
	for {
		gameCtx, cancelGameCtxFn := context.WithCancel(ctx)
		defer cancelGameCtxFn()

		programUIs, gameErrors, err := startApp(gameCtx, o)
		if err != nil {
			goto onGameExit
		}

		select {
		case <-ctx.Done():
		case err = <-gameErrors:
		}

	onGameExit:
		log.Printf("app loop error - %v", err)

		cancelGameCtxFn()

		if err != nil {
			o.setError(err)
		}

		select {
		case <-ctx.Done():
			log.Printf("app loop exited - %s", ctx.Err())
			return
		case <-time.After(5 * time.Second):
			for _, ui := range programUIs {
				ui.hide()
			}

			continue
		}
	}
}

func (o *app) exit() {
	systray.Quit()
}

func newProgramUI(program *appconfig.ProgramConfig, parent *app) *programUI {
	gui := &programUI{
		// TODO: maybe use the INI header
		app:         parent,
		runningMenu: systray.AddMenuItem(program.General.ExeName, ""),
		errorMenu:   systray.AddMenuItem(program.General.ExeName, ":c"),
	}

	gui.runningMenu.SetIcon(statusCheckingIcon)
	gui.errorSubMenu = gui.errorMenu.AddSubMenuItem("", "")
	gui.errorMenu.Hide()

	return gui
}

type programUI struct {
	app          *app
	runningMenu  *systray.MenuItem
	errorMenu    *systray.MenuItem
	errorSubMenu *systray.MenuItem
}

func (o *programUI) GameStarted(exename string) {
	log.Printf("connected to %s", exename)

	o.app.setRunning()

	o.runningMenu.SetIcon(statusRunningIcon)
	o.runningMenu.Show()

	o.errorMenu.Hide()
}

func (o *programUI) GameStopped(exename string, err error) {
	log.Printf("disconnected from %s", exename)

	if err != nil {
		o.app.setError(err)
		o.app.errorLog.addEntry(exename + ": " + err.Error())

		o.runningMenu.Hide()
	} else {
		o.runningMenu.SetIcon(statusCheckingIcon)
	}
}

func (o *programUI) hide() {
	o.runningMenu.Hide()
	o.errorMenu.Hide()
	o.errorSubMenu.Hide()
}

func startApp(ctx context.Context, parent *app) ([]*programUI, <-chan error, error) {
	user32, err := user32util.LoadUser32DLL()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load user32.dll - %s", err.Error())
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user home dir - %w", err)
	}

	configDir := filepath.Join(homeDir, "."+appName)
	pathInfos, err := os.ReadDir(configDir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config directory - %w", err)
	}

	var programConfigs []*appconfig.ProgramConfig
	for _, pathInfo := range pathInfos {
		if pathInfo.IsDir() {
			continue
		}

		if strings.HasSuffix(pathInfo.Name(), ".conf") {
			configPath := filepath.Join(configDir, pathInfo.Name())
			programConfig, err := appconfig.ProgramConfigFromPath(configPath)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to create program config from path - %w", err)
			}

			if programConfig.General.Disabled {
				log.Printf("%s set to disabled", pathInfo.Name())
				continue
			}

			programConfigs = append(programConfigs, programConfig)
		}
	}

	gameUIs := make([]*programUI, len(programConfigs))
	programRoutinesExited := make(chan error, len(programConfigs))

	for i, program := range programConfigs {
		program := program

		gameUIs[i] = newProgramUI(program, parent)

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

func newLogUI(menuItemName string) *logUI {
	return &logUI{parent: systray.AddMenuItem(menuItemName, "")}
}

type logUI struct {
	parent  *systray.MenuItem
	entries []*systray.MenuItem
}

func (o *logUI) addEntry(message string) {
	// TODO: make more efficient
	newEntry := o.parent.AddSubMenuItem(message, "")
	if len(o.entries) == 5 {
		o.entries[0].Hide()
		o.entries = append(o.entries[1:], newEntry)
	} else {
		o.entries = append(o.entries, newEntry)
	}
}
