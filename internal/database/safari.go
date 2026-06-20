package database

import (
	"database/sql"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	_ "modernc.org/sqlite"
)

// SafariHandler handles Safari browser history
type SafariHandler struct {
	dbPath  string
	profile string
}

// NewSafariHandler creates a new Safari history handler
func NewSafariHandler(dbPath string, profile string) *SafariHandler {
	return &SafariHandler{
		dbPath:  dbPath,
		profile: profile,
	}
}

// GetHistory retrieves history entries from Safari
func (h *SafariHandler) GetHistory(startDate, endDate time.Time) ([]models.HistoryEntry, error) {
	if runtime.GOOS != "darwin" {
		return nil, ErrSafariNotAvailable
	}

	// Copy database to temp location to avoid locking issues
	tempDB, err := h.copyDatabase()
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempDB)

	db, err := sql.Open("sqlite", tempDB)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var query string
	var args []interface{}

	selectFields := `
		hv.visit_time,
		hi.url,
		COALESCE(hv.title, hi.url) as title,
		hi.visit_count,
		COALESCE(hv.redirect_source, 0) as redirect_source,
		COALESCE(hv.redirect_destination, 0) as redirect_destination,
		COALESCE(hv.origin, 0) as origin,
		COALESCE(hv.generation_type, 0) as generation_type,
		COALESCE(hv.load_successful, 1) as load_successful,
		COALESCE(hv.http_non_get, 0) as http_non_get,
		COALESCE(hv.synthesized, 0) as synthesized
	`

	if !startDate.IsZero() || !endDate.IsZero() {
		query = "SELECT " + selectFields + `
		FROM history_visits hv
		JOIN history_items hi ON hv.history_item = hi.id
		WHERE hv.visit_time > 0
		`

		if !startDate.IsZero() {
			const safariEpochDiff = 978307200
			safariStart := startDate.Unix() - safariEpochDiff
			query += ` AND hv.visit_time >= ?`
			args = append(args, safariStart)
		}

		if !endDate.IsZero() {
			endTimestamp := endDate.Unix()
			if endDate.Hour() == 0 && endDate.Minute() == 0 && endDate.Second() == 0 {
				endTimestamp += 86400
			}
			const safariEpochDiff = 978307200
			safariEnd := endTimestamp - safariEpochDiff
			query += ` AND hv.visit_time < ?`
			args = append(args, safariEnd)
		}

		query += ` ORDER BY hv.visit_time DESC`
	} else {
		query = "SELECT " + selectFields + `
		FROM history_visits hv
		JOIN history_items hi ON hv.history_item = hi.id
		WHERE hv.visit_time > 0
		ORDER BY hv.visit_time DESC
		LIMIT 10000
		`
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.HistoryEntry

	for rows.Next() {
		var safariTime float64
		var url, title string
		var visitCount int
		var redirectSource, redirectDestination, origin, generationType int64
		var loadSuccessful, httpNonGET, synthesized int

		if err := rows.Scan(&safariTime, &url, &title, &visitCount, &redirectSource, &redirectDestination, &origin, &generationType, &loadSuccessful, &httpNonGET, &synthesized); err != nil {
			return nil, err
		}

		timestamp := ConvertSafariTimestamp(safariTime)
		if timestamp.IsZero() {
			continue
		}

		entries = append(entries, models.HistoryEntry{
			Timestamp:           timestamp,
			URL:                 url,
			Title:               title,
			VisitCount:          visitCount,
			Domain:              ExtractDomain(url),
			Browser:             "safari",
			Profile:             h.profile,
			RedirectSource:      redirectSource,
			RedirectDestination: redirectDestination,
			Origin:              origin,
			GenerationType:      generationType,
			LoadSuccessful:      loadSuccessful != 0,
			HTTPNonGET:          httpNonGET != 0,
			Synthesized:         synthesized != 0,
		})
	}

	return entries, rows.Err()
}

// copyDatabase copies the Safari database to a temporary file
func (h *SafariHandler) copyDatabase() (string, error) {
	src, err := os.Open(h.dbPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.CreateTemp("", "web-recap-safari-*.db")
	if err != nil {
		return "", err
	}
	tmpFile := dst.Name()
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(tmpFile)
		return "", err
	}

	return tmpFile, nil
}
