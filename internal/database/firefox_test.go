package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
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

	// Create Firefox schema tables
	_, err = db.Exec(`
		CREATE TABLE moz_places (
			id INTEGER PRIMARY KEY,
			url TEXT,
			title TEXT,
			visit_count INTEGER,
			frecency INTEGER, -- testing 'frecency' column fallback
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
	// We also insert an entry with visit_date = 0 to cover the isZero() check skip.
	_, err = db.Exec(`
		INSERT INTO moz_places (id, url, title, visit_count, frecency, typed) VALUES (1, 'https://example.com/firefox1', 'Firefox Page', 3, 15, 1);
		INSERT INTO moz_historyvisits (id, place_id, visit_date, from_visit, visit_type, session) VALUES (1, 1, 1781956800000000, 0, 5, 12345);

		INSERT INTO moz_places (id, url, title, visit_count, frecency, typed) VALUES (2, 'https://example.com/zero', 'Zero Page', 1, 0, 0);
		INSERT INTO moz_historyvisits (id, place_id, visit_date, from_visit, visit_type, session) VALUES (2, 2, 0, 0, 0, 0);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewFirefoxHandler(dbPath, "firefox", "test-firefox-profile")
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
	if entries[0].URL != "https://example.com/firefox1" {
		t.Errorf("expected URL 'https://example.com/firefox1', got %q", entries[0].URL)
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
	entries, err := handler.GetHistory(time.Time{}, time.Time{})
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
	_, err := handler.GetHistory(time.Time{}, time.Time{})
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
	_, err = handler2.GetHistory(time.Time{}, time.Time{})
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
	entries, err := handler.GetHistory(time.Time{}, time.Time{})
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
	_, err = handler.GetHistory(time.Time{}, time.Time{})
	if err == nil {
		t.Errorf("expected error during rows.Scan, got nil")
	}
}
