//go:build !noingest

package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

)

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

func TestCLI_IngestFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli-ingest-err-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := createMockChromeDB(t, tempDir)

	resetFlags()
	rootCmd.SetArgs([]string{"ingest", "-b", "chrome", "-d", "chrome:" + dbPath, "-f", "10 days ago", "-c", "sqlite:///nonexistent-dir-for-ingest/target.db"})
	_, _, err = captureOutput(func() error {
		return rootCmd.Execute()
	})
	if err == nil {
		t.Errorf("expected error during ingest failure path, got nil")
	}
}
