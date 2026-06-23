package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestFirefoxHandler_GetHistory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "firefox-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "places.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create Firefox schema tables
	_, err = db.Exec(`
		CREATE TABLE moz_places (
			id INTEGER PRIMARY KEY,
			url TEXT,
			title TEXT,
			visit_count INTEGER,
			frequency INTEGER,
			typed INTEGER
		);
		CREATE TABLE moz_historyvisits (
			id INTEGER PRIMARY KEY,
			place_id INTEGER,
			visit_date INTEGER,
			from_visit INTEGER,
			visit_type INTEGER,
			session INTEGER
		);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create tables: %v", err)
	}

	// Insert mock data
	// Firefox time = 1781956800 * 1000000 = 1781956800000000.
	_, err = db.Exec(`
		INSERT INTO moz_places (id, url, title, visit_count, frequency, typed) VALUES (1, 'https://example.com/firefox1', 'Firefox Page', 3, 15, 1);
		INSERT INTO moz_historyvisits (id, place_id, visit_date, from_visit, visit_type, session) VALUES (1, 1, 1781956800000000, 0, 5, 12345);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewFirefoxHandler(dbPath, "firefox", "test-firefox-profile")
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
	if entry.URL != "https://example.com/firefox1" {
		t.Errorf("expected URL 'https://example.com/firefox1', got %q", entry.URL)
	}
	if entry.Title != "Firefox Page" {
		t.Errorf("expected Title 'Firefox Page', got %q", entry.Title)
	}
	if entry.Browser != "firefox" {
		t.Errorf("expected Browser 'firefox', got %q", entry.Browser)
	}
	if entry.Profile != "test-firefox-profile" {
		t.Errorf("expected Profile 'test-firefox-profile', got %q", entry.Profile)
	}
	if entry.VisitType != 5 {
		t.Errorf("expected VisitType 5, got %d", entry.VisitType)
	}
	if entry.Session != 12345 {
		t.Errorf("expected Session 12345, got %d", entry.Session)
	}
	if entry.Frequency != 15 {
		t.Errorf("expected Frequency 15, got %d", entry.Frequency)
	}
	if entry.Typed != 1 {
		t.Errorf("expected Typed 1, got %d", entry.Typed)
	}

	expectedTime := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	if !entry.Timestamp.Equal(expectedTime) {
		t.Errorf("expected timestamp %v, got %v", expectedTime, entry.Timestamp)
	}
}
