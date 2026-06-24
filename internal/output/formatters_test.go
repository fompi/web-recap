package output

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
)

func TestFormatCSV(t *testing.T) {
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

	var buf bytes.Buffer
	err := FormatCSV(&buf, entries)
	if err != nil {
		t.Fatalf("unexpected error formatting CSV: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "browser,profile,timestamp,domain,title,url,visit_count") {
		t.Errorf("missing header in CSV output: %q", output)
	}
	if !strings.Contains(output, "Chrome,Default,2026-06-20T12:00:00Z,google.com,Google,https://google.com,3") {
		t.Errorf("missing data row in CSV output: %q", output)
	}
}

type errorWriter struct{}

func (errorWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write error")
}

func TestFormatCSV_Error(t *testing.T) {
	entries := []models.HistoryEntry{
		{Browser: "Chrome"},
	}
	err := FormatCSV(errorWriter{}, entries)
	if err == nil {
		t.Errorf("expected error, got nil")
	}
}

func TestFormatText(t *testing.T) {
	entries := []models.HistoryEntry{
		{
			Browser:    "Chrome",
			Profile:    "Default",
			Timestamp:  time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			Domain:     "google.com",
			Title:      "Google Short Title",
			URL:        "https://google.com",
			VisitCount: 3,
		},
		{
			Browser:    "Safari",
			Profile:    "Profile1",
			Timestamp:  time.Date(2026, 6, 20, 12, 5, 0, 0, time.UTC),
			Domain:     "somelongdomainname.com",
			Title:      "Very long title that exceeds forty characters limit",
			URL:        "https://somelongdomainname.com/very/long/url/path/that/exceeds/sixty/characters/limit/to/verify/truncation/works/correctly",
			VisitCount: 1,
		},
	}

	var buf bytes.Buffer
	err := FormatText(&buf, entries)
	if err != nil {
		t.Fatalf("unexpected error formatting text: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "BROWSER") || !strings.Contains(output, "PROFILE") {
		t.Errorf("missing header in text output: %q", output)
	}
	if !strings.Contains(output, "Google Short Title") {
		t.Errorf("missing short title in text output")
	}

	// Verify NO truncation occurs on non-terminal (bytes.Buffer) outputs:
	fullTitle := "Very long title that exceeds forty characters limit"
	if !strings.Contains(output, fullTitle) {
		t.Errorf("expected full title %q to be in output: %q", fullTitle, output)
	}

	fullURL := "https://somelongdomainname.com/very/long/url/path/that/exceeds/sixty/characters/limit/to/verify/truncation/works/correctly"
	if !strings.Contains(output, fullURL) {
		t.Errorf("expected full URL %q to be in output: %q", fullURL, output)
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 0, "hello"},
		{"hello", -5, "hello"},
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 8, "hello..."},
		{"hello world", 2, "he"},
		{"español", 6, "esp..."}, // multi-byte rune test
	}

	for _, tc := range tests {
		got := truncateString(tc.input, tc.max)
		if got != tc.expected {
			t.Errorf("truncateString(%q, %d) = %q; expected %q", tc.input, tc.max, got, tc.expected)
		}
	}
}
