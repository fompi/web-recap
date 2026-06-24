package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func ingestSQL(driverName, dsn string, entries []models.HistoryEntry, conflictStrategy, mode string, flat bool) (int, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	if driverName == "sqlite" {
		if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
			return 0, err
		}
	}

	// Ensure all required tables exist
	if err := createSQLTables(db, driverName, mode, flat); err != nil {
		return 0, err
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	insertedCount := 0

	for _, entry := range entries {
		success := false
		insertedParent := false

		// 1. Insert into merged/flat table if applicable
		if mode == "merged" || mode == "both" {
			tbl := "history"
			var query string
			var args []interface{}

			if flat {
				query, args = buildSQLInsertFlat(driverName, tbl, entry, conflictStrategy)
			} else {
				query, args = buildSQLInsertMerged(driverName, tbl, entry, conflictStrategy)
			}

			res, err := tx.Exec(query, args...)
			if err != nil {
				return insertedCount, fmt.Errorf("failed to insert history entry: %w", err)
			}
			rows, _ := res.RowsAffected()
			if rows > 0 {
				success = true
				insertedParent = true
			}
		}

		// 2. Insert into browser-specific table if applicable
		if mode == "split" || mode == "both" {
			tbl := getBrowserSpecificTableName(entry.Browser)
			var query string
			var args []interface{}

			if mode == "both" && !flat {
				// Relational mode child insertion
				if insertedParent || conflictStrategy == "replace" {
					parentID, err := getParentID(tx, driverName, entry)
					if err != nil {
						if err == sql.ErrNoRows {
							continue
						}
						return insertedCount, fmt.Errorf("failed to get parent ID: %w", err)
					}
					if parentID > 0 {
						switch getBrowserClass(entry.Browser) {
						case "firefox":
							query, args = buildSQLInsertChildFirefox(driverName, tbl, parentID, entry, conflictStrategy)
						case "safari":
							query, args = buildSQLInsertChildSafari(driverName, tbl, parentID, entry, conflictStrategy)
						default: // chrome and other chromium-based browsers
							query, args = buildSQLInsertChildChrome(driverName, tbl, parentID, entry, conflictStrategy)
						}
						res, err := tx.Exec(query, args...)
						if err != nil {
							return insertedCount, fmt.Errorf("failed to insert child entry: %w", err)
						}
						rows, _ := res.RowsAffected()
						if rows > 0 {
							success = true
						}
					}
				}
			} else {
				// Flat mode or split mode child insertion
				switch getBrowserClass(entry.Browser) {
				case "firefox":
					query, args = buildSQLInsertFirefox(driverName, tbl, entry, conflictStrategy)
				case "safari":
					query, args = buildSQLInsertSafari(driverName, tbl, entry, conflictStrategy)
				default: // chrome and other chromium-based browsers
					query, args = buildSQLInsertChrome(driverName, tbl, entry, conflictStrategy)
				}
				res, err := tx.Exec(query, args...)
				if err != nil {
					return insertedCount, fmt.Errorf("failed to insert child entry: %w", err)
				}
				rows, _ := res.RowsAffected()
				if rows > 0 {
					success = true
				}
			}
		}

		if success {
			insertedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return insertedCount, err
	}

	return insertedCount, nil
}

func getBrowserClass(browser string) string {
	b := strings.ToLower(browser)
	if strings.Contains(b, "firefox") {
		return "firefox"
	}
	if strings.Contains(b, "safari") {
		return "safari"
	}
	return "chrome" // default to chrome schema for chromium browsers
}

func getBrowserSpecificTableName(browser string) string {
	b := strings.ToLower(browser)
	if strings.Contains(b, "google chrome") || b == "chrome" {
		return "history_chrome"
	}
	if strings.Contains(b, "microsoft edge") || b == "edge" {
		return "history_edge"
	}
	if strings.Contains(b, "brave") {
		return "history_brave"
	}
	if strings.Contains(b, "chromium") {
		return "history_chromium"
	}
	if strings.Contains(b, "firefox") {
		return "history_firefox"
	}
	if strings.Contains(b, "safari") {
		return "history_safari"
	}
	// Sanitize to prevent SQL injection in table name
	b = strings.ReplaceAll(b, " ", "_")
	b = strings.ReplaceAll(b, "-", "_")
	var sb strings.Builder
	for _, r := range b {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		}
	}
	b = sb.String()
	if b == "" {
		b = "other"
	}
	return "history_" + b
}

func createSQLTables(db *sql.DB, driverName, mode string, flat bool) error {
	var queries []string

	// Flat table query
	flatQueryMySQL := `
	CREATE TABLE IF NOT EXISTS history (
		browser VARCHAR(50),
		profile VARCHAR(100),
		timestamp DATETIME(6),
		url TEXT,
		title TEXT,
		domain VARCHAR(255),
		visit_count INT,
		visit_duration BIGINT,
		transition BIGINT,
		from_visit BIGINT,
		segment_id BIGINT,
		typed_count BIGINT,
		visit_type BIGINT,
		session BIGINT,
		frequency BIGINT,
		typed BIGINT,
		redirect_source BIGINT,
		redirect_destination BIGINT,
		origin BIGINT,
		generation_type BIGINT,
		load_successful TINYINT,
		http_non_get TINYINT,
		synthesized TINYINT,
		UNIQUE KEY unique_visit (browser, profile, timestamp, url(255))
	)`

	flatQueryPostgres := `
	CREATE TABLE IF NOT EXISTS history (
		browser VARCHAR(50),
		profile VARCHAR(100),
		timestamp TIMESTAMPTZ,
		url TEXT,
		title TEXT,
		domain VARCHAR(255),
		visit_count INT,
		visit_duration BIGINT,
		transition BIGINT,
		from_visit BIGINT,
		segment_id BIGINT,
		typed_count BIGINT,
		visit_type BIGINT,
		session BIGINT,
		frequency BIGINT,
		typed BIGINT,
		redirect_source BIGINT,
		redirect_destination BIGINT,
		origin BIGINT,
		generation_type BIGINT,
		load_successful INT,
		http_non_get INT,
		synthesized INT,
		CONSTRAINT unique_flat_visit UNIQUE (browser, profile, timestamp, url)
	)`

	flatQuerySQLite := `
	CREATE TABLE IF NOT EXISTS history (
		browser TEXT,
		profile TEXT,
		timestamp TIMESTAMP,
		url TEXT,
		title TEXT,
		domain TEXT,
		visit_count INTEGER,
		visit_duration INTEGER,
		transition INTEGER,
		from_visit INTEGER,
		segment_id INTEGER,
		typed_count INTEGER,
		visit_type INTEGER,
		session INTEGER,
		frequency INTEGER,
		typed INTEGER,
		redirect_source INTEGER,
		redirect_destination INTEGER,
		origin INTEGER,
		generation_type INTEGER,
		load_successful INTEGER,
		http_non_get INTEGER,
		synthesized INTEGER,
		UNIQUE (browser, profile, timestamp, url)
	)`

	// Merged table query (standard fields only)
	mergedQueryMySQL := `
	CREATE TABLE IF NOT EXISTS history (
		browser VARCHAR(50),
		profile VARCHAR(100),
		timestamp DATETIME(6),
		url TEXT,
		title TEXT,
		domain VARCHAR(255),
		visit_count INT,
		UNIQUE KEY unique_visit (browser, profile, timestamp, url(255))
	)`

	mergedQueryPostgres := `
	CREATE TABLE IF NOT EXISTS history (
		browser VARCHAR(50),
		profile VARCHAR(100),
		timestamp TIMESTAMPTZ,
		url TEXT,
		title TEXT,
		domain VARCHAR(255),
		visit_count INT,
		CONSTRAINT unique_merged_visit UNIQUE (browser, profile, timestamp, url)
	)`

	mergedQuerySQLite := `
	CREATE TABLE IF NOT EXISTS history (
		browser TEXT,
		profile TEXT,
		timestamp TIMESTAMP,
		url TEXT,
		title TEXT,
		domain TEXT,
		visit_count INTEGER,
		UNIQUE (browser, profile, timestamp, url)
	)`

	// Merged table query with ID (for relational mode: mode=both, flat=false)
	mergedWithIDQueryMySQL := `
	CREATE TABLE IF NOT EXISTS history (
		id INT AUTO_INCREMENT PRIMARY KEY,
		browser VARCHAR(50),
		profile VARCHAR(100),
		timestamp DATETIME(6),
		url TEXT,
		title TEXT,
		domain VARCHAR(255),
		visit_count INT,
		UNIQUE KEY unique_visit (browser, profile, timestamp, url(255))
	)`

	mergedWithIDQueryPostgres := `
	CREATE TABLE IF NOT EXISTS history (
		id SERIAL PRIMARY KEY,
		browser VARCHAR(50),
		profile VARCHAR(100),
		timestamp TIMESTAMPTZ,
		url TEXT,
		title TEXT,
		domain VARCHAR(255),
		visit_count INT,
		CONSTRAINT unique_merged_visit UNIQUE (browser, profile, timestamp, url)
	)`

	mergedWithIDQuerySQLite := `
	CREATE TABLE IF NOT EXISTS history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		browser TEXT,
		profile TEXT,
		timestamp TIMESTAMP,
		url TEXT,
		title TEXT,
		domain TEXT,
		visit_count INTEGER,
		UNIQUE (browser, profile, timestamp, url)
	)`

	// Sub-tables queries helper helper
	createChromeTable := func(tbl string) string {
		switch driverName {
		case "mysql":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				browser VARCHAR(50),
				profile VARCHAR(100),
				timestamp DATETIME(6),
				url TEXT,
				title TEXT,
				domain VARCHAR(255),
				visit_count INT,
				visit_duration BIGINT,
				transition BIGINT,
				from_visit BIGINT,
				segment_id BIGINT,
				typed_count BIGINT,
				UNIQUE KEY unique_visit (browser, profile, timestamp, url(255))
			)`, tbl)
		case "postgres":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				browser VARCHAR(50),
				profile VARCHAR(100),
				timestamp TIMESTAMPTZ,
				url TEXT,
				title TEXT,
				domain VARCHAR(255),
				visit_count INT,
				visit_duration BIGINT,
				transition BIGINT,
				from_visit BIGINT,
				segment_id BIGINT,
				typed_count BIGINT,
				CONSTRAINT unique_%s_visit UNIQUE (browser, profile, timestamp, url)
			)`, tbl, tbl)
		default: // sqlite
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				browser TEXT,
				profile TEXT,
				timestamp TIMESTAMP,
				url TEXT,
				title TEXT,
				domain TEXT,
				visit_count INTEGER,
				visit_duration INTEGER,
				transition INTEGER,
				from_visit INTEGER,
				segment_id INTEGER,
				typed_count INTEGER,
				UNIQUE (browser, profile, timestamp, url)
			)`, tbl)
		}
	}

	createFirefoxTable := func(tbl string) string {
		switch driverName {
		case "mysql":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				browser VARCHAR(50),
				profile VARCHAR(100),
				timestamp DATETIME(6),
				url TEXT,
				title TEXT,
				domain VARCHAR(255),
				visit_count INT,
				from_visit BIGINT,
				visit_type BIGINT,
				session BIGINT,
				frequency BIGINT,
				typed BIGINT,
				UNIQUE KEY unique_visit (browser, profile, timestamp, url(255))
			)`, tbl)
		case "postgres":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				browser VARCHAR(50),
				profile VARCHAR(100),
				timestamp TIMESTAMPTZ,
				url TEXT,
				title TEXT,
				domain VARCHAR(255),
				visit_count INT,
				from_visit BIGINT,
				visit_type BIGINT,
				session BIGINT,
				frequency BIGINT,
				typed BIGINT,
				CONSTRAINT unique_%s_visit UNIQUE (browser, profile, timestamp, url)
			)`, tbl, tbl)
		default: // sqlite
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				browser TEXT,
				profile TEXT,
				timestamp TIMESTAMP,
				url TEXT,
				title TEXT,
				domain TEXT,
				visit_count INTEGER,
				from_visit INTEGER,
				visit_type INTEGER,
				session INTEGER,
				frequency INTEGER,
				typed INTEGER,
				UNIQUE (browser, profile, timestamp, url)
			)`, tbl)
		}
	}

	createSafariTable := func(tbl string) string {
		switch driverName {
		case "mysql":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				browser VARCHAR(50),
				profile VARCHAR(100),
				timestamp DATETIME(6),
				url TEXT,
				title TEXT,
				domain VARCHAR(255),
				visit_count INT,
				redirect_source BIGINT,
				redirect_destination BIGINT,
				origin BIGINT,
				generation_type BIGINT,
				load_successful TINYINT,
				http_non_get TINYINT,
				synthesized TINYINT,
				UNIQUE KEY unique_visit (browser, profile, timestamp, url(255))
			)`, tbl)
		case "postgres":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				browser VARCHAR(50),
				profile VARCHAR(100),
				timestamp TIMESTAMPTZ,
				url TEXT,
				title TEXT,
				domain VARCHAR(255),
				visit_count INT,
				redirect_source BIGINT,
				redirect_destination BIGINT,
				origin BIGINT,
				generation_type BIGINT,
				load_successful INT,
				http_non_get INT,
				synthesized INT,
				CONSTRAINT unique_%s_visit UNIQUE (browser, profile, timestamp, url)
			)`, tbl, tbl)
		default: // sqlite
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				browser TEXT,
				profile TEXT,
				timestamp TIMESTAMP,
				url TEXT,
				title TEXT,
				domain TEXT,
				visit_count INTEGER,
				redirect_source INTEGER,
				redirect_destination INTEGER,
				origin INTEGER,
				generation_type INTEGER,
				load_successful INTEGER,
				http_non_get INTEGER,
				synthesized INTEGER,
				UNIQUE (browser, profile, timestamp, url)
			)`, tbl)
		}
	}

	if mode == "merged" {
		if flat {
			switch driverName {
			case "mysql":
				queries = append(queries, flatQueryMySQL)
			case "postgres":
				queries = append(queries, flatQueryPostgres)
			default:
				queries = append(queries, flatQuerySQLite)
			}
		} else {
			switch driverName {
			case "mysql":
				queries = append(queries, mergedQueryMySQL)
			case "postgres":
				queries = append(queries, mergedQueryPostgres)
			default:
				queries = append(queries, mergedQuerySQLite)
			}
		}
	} else if mode == "split" {
		queries = append(queries, createChromeTable("history_chrome"))
		queries = append(queries, createChromeTable("history_chromium"))
		queries = append(queries, createChromeTable("history_edge"))
		queries = append(queries, createChromeTable("history_brave"))
		queries = append(queries, createFirefoxTable("history_firefox"))
		queries = append(queries, createSafariTable("history_safari"))
	} else if mode == "both" {
		if flat {
			switch driverName {
			case "mysql":
				queries = append(queries, flatQueryMySQL)
			case "postgres":
				queries = append(queries, flatQueryPostgres)
			default:
				queries = append(queries, flatQuerySQLite)
			}
			queries = append(queries, createChromeTable("history_chrome"))
			queries = append(queries, createChromeTable("history_chromium"))
			queries = append(queries, createChromeTable("history_edge"))
			queries = append(queries, createChromeTable("history_brave"))
			queries = append(queries, createFirefoxTable("history_firefox"))
			queries = append(queries, createSafariTable("history_safari"))
		} else {
			switch driverName {
			case "mysql":
				queries = append(queries, mergedWithIDQueryMySQL)
			case "postgres":
				queries = append(queries, mergedWithIDQueryPostgres)
			default:
				queries = append(queries, mergedWithIDQuerySQLite)
			}
			queries = append(queries, createRelationalChildTable(driverName, "history_chrome", "chrome"))
			queries = append(queries, createRelationalChildTable(driverName, "history_chromium", "chrome"))
			queries = append(queries, createRelationalChildTable(driverName, "history_edge", "chrome"))
			queries = append(queries, createRelationalChildTable(driverName, "history_brave", "chrome"))
			queries = append(queries, createRelationalChildTable(driverName, "history_firefox", "firefox"))
			queries = append(queries, createRelationalChildTable(driverName, "history_safari", "safari"))
		}
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("table creation query failed: %v\nQuery: %s", err, q)
		}
	}
	return nil
}

func createRelationalChildTable(driverName, tbl, browserType string) string {
	switch browserType {
	case "chrome":
		switch driverName {
		case "mysql":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				history_id INT PRIMARY KEY,
				visit_duration BIGINT,
				transition BIGINT,
				from_visit BIGINT,
				segment_id BIGINT,
				typed_count BIGINT,
				FOREIGN KEY (history_id) REFERENCES history(id) ON DELETE CASCADE
			)`, tbl)
		case "postgres":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				history_id INT PRIMARY KEY,
				visit_duration BIGINT,
				transition BIGINT,
				from_visit BIGINT,
				segment_id BIGINT,
				typed_count BIGINT,
				FOREIGN KEY (history_id) REFERENCES history(id) ON DELETE CASCADE
			)`, tbl)
		default: // sqlite
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				history_id INTEGER PRIMARY KEY,
				visit_duration INTEGER,
				transition INTEGER,
				from_visit INTEGER,
				segment_id INTEGER,
				typed_count INTEGER,
				FOREIGN KEY (history_id) REFERENCES history(id) ON DELETE CASCADE
			)`, tbl)
		}
	case "firefox":
		switch driverName {
		case "mysql":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				history_id INT PRIMARY KEY,
				from_visit BIGINT,
				visit_type BIGINT,
				session BIGINT,
				frequency BIGINT,
				typed BIGINT,
				FOREIGN KEY (history_id) REFERENCES history(id) ON DELETE CASCADE
			)`, tbl)
		case "postgres":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				history_id INT PRIMARY KEY,
				from_visit BIGINT,
				visit_type BIGINT,
				session BIGINT,
				frequency BIGINT,
				typed BIGINT,
				FOREIGN KEY (history_id) REFERENCES history(id) ON DELETE CASCADE
			)`, tbl)
		default: // sqlite
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				history_id INTEGER PRIMARY KEY,
				from_visit INTEGER,
				visit_type INTEGER,
				session INTEGER,
				frequency INTEGER,
				typed INTEGER,
				FOREIGN KEY (history_id) REFERENCES history(id) ON DELETE CASCADE
			)`, tbl)
		}
	case "safari":
		switch driverName {
		case "mysql":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				history_id INT PRIMARY KEY,
				redirect_source BIGINT,
				redirect_destination BIGINT,
				origin BIGINT,
				generation_type BIGINT,
				load_successful TINYINT,
				http_non_get TINYINT,
				synthesized TINYINT,
				FOREIGN KEY (history_id) REFERENCES history(id) ON DELETE CASCADE
			)`, tbl)
		case "postgres":
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				history_id INT PRIMARY KEY,
				redirect_source BIGINT,
				redirect_destination BIGINT,
				origin BIGINT,
				generation_type BIGINT,
				load_successful INT,
				http_non_get INT,
				synthesized INT,
				FOREIGN KEY (history_id) REFERENCES history(id) ON DELETE CASCADE
			)`, tbl)
		default: // sqlite
			return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
				history_id INTEGER PRIMARY KEY,
				redirect_source INTEGER,
				redirect_destination INTEGER,
				origin INTEGER,
				generation_type INTEGER,
				load_successful INTEGER,
				http_non_get INTEGER,
				synthesized INTEGER,
				FOREIGN KEY (history_id) REFERENCES history(id) ON DELETE CASCADE
			)`, tbl)
		}
	}
	return ""
}

func getParentID(tx *sql.Tx, driverName string, entry models.HistoryEntry) (int64, error) {
	var query string
	if driverName == "postgres" {
		query = "SELECT id FROM history WHERE browser = $1 AND profile = $2 AND timestamp = $3 AND url = $4"
	} else {
		query = "SELECT id FROM history WHERE browser = ? AND profile = ? AND timestamp = ? AND url = ?"
	}

	var id int64
	err := tx.QueryRow(query, entry.Browser, entry.Profile, getSQLTime(driverName, entry.Timestamp), entry.URL).Scan(&id)
	return id, err
}


// SQL Query Builders

func getSQLTime(driverName string, t time.Time) interface{} {
	if driverName == "postgres" {
		return t
	}
	return t.Format("2006-01-02 15:04:05.000000")
}

func buildSQLInsertMerged(driver, tbl string, e models.HistoryEntry, conflict string) (string, []interface{}) {
	var q string
	args := []interface{}{e.Browser, e.Profile, getSQLTime(driver, e.Timestamp), e.URL, e.Title, e.Domain, e.VisitCount}

	if driver == "postgres" {
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (browser, profile, timestamp, url, title, domain, visit_count) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (browser, profile, timestamp, url) DO UPDATE SET title = EXCLUDED.title, visit_count = EXCLUDED.visit_count", tbl)
		default:
			q = fmt.Sprintf("INSERT INTO %s (browser, profile, timestamp, url, title, domain, visit_count) VALUES ($1, $2, $3, $4, $5, $6, $7) ON CONFLICT (browser, profile, timestamp, url) DO NOTHING", tbl)
		}
	} else if driver == "sqlite" {
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (browser, profile, timestamp, url, title, domain, visit_count) VALUES (?, ?, ?, ?, ?, ?, ?) ON CONFLICT (browser, profile, timestamp, url) DO UPDATE SET title = excluded.title, visit_count = excluded.visit_count", tbl)
		default:
			q = fmt.Sprintf("INSERT OR IGNORE INTO %s (browser, profile, timestamp, url, title, domain, visit_count) VALUES (?, ?, ?, ?, ?, ?, ?)", tbl)
		}
	} else { // mysql
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (browser, profile, timestamp, url, title, domain, visit_count) VALUES (?, ?, ?, ?, ?, ?, ?) ON DUPLICATE KEY UPDATE title = VALUES(title), visit_count = VALUES(visit_count)", tbl)
		default:
			q = fmt.Sprintf("INSERT IGNORE INTO %s (browser, profile, timestamp, url, title, domain, visit_count) VALUES (?, ?, ?, ?, ?, ?, ?)", tbl)
		}
	}
	return q, args
}

func buildSQLInsertFlat(driver, tbl string, e models.HistoryEntry, conflict string) (string, []interface{}) {
	var q string
	args := []interface{}{
		e.Browser, e.Profile, getSQLTime(driver, e.Timestamp), e.URL, e.Title, e.Domain, e.VisitCount,
		e.VisitDuration, e.Transition, e.FromVisit, e.SegmentID, e.TypedCount,
		e.VisitType, e.Session, e.Frequency, e.Typed,
		e.RedirectSource, e.RedirectDestination, e.Origin, e.GenerationType,
		boolToInt(e.LoadSuccessful), boolToInt(e.HTTPNonGET), boolToInt(e.Synthesized),
	}

	cols := "browser, profile, timestamp, url, title, domain, visit_count, visit_duration, transition, from_visit, segment_id, typed_count, visit_type, session, frequency, typed, redirect_source, redirect_destination, origin, generation_type, load_successful, http_non_get, synthesized"

	if driver == "postgres" {
		placeholders := "$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO UPDATE SET title = EXCLUDED.title, domain = EXCLUDED.domain, visit_count = EXCLUDED.visit_count, visit_duration = EXCLUDED.visit_duration, transition = EXCLUDED.transition, from_visit = EXCLUDED.from_visit, segment_id = EXCLUDED.segment_id, typed_count = EXCLUDED.typed_count, visit_type = EXCLUDED.visit_type, session = EXCLUDED.session, frequency = EXCLUDED.frequency, typed = EXCLUDED.typed, redirect_source = EXCLUDED.redirect_source, redirect_destination = EXCLUDED.redirect_destination, origin = EXCLUDED.origin, generation_type = EXCLUDED.generation_type, load_successful = EXCLUDED.load_successful, http_non_get = EXCLUDED.http_non_get, synthesized = EXCLUDED.synthesized", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO NOTHING", tbl, cols, placeholders)
		}
	} else if driver == "sqlite" {
		placeholders := "?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO UPDATE SET title = excluded.title, domain = excluded.domain, visit_count = excluded.visit_count, visit_duration = excluded.visit_duration, transition = excluded.transition, from_visit = excluded.from_visit, segment_id = excluded.segment_id, typed_count = excluded.typed_count, visit_type = excluded.visit_type, session = excluded.session, frequency = excluded.frequency, typed = excluded.typed, redirect_source = excluded.redirect_source, redirect_destination = excluded.redirect_destination, origin = excluded.origin, generation_type = excluded.generation_type, load_successful = excluded.load_successful, http_non_get = excluded.http_non_get, synthesized = excluded.synthesized", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	} else { // mysql
		placeholders := "?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE title = VALUES(title), domain = VALUES(domain), visit_count = VALUES(visit_count), visit_duration = VALUES(visit_duration), transition = VALUES(transition), from_visit = VALUES(from_visit), segment_id = VALUES(segment_id), typed_count = VALUES(typed_count), visit_type = VALUES(visit_type), session = VALUES(session), frequency = VALUES(frequency), typed = VALUES(typed), redirect_source = VALUES(redirect_source), redirect_destination = VALUES(redirect_destination), origin = VALUES(origin), generation_type = VALUES(generation_type), load_successful = VALUES(load_successful), http_non_get = VALUES(http_non_get), synthesized = VALUES(synthesized)", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	}
	return q, args
}

func buildSQLInsertChrome(driver, tbl string, e models.HistoryEntry, conflict string) (string, []interface{}) {
	var q string
	args := []interface{}{
		e.Browser, e.Profile, getSQLTime(driver, e.Timestamp), e.URL, e.Title, e.Domain, e.VisitCount,
		e.VisitDuration, e.Transition, e.FromVisit, e.SegmentID, e.TypedCount,
	}
	cols := "browser, profile, timestamp, url, title, domain, visit_count, visit_duration, transition, from_visit, segment_id, typed_count"

	if driver == "postgres" {
		placeholders := "$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO UPDATE SET title = EXCLUDED.title, domain = EXCLUDED.domain, visit_count = EXCLUDED.visit_count, visit_duration = EXCLUDED.visit_duration, transition = EXCLUDED.transition, from_visit = EXCLUDED.from_visit, segment_id = EXCLUDED.segment_id, typed_count = EXCLUDED.typed_count", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO NOTHING", tbl, cols, placeholders)
		}
	} else if driver == "sqlite" {
		placeholders := "?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO UPDATE SET title = excluded.title, domain = excluded.domain, visit_count = excluded.visit_count, visit_duration = excluded.visit_duration, transition = excluded.transition, from_visit = excluded.from_visit, segment_id = excluded.segment_id, typed_count = excluded.typed_count", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	} else { // mysql
		placeholders := "?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE title = VALUES(title), domain = VALUES(domain), visit_count = VALUES(visit_count), visit_duration = VALUES(visit_duration), transition = VALUES(transition), from_visit = VALUES(from_visit), segment_id = VALUES(segment_id), typed_count = VALUES(typed_count)", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	}
	return q, args
}

func buildSQLInsertFirefox(driver, tbl string, e models.HistoryEntry, conflict string) (string, []interface{}) {
	var q string
	args := []interface{}{
		e.Browser, e.Profile, getSQLTime(driver, e.Timestamp), e.URL, e.Title, e.Domain, e.VisitCount,
		e.FromVisit, e.VisitType, e.Session, e.Frequency, e.Typed,
	}
	cols := "browser, profile, timestamp, url, title, domain, visit_count, from_visit, visit_type, session, frequency, typed"

	if driver == "postgres" {
		placeholders := "$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO UPDATE SET title = EXCLUDED.title, domain = EXCLUDED.domain, visit_count = EXCLUDED.visit_count, from_visit = EXCLUDED.from_visit, visit_type = EXCLUDED.visit_type, session = EXCLUDED.session, frequency = EXCLUDED.frequency, typed = EXCLUDED.typed", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO NOTHING", tbl, cols, placeholders)
		}
	} else if driver == "sqlite" {
		placeholders := "?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO UPDATE SET title = excluded.title, domain = excluded.domain, visit_count = excluded.visit_count, from_visit = excluded.from_visit, visit_type = excluded.visit_type, session = excluded.session, frequency = excluded.frequency, typed = excluded.typed", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	} else { // mysql
		placeholders := "?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE title = VALUES(title), domain = VALUES(domain), visit_count = VALUES(visit_count), from_visit = VALUES(from_visit), visit_type = VALUES(visit_type), session = VALUES(session), frequency = VALUES(frequency), typed = VALUES(typed)", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	}
	return q, args
}

func buildSQLInsertSafari(driver, tbl string, e models.HistoryEntry, conflict string) (string, []interface{}) {
	var q string
	args := []interface{}{
		e.Browser, e.Profile, getSQLTime(driver, e.Timestamp), e.URL, e.Title, e.Domain, e.VisitCount,
		e.RedirectSource, e.RedirectDestination, e.Origin, e.GenerationType,
		boolToInt(e.LoadSuccessful), boolToInt(e.HTTPNonGET), boolToInt(e.Synthesized),
	}
	cols := "browser, profile, timestamp, url, title, domain, visit_count, redirect_source, redirect_destination, origin, generation_type, load_successful, http_non_get, synthesized"

	if driver == "postgres" {
		placeholders := "$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO UPDATE SET title = EXCLUDED.title, domain = EXCLUDED.domain, visit_count = EXCLUDED.visit_count, redirect_source = EXCLUDED.redirect_source, redirect_destination = EXCLUDED.redirect_destination, origin = EXCLUDED.origin, generation_type = EXCLUDED.generation_type, load_successful = EXCLUDED.load_successful, http_non_get = EXCLUDED.http_non_get, synthesized = EXCLUDED.synthesized", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO NOTHING", tbl, cols, placeholders)
		}
	} else if driver == "sqlite" {
		placeholders := "?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (browser, profile, timestamp, url) DO UPDATE SET title = excluded.title, domain = excluded.domain, visit_count = excluded.visit_count, redirect_source = excluded.redirect_source, redirect_destination = excluded.redirect_destination, origin = excluded.origin, generation_type = excluded.generation_type, load_successful = excluded.load_successful, http_non_get = excluded.http_non_get, synthesized = excluded.synthesized", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	} else { // mysql
		placeholders := "?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE title = VALUES(title), domain = VALUES(domain), visit_count = VALUES(visit_count), redirect_source = VALUES(redirect_source), redirect_destination = VALUES(redirect_destination), origin = VALUES(origin), generation_type = VALUES(generation_type), load_successful = VALUES(load_successful), http_non_get = VALUES(http_non_get), synthesized = VALUES(synthesized)", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	}
	return q, args
}

func buildSQLInsertChildChrome(driver, tbl string, historyID int64, e models.HistoryEntry, conflict string) (string, []interface{}) {
	var q string
	args := []interface{}{
		historyID, e.VisitDuration, e.Transition, e.FromVisit, e.SegmentID, e.TypedCount,
	}
	cols := "history_id, visit_duration, transition, from_visit, segment_id, typed_count"

	if driver == "postgres" {
		placeholders := "$1, $2, $3, $4, $5, $6"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (history_id) DO UPDATE SET visit_duration = EXCLUDED.visit_duration, transition = EXCLUDED.transition, from_visit = EXCLUDED.from_visit, segment_id = EXCLUDED.segment_id, typed_count = EXCLUDED.typed_count", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (history_id) DO NOTHING", tbl, cols, placeholders)
		}
	} else if driver == "sqlite" {
		placeholders := "?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (history_id) DO UPDATE SET visit_duration = excluded.visit_duration, transition = excluded.transition, from_visit = excluded.from_visit, segment_id = excluded.segment_id, typed_count = excluded.typed_count", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	} else { // mysql
		placeholders := "?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE visit_duration = VALUES(visit_duration), transition = VALUES(transition), from_visit = VALUES(from_visit), segment_id = VALUES(segment_id), typed_count = VALUES(typed_count)", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	}
	return q, args
}

func buildSQLInsertChildFirefox(driver, tbl string, historyID int64, e models.HistoryEntry, conflict string) (string, []interface{}) {
	var q string
	args := []interface{}{
		historyID, e.FromVisit, e.VisitType, e.Session, e.Frequency, e.Typed,
	}
	cols := "history_id, from_visit, visit_type, session, frequency, typed"

	if driver == "postgres" {
		placeholders := "$1, $2, $3, $4, $5, $6"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (history_id) DO UPDATE SET from_visit = EXCLUDED.from_visit, visit_type = EXCLUDED.visit_type, session = EXCLUDED.session, frequency = EXCLUDED.frequency, typed = EXCLUDED.typed", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (history_id) DO NOTHING", tbl, cols, placeholders)
		}
	} else if driver == "sqlite" {
		placeholders := "?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (history_id) DO UPDATE SET from_visit = excluded.from_visit, visit_type = excluded.visit_type, session = excluded.session, frequency = excluded.frequency, typed = excluded.typed", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	} else { // mysql
		placeholders := "?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE from_visit = VALUES(from_visit), visit_type = VALUES(visit_type), session = VALUES(session), frequency = VALUES(frequency), typed = VALUES(typed)", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	}
	return q, args
}

func buildSQLInsertChildSafari(driver, tbl string, historyID int64, e models.HistoryEntry, conflict string) (string, []interface{}) {
	var q string
	args := []interface{}{
		historyID, e.RedirectSource, e.RedirectDestination, e.Origin, e.GenerationType,
		boolToInt(e.LoadSuccessful), boolToInt(e.HTTPNonGET), boolToInt(e.Synthesized),
	}
	cols := "history_id, redirect_source, redirect_destination, origin, generation_type, load_successful, http_non_get, synthesized"

	if driver == "postgres" {
		placeholders := "$1, $2, $3, $4, $5, $6, $7, $8"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (history_id) DO UPDATE SET redirect_source = EXCLUDED.redirect_source, redirect_destination = EXCLUDED.redirect_destination, origin = EXCLUDED.origin, generation_type = EXCLUDED.generation_type, load_successful = EXCLUDED.load_successful, http_non_get = EXCLUDED.http_non_get, synthesized = EXCLUDED.synthesized", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (history_id) DO NOTHING", tbl, cols, placeholders)
		}
	} else if driver == "sqlite" {
		placeholders := "?, ?, ?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (history_id) DO UPDATE SET redirect_source = excluded.redirect_source, redirect_destination = excluded.redirect_destination, origin = excluded.origin, generation_type = excluded.generation_type, load_successful = excluded.load_successful, http_non_get = excluded.http_non_get, synthesized = excluded.synthesized", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	} else { // mysql
		placeholders := "?, ?, ?, ?, ?, ?, ?, ?"
		switch conflict {
		case "replace":
			q = fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE redirect_source = VALUES(redirect_source), redirect_destination = VALUES(redirect_destination), origin = VALUES(origin), generation_type = VALUES(generation_type), load_successful = VALUES(load_successful), http_non_get = VALUES(http_non_get), synthesized = VALUES(synthesized)", tbl, cols, placeholders)
		default:
			q = fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tbl, cols, placeholders)
		}
	}
	return q, args
}


func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func parseMySQLDSN(connectStr string) (string, error) {
	u, err := url.Parse(connectStr)
	if err != nil {
		return "", err
	}

	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Host

	if !strings.Contains(host, ":") {
		host = host + ":3306"
	}

	dbName := strings.TrimPrefix(u.Path, "/")

	var userPass string
	if user != "" {
		if pass != "" {
			userPass = fmt.Sprintf("%s:%s@", user, pass)
		} else {
			userPass = fmt.Sprintf("%s@", user)
		}
	}

	dsn := fmt.Sprintf("%stcp(%s)/%s?parseTime=true", userPass, host, dbName)
	if u.RawQuery != "" {
		dsn += "&" + u.RawQuery
	}
	return dsn, nil
}
