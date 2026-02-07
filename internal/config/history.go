package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const MaxHistoryEntries = 50

// HistoryEntry represents a single transcription result.
type HistoryEntry struct {
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
	Language  string `json:"language"`
}

func historyPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "history.json"), nil
}

// LoadHistory reads transcription history from disk.
func LoadHistory() ([]HistoryEntry, error) {
	path, err := historyPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}

	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// SaveHistory writes history to disk.
func SaveHistory(entries []HistoryEntry) error {
	path, err := historyPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// AppendHistory adds a new entry at the beginning, trims to MaxHistoryEntries.
func AppendHistory(text, language string) error {
	entries, _ := LoadHistory()

	entry := HistoryEntry{
		Text:      text,
		Timestamp: time.Now().UnixMilli(),
		Language:  language,
	}

	entries = append([]HistoryEntry{entry}, entries...)
	if len(entries) > MaxHistoryEntries {
		entries = entries[:MaxHistoryEntries]
	}

	return SaveHistory(entries)
}

// ClearHistory removes all history entries.
func ClearHistory() error {
	return SaveHistory(nil)
}

// DeleteHistoryEntry removes an entry by timestamp.
func DeleteHistoryEntry(timestamp int64) error {
	entries, _ := LoadHistory()
	for i, e := range entries {
		if e.Timestamp == timestamp {
			entries = append(entries[:i], entries[i+1:]...)
			return SaveHistory(entries)
		}
	}
	return nil
}
