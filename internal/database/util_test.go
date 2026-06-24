package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

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
		{
			name:     "Invalid escape sequence fallback http",
			url:      "http://%4/foo",
			expected: "%4",
		},
		{
			name:     "Invalid escape sequence fallback https",
			url:      "https://%4/foo",
			expected: "%4",
		},
		{
			name:     "Invalid escape sequence no path http",
			url:      "http://%4",
			expected: "%4",
		},
		{
			name:     "Mailto protocol",
			url:      "mailto:test@example.com",
			expected: "mailto:test@example.com",
		},
		{
			name:     "Relative path",
			url:      "relative/path",
			expected: "relative/path",
		},
		{
			name:     "Invalid escape sequence no prefix",
			url:      "%4",
			expected: "%4",
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

	// Test table not exists returns error
	_, err = HasColumn(db, "nonexistent", "id")
	if err == nil {
		t.Errorf("expected error for non-existent table, got nil")
	}
}

func TestCopyDatabaseWithWAL(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "wal-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	if err := os.WriteFile(dbPath, []byte("main db"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create aux files
	if err := os.WriteFile(dbPath+"-wal", []byte("wal"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath+"-shm", []byte("shm"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath+"-journal", []byte("journal"), 0644); err != nil {
		t.Fatal(err)
	}

	tmpPath, cleanup, err := CopyDatabaseWithWAL(dbPath, "copy-test")
	if err != nil {
		t.Fatalf("unexpected error copying database: %v", err)
	}
	defer cleanup()

	// Verify copies exist
	if _, err := os.Stat(tmpPath); err != nil {
		t.Errorf("expected temp db to exist, got error: %v", err)
	}
	if _, err := os.Stat(tmpPath + "-wal"); err != nil {
		t.Errorf("expected temp db-wal to exist, got error: %v", err)
	}
	if _, err := os.Stat(tmpPath + "-shm"); err != nil {
		t.Errorf("expected temp db-shm to exist, got error: %v", err)
	}
	if _, err := os.Stat(tmpPath + "-journal"); err != nil {
		t.Errorf("expected temp db-journal to exist, got error: %v", err)
	}

	// Test non-existent path
	_, _, err = CopyDatabaseWithWAL(filepath.Join(tempDir, "nonexistent.db"), "copy-test")
	if err == nil {
		t.Errorf("expected error copying non-existent database, got nil")
	}

	// Test permission denied error (using a path we cannot open)
	noPermPath := filepath.Join(tempDir, "noperm.db")
	if err := os.WriteFile(noPermPath, []byte("noperm"), 0000); err == nil {
		// Only run this check if we successfully created a 0000 file and cannot read it
		_, _, err2 := CopyDatabaseWithWAL(noPermPath, "copy-test")
		// Clean up permission so we can delete the temp dir
		os.Chmod(noPermPath, 0644)
		if err2 == nil {
			t.Errorf("expected permission denied error, got nil")
		}
	}

	// Test CreateTemp error by setting invalid TMPDIR
	oldTmpDir := os.Getenv("TMPDIR")
	os.Setenv("TMPDIR", "/nonexistent-dir-for-test")
	_, _, errTemp := CopyDatabaseWithWAL(dbPath, "copy-test")
	if oldTmpDir != "" {
		os.Setenv("TMPDIR", oldTmpDir)
	} else {
		os.Unsetenv("TMPDIR")
	}
	if errTemp == nil {
		t.Errorf("expected CreateTemp error, got nil")
	}

	// Test io.Copy error by passing a directory as database source
	_, _, errCopy := CopyDatabaseWithWAL(tempDir, "copy-test")
	if errCopy == nil {
		t.Errorf("expected io.Copy error when passing a directory, got nil")
	}
}

