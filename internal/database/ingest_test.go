package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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
			LoadSuccessful:  func() *bool { b := true; return &b }(),
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

func TestParseMySQLDSN(t *testing.T) {
	tests := []struct {
		input     string
		expected  string
		expectErr bool
	}{
		{
			input:    "mysql://user:pass@localhost:3306/dbname",
			expected: "user:pass@tcp(localhost:3306)/dbname?parseTime=true",
		},
		{
			input:    "mysql://user@localhost/dbname?timeout=5s",
			expected: "user@tcp(localhost:3306)/dbname?parseTime=true&timeout=5s",
		},
		{
			input:    "mysql://localhost/dbname",
			expected: "tcp(localhost:3306)/dbname?parseTime=true",
		},
		{
			input:     "mysql://%4",
			expectErr: true,
		},
	}

	for _, tc := range tests {
		actual, err := parseMySQLDSN(tc.input)
		if tc.expectErr {
			if err == nil {
				t.Errorf("expected error for %q, got nil", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tc.input, err)
			}
			if actual != tc.expected {
				t.Errorf("parseMySQLDSN(%q) = %q; expected %q", tc.input, actual, tc.expected)
			}
		}
	}
}

func TestGetSQLTime(t *testing.T) {
	tm := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	
	// Postgres returns time.Time
	resPostgres := getSQLTime("postgres", tm)
	if resPostgres != tm {
		t.Errorf("expected time.Time, got %v", resPostgres)
	}

	// Other returns string
	resOther := getSQLTime("sqlite", tm)
	expectedStr := "2026-06-20 12:00:00.000000"
	if resOther != expectedStr {
		t.Errorf("expected %q, got %v", expectedStr, resOther)
	}
}

func TestSQLQueryBuilders(t *testing.T) {
	e := models.HistoryEntry{
		Browser:   "Chrome",
		Profile:   "Default",
		Timestamp: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
		URL:       "https://google.com",
	}

	drivers := []string{"postgres", "sqlite", "mysql"}
	conflicts := []string{"replace", "skip"}

	for _, d := range drivers {
		for _, c := range conflicts {
			buildSQLInsertMerged(d, "tbl", e, c)
			buildSQLInsertFlat(d, "tbl", e, c)
			buildSQLInsertChrome(d, "tbl", e, c)
			buildSQLInsertFirefox(d, "tbl", e, c)
			buildSQLInsertSafari(d, "tbl", e, c)
			buildSQLInsertChildChrome(d, "tbl", 123, e, c)
			buildSQLInsertChildFirefox(d, "tbl", 123, e, c)
			buildSQLInsertChildSafari(d, "tbl", 123, e, c)
		}
	}
}

func TestIngest_ErrorPaths(t *testing.T) {
	// 1. Invalid conflict strategy
	_, err := Ingest("sqlite://test.db", nil, "invalid", "merged", false)
	if err == nil {
		t.Errorf("expected error for invalid conflict strategy, got nil")
	}

	// 2. Invalid mysql connection string
	_, err = Ingest("mysql://%4", nil, "skip", "merged", false)
	if err == nil {
		t.Errorf("expected error for invalid mysql DSN, got nil")
	}

	// 3. PostgreSQL connection error (returns error)
	_, err = Ingest("postgres://localhost:5432/nonexistent", nil, "skip", "merged", false)
	if err == nil {
		t.Errorf("expected error connecting to postgres, got nil")
	}
}

var mockDriverRegistered = false

func registerMockDriver() {
	if !mockDriverRegistered {
		sql.Register("mock-sql-driver", &mockDriver{})
		mockDriverRegistered = true
	}
}

var (
	mockTxError     = false
	mockExecError   = false
	mockRowsEmpty   = false
	mockRowsError   = false
	mockCommitError = false
)

type mockDriver struct{}

func (d *mockDriver) Open(name string) (driver.Conn, error) {
	return &mockConn{}, nil
}

type mockConn struct{}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return &mockStmt{}, nil
}

func (c *mockConn) Close() error {
	return nil
}

func (c *mockConn) Begin() (driver.Tx, error) {
	if mockTxError {
		return nil, errors.New("mock tx error")
	}
	return &mockTx{}, nil
}

func (c *mockConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	if mockExecError && strings.Contains(query, "INSERT") {
		return nil, errors.New("mock exec error")
	}
	return &mockResult{}, nil
}

func (c *mockConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if mockExecError && strings.Contains(query, "INSERT") {
		return nil, errors.New("mock exec error")
	}
	return &mockResult{}, nil
}

type mockTx struct{}

func (t *mockTx) Commit() error {
	if mockCommitError {
		return errors.New("mock commit error")
	}
	return nil
}

func (t *mockTx) Rollback() error {
	return nil
}

type mockStmt struct{}

func (s *mockStmt) Close() error {
	return nil
}

func (s *mockStmt) NumInput() int {
	return -1
}

func (s *mockStmt) Exec(args []driver.Value) (driver.Result, error) {
	if mockExecError {
		return nil, errors.New("mock exec error")
	}
	return &mockResult{}, nil
}

func (s *mockStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &mockRows{}, nil
}

type mockResult struct{}

func (r *mockResult) LastInsertId() (int64, error) {
	return 1, nil
}

func (r *mockResult) RowsAffected() (int64, error) {
	return 1, nil
}

type mockRows struct {
	nextCalled bool
}

func (r *mockRows) Columns() []string {
	return []string{"id"}
}

func (r *mockRows) Close() error {
	return nil
}

func (r *mockRows) Next(dest []driver.Value) error {
	if mockRowsEmpty {
		return io.EOF
	}
	if mockRowsError {
		return errors.New("mock rows error")
	}
	if r.nextCalled {
		return io.EOF
	}
	r.nextCalled = true
	dest[0] = int64(42)
	return nil
}

func TestSQLTables_PostgresAndMySQL(t *testing.T) {
	registerMockDriver()

	db, err := sql.Open("mock-sql-driver", "whatever")
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	defer db.Close()

	drivers := []string{"postgres", "mysql", "sqlite"}
	modes := []string{"merged", "split", "both"}
	flats := []bool{true, false}

	for _, driverName := range drivers {
		for _, mode := range modes {
			for _, flat := range flats {
				err := createSQLTables(db, driverName, mode, flat)
				if err != nil {
					t.Errorf("unexpected error in createSQLTables(%s, %s, %v): %v", driverName, mode, flat, err)
				}
			}
		}
	}
}

func TestGetParentID_Postgres(t *testing.T) {
	registerMockDriver()

	db, err := sql.Open("mock-sql-driver", "whatever")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	entry := models.HistoryEntry{
		Browser:   "Chrome",
		Profile:   "Default",
		Timestamp: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
		URL:       "https://google.com",
	}

	id, err := getParentID(tx, "postgres", entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 42 {
		t.Errorf("expected parent ID 42, got %d", id)
	}
}

func TestIngest_CoverBranches(t *testing.T) {
	registerMockDriver()

	// Reset mock variables on exit
	defer func() {
		mockTxError = false
		mockExecError = false
		mockRowsEmpty = false
		mockRowsError = false
		mockCommitError = false
	}()

	entries := []models.HistoryEntry{
		{
			Browser:   "Chrome",
			Profile:   "Default",
			Timestamp: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			URL:       "https://google.com",
		},
	}

	// 1. empty conflict strategy -> defaults to skip
	count, err := Ingest("sqlite://:memory:", entries, "", "merged", false)
	if err != nil || count != 1 {
		t.Errorf("expected success with empty conflict strategy, got count=%d, err=%v", count, err)
	}

	// 2. default invalid mode -> defaults to merged
	count, err = Ingest("sqlite://:memory:", entries, "skip", "invalid-mode", false)
	if err != nil || count != 1 {
		t.Errorf("expected success with invalid mode falling back, got count=%d, err=%v", count, err)
	}

	// 3. sqlite3:// prefix and postgresql:// prefix (postgresql will connect error, but tests the prefix path)
	_, err = Ingest("sqlite3://:memory:", entries, "skip", "merged", false)
	if err != nil {
		t.Errorf("expected success with sqlite3 prefix, got %v", err)
	}
	_, err = Ingest("postgresql://localhost:5432/nonexistent", nil, "skip", "merged", false)
	if err == nil {
		t.Errorf("expected error with postgresql prefix, got nil")
	}

	// 4. No prefix fallback to sqlite
	tempDir, err := os.MkdirTemp("", "fallback-test-*")
	if err == nil {
		defer os.RemoveAll(tempDir)
		dbPath := filepath.Join(tempDir, "fallback.db")
		_, err = Ingest(dbPath, entries, "skip", "merged", false)
		if err != nil {
			t.Errorf("expected success with no prefix SQLite, got %v", err)
		}
	}

	// 5. sql.Open error in ingestSQL
	_, err = ingestSQL("invalid-driver", "...", nil, "skip", "merged", false)
	if err == nil {
		t.Errorf("expected error from invalid sql driver, got nil")
	}

	// 6. sqlite PRAGMA error by passing nonexistent directory path
	_, err = ingestSQL("sqlite", "/nonexistent-dir-for-pragma/test.db", nil, "skip", "merged", false)
	if err == nil {
		t.Errorf("expected error on nonexistent directory sqlite database, got nil")
	}

	// 7. createRelationalChildTable default return case
	res := createRelationalChildTable("sqlite", "history_other", "other")
	if res != "" {
		t.Errorf("expected empty string, got %q", res)
	}

	// 8. mockConn Begin() error
	mockTxError = true
	_, err = ingestSQL("mock-sql-driver", "whatever", entries, "skip", "merged", false)
	if err == nil {
		t.Errorf("expected Begin() error, got nil")
	}
	mockTxError = false

	// 9. parent insert tx.Exec error
	mockExecError = true
	_, err = ingestSQL("mock-sql-driver", "whatever", entries, "skip", "merged", false)
	if err == nil {
		t.Errorf("expected Exec() error on parent, got nil")
	}
	mockExecError = false

	// 10. getParentID returning sql.ErrNoRows (causes continue in relational child insert)
	mockRowsEmpty = true
	entriesBoth := []models.HistoryEntry{
		{
			Browser:   "Chrome",
			Profile:   "Default",
			Timestamp: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			URL:       "https://google.com",
		},
	}
	// Run both mode, relational. It will insert parent, then call getParentID, which returns ErrNoRows. It should continue without error.
	_, err = ingestSQL("mock-sql-driver", "whatever", entriesBoth, "skip", "both", false)
	if err != nil {
		t.Errorf("expected success despite ErrNoRows on child insert, got %v", err)
	}
	mockRowsEmpty = false

	// 11. getParentID returning rows error
	mockRowsError = true
	_, err = ingestSQL("mock-sql-driver", "whatever", entriesBoth, "skip", "both", false)
	if err == nil {
		t.Errorf("expected error from getParentID database error, got nil")
	}
	mockRowsError = false

	// 12. child insert tx.Exec error (Firefox and Safari child insertions)
	// We'll set mockExecError to true but only after the first Exec (parent insert).
	// To do this simply without complex state in mock driver, we can use both mode and since parent insert succeeds if mockExecError is false,
	// wait, we can just trigger it by setting mockExecError = true. But then parent insert will fail first.
	// Oh! What if mode = "split"? In split mode, there is NO parent insert! It directly inserts child!
	// So in split mode, the first Exec is the child insert!
	// If we set mockExecError = true, the split mode child insert fails, covering the flat/split child insert error!
	mockExecError = true
	_, err = ingestSQL("mock-sql-driver", "whatever", entries, "skip", "split", false)
	if err == nil {
		t.Errorf("expected child insert Exec error, got nil")
	}
	mockExecError = false

	// 13. child relational insert tx.Exec error
	// To make parent insert succeed but child relational insert fail:
	// We can use conflictStrategy = "replace" and mockRowsEmpty = false.
	// In mockConn.ExecContext, we can count the executions and return error on the second execution!
	// Let's implement execution counter in mockConn
}

type countingConn struct {
	execCount int
}

func (c *countingConn) Prepare(query string) (driver.Stmt, error) {
	return &mockStmt{}, nil
}

func (c *countingConn) Close() error {
	return nil
}

func (c *countingConn) Begin() (driver.Tx, error) {
	return &countingTx{c: c}, nil
}

func (c *countingConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(query, "INSERT") {
		c.execCount++
		if c.execCount == 2 {
			return nil, errors.New("mock child exec error")
		}
	}
	return &mockResult{}, nil
}

type countingTx struct {
	c *countingConn
}

func (t *countingTx) Commit() error   { return nil }
func (t *countingTx) Rollback() error { return nil }

type countingDriver struct{}

func (d *countingDriver) Open(name string) (driver.Conn, error) {
	return &countingConn{}, nil
}

func TestChildRelationalInsert_ExecError(t *testing.T) {
	sql.Register("counting-sql-driver", &countingDriver{})

	entries := []models.HistoryEntry{
		{
			Browser:   "Chrome",
			Profile:   "Default",
			Timestamp: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			URL:       "https://google.com",
		},
	}

	// In both mode, relational (flat=false), with replace strategy, parent insert is Exec 1.
	// Then getParentID runs (returns 42 from mockRows since mockRows is still using mockRows helper).
	// Then child relational insert runs as Exec 2.
	// Our counting driver will return error on Exec 2, triggering child insert failure!
	_, err := ingestSQL("counting-sql-driver", "whatever", entries, "replace", "both", false)
	if err == nil {
		t.Errorf("expected child relational insert error, got nil")
	}
}

func TestCommitError(t *testing.T) {
	registerMockDriver()
	mockCommitError = true
	defer func() { mockCommitError = false }()

	entries := []models.HistoryEntry{
		{
			Browser:   "Chrome",
			Profile:   "Default",
			Timestamp: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			URL:       "https://google.com",
		},
	}

	_, err := ingestSQL("mock-sql-driver", "whatever", entries, "skip", "merged", false)
	if err == nil {
		t.Errorf("expected commit error, got nil")
	}
}

