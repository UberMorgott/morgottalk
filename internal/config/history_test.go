package config

import (
	"fmt"
	"os"
	"testing"
)

// cleanupHistory removes the history file used by tests.
func cleanupHistory() {
	path, err := historyPath()
	if err != nil {
		return
	}
	os.Remove(path)
}

func TestMain(m *testing.M) {
	cleanupHistory()
	code := m.Run()
	cleanupHistory()
	os.Exit(code)
}

func TestAppendHistory_Trim(t *testing.T) {
	cleanupHistory()
	t.Cleanup(cleanupHistory)

	// Append 52 entries — should trim to MaxHistoryEntries (50).
	for i := 0; i < 52; i++ {
		if err := AppendHistory(fmt.Sprintf("entry-%d", i), "en"); err != nil {
			t.Fatalf("AppendHistory(%d): %v", i, err)
		}
	}

	entries, err := LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(entries) != MaxHistoryEntries {
		t.Errorf("len(entries) = %d, want %d", len(entries), MaxHistoryEntries)
	}

	// Most recent entry should be first (prepend order).
	if entries[0].Text != "entry-51" {
		t.Errorf("entries[0].Text = %q, want %q", entries[0].Text, "entry-51")
	}
	// Oldest surviving entry should be entry-2 (entries 0 and 1 were trimmed).
	if entries[MaxHistoryEntries-1].Text != "entry-2" {
		t.Errorf("entries[%d].Text = %q, want %q", MaxHistoryEntries-1, entries[MaxHistoryEntries-1].Text, "entry-2")
	}
}

func TestDeleteHistoryEntry(t *testing.T) {
	cleanupHistory()
	t.Cleanup(cleanupHistory)

	if err := AppendHistory("first", "en"); err != nil {
		t.Fatalf("AppendHistory(first): %v", err)
	}
	if err := AppendHistory("second", "ru"); err != nil {
		t.Fatalf("AppendHistory(second): %v", err)
	}

	entries, err := LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) = %d, want 2", len(entries))
	}

	// Delete the first entry (most recent = "second").
	tsToDelete := entries[0].Timestamp
	if err := DeleteHistoryEntry(tsToDelete); err != nil {
		t.Fatalf("DeleteHistoryEntry: %v", err)
	}

	entries, err = LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory after delete: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) after delete = %d, want 1", len(entries))
	}
	if entries[0].Text != "first" {
		t.Errorf("remaining entry Text = %q, want %q", entries[0].Text, "first")
	}
	if entries[0].Language != "en" {
		t.Errorf("remaining entry Language = %q, want %q", entries[0].Language, "en")
	}
}

func TestClearHistory(t *testing.T) {
	cleanupHistory()
	t.Cleanup(cleanupHistory)

	for i := 0; i < 5; i++ {
		if err := AppendHistory(fmt.Sprintf("item-%d", i), "de"); err != nil {
			t.Fatalf("AppendHistory(%d): %v", i, err)
		}
	}

	entries, err := LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory before clear: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("len(entries) = %d, want 5", len(entries))
	}

	if err := ClearHistory(); err != nil {
		t.Fatalf("ClearHistory: %v", err)
	}

	entries, err = LoadHistory()
	if err != nil {
		t.Fatalf("LoadHistory after clear: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("len(entries) after clear = %d, want 0", len(entries))
	}
}
