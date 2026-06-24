package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
)

func TestIngestSQL_SQLiteModes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "web-recap-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	entries := []models.HistoryEntry{
		{
			Browser:       "Chrome",
			Profile:       "Default",
			Timestamp:     time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			URL:           "https://google.com",
			Title:         "Google",
			Domain:        "google.com",
			VisitCount:    3,
			VisitDuration: 5000,
			Transition:    1,
			FromVisit:     10,
			SegmentID:     2,
			TypedCount:    1,
		},
		{
			Browser:    "Firefox",
			Profile:    "profile1",
			Timestamp:  time.Date(2026, 6, 20, 12, 5, 0, 0, time.UTC),
			URL:        "https://firefox.com",
			Title:      "Firefox Browser",
			Domain:     "firefox.com",
			VisitCount: 1,
			FromVisit:  0,
			VisitType:  2,
			Session:    45,
			Frequency:  3,
			Typed:      0,
		},
		{
			Browser:         "Safari",
			Profile:         "Default",
			Timestamp:       time.Date(2026, 6, 20, 12, 10, 0, 0, time.UTC),
			URL:             "https://apple.com",
			Title:           "Apple",
			Domain:          "apple.com",
			VisitCount:      2,
			RedirectSource:  0,
			Origin:          1,
			GenerationType:  3,
			LoadSuccessful:  true,
			HTTPNonGET:      false,
			Synthesized:     false,
		},
	}

	t.Run("both mode - relational (flat=false)", func(t *testing.T) {
		dbPath := filepath.Join(tempDir, "relational.db")
		connStr := "sqlite://" + dbPath

		count, err := Ingest(connStr, entries, "skip", "both", false)
		if err != nil {
			t.Fatalf("failed to ingest relational: %v", err)
		}
		if count != 3 {
			t.Errorf("expected 3 ingested entries, got %d", count)
		}

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("failed to open sqlite DB: %v", err)
		}
		defer db.Close()

		if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
			t.Fatalf("failed to enable foreign keys: %v", err)
		}

		// Verify parent table 'history' structure and records
		var hasID bool
		err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name='history')").Scan(&hasID)
		if err != nil || !hasID {
			t.Fatalf("expected 'history' table to exist")
		}

		// Verify columns in 'history'
		rows, err := db.Query("SELECT id, browser, profile, timestamp, url, title, domain, visit_count FROM history")
		if err != nil {
			t.Fatalf("history query failed: %v", err)
		}
		var historyRows []struct {
			id         int64
			browser    string
			profile    string
			timestamp  string
			url        string
			title      string
			domain     string
			visitCount int
		}
		for rows.Next() {
			var r struct {
				id         int64
				browser    string
				profile    string
				timestamp  string
				url        string
				title      string
				domain     string
				visitCount int
			}
			if err := rows.Scan(&r.id, &r.browser, &r.profile, &r.timestamp, &r.url, &r.title, &r.domain, &r.visitCount); err != nil {
				t.Fatalf("failed to scan history: %v", err)
			}
			historyRows = append(historyRows, r)
		}
		rows.Close()

		if len(historyRows) != 3 {
			t.Errorf("expected 3 history rows, got %d", len(historyRows))
		}

		// Check relational child table history_chrome contains history_id and specific columns
		var hasChrome bool
		err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name='history_chrome')").Scan(&hasChrome)
		if err != nil || !hasChrome {
			t.Fatalf("expected 'history_chrome' table to exist")
		}

		// It should NOT contain browser or profile columns in relational mode
		_, err = db.Query("SELECT browser FROM history_chrome")
		if err == nil {
			t.Errorf("expected error querying browser column on relational child table, but got none")
		}

		// Query correct columns
		var historyID int64
		var duration int64
		err = db.QueryRow("SELECT history_id, visit_duration FROM history_chrome").Scan(&historyID, &duration)
		if err != nil {
			t.Fatalf("child query failed: %v", err)
		}
		if duration != 5000 {
			t.Errorf("expected duration 5000, got %d", duration)
		}

		// Check foreign key link to history table
		var chromeBrowser string
		err = db.QueryRow("SELECT browser FROM history WHERE id = ?", historyID).Scan(&chromeBrowser)
		if err != nil {
			t.Fatalf("failed to lookup parent from child ID: %v", err)
		}
		if chromeBrowser != "Chrome" {
			t.Errorf("expected parent browser 'Chrome', got %q", chromeBrowser)
		}

		// Test delete cascade
		_, err = db.Exec("DELETE FROM history WHERE id = ?", historyID)
		if err != nil {
			t.Fatalf("failed to delete parent row: %v", err)
		}

		var childCount int
		err = db.QueryRow("SELECT count(*) FROM history_chrome").Scan(&childCount)
		if err != nil {
			t.Fatalf("child count failed: %v", err)
		}
		if childCount != 0 {
			t.Errorf("expected child row to be cascade-deleted, but it is still there")
		}
	})

	t.Run("both mode - flat (flat=true)", func(t *testing.T) {
		dbPath := filepath.Join(tempDir, "flat.db")
		connStr := "sqlite://" + dbPath

		count, err := Ingest(connStr, entries, "skip", "both", true)
		if err != nil {
			t.Fatalf("failed to ingest flat: %v", err)
		}
		if count != 3 {
			t.Errorf("expected 3 ingested entries, got %d", count)
		}

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("failed to open sqlite DB: %v", err)
		}
		defer db.Close()

		// In flat mode, 'history_chrome' should contain both common and specific columns
		var browserName string
		var duration int64
		err = db.QueryRow("SELECT browser, visit_duration FROM history_chrome").Scan(&browserName, &duration)
		if err != nil {
			t.Fatalf("flat child query failed: %v", err)
		}
		if browserName != "Chrome" || duration != 5000 {
			t.Errorf("unexpected flat child values: %s, %d", browserName, duration)
		}

		// history table should also contain specific columns in flat mode
		var historyDuration int64
		err = db.QueryRow("SELECT visit_duration FROM history WHERE browser = 'Chrome'").Scan(&historyDuration)
		if err != nil {
			t.Fatalf("flat parent query failed: %v", err)
		}
		if historyDuration != 5000 {
			t.Errorf("expected history duration 5000, got %d", historyDuration)
		}
	})

	t.Run("merged mode - relational (flat=false)", func(t *testing.T) {
		dbPath := filepath.Join(tempDir, "merged_relational.db")
		connStr := "sqlite://" + dbPath

		_, err := Ingest(connStr, entries, "skip", "merged", false)
		if err != nil {
			t.Fatalf("failed to ingest: %v", err)
		}

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("failed to open DB: %v", err)
		}
		defer db.Close()

		// history_chrome should NOT exist in merged mode
		var exists bool
		err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name='history_chrome')").Scan(&exists)
		if err != nil {
			t.Fatalf("table check failed: %v", err)
		}
		if exists {
			t.Errorf("history_chrome table should not exist in merged mode")
		}

		// history table should not contain visit_duration
		_, err = db.Query("SELECT visit_duration FROM history")
		if err == nil {
			t.Errorf("expected error querying visit_duration from relational merged table")
		}
	})

	t.Run("split mode", func(t *testing.T) {
		dbPath := filepath.Join(tempDir, "split.db")
		connStr := "sqlite://" + dbPath

		_, err := Ingest(connStr, entries, "skip", "split", false)
		if err != nil {
			t.Fatalf("failed to ingest: %v", err)
		}

		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			t.Fatalf("failed to open DB: %v", err)
		}
		defer db.Close()

		// history table should NOT exist in split mode
		var exists bool
		err = db.QueryRow("SELECT EXISTS (SELECT 1 FROM sqlite_master WHERE type='table' AND name='history')").Scan(&exists)
		if err != nil {
			t.Fatalf("table check failed: %v", err)
		}
		if exists {
			t.Errorf("history table should not exist in split mode")
		}

		// history_chrome should exist and be flat
		var browserName string
		err = db.QueryRow("SELECT browser FROM history_chrome").Scan(&browserName)
		if err != nil {
			t.Fatalf("failed to query split child: %v", err)
		}
		if browserName != "Chrome" {
			t.Errorf("expected browser Chrome, got %q", browserName)
		}
	})
}

func TestGetBrowserSpecificTableName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Chrome", "history_chrome"},
		{"Google Chrome", "history_chrome"},
		{"Microsoft Edge", "history_edge"},
		{"Edge", "history_edge"},
		{"Brave", "history_brave"},
		{"Chromium", "history_chromium"},
		{"Firefox", "history_firefox"},
		{"Safari", "history_safari"},
		// Custom / unknown browsers
		{"My Browser", "history_my_browser"},
		{"Custom-Browser-123", "history_custom_browser_123"},
		// SQL Injection / special character inputs
		{"chrome;DROP TABLE history--", "history_chromedrop_table_history__"},
		{"brave' OR '1'='1", "history_brave"},
		{"", "history_other"},
		{"!!!", "history_other"},
	}

	for _, tc := range tests {
		actual := getBrowserSpecificTableName(tc.input)
		if actual != tc.expected {
			t.Errorf("getBrowserSpecificTableName(%q) = %q; expected %q", tc.input, actual, tc.expected)
		}
	}
}

func TestGetDeterministicObjectID(t *testing.T) {
	browser := "Chrome"
	profile := "Default"
	timestamp := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	urlStr := "https://google.com"

	id1 := getDeterministicObjectID(browser, profile, timestamp, urlStr)
	id2 := getDeterministicObjectID(browser, profile, timestamp, urlStr)

	if id1 != id2 {
		t.Errorf("expected deterministic ObjectIDs, but got %v and %v", id1, id2)
	}

	// Verify that different inputs result in different IDs
	id3 := getDeterministicObjectID(browser, profile, timestamp, "https://google.com/other")
	if id1 == id3 {
		t.Errorf("expected different ObjectIDs for different URLs, but they were identical")
	}

	// Verify the length of the ID bytes
	if len(id1) != 12 {
		t.Errorf("expected 12 bytes ObjectID, got %d bytes", len(id1))
	}
}

