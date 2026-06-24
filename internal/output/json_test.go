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

	// Test pretty-printed JSON
	var buf1 bytes.Buffer
	err := FormatJSON(&buf1, entries, true)
	if err != nil {
		t.Fatalf("unexpected error formatting JSON: %v", err)
	}

	output1 := buf1.String()
	if !strings.Contains(output1, "  \"browser\": \"Chrome\"") {
		t.Errorf("expected pretty printed output with indentation, got: %q", output1)
	}

	var parsed1 []models.HistoryEntry
	if err := json.Unmarshal(buf1.Bytes(), &parsed1); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}
	if len(parsed1) != 1 || parsed1[0].Browser != "Chrome" {
		t.Errorf("incorrect parsed entries: %+v", parsed1)
	}

	// Test compact JSON
	var buf2 bytes.Buffer
	err = FormatJSON(&buf2, entries, false)
	if err != nil {
		t.Fatalf("unexpected error formatting JSON: %v", err)
	}

	output2 := buf2.String()
	if strings.Contains(strings.TrimSpace(output2), "\n") {
		t.Errorf("expected compact JSON without newlines, got: %q", output2)
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

func TestFormatJSON_NilEntries(t *testing.T) {
	var buf bytes.Buffer
	err := FormatJSON(&buf, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "[]" {
		t.Errorf("expected [] for nil entries, got %q", got)
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
