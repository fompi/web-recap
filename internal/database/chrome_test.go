package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestChromeHandler_GetHistory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "chrome-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "History")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create Chrome schema tables
	_, err = db.Exec(`
		CREATE TABLE urls (
			id INTEGER PRIMARY KEY,
			url TEXT,
			title TEXT,
			visit_count INTEGER,
			typed_count INTEGER
		);
		CREATE TABLE visits (
			id INTEGER PRIMARY KEY,
			url INTEGER,
			visit_time INTEGER,
			visit_duration INTEGER,
			transition INTEGER,
			from_visit INTEGER,
			segment_id INTEGER
		);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create tables: %v", err)
	}

	// Insert mock data
	// Chrome time = (1781956800 + 11644473600) * 1000000 = 13426430400000000.
	_, err = db.Exec(`
		INSERT INTO urls (id, url, title, visit_count, typed_count) VALUES (1, 'https://example.com/chrome1', 'Chrome Page', 10, 2);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (1, 1, 13426430400000000, 500, 1, 0, 99);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewChromeHandler(dbPath, "chrome", "test-chrome-profile")
	startDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)

	entries, err := handler.GetHistory(startDate, endDate)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.URL != "https://example.com/chrome1" {
		t.Errorf("expected URL 'https://example.com/chrome1', got %q", entry.URL)
	}
	if entry.Title != "Chrome Page" {
		t.Errorf("expected Title 'Chrome Page', got %q", entry.Title)
	}
	if entry.Browser != "chrome" {
		t.Errorf("expected Browser 'chrome', got %q", entry.Browser)
	}
	if entry.Profile != "test-chrome-profile" {
		t.Errorf("expected Profile 'test-chrome-profile', got %q", entry.Profile)
	}
	if entry.VisitDuration != 500 {
		t.Errorf("expected VisitDuration 500, got %d", entry.VisitDuration)
	}
	if entry.Transition != 1 {
		t.Errorf("expected Transition 1, got %d", entry.Transition)
	}
	if entry.SegmentID != 99 {
		t.Errorf("expected SegmentID 99, got %d", entry.SegmentID)
	}
	if entry.TypedCount != 2 {
		t.Errorf("expected TypedCount 2, got %d", entry.TypedCount)
	}

	expectedTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	if !entry.Timestamp.Equal(expectedTime) {
		t.Errorf("expected timestamp %v, got %v", expectedTime, entry.Timestamp)
	}
}
