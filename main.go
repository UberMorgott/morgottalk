package main

import (
	"embed"
	_ "embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/UberMorgott/transcribation/services"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	transcriptionService := services.NewTranscriptionService()
	settingsService := services.NewSettingsService()

	app := application.New(application.Options{
		Name:        "Transcribation",
		Description: "Push-to-talk voice transcription & translation",
		Services: []application.Service{
			application.NewService(transcriptionService),
			application.NewService(settingsService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Transcribation",
		Width:  480,
		Height: 640,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(18, 18, 24),
		URL:              "/",
	})

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
