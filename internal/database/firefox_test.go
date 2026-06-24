package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

)

func TestFirefoxHandler_NewFirefoxHandler_EmptyBrowser(t *testing.T) {
	handler := NewFirefoxHandler("path", "", "profile")
	if handler.browserName != "firefox" {
		t.Errorf("expected default browserName 'firefox', got %q", handler.browserName)
	}
}

func TestFirefoxHandler_GetHistory_AllBranches(t *testing.T) {
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

	// Create Firefox schema tables — includes 'hidden' so the filter is exercised.
	_, err = db.Exec(`
		CREATE TABLE moz_places (
			id INTEGER PRIMARY KEY,
			url TEXT,
			title TEXT,
			visit_count INTEGER,
			frecency INTEGER,
			typed INTEGER,
			hidden INTEGER DEFAULT 0
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

	// Insert mock data.
	// Firefox time = 1781956800 * 1000000 = 1781956800000000.
	// hidden=0 → visible page; hidden=1 → subframe URL.
	// visit_date=0 → excluded by WHERE visit_date > 0.
	_, err = db.Exec(`
		INSERT INTO moz_places (id, url, title, visit_count, frecency, typed, hidden) VALUES (1, 'https://example.com/firefox1', 'Firefox Page', 3, 15, 1, 0);
		INSERT INTO moz_historyvisits (id, place_id, visit_date, from_visit, visit_type, session) VALUES (1, 1, 1781956800000000, 0, 5, 12345);

		INSERT INTO moz_places (id, url, title, visit_count, frecency, typed, hidden) VALUES (2, 'https://internal.example.com/frame', 'Hidden Frame', 1, 0, 0, 1);
		INSERT INTO moz_historyvisits (id, place_id, visit_date, from_visit, visit_type, session) VALUES (2, 2, 1781956800000000, 0, 1, 0);

		INSERT INTO moz_places (id, url, title, visit_count, frecency, typed, hidden) VALUES (3, 'https://example.com/zero', 'Zero Page', 1, 0, 0, 0);
		INSERT INTO moz_historyvisits (id, place_id, visit_date, from_visit, visit_type, session) VALUES (3, 3, 0, 0, 0, 0);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewFirefoxHandler(dbPath, "firefox", "test-firefox-profile")
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
	if entries[0].URL != "https://example.com/firefox1" {
		t.Errorf("expected URL 'https://example.com/firefox1', got %q", entries[0].URL)
	}
	// visit_type=5 → "redirect"
	if entries[0].VisitTypeLabel != "redirect" {
		t.Errorf("expected VisitTypeLabel 'redirect', got %q", entries[0].VisitTypeLabel)
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
	hiddenFound := false
	for _, e := range entries {
		if e.URL == "https://internal.example.com/frame" {
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

func TestFirefoxHandler_GetHistory_MissingColumns(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "firefox-missing-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "places.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create Firefox schema tables WITHOUT optional columns
	_, err = db.Exec(`
		CREATE TABLE moz_places (
			id INTEGER PRIMARY KEY,
			url TEXT
			-- title, visit_count, frecency, frequency, typed are missing
		);
		CREATE TABLE moz_historyvisits (
			id INTEGER PRIMARY KEY,
			place_id INTEGER,
			visit_date INTEGER
			-- from_visit, visit_type, session are missing
		);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create tables: %v", err)
	}

	// Insert mock data
	_, err = db.Exec(`
		INSERT INTO moz_places (id, url) VALUES (1, 'https://example.com/firefox1');
		INSERT INTO moz_historyvisits (id, place_id, visit_date) VALUES (1, 1, 1781956800000000);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewFirefoxHandler(dbPath, "firefox", "test-firefox-profile")
	entries, err := handler.GetHistory(time.Time{}, time.Time{}, false)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Title != "" {
		t.Errorf("expected empty Title, got %q", entry.Title)
	}
	if entry.VisitCount != 0 || entry.Frequency != 0 || entry.VisitType != 0 || entry.Session != 0 {
		t.Errorf("unexpected defaults for missing columns: %+v", entry)
	}
}

func TestFirefoxHandler_GetHistory_Errors(t *testing.T) {
	// 1. Copy database error (non-existent path)
	handler := NewFirefoxHandler("/nonexistent/firefox/places.sqlite", "firefox", "profile")
	_, err := handler.GetHistory(time.Time{}, time.Time{}, false)
	if err == nil {
		t.Errorf("expected error copying non-existent database, got nil")
	}

	// 2. Query execution error (missing tables)
	tempDir, err := os.MkdirTemp("", "firefox-err-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	dbPath := filepath.Join(tempDir, "places.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = db.Exec("CREATE TABLE dummy (id INTEGER)")
	db.Close()

	handler2 := NewFirefoxHandler(dbPath, "firefox", "profile")
	_, err = handler2.GetHistory(time.Time{}, time.Time{}, false)
	if err == nil {
		t.Errorf("expected error executing query on empty schema, got nil")
	}
}

func TestFirefoxHandler_GetHistory_FrequencyColumn(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "firefox-freq-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "places.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create Firefox schema tables WITH 'frequency' instead of 'frecency'
	_, err = db.Exec(`
		CREATE TABLE moz_places (
			id INTEGER PRIMARY KEY,
			url TEXT,
			frequency INTEGER
		);
		CREATE TABLE moz_historyvisits (
			id INTEGER PRIMARY KEY,
			place_id INTEGER,
			visit_date INTEGER
		);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create tables: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO moz_places (id, url, frequency) VALUES (1, 'https://example.com/firefox1', 42);
		INSERT INTO moz_historyvisits (id, place_id, visit_date) VALUES (1, 1, 1781956800000000);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewFirefoxHandler(dbPath, "firefox", "profile")
	entries, err := handler.GetHistory(time.Time{}, time.Time{}, false)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Frequency != 42 {
		t.Errorf("expected Frequency 42, got %d", entries[0].Frequency)
	}
}

func TestFirefoxHandler_GetHistory_ScanError(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "firefox-scan-err-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "places.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE moz_places (
			id INTEGER PRIMARY KEY,
			url TEXT,
			visit_count INTEGER
		);
		CREATE TABLE moz_historyvisits (
			id INTEGER PRIMARY KEY,
			place_id INTEGER,
			visit_date INTEGER
		);
	`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	// Insert invalid type for visit_count (string that cannot be parsed as int)
	_, err = db.Exec(`
		INSERT INTO moz_places (id, url, visit_count) VALUES (1, 'https://example.com/firefox1', 'invalid_int');
		INSERT INTO moz_historyvisits (id, place_id, visit_date) VALUES (1, 1, 1781956800000000);
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	handler := NewFirefoxHandler(dbPath, "firefox", "profile")
	_, err = handler.GetHistory(time.Time{}, time.Time{}, false)
	if err == nil {
		t.Errorf("expected error during rows.Scan, got nil")
	}
}
