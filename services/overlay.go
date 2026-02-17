package services

import (
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// showOverlay creates (if needed) and shows the recording/processing overlay window.
func showOverlay(state string) {
	if runtime.GOOS != "windows" {
		return
	}
	app := application.Get()
	if app == nil {
		return
	}

	// If overlay already exists, emit state event and show it.
	if w, exists := app.Window.GetByName("overlay"); exists {
		app.Event.Emit("overlay:state", map[string]any{"state": state})
		if !w.IsVisible() {
			w.Show()
		}
		return
	}

	// First time: pass initial state via URL param so the page reads it on mount
	// (event would be missed because webview hasn't loaded yet).
	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:              "overlay",
		Width:             220,
		Height:            220,
		Frameless:         true,
		AlwaysOnTop:       true,
		BackgroundType:    application.BackgroundTypeTransparent,
		IgnoreMouseEvents: true,
		Hidden:            false,
		DisableResize:     true,
		URL:               "/?window=overlay&state=" + state,
		Windows: application.WindowsWindow{
			HiddenOnTaskbar:                  true,
			DisableFramelessWindowDecorations: true,
		},
	})
	w.Center()
}

// hideOverlay hides the overlay window.
func hideOverlay() {
	app := application.Get()
	if app == nil {
		return
	}
	if w, exists := app.Window.GetByName("overlay"); exists {
		w.Hide()
	}
}
