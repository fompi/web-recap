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

func TestFormatTable(t *testing.T) {
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
	err := FormatTable(&buf, entries)
	if err != nil {
		t.Fatalf("unexpected error formatting table: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "BROWSER") || !strings.Contains(output, "PROFILE") {
		t.Errorf("missing header in table output: %q", output)
	}
	if !strings.Contains(output, "Google Short Title") {
		t.Errorf("missing short title in table output")
	}

	// Verify truncation works:
	// "Very long title that exceeds forty characters limit" -> len 52
	// Expected: "Very long title that exceeds forty ch..." (37 + 3 = 40)
	expectedTitle := "Very long title that exceeds forty ch..."
	if !strings.Contains(output, expectedTitle) {
		t.Errorf("expected truncated title %q to be in output: %q", expectedTitle, output)
	}

	// URL: "https://somelongdomainname.com/very/long/url/path/that/exceeds/sixty/characters/limit/to/verify/truncation/works/correctly" -> len 120
	// Expected: "https://somelongdomainname.com/very/long/url/path/that/ex..." (57 + 3 = 60)
	expectedURL := "https://somelongdomainname.com/very/long/url/path/that/ex..."
	if !strings.Contains(output, expectedURL) {
		t.Errorf("expected truncated URL %q to be in output: %q", expectedURL, output)
	}
}
