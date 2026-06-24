package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

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

	// Create Chrome schema tables — includes 'hidden' so the filter is exercised.
	_, err = db.Exec(`
		CREATE TABLE urls (
			id INTEGER PRIMARY KEY,
			url TEXT,
			title TEXT,
			visit_count INTEGER,
			typed_count INTEGER,
			hidden INTEGER DEFAULT 0
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

	// Insert mock data.
	// Chrome time = (1781956800 + 11644473600) * 1000000 = 13426430400000000.
	// hidden=0 → visible; hidden=1 → subframe-only.
	// visit_time=0 → excluded by WHERE visit_time > 0.
	_, err = db.Exec(`
		INSERT INTO urls (id, url, title, visit_count, typed_count, hidden) VALUES (1, 'https://example.com/chrome1', 'Chrome Page', 10, 2, 0);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (1, 1, 13426430400000000, 500, 1, 0, 99);

		INSERT INTO urls (id, url, title, visit_count, typed_count, hidden) VALUES (2, 'https://example.com/hidden', 'Hidden Page', 1, 0, 1);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (2, 2, 13426430400000000, 0, 0, 0, 0);

		INSERT INTO urls (id, url, title, visit_count, typed_count, hidden) VALUES (3, 'https://example.com/zero', 'Zero Page', 1, 0, 0);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (3, 3, 0, 0, 0, 0, 0);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewChromeHandler(dbPath, "chrome", "test-chrome-profile")
	startDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)

	// Test 1: validOnly=true — hidden entry filtered, only visible entry returned
	entries, err := handler.GetHistory(startDate, endDate, true)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry with validOnly=true, got %d", len(entries))
	}
	if entries[0].URL != "https://example.com/chrome1" {
		t.Errorf("expected URL 'https://example.com/chrome1', got %q", entries[0].URL)
	}
	// transition=1 → "typed"
	if entries[0].VisitTypeLabel != "typed" {
		t.Errorf("expected VisitTypeLabel 'typed', got %q", entries[0].VisitTypeLabel)
	}
	// no visit_source table → local
	if entries[0].Source != "local" {
		t.Errorf("expected Source 'local', got %q", entries[0].Source)
	}
	if entries[0].Hidden {
		t.Errorf("expected Hidden=false for visible entry")
	}

	// Test 2: validOnly=false — both hidden and visible entries returned
	entries, err = handler.GetHistory(startDate, endDate, false)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries with validOnly=false, got %d", len(entries))
	}
	// Find the hidden entry and verify its Hidden field
	hiddenFound := false
	for _, e := range entries {
		if e.URL == "https://example.com/hidden" {
			hiddenFound = true
			if !e.Hidden {
				t.Errorf("expected Hidden=true for hidden entry")
			}
		}
	}
	if !hiddenFound {
		t.Errorf("hidden entry not found in results")
	}

	// Test 3: only start date, validOnly=false — 2 entries
	entries, _ = handler.GetHistory(startDate, time.Time{}, false)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (start date only, validOnly=false), got %d", len(entries))
	}

	// Test 4: only end date (non-zero time), validOnly=false — 2 entries
	customEndDate := time.Date(2026, 6, 20, 15, 0, 0, 0, time.UTC)
	entries, _ = handler.GetHistory(time.Time{}, customEndDate, false)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	// Test 4b: end date with non-zero nanoseconds at 00:00:00 (does not add 86400)
	nsEndDate := time.Date(2026, 6, 20, 0, 0, 0, 100, time.UTC)
	entries, _ = handler.GetHistory(time.Time{}, nsEndDate, false)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (excluding 12:00:00 visit), got %d", len(entries))
	}

	// Test 5: empty dates, validOnly=false — both non-zero entries
	entries, _ = handler.GetHistory(time.Time{}, time.Time{}, false)
	if len(entries) != 2 {
		t.Errorf("expected 2 entries (empty dates, validOnly=false), got %d", len(entries))
	}
}

func TestChromeHandler_GetHistory_Errors(t *testing.T) {
	// 1. Copy database error (non-existent path)
	handler := NewChromeHandler("/nonexistent/chrome/History", "chrome", "profile")
	_, err := handler.GetHistory(time.Time{}, time.Time{}, false)
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
	_, err = handler2.GetHistory(time.Time{}, time.Time{}, false)
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
			typed_count INTEGER,
			hidden INTEGER DEFAULT 0
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
		INSERT INTO urls (id, url, title, visit_count, typed_count, hidden) VALUES (1, 'https://example.com/chrome1', 'Chrome Page', 'invalid_int', 2, 0);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (1, 1, 13426430400000000, 500, 1, 0, 99);
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	handler := NewChromeHandler(dbPath, "chrome", "profile")
	_, err = handler.GetHistory(time.Time{}, time.Time{}, false)
	if err == nil {
		t.Errorf("expected error during rows.Scan, got nil")
	}
}
