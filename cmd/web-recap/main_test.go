package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rzolkos/web-recap/internal/browser"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	_ "modernc.org/sqlite"
)

func init() {
	osExit = func(code int) {
		// Mock exit to prevent test runner from exiting
	}
}

// Helper to reset Cobra's global flags before each test run.
func resetFlags() {
	for _, cmd := range []*cobra.Command{rootCmd, dumpCmd, statsCmd, ingestCmd, listCmd} {
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		})
		cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) {
			_ = f.Value.Set(f.DefValue)
			f.Changed = false
		})
	}
}

// Helper to capture stdout and stderr during test execution.
func captureOutput(f func() error) (string, string, error) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	outChan := make(chan string)
	errChan := make(chan string)

	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rOut)
		outChan <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, rErr)
		errChan <- buf.String()
	}()

	err := f()
	wOut.Close()
	wErr.Close()

	return <-outChan, <-errChan, err
}

func TestCLI_Version(t *testing.T) {
	resetFlags()

	// Test --version
	rootCmd.SetArgs([]string{"--version"})
	stdout, _, err := captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("--version failed: %v", err)
	}
	if !strings.Contains(stdout, "web-recap version") {
		t.Errorf("expected version output, got: %q", stdout)
	}

	// Test -V
	rootCmd.SetArgs([]string{"-V"})
	stdout, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("-V failed: %v", err)
	}
	if !strings.Contains(stdout, "web-recap version") {
		t.Errorf("expected version output, got: %q", stdout)
	}
}

func TestCLI_List(t *testing.T) {
	resetFlags()
	// Test with invalid environment to trigger GetHomeDirForUser error
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")

	rootCmd.SetArgs([]string{"list"})
	_, _, err := captureOutput(func() error {
		return rootCmd.Execute()
	})

	// Restore env
	if oldHome != "" {
		os.Setenv("HOME", oldHome)
	}
	if oldUserProfile != "" {
		os.Setenv("USERPROFILE", oldUserProfile)
	}

	if err == nil {
		t.Errorf("expected error listing with cleared HOME env, got nil")
	}

	// Test list with nonexistent user (returns No browsers detected)
	resetFlags()
	rootCmd.SetArgs([]string{"list", "-u", "nonexistent_user_xyz_123"})
	stdout, _, err := captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("list nonexistent user failed: %v", err)
	}
	if !strings.Contains(stdout, "No browsers detected") {
		t.Errorf("expected 'No browsers detected' for nonexistent user, got: %q", stdout)
	}

	// Test default listing (will either list or say "No browsers detected" in CI)
	resetFlags()
	rootCmd.SetArgs([]string{"list"})
	stdout, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("list cmd failed: %v", err)
	}
	if !strings.Contains(stdout, "Detected browsers") && !strings.Contains(stdout, "No browsers detected") {
		t.Errorf("unexpected list output: %q", stdout)
	}
}

// Helper to create a mock Chrome database
func createMockChromeDB(t *testing.T, dir string) string {
	dbPath := filepath.Join(dir, "History")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to create mock DB: %v", err)
	}
	defer db.Close()

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
		t.Fatalf("failed to create tables: %v", err)
	}

	// Chrome time: 2026-06-20 12:00:00 UTC
	chromeTime := int64((1781956800 + 11644473600) * 1000000)
	_, err = db.Exec(`
		INSERT INTO urls (id, url, title, visit_count, typed_count) VALUES (1, 'https://example.com/cli', 'CLI Page', 1, 0);
		INSERT INTO visits (id, url, visit_time, visit_duration, transition, from_visit, segment_id) VALUES (1, 1, ?, 1000, 0, 0, 0);
	`, chromeTime)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	return dbPath
}

func TestCLI_DumpFormatsAndCompression(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := createMockChromeDB(t, tempDir)

	formats := []string{"text", "csv", "json", "jsonl"}
	for _, format := range formats {
		t.Run("format "+format, func(t *testing.T) {
			resetFlags()
			rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "chrome:" + dbPath, "-F", format, "--summary=false", "-f", "10 days ago"})
			stdout, _, err := captureOutput(func() error {
				return rootCmd.Execute()
			})
			if err != nil {
				t.Fatalf("dump format %s failed: %v", format, err)
			}
			if format == "csv" && !strings.Contains(stdout, "https://example.com/cli") {
				t.Errorf("expected URL in csv, got: %s", stdout)
			}
		})
	}

	// Test compression options
	compressions := []struct {
		args   []string
		suffix string
	}{
		{[]string{"-z"}, ".gz"},
		{[]string{"-zz"}, ".bz2"},
		{[]string{"-zzz"}, ".xz"},
	}

	for _, tc := range compressions {
		t.Run("compression "+tc.suffix, func(t *testing.T) {
			resetFlags()
			outFile := filepath.Join(tempDir, "out"+tc.suffix)
			args := append([]string{"dump", "-b", "chrome", "-d", "chrome:" + dbPath, "-o", outFile, "-f", "10 days ago"}, tc.args...)
			rootCmd.SetArgs(args)

			_, _, err := captureOutput(func() error {
				return rootCmd.Execute()
			})
			if err != nil {
				t.Fatalf("dump compression failed: %v", err)
			}

			if _, err := os.Stat(outFile); err != nil {
				t.Errorf("expected output file %s to exist", outFile)
			}
		})
	}
}

func TestCLI_Stats(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli-stats-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := createMockChromeDB(t, tempDir)

	resetFlags()
	rootCmd.SetArgs([]string{"stats", "-b", "chrome", "-d", "chrome:" + dbPath, "-f", "10 days ago"})
	stdout, _, err := captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Fatalf("stats failed: %v", err)
	}
	if !strings.Contains(stdout, "WEB HISTORY STATISTICS") {
		t.Errorf("expected stats output, got: %s", stdout)
	}
}

func TestCLI_IngestModes(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli-ingest-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	srcDB := createMockChromeDB(t, tempDir)

	modes := []struct {
		mode string
		flat bool
	}{
		{"merged", false},
		{"split", false},
		{"both", false},
		{"both", true},
	}

	for _, tc := range modes {
		t.Run(fmt.Sprintf("mode=%s_flat=%t", tc.mode, tc.flat), func(t *testing.T) {
			resetFlags()
			targetDB := filepath.Join(tempDir, fmt.Sprintf("target_%s_%t.db", tc.mode, tc.flat))
			args := []string{
				"ingest",
				"-b", "chrome",
				"-d", "chrome:" + srcDB,
				"-c", "sqlite://" + targetDB,
				"-M", tc.mode,
				"-f", "10 days ago",
			}
			if tc.flat {
				args = append(args, "-x")
			}
			rootCmd.SetArgs(args)

			_, _, err := captureOutput(func() error {
				return rootCmd.Execute()
			})
			if err != nil {
				t.Fatalf("ingest failed: %v", err)
			}

			// Verify target db exists and contains data
			db, err := sql.Open("sqlite", targetDB)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var count int
			tableName := "history"
			if tc.mode == "split" {
				tableName = "history_chrome"
			}
			err = db.QueryRow("SELECT count(*) FROM " + tableName).Scan(&count)
			if err != nil || count == 0 {
				t.Errorf("expected data in table %s, got count=%d, err=%v", tableName, count, err)
			}
		})
	}
}

func TestCLI_ErrorsAndEdgeCases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli-errors-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := createMockChromeDB(t, tempDir)

	// 1. Invalid timezone
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "chrome:" + dbPath, "-Z", "Invalid/Timezone"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error for invalid timezone, got nil")
	}

	// 1b. Valid local timezone
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "chrome:" + dbPath, "-Z", "local", "-f", "10 days ago"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Errorf("unexpected error for local timezone: %v", err)
	}

	// 2. Invalid date range format
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "chrome:" + dbPath, "-f", "invalid-date"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error for invalid from date, got nil")
	}

	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "chrome:" + dbPath, "-t", "invalid-date"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error for invalid to date, got nil")
	}

	// 3. Output file creation error
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "chrome:" + dbPath, "-o", "/nonexistent/dir/file.txt"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error creating file in nonexistent dir, got nil")
	}

	// 4. Unsupported output format
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "chrome:" + dbPath, "-F", "unsupported_fmt"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error for unsupported format, got nil")
	}

	// 5. Invalid DB path format (unrecognized)
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "/path/to/unrecognized"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error for invalid DB path prefix, got nil")
	}

	// 6. Unsupported browser flag
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "invalid_browser"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error for unsupported browser flag, got nil")
	}

	// 7. Invalid limit formats
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-l", "chrome:invalid"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error for invalid limit flag, got nil")
	}

	// 8. Query warning vs error on nonexistent DB
	// With browser flag explicit: returns query error
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "chrome:/nonexistent/path/History"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error querying nonexistent chrome path explicitly, got nil")
	}

	// Without browser flag explicit: prints warning to stderr and returns no error
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-d", "chrome:/nonexistent/path/History"})
	_, stderr, err := captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Errorf("unexpected error for query warning path: %v", err)
	}
	if !strings.Contains(stderr, "Warning: failed to query") {
		t.Errorf("expected warning printed to stderr, got: %q", stderr)
	}

	// 9. Browser specific limits and total limits checks
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "chrome:" + dbPath, "-f", "10 days ago", "-l", "chrome:0,google chrome:0::1"})
	stdout, _, err := captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	// Chrome is limited to 0, so no entries should be output
	if strings.Contains(stdout, "https://example.com/cli") {
		t.Errorf("expected 0 entries due to chrome:0 limit, but found URL in output: %s", stdout)
	}

	// 10. Ingest failure
	resetFlags()
	rootCmd.SetArgs([]string{"ingest", "-b", "chrome", "-d", "chrome:" + dbPath, "-f", "10 days ago", "-c", "sqlite:///nonexistent-dir-for-ingest/target.db"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error during ingest failure path, got nil")
	}

	// 11. Compression without output file error
	resetFlags()
	rootCmd.SetArgs([]string{"dump", "-b", "chrome", "-d", "chrome:" + dbPath, "-z"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error when compression is used without output file, got nil")
	} else if !strings.Contains(err.Error(), "compression cannot be used when outputting to stdout") {
		t.Errorf("expected compression stdout error, got: %v", err)
	}
}

func TestParseDBPaths(t *testing.T) {
	tests := []struct {
		dbFlag    string
		browsers  []string
		expectErr bool
	}{
		{"", nil, false},
		{"chrome:/path/to/db", nil, false},
		{"chrome:/path/to/db,firefox:/other/path", nil, false},
		{"/path/to/single/db", []string{"chrome"}, false},
		{"/path/to/History", []string{"chrome", "firefox"}, false},
		{"/path/to/places.sqlite", []string{"chrome", "firefox"}, false},
		{"/path/to/History.db", []string{"chrome", "firefox"}, false},
		{"/path/to/unrecognized", []string{"chrome", "firefox"}, true}, // Ambiguous
	}

	for _, tc := range tests {
		t.Run(tc.dbFlag, func(t *testing.T) {
			_, err := parseDBPaths(tc.dbFlag, tc.browsers)
			if tc.expectErr && err == nil {
				t.Errorf("expected error for %q, got nil", tc.dbFlag)
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tc.dbFlag, err)
			}
		})
	}
}

func TestResolveBrowsers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "resolve-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create mock Chrome History files for all platform structures under tempDir to ensure it's detected
	chromePaths := []string{
		filepath.Join(tempDir, "Library/Application Support/Google/Chrome/Default/History"),
		filepath.Join(tempDir, ".config/google-chrome/Default/History"),
		filepath.Join(tempDir, "AppData/Local/Google/Chrome/User Data/Default/History"),
	}
	for _, p := range chromePaths {
		os.MkdirAll(filepath.Dir(p), 0755)
		os.WriteFile(p, []byte(""), 0644)
	}

	detector := browser.NewDetectorForUser(tempDir)

	// 1. Invalid browser type
	_, err = resolveBrowsers("invalid_browser", detector, nil)
	if err == nil {
		t.Errorf("expected error for invalid browser type, got nil")
	}

	// 2. Browser not detected (e.g. Firefox)
	_, err = resolveBrowsers("firefox", detector, nil)
	if err == nil {
		t.Errorf("expected error for not installed browser, got nil")
	}

	// 3. Custom DB paths override for detected Chrome
	res, err := resolveBrowsers("chrome", detector, map[string]string{"chrome": "/custom/path/history"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0].Path != "/custom/path/history" {
		t.Errorf("expected custom path override, got: %+v", res)
	}

	// 4. Custom DB paths override for non-detected Safari
	res, err = resolveBrowsers("safari", detector, map[string]string{"safari": "/custom/safari/history"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 1 || res[0].Path != "/custom/safari/history" {
		t.Errorf("expected custom path for Safari, got: %+v", res)
	}

	// 5. Empty browser flag (default to all override dbPaths + detected)
	res, err = resolveBrowsers("", detector, map[string]string{"chrome": "/path/to/chrome/history"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) < 1 {
		t.Errorf("expected at least 1 resolved browser, got: %+v", res)
	}
}

func TestParseLimit(t *testing.T) {
	tests := []struct {
		limitStr  string
		expectErr bool
	}{
		{"", false},
		{"100", false},
		{"chrome:50,safari:20", false},
		{"chrome:50::100", false},
		{"chrome:invalid", true},
		{"chrome:50::invalid", true},
		{"chrome:50,invalid", true},
		{"chrome:50::100::200", true},
		{"chrome::100", true},          // invalid double colon format
		{"chrome:invalid::100", true},  // invalid limit value in double colon
		{"-50", true},                  // negative limit
		{"chrome:50::-5", true},        // negative total limit
	}

	for _, tc := range tests {
		t.Run(tc.limitStr, func(t *testing.T) {
			_, _, err := parseLimit(tc.limitStr)
			if tc.expectErr && err == nil {
				t.Errorf("expected error for %q, got nil", tc.limitStr)
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tc.limitStr, err)
			}
		})
	}
}

func TestCLI_Main(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"--version"})
	_, _, err := captureOutput(func() error {
		main()
		return nil
	})
	if err != nil {
		t.Errorf("main failed: %v", err)
	}
}
