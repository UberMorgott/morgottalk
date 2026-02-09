package main

import (
	"embed"
	"log"
	"os"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/UberMorgott/transcribation/internal/config"
	"github.com/UberMorgott/transcribation/internal/i18n"
	"github.com/UberMorgott/transcribation/services"
)

const AppVersion = "1.1.0"

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	historyService := services.NewHistoryService()
	modelService := services.NewModelService()
	presetService := services.NewPresetService(historyService, modelService)
	settingsService := services.NewSettingsService()

	go func() {
		if err := presetService.Init(); err != nil {
			log.Printf("WARNING: preset service init failed: %v", err)
		}
	}()

	installDesktopEntry(appIcon)

	app := application.New(application.Options{
		Name:        "MorgoTTalk",
		Description: "Push-to-talk voice transcription & translation",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(presetService),
			application.NewService(settingsService),
			application.NewService(historyService),
			application.NewService(modelService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Linux: application.LinuxOptions{
			ProgramName: "morgottalk",
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	mainWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "",
		Width:            500,
		Height:           720,
		MinWidth:         400,
		MinHeight:        560,
		BackgroundColour: application.NewRGB(13, 11, 8),
		URL:              "/",
	})

	// --- Helper: actually quit the app ---
	doQuit := func() {
		go presetService.Shutdown()
		time.AfterFunc(2*time.Second, func() {
			log.Println("Force exit: shutdown timeout")
			os.Exit(0)
		})
		app.Quit()
	}

	// --- UI language for Go-side strings ---
	cfg, _ := config.Load()
	lang := cfg.UILang
	if lang == "" {
		lang = "en"
	}

	// --- System tray ---
	trayMenu := app.NewMenu()
	trayMenu.Add(i18n.T(lang, "tray_show")).OnClick(func(_ *application.Context) {
		mainWindow.Show()
		mainWindow.Focus()
	})
	trayMenu.Add(i18n.T(lang, "tray_history")).OnClick(func(_ *application.Context) {
		historyService.OpenHistoryWindow()
	})
	trayMenu.AddSeparator()
	trayMenu.Add(i18n.T(lang, "tray_quit")).OnClick(func(_ *application.Context) {
		doQuit()
	})

	tray := app.SystemTray.New()
	tray.SetIcon(appIcon)
	tray.SetMenu(trayMenu)
	tray.SetTooltip("MorgoTTalk")
	tray.OnClick(func() {
		mainWindow.Show()
		mainWindow.Focus()
	})
	tray.OnDoubleClick(func() {
		mainWindow.Show()
		mainWindow.Focus()
	})

	// --- Start minimized ---
	if cfg.StartMinimized {
		mainWindow.Hide()
	}

	// --- Close-to-tray via RegisterHook ---
	mainWindow.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		cfg, _ := config.Load()
		action := cfg.CloseAction
		uiLang := cfg.UILang
		if uiLang == "" {
			uiLang = "en"
		}

		switch action {
		case "quit":
			doQuit()
			return
		case "tray":
			e.Cancel()
			mainWindow.Hide()
			return
		default:
			// First time — default to tray (Wails v3 Question dialog callbacks
			// don't work on Windows; user can change behavior in Settings).
			e.Cancel()
			cfg.CloseAction = "tray"
			_ = config.Save(cfg)
			mainWindow.Hide()
		}
	})

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}

	presetService.Shutdown()
}
