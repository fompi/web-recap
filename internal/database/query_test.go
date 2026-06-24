package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rzolkos/web-recap/internal/browser"
	"github.com/rzolkos/web-recap/internal/models"
)

func TestQuery_NewQuerier(t *testing.T) {
	oldIsDarwinOS := isDarwinOS
	isDarwinOS = true
	defer func() { isDarwinOS = oldIsDarwinOS }()

	tests := []struct {
		bType     browser.Type
		expectErr bool
	}{
		{browser.Chrome, false},
		{browser.Chromium, false},
		{browser.Edge, false},
		{browser.Brave, false},
		{browser.Firefox, false},
		{browser.Safari, false},
		{browser.Type("unknown"), true},
	}

	for _, tc := range tests {
		b := &browser.Browser{Type: tc.bType, Path: "dummy-path"}
		q, err := NewQuerier(b)
		if tc.expectErr {
			if err == nil {
				t.Errorf("expected error for browser type %q, got nil", tc.bType)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for browser type %q: %v", tc.bType, err)
			}
			if q == nil {
				t.Errorf("expected querier for browser type %q, got nil", tc.bType)
			}
		}
	}
}

func TestQuery_QueryError(t *testing.T) {
	// Call Query with unsupported type to check error
	b := &browser.Browser{Type: browser.Type("unknown"), Path: "dummy"}
	_, err := Query(b, time.Time{}, time.Time{})
	if err == nil {
		t.Errorf("expected error, got nil")
	}

	// Call Query with a type that fails to query (e.g. non-existent SQLite path)
	b2 := &browser.Browser{Type: browser.Chrome, Path: "/nonexistent/db"}
	_, err = Query(b2, time.Time{}, time.Time{})
	if err == nil {
		t.Errorf("expected error querying non-existent path, got nil")
	}
}

func TestQuery_SortEntriesDescending(t *testing.T) {
	entries := []models.HistoryEntry{
		{Timestamp: time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC), URL: "url1"},
		{Timestamp: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC), URL: "url2"},
		{Timestamp: time.Date(2026, 6, 20, 11, 0, 0, 0, time.UTC), URL: "url3"},
	}

	SortEntriesDescending(entries)

	if entries[0].URL != "url2" || entries[1].URL != "url3" || entries[2].URL != "url1" {
		t.Errorf("incorrect sort order: %+v", entries)
	}
}

func TestQuery_Success(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "query-success-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "History")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
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
		t.Fatalf("failed to create tables: %v", err)
	}

	// Insert mock data out of order by timestamp
	// Chrome times:
	// 2026-06-20 10:00:00 UTC -> (1781949600 + 11644473600) * 1000000 = 13426423200000000
	// 2026-06-20 12:00:00 UTC -> (1781956800 + 11644473600) * 1000000 = 13426430400000000
	// 2026-06-20 11:00:00 UTC -> (1781953200 + 11644473600) * 1000000 = 13426426800000000
	_, err = db.Exec(`
		INSERT INTO urls (id, url, title, visit_count, typed_count) VALUES (1, 'https://example.com/url1', 'Page 1', 1, 0);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (1, 1, 13426423200000000, 0, 0, 0, 0);

		INSERT INTO urls (id, url, title, visit_count, typed_count) VALUES (2, 'https://example.com/url2', 'Page 2', 1, 0);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (2, 2, 13426430400000000, 0, 0, 0, 0);

		INSERT INTO urls (id, url, title, visit_count, typed_count) VALUES (3, 'https://example.com/url3', 'Page 3', 1, 0);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (3, 3, 13426426800000000, 0, 0, 0, 0);
	`)
	db.Close()
	if err != nil {
		t.Fatalf("failed to insert mock data: %v", err)
	}

	b := &browser.Browser{
		Type: browser.Chrome,
		Path: dbPath,
		Name: "chrome",
	}

	entries, err := Query(b, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("failed to query: %v", err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify that the entries are sorted descending by timestamp
	if entries[0].URL != "https://example.com/url2" || entries[1].URL != "https://example.com/url3" || entries[2].URL != "https://example.com/url1" {
		t.Errorf("expected sorted descending order: entries[0]=url2, entries[1]=url3, entries[2]=url1; got: entries[0]=%s, entries[1]=%s, entries[2]=%s", entries[0].URL, entries[1].URL, entries[2].URL)
	}
}
