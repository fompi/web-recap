package database

import (
	"database/sql"
	"testing"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	_ "modernc.org/sqlite"
)

func TestConvertChromeTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		chromeVal int64
		expectZero bool
	}{
		{
			name:       "Zero timestamp",
			chromeVal:  0,
			expectZero: true,
		},
		{
			name:       "Valid timestamp",
			chromeVal:  13289816330000000, // Some arbitrary timestamp
			expectZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertChromeTimestamp(tt.chromeVal)

			if tt.expectZero && !result.IsZero() {
				t.Errorf("expected zero time, got %v", result)
			}

			if !tt.expectZero && result.IsZero() {
				t.Errorf("expected non-zero time, got zero")
			}
		})
	}
}

func TestConvertFirefoxTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		firefoxVal int64
		expectZero bool
	}{
		{
			name:       "Zero timestamp",
			firefoxVal: 0,
			expectZero: true,
		},
		{
			name:       "Valid timestamp",
			firefoxVal: 1702742400000000, // December 16, 2023
			expectZero: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertFirefoxTimestamp(tt.firefoxVal)

			if tt.expectZero && !result.IsZero() {
				t.Errorf("expected zero time, got %v", result)
			}

			if !tt.expectZero && result.IsZero() {
				t.Errorf("expected non-zero time, got zero")
			}
		})
	}
}

func TestConvertSafariTimestamp(t *testing.T) {
	tests := []struct {
		name            string
		safariVal       float64
		expectZero      bool
		minExpectedYear int
	}{
		{
			name:            "Zero timestamp (Safari epoch)",
			safariVal:       0.0,
			expectZero:      false, // 0 = 2001-01-01
			minExpectedYear: 2001,
		},
		{
			name:            "Valid timestamp",
			safariVal:       730000000.0, // Some arbitrary seconds since 2001
			expectZero:      false,
			minExpectedYear: 2024, // 730M seconds since 2001 is ~2024
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertSafariTimestamp(tt.safariVal)

			if tt.expectZero && !result.IsZero() {
				t.Errorf("expected zero time, got %v", result)
			}

			if !tt.expectZero && result.IsZero() {
				t.Errorf("expected non-zero time, got zero")
			}

			// Verify result is a valid time (year >= min expected)
			if result.Year() < tt.minExpectedYear {
				t.Errorf("expected year >= %d, got %d", tt.minExpectedYear, result.Year())
			}
		})
	}
}

func TestExtractDomain(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "Valid HTTPS URL",
			url:      "https://example.com/page",
			expected: "example.com",
		},
		{
			name:     "Valid HTTP URL",
			url:      "http://www.google.com/search",
			expected: "www.google.com",
		},
		{
			name:     "URL with port",
			url:      "https://localhost:8080/app",
			expected: "localhost:8080",
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: "",
		},
		{
			name:     "Subdomain",
			url:      "https://api.github.com/repos",
			expected: "api.github.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractDomain(tt.url)

			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFilterByDateRange(t *testing.T) {
	startDate := time.Date(2025, 12, 15, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 12, 16, 0, 0, 0, 0, time.UTC)

	entries := []interface{}{
		models.HistoryEntry{Timestamp: time.Date(2025, 12, 14, 12, 0, 0, 0, time.UTC)}, // Out (before)
		models.HistoryEntry{Timestamp: time.Date(2025, 12, 15, 12, 0, 0, 0, time.UTC)}, // In
		models.HistoryEntry{Timestamp: time.Date(2025, 12, 16, 12, 0, 0, 0, time.UTC)}, // In
		models.HistoryEntry{Timestamp: time.Date(2025, 12, 17, 12, 0, 0, 0, time.UTC)}, // Out (after)
	}

	t.Run("No date filter", func(t *testing.T) {
		result := FilterByDateRange(entries, time.Time{}, time.Time{})
		if len(result) != 4 {
			t.Errorf("expected 4 entries, got %d", len(result))
		}
	})

	t.Run("With start and end date range", func(t *testing.T) {
		result := FilterByDateRange(entries, startDate, endDate)
		if len(result) != 2 {
			t.Errorf("expected 2 entries, got %d", len(result))
		}
		
		e1 := result[0].(models.HistoryEntry)
		e2 := result[1].(models.HistoryEntry)
		if e1.Timestamp.Day() != 15 || e2.Timestamp.Day() != 16 {
			t.Errorf("unexpected elements kept in range")
		}
	})

	t.Run("Only start date", func(t *testing.T) {
		result := FilterByDateRange(entries, startDate, time.Time{})
		if len(result) != 3 {
			t.Errorf("expected 3 entries, got %d", len(result))
		}
	})

	t.Run("Only end date", func(t *testing.T) {
		result := FilterByDateRange(entries, time.Time{}, endDate)
		if len(result) != 3 {
			t.Errorf("expected 3 entries, got %d", len(result))
		}
	})
}

func TestHasColumn(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory database: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test_table (id INTEGER, name TEXT, created_at TIMESTAMP)")
	if err != nil {
		t.Fatalf("failed to create test table: %v", err)
	}

	tests := []struct {
		column   string
		expected bool
	}{
		{"id", true},
		{"name", true},
		{"created_at", true},
		{"ID", true}, // Case-insensitivity check
		{"NAME", true},
		{"missing", false},
		{"other", false},
	}

	for _, tt := range tests {
		t.Run(tt.column, func(t *testing.T) {
			exists, err := HasColumn(db, "test_table", tt.column)
			if err != nil {
				t.Errorf("unexpected error checking column %q: %v", tt.column, err)
			}
			if exists != tt.expected {
				t.Errorf("expected exists=%v for column %q, got %v", tt.expected, tt.column, exists)
			}
		})
	}
}

