// Command valorant-rpc-gui runs the daemon and the desktop GUI in one process.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/its-haze/valorant-rpc/frontend"
	"github.com/its-haze/valorant-rpc/internal/config"
	"github.com/its-haze/valorant-rpc/internal/logging"
	"github.com/its-haze/valorant-rpc/internal/startup"
	"github.com/its-haze/valorant-rpc/internal/updates"
	"github.com/its-haze/valorant-rpc/internal/version"
	"github.com/its-haze/valorant-rpc/pkg/constants"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/icons"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// singleInstanceID guards against a second daemon. It must differ from the
// sibling app's, so a user running both gets two instances rather than one.
const singleInstanceID = "com.its-haze.valorant-rpc"

// updateChangedEvent carries an updates.Status to the frontend whenever the
// App Update coordinator's launch or periodic check finds something new.
const updateChangedEvent = "update:changed"

// closeRequestedEvent asks the frontend to raise the close confirmation
// dialog. Only emitted while close_action is "ask".
const closeRequestedEvent = "window:close-requested"

// navigateAboutEvent asks the frontend to switch to the About screen. Emitted
// when the user clicks the update-available toast notification.
const navigateAboutEvent = "navigate:about"

// updateReadyNotificationID identifies the toast shown once a new version is
// first discovered, distinguishing its click handler from future toasts.
const updateReadyNotificationID = "update-ready"

func main() {
	// A run launched by the Run entry carries the hidden marker. Drop any
	// console Windows attached to it before anything can write there.
	startHidden := startup.StartedHidden(os.Args[1:])
	if startHidden {
		startup.DetachConsole()
	}

	cfg, err := config.LoadOrCreate()
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	sink, err := logging.New(logging.Options{Debug: cfg.Advanced.DebugMode})
	if err != nil {
		log.Fatalf("init logging: %v", err)
	}
	defer sink.Close()

	store := config.NewStore(cfg)

	// Stands in for the daemon's pause flag until there is a daemon to pause.
	pause := &localPause{}

	// Keep the "start with Windows" registry entry matching the setting, both
	// now and whenever the GUI toggles it.
	reconciler := startup.New(startup.SystemRunKey())
	if err := reconciler.Reconcile(cfg.Behavior.LaunchAtStartup); err != nil {
		sink.Logger.Warn().Err(err).Msg("could not reconcile start-with-Windows entry")
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Assigned once the window exists; the single-instance callback reads it.
	var mainWindow *application.WebviewWindow

	wailsApp := application.New(application.Options{
		Name:        constants.AppName,
		Description: "Valorant Discord Rich Presence",
		Icon:        appIcon,
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(frontend.Assets())},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: singleInstanceID,
			OnSecondInstanceLaunch: func(application.SecondInstanceData) {
				if mainWindow != nil {
					mainWindow.Show()
					mainWindow.Focus()
				}
			},
		},
		OnShutdown: func() { cancel() },
	})

	// wailsApp.Updater exists only once application.New has run, hence wiring
	// App Update here rather than earlier.
	updateCoord := updates.New(wailsApp.Updater, updates.NewProductionHTTPDoer(), version.IsDev(), sink.Logger)
	if updCfg, err := updates.BuildConfig(version.Version()); err != nil {
		sink.Logger.Warn().Err(err).Msg("could not configure the app updater")
	} else if err := wailsApp.Updater.Init(updCfg); err != nil {
		sink.Logger.Warn().Err(err).Msg("could not initialize the app updater")
	}

	// Registered so an update-ready toast can be pushed from OnChange below;
	// clicking it brings the window forward to the About screen.
	notifier := notifications.New()
	wailsApp.RegisterService(application.NewService(notifier))
	notifier.OnNotificationResponse(func(result notifications.NotificationResult) {
		if result.Response.ID != updateReadyNotificationID {
			return
		}
		application.InvokeAsync(func() {
			if mainWindow != nil {
				mainWindow.Show()
				mainWindow.Focus()
			}
			wailsApp.Event.Emit(navigateAboutEvent)
		})
	})

	// Apply a start-with-Windows toggle to the registry the moment it changes,
	// not just on the next launch.
	go watchConfigField(ctx, store,
		func(c *config.Config) bool { return c.Behavior.LaunchAtStartup },
		func(want bool) {
			if err := reconciler.Reconcile(want); err != nil {
				sink.Logger.Warn().Err(err).Msg("could not update start-with-Windows entry")
			}
		})

	// Follow a debug-logging toggle immediately, not just on the next launch.
	go watchConfigField(ctx, store, func(c *config.Config) bool { return c.Advanced.DebugMode }, logging.SetDebug)

	// A hidden launch opens straight to the tray; a manual run shows the
	// window.
	windowWidth, windowHeight := defaultWindowSize()
	mainWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            constants.AppName,
		Width:            windowWidth,
		Height:           windowHeight,
		MinWidth:         minWindowWidth,
		MinHeight:        minWindowHeight,
		Hidden:           startHidden,
		BackgroundColour: application.NewRGB(15, 17, 23),
		URL:              "/",
		// Custom chrome replaces the native titlebar, so the window carries no
		// OS decorations.
		Frameless: true,
	})

	tray := newTrayController(windowAdapter{mainWindow}, pause)
	tray.closeAction = func() string { return store.Load().Behavior.CloseAction }
	tray.quit = wailsApp.Quit
	tray.askClose = func() {
		mainWindow.Show() // a close from the taskbar can arrive minimized
		wailsApp.Event.Emit(closeRequestedEvent)
	}

	// Every close is cancelled and re-decided by the tray controller, so the
	// window only ever goes away by hiding or by a real quit.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		tray.handleClose()
	})

	systemTray := wailsApp.SystemTray.New()
	systemTray.SetIcon(trayIcon)
	systemTray.SetDarkModeIcon(trayIcon)
	systemTray.SetTooltip(constants.AppName)
	if runtime.GOOS == "darwin" {
		systemTray.SetTemplateIcon(icons.SystrayMacTemplate)
	}
	systemTray.OnClick(tray.showWindow)
	systemTray.SetMenu(buildTrayMenu(wailsApp, tray, updateCoord))

	// Push App Update status to the frontend and the tray tooltip, and fire a
	// one-time toast once a new version is first discovered.
	var notifyMu sync.Mutex
	var notifiedVersion string
	updateCoord.OnChange(func(s updates.Status) {
		wailsApp.Event.Emit(updateChangedEvent, s)
		if s.Available {
			systemTray.SetTooltip(constants.AppName + " (update available)")
		} else {
			systemTray.SetTooltip(constants.AppName)
		}

		if !s.Available {
			return
		}
		notifyMu.Lock()
		alreadyNotified := notifiedVersion == s.Version
		notifiedVersion = s.Version
		notifyMu.Unlock()
		// Still tracked above even when suppressed, so re-enabling the
		// setting later doesn't fire a toast for a version already seen.
		if alreadyNotified || !store.Load().Behavior.NotifyUpdates {
			return
		}
		if err := notifier.SendNotification(notifications.NotificationOptions{
			ID:    updateReadyNotificationID,
			Title: constants.AppName + " update available",
			Body:  fmt.Sprintf("Version %s is available. Click to review and install.", s.Version),
		}); err != nil {
			sink.Logger.Warn().Err(err).Msg("could not show update-available notification")
		}
	})
	go updateCoord.Run(ctx)

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// watchConfigField calls fn whenever extract's live-config reading changes,
// until ctx is canceled. Shared by every apply-immediately watcher below.
func watchConfigField[T comparable](ctx context.Context, store *config.Store, extract func(*config.Config) T, fn func(T)) {
	changes := store.Subscribe()
	last := extract(store.Load())
	for {
		select {
		case <-ctx.Done():
			return
		case cfg, ok := <-changes:
			if !ok {
				return
			}
			if next := extract(cfg); next != last {
				last = next
				fn(next)
			}
		}
	}
}

// buildTrayMenu assembles the right-click menu: Open, Pause presence, Check
// for updates, Quit.
func buildTrayMenu(wailsApp *application.App, tray *trayController, updateCoord *updates.Coordinator) *application.Menu {
	menu := wailsApp.NewMenu()
	menu.Add("Open").OnClick(func(*application.Context) { tray.showWindow() })

	pauseItem := menu.AddCheckbox("Pause presence", tray.pause.IsPaused())
	// A frontend toggle runs off the main thread; marshal the native menu update.
	tray.reflectChecked = func(paused bool) {
		application.InvokeAsync(func() { pauseItem.SetChecked(paused) })
	}
	pauseItem.OnClick(func(*application.Context) { tray.togglePause() })

	menu.AddSeparator()
	// Fire-and-forget: the result reaches the window (and this tooltip) the
	// same way the launch/periodic checks already do, via OnChange.
	menu.Add("Check for updates").OnClick(func(*application.Context) {
		go func() { _, _ = updateCoord.Check(context.Background()) }()
	})

	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(*application.Context) { wailsApp.Quit() })
	return menu
}

// windowAdapter fits *application.WebviewWindow to the tray's windowController;
// the window's Show/Hide return a value the interface does not.
type windowAdapter struct{ w *application.WebviewWindow }

func (a windowAdapter) Show()  { a.w.Show() }
func (a windowAdapter) Hide()  { a.w.Hide() }
func (a windowAdapter) Focus() { a.w.Focus() }

// localPause holds the pause flag while there is no daemon to hold it, and
// goes away once the daemon owns it.
type localPause struct {
	mu     sync.Mutex
	paused bool
}

func (p *localPause) SetPaused(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paused = v
}

func (p *localPause) IsPaused() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}
