package main

import (
	"context"
	_ "embed"
	"fmt"
	"github.com/SeungKang/speedometer/internal/appconfig"
	"github.com/SeungKang/speedometer/internal/gamectl"
	"github.com/getlantern/systray"
	"github.com/stephen-fox/user32util"
	_ "image/png"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const appName = "buh"

var (
	//go:embed juul-green.ico
	juulGreen []byte

	//go:embed juul-red.ico
	juulRed []byte

	//go:embed juul-red.ico
	gameErrorIcon []byte

	//go:embed juul-red.ico
	gameNotRunningIcon []byte

	//go:embed juul-green.ico
	gameRunningIcon []byte
)

func main() {
	a := &app{}
	systray.Run(a.ready, a.exit)
}

type app struct {
	status      *systray.MenuItem
	statusChild *systray.MenuItem
}

func (o *app) ready() {
	systray.SetTitle(appName)
	systray.SetIcon(juulGreen)

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
		var cancelFn func()
		ctx, cancelFn = context.WithCancel(ctx)
		defer cancelFn()

		gameUIs, gameErrors, err := startApp(ctx)
		if err != nil {
			goto onError
		}

		o.status.SetTitle("Status: running")
		systray.SetIcon(juulGreen)
		o.statusChild.Hide()

		select {
		case <-ctx.Done():
		case err = <-gameErrors:
		}

	onError:
		cancelFn()

		o.status.SetTitle("Status: error")
		systray.SetIcon(juulRed)
		o.statusChild.SetTitle(err.Error())
		o.statusChild.Show()

		select {
		case <-ctx.Done():
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
	os.Exit(0)
}

func newGameUI(game *appconfig.Game) *gameUI {
	gui := &gameUI{
		// TODO: maybe use the INI header
		runningMenu: systray.AddMenuItem(game.ExeName, ""),
		errorMenu:   systray.AddMenuItem(game.ExeName, ":c"),
	}

	gui.runningMenu.SetIcon(gameNotRunningIcon)
	gui.errorSubMenu = gui.errorMenu.AddSubMenuItem("", "")
	gui.errorMenu.Hide()

	return gui
}

type gameUI struct {
	runningMenu  *systray.MenuItem
	errorMenu    *systray.MenuItem
	errorSubMenu *systray.MenuItem
}

func (o *gameUI) GameStarted(exename string) {
	o.runningMenu.SetIcon(gameRunningIcon)
	o.runningMenu.Show()

	o.errorMenu.Hide()
}

func (o *gameUI) GameStopped(exename string, err error) {
	if err != nil {
		o.errorMenu.SetIcon(gameErrorIcon)
		o.errorMenu.Show()
		o.errorSubMenu.SetTitle(err.Error())

		o.runningMenu.Hide()
	} else {
		o.runningMenu.SetIcon(gameNotRunningIcon)
	}
}

func (o *gameUI) hide() {
	o.runningMenu.Hide()
	o.errorMenu.Hide()
	o.errorSubMenu.Hide()
}

func startApp(ctx context.Context) ([]*gameUI, <-chan error, error) {
	user32, err := user32util.LoadUser32DLL()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load user32.dll - %s", err.Error())
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user home dir - %w", err)
	}

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

	gameUIs := make([]*gameUI, len(config.Games))
	gameRoutinesExited := make(chan error, len(config.Games))

	for i, game := range config.Games {
		game := game

		gameUIs[i] = newGameUI(game)

		// TODO: write function that creates and starts game routine
		gameRoutine := &gamectl.Routine{
			Game:   game,
			User32: user32,
			Notif:  gameUIs[i],
		}

		gameRoutine.Start(ctx)

		go func() {
			<-gameRoutine.Done()
			gameRoutinesExited <- fmt.Errorf("%s exited - %w",
				game.ExeName, gameRoutine.Err())
		}()
	}

	return gameUIs, gameRoutinesExited, nil
}
