package services

import (
	"sync"

	"github.com/UberMorgott/transcribation/internal/config"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// HistoryService provides transcription history to the frontend.
type HistoryService struct {
	mu sync.Mutex
}

func NewHistoryService() *HistoryService {
	return &HistoryService{}
}

// GetHistory returns all history entries (newest first).
func (s *HistoryService) GetHistory() []config.HistoryEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, _ := config.LoadHistory()
	if entries == nil {
		return []config.HistoryEntry{}
	}
	return entries
}

// AddEntry saves a new transcription result.
func (s *HistoryService) AddEntry(text, language string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := config.AppendHistory(text, language); err != nil {
		return err
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("history:new", nil)
	}
	return nil
}

// ClearHistory removes all entries.
func (s *HistoryService) ClearHistory() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return config.ClearHistory()
}

// DeleteEntry removes an entry by timestamp.
func (s *HistoryService) DeleteEntry(timestamp int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return config.DeleteHistoryEntry(timestamp)
}

// OpenHistoryWindow opens a separate window to display transcription history.
func (s *HistoryService) OpenHistoryWindow() {
	app := application.Get()

	if w, exists := app.Window.GetByName("history"); exists {
		w.Focus()
		return
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "history",
		Title:            "Transcribation — History",
		Width:            550,
		Height:           600,
		MinWidth:         400,
		MinHeight:        400,
		BackgroundColour: application.NewRGB(13, 11, 8),
		URL:              "/?window=history",
	})
}
