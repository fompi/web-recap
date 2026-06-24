package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestChromeHandler_NewChromeHandler_EmptyBrowser(t *testing.T) {
	handler := NewChromeHandler("path", "", "profile")
	if handler.browserName != "chrome" {
		t.Errorf("expected default browserName 'chrome', got %q", handler.browserName)
	}
}

func TestChromeHandler_GetHistory_AllBranches(t *testing.T) {
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
	// We also insert an entry with visit_time = 0 to cover the isZero() check skip.
	_, err = db.Exec(`
		INSERT INTO urls (id, url, title, visit_count, typed_count) VALUES (1, 'https://example.com/chrome1', 'Chrome Page', 10, 2);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (1, 1, 13426430400000000, 500, 1, 0, 99);
		
		INSERT INTO urls (id, url, title, visit_count, typed_count) VALUES (2, 'https://example.com/zero', 'Zero Page', 1, 0);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (2, 2, 0, 0, 0, 0, 0);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewChromeHandler(dbPath, "chrome", "test-chrome-profile")
	startDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)

	// Test 1: date range
	entries, err := handler.GetHistory(startDate, endDate)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].URL != "https://example.com/chrome1" {
		t.Errorf("expected URL 'https://example.com/chrome1', got %q", entries[0].URL)
	}

	// Test 2: only start date
	entries, _ = handler.GetHistory(startDate, time.Time{})
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	// Test 3: only end date (non-zero time, does not add 86400)
	customEndDate := time.Date(2026, 6, 20, 15, 0, 0, 0, time.UTC)
	entries, _ = handler.GetHistory(time.Time{}, customEndDate)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	// Test 4: empty dates (limits to 10000)
	entries, _ = handler.GetHistory(time.Time{}, time.Time{})
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestChromeHandler_GetHistory_Errors(t *testing.T) {
	// 1. Copy database error (non-existent path)
	handler := NewChromeHandler("/nonexistent/chrome/History", "chrome", "profile")
	_, err := handler.GetHistory(time.Time{}, time.Time{})
	if err == nil {
		t.Errorf("expected error copying non-existent database, got nil")
	}

	// 2. Query execution error (missing tables)
	tempDir, err := os.MkdirTemp("", "chrome-err-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	dbPath := filepath.Join(tempDir, "History")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec("CREATE TABLE dummy (id INTEGER)")
	db.Close()

	handler2 := NewChromeHandler(dbPath, "chrome", "profile")
	_, err = handler2.GetHistory(time.Time{}, time.Time{})
	if err == nil {
		t.Errorf("expected error executing query on empty schema, got nil")
	}
}

func TestChromeHandler_GetHistory_ScanError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "chrome-scan-err-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "History")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

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
		t.Fatal(err)
	}

	// Insert invalid type for visit_count (string that cannot be parsed as int)
	_, err = db.Exec(`
		INSERT INTO urls (id, url, title, visit_count, typed_count) VALUES (1, 'https://example.com/chrome1', 'Chrome Page', 'invalid_int', 2);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (1, 1, 13426430400000000, 500, 1, 0, 99);
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	handler := NewChromeHandler(dbPath, "chrome", "profile")
	_, err = handler.GetHistory(time.Time{}, time.Time{})
	if err == nil {
		t.Errorf("expected error during rows.Scan, got nil")
	}
}
