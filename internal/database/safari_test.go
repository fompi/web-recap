package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSafariHandler_GetHistory(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("skipping macOS Safari handler test on non-macOS platform")
	}

	tempDir, err := os.MkdirTemp("", "safari-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "History.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create Safari schema tables
	_, err = db.Exec(`
		CREATE TABLE history_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			visit_count INTEGER NOT NULL
		);
		CREATE TABLE history_visits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			history_item INTEGER NOT NULL,
			visit_time REAL NOT NULL,
			title TEXT,
			redirect_source INTEGER,
			redirect_destination INTEGER,
			origin INTEGER,
			generation_type INTEGER,
			load_successful INTEGER,
			http_non_get INTEGER,
			synthesized INTEGER
		);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create tables: %v", err)
	}

	// Insert mock data
	// Safari time = 1781956800.5 - 978307200 = 803649600.5.
	_, err = db.Exec(`
		INSERT INTO history_items (id, url, visit_count) VALUES (1, 'https://example.com/page1', 5);
		INSERT INTO history_visits (id, history_item, visit_time, title, redirect_source, redirect_destination, origin, generation_type, load_successful, http_non_get, synthesized) 
		VALUES (1, 1, 803649600.5, 'Example Title', 4, 0, 1, 3, 1, 0, 1);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewSafariHandler(dbPath, "test-safari-profile")
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
	if entry.URL != "https://example.com/page1" {
		t.Errorf("expected URL 'https://example.com/page1', got %q", entry.URL)
	}
	if entry.Title != "Example Title" {
		t.Errorf("expected Title 'Example Title', got %q", entry.Title)
	}
	if entry.Browser != "safari" {
		t.Errorf("expected Browser 'safari', got %q", entry.Browser)
	}
	if entry.Profile != "test-safari-profile" {
		t.Errorf("expected Profile 'test-safari-profile', got %q", entry.Profile)
	}
	if entry.RedirectSource != 4 {
		t.Errorf("expected RedirectSource 4, got %d", entry.RedirectSource)
	}
	if entry.RedirectDestination != 0 {
		t.Errorf("expected RedirectDestination 0, got %d", entry.RedirectDestination)
	}
	if entry.Origin != 1 {
		t.Errorf("expected Origin 1, got %d", entry.Origin)
	}
	if entry.GenerationType != 3 {
		t.Errorf("expected GenerationType 3, got %d", entry.GenerationType)
	}
	if !entry.LoadSuccessful {
		t.Errorf("expected LoadSuccessful true, got false")
	}
	if entry.HTTPNonGET {
		t.Errorf("expected HTTPNonGET false, got true")
	}
	if !entry.Synthesized {
		t.Errorf("expected Synthesized true, got false")
	}

	expectedTime := time.Date(2026, 6, 20, 12, 0, 0, 500000000, time.UTC)
	if !entry.Timestamp.Equal(expectedTime) {
		t.Errorf("expected timestamp %v, got %v", expectedTime, entry.Timestamp)
	}
}
