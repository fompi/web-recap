package database

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSafariHandler_NewSafariHandler_EmptyBrowser(t *testing.T) {
	handler := NewSafariHandler("path", "", "profile")
	if handler.browserName != "safari" {
		t.Errorf("expected default browserName 'safari', got %q", handler.browserName)
	}
}

func TestSafariHandler_GetHistory_NonDarwin(t *testing.T) {
	// Temporarily force isDarwinOS to false to verify GetHistory returns error on non-darwin OS.
	oldIsDarwinOS := isDarwinOS
	isDarwinOS = false
	defer func() { isDarwinOS = oldIsDarwinOS }()

	handler := NewSafariHandler("some-path", "safari", "profile")
	_, err := handler.GetHistory(time.Time{}, time.Time{})
	if !errors.Is(err, ErrSafariNotAvailable) {
		t.Errorf("expected ErrSafariNotAvailable, got %v", err)
	}
}

func TestSafariHandler_GetHistory_AllOS_FullColumns(t *testing.T) {
	// Temporarily override isDarwinOS to true so we can test the SQLite retrieval logic on Linux/Windows.
	oldIsDarwinOS := isDarwinOS
	isDarwinOS = true
	defer func() { isDarwinOS = oldIsDarwinOS }()

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

	handler := NewSafariHandler(dbPath, "safari", "test-safari-profile")

	// Test 1: date range (returns mock)
	startDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	entries, err := handler.GetHistory(startDate, endDate)
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Title != "Example Title" {
		t.Errorf("expected Title 'Example Title', got %q", entries[0].Title)
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

	// Test 3b: end date with non-zero nanoseconds at 00:00:00 (does not add 86400)
	nsEndDate := time.Date(2026, 6, 20, 0, 0, 0, 100, time.UTC)
	entries, _ = handler.GetHistory(time.Time{}, nsEndDate)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (excluding 12:00:00 visit), got %d", len(entries))
	}

	// Test 4: empty dates (limits to 10000)
	entries, _ = handler.GetHistory(time.Time{}, time.Time{})
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}
}

func TestSafariHandler_GetHistory_MissingColumns(t *testing.T) {
	oldIsDarwinOS := isDarwinOS
	isDarwinOS = true
	defer func() { isDarwinOS = oldIsDarwinOS }()

	tempDir, err := os.MkdirTemp("", "safari-missing-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "History.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create Safari schema tables WITHOUT optional columns, but WITH title in history_items (Case 2)
	_, err = db.Exec(`
		CREATE TABLE history_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			title TEXT, -- title is on items instead of visits
			visit_count INTEGER NOT NULL
		);
		CREATE TABLE history_visits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			history_item INTEGER NOT NULL,
			visit_time REAL NOT NULL
		);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create tables: %v", err)
	}

	// Insert mock data
	_, err = db.Exec(`
		INSERT INTO history_items (id, url, title, visit_count) VALUES (1, 'https://example.com/page1', 'Item Title', 5);
		INSERT INTO history_visits (id, history_item, visit_time) VALUES (1, 1, 803649600.5);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewSafariHandler(dbPath, "safari", "test-safari-profile")
	entries, err := handler.GetHistory(time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Title != "Item Title" {
		t.Errorf("expected Title 'Item Title' (fallback to items table), got %q", entry.Title)
	}
	// Verify missing columns defaults
	if entry.RedirectSource != 0 || entry.Origin != 0 || entry.GenerationType != 0 || !entry.LoadSuccessful {
		t.Errorf("unexpected defaults for missing columns: %+v", entry)
	}
}

func TestSafariHandler_GetHistory_NoTitleAnywhere(t *testing.T) {
	oldIsDarwinOS := isDarwinOS
	isDarwinOS = true
	defer func() { isDarwinOS = oldIsDarwinOS }()

	tempDir, err := os.MkdirTemp("", "safari-notitle-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "History.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Create Safari schema tables WITHOUT any title column (Case 3)
	_, err = db.Exec(`
		CREATE TABLE history_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			visit_count INTEGER NOT NULL
		);
		CREATE TABLE history_visits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			history_item INTEGER NOT NULL,
			visit_time REAL NOT NULL
		);
	`)
	if err != nil {
		db.Close()
		t.Fatalf("failed to create tables: %v", err)
	}

	// Insert mock data including a zero-timestamp visit
	_, err = db.Exec(`
		INSERT INTO history_items (id, url, visit_count) VALUES (1, 'https://example.com/page1', 5);
		INSERT INTO history_visits (id, history_item, visit_time) VALUES (1, 1, 803649600.5);
		INSERT INTO history_visits (id, history_item, visit_time) VALUES (2, 1, -63113904000.0);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	handler := NewSafariHandler(dbPath, "safari", "test-safari-profile")
	entries, err := handler.GetHistory(time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("failed to get history: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Title != "https://example.com/page1" {
		t.Errorf("expected Title fallback to URL 'https://example.com/page1', got %q", entry.Title)
	}
}

func TestSafariHandler_GetHistory_Errors(t *testing.T) {
	oldIsDarwinOS := isDarwinOS
	isDarwinOS = true
	defer func() { isDarwinOS = oldIsDarwinOS }()

	// 1. Copy database error (non-existent path)
	handler := NewSafariHandler("/nonexistent/safari/History.db", "safari", "profile")
	_, err := handler.GetHistory(time.Time{}, time.Time{})
	if err == nil {
		t.Errorf("expected error copying non-existent database, got nil")
	}

	// 2. Query execution error (missing tables)
	tempDir, err := os.MkdirTemp("", "safari-err-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)
	dbPath := filepath.Join(tempDir, "History.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	// Just create an unrelated table
	_, _ = db.Exec("CREATE TABLE dummy (id INTEGER)")
	db.Close()

	handler2 := NewSafariHandler(dbPath, "safari", "profile")
	_, err = handler2.GetHistory(time.Time{}, time.Time{})
	if err == nil {
		t.Errorf("expected error executing query on empty schema, got nil")
	}
}

func TestSafariHandler_GetHistory_ScanError(t *testing.T) {
	oldIsDarwinOS := isDarwinOS
	isDarwinOS = true
	defer func() { isDarwinOS = oldIsDarwinOS }()

	tempDir, err := os.MkdirTemp("", "safari-scan-err-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "History.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE history_items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			url TEXT NOT NULL,
			visit_count INTEGER NOT NULL
		);
		CREATE TABLE history_visits (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			history_item INTEGER NOT NULL,
			visit_time REAL NOT NULL
		);
	`)
	if err != nil {
		db.Close()
		t.Fatal(err)
	}

	// Insert invalid type for visit_count
	_, err = db.Exec(`
		INSERT INTO history_items (id, url, visit_count) VALUES (1, 'https://example.com/page1', 'invalid_int');
		INSERT INTO history_visits (id, history_item, visit_time) VALUES (1, 1, 803649600.5);
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	handler := NewSafariHandler(dbPath, "safari", "profile")
	_, err = handler.GetHistory(time.Time{}, time.Time{})
	if err == nil {
		t.Errorf("expected error during rows.Scan, got nil")
	}
}
