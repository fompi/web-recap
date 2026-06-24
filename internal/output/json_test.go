package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
)

func TestFormatJSON(t *testing.T) {
	entries := []models.HistoryEntry{
		{
			Browser:    "Chrome",
			Profile:    "Default",
			Timestamp:  time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			Domain:     "google.com",
			Title:      "Google",
			URL:        "https://google.com",
			VisitCount: 3,
		},
	}

	startDate := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)

	// Test with timezone = "" (defaults to UTC)
	var buf1 bytes.Buffer
	err := FormatJSON(&buf1, entries, "Chrome", startDate, endDate, "")
	if err != nil {
		t.Fatalf("unexpected error formatting JSON: %v", err)
	}

	var report1 models.HistoryReport
	if err := json.Unmarshal(buf1.Bytes(), &report1); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if report1.Browser != "Chrome" || report1.Timezone != "UTC" || report1.TotalEntries != 1 {
		t.Errorf("incorrect report fields: %+v", report1)
	}

	// Test with specific timezone
	var buf2 bytes.Buffer
	err = FormatJSON(&buf2, entries, "Chrome", startDate, endDate, "America/New_York")
	if err != nil {
		t.Fatalf("unexpected error formatting JSON: %v", err)
	}

	var report2 models.HistoryReport
	if err := json.Unmarshal(buf2.Bytes(), &report2); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if report2.Timezone != "America/New_York" {
		t.Errorf("expected timezone America/New_York, got %q", report2.Timezone)
	}
}

func TestFormatJSONLines(t *testing.T) {
	entries := []models.HistoryEntry{
		{
			Browser:    "Chrome",
			Profile:    "Default",
			Timestamp:  time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			Domain:     "google.com",
			Title:      "Google",
			URL:        "https://google.com",
			VisitCount: 3,
		},
		{
			Browser:    "Firefox",
			Profile:    "Profile1",
			Timestamp:  time.Date(2026, 6, 20, 12, 5, 0, 0, time.UTC),
			Domain:     "firefox.com",
			Title:      "Firefox",
			URL:        "https://firefox.com",
			VisitCount: 1,
		},
	}

	var buf bytes.Buffer
	err := FormatJSONLines(&buf, entries)
	if err != nil {
		t.Fatalf("unexpected error formatting JSONLines: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines of JSON, got %d: %q", len(lines), buf.String())
	}

	var e1, e2 models.HistoryEntry
	if err := json.Unmarshal([]byte(lines[0]), &e1); err != nil {
		t.Errorf("failed to unmarshal first line: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &e2); err != nil {
		t.Errorf("failed to unmarshal second line: %v", err)
	}

	if e1.Browser != "Chrome" || e2.Browser != "Firefox" {
		t.Errorf("incorrect data in JSONLines: %+v, %+v", e1, e2)
	}
}

func TestFormatJSONLines_Error(t *testing.T) {
	entries := []models.HistoryEntry{
		{Browser: "Chrome"},
	}
	err := FormatJSONLines(errorWriter{}, entries)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}
