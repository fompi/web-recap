package database

import (
	"database/sql"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	_ "modernc.org/sqlite"
)

// ChromeHandler handles Chrome/Chromium/Edge browser history
type ChromeHandler struct {
	dbPath      string
	browserName string
	profile     string
}

// NewChromeHandler creates a new Chrome history handler
func NewChromeHandler(dbPath string, browserName string, profile string) *ChromeHandler {
	if browserName == "" {
		browserName = "chrome"
	}
	return &ChromeHandler{
		dbPath:      dbPath,
		browserName: browserName,
		profile:     profile,
	}
}

// GetHistory retrieves history entries from Chrome
func (h *ChromeHandler) GetHistory(startDate, endDate time.Time) ([]models.HistoryEntry, error) {
	// Copy database to temp location to avoid locking issues
	tempDB, cleanup, err := CopyDatabaseWithWAL(h.dbPath, "web-recap-chrome")
	if err != nil {
		return nil, err
	}
	defer cleanup()

	db, _ := sql.Open("sqlite", tempDB)
	defer db.Close()

	var query string
	var args []interface{}

	selectFields := `
		v.visit_time,
		u.url,
		COALESCE(u.title, '') as title,
		u.visit_count,
		COALESCE(v.visit_duration, 0) as visit_duration,
		COALESCE(v.transition, 0) as transition,
		COALESCE(v.from_visit, 0) as from_visit,
		COALESCE(v.segment_id, 0) as segment_id,
		COALESCE(u.typed_count, 0) as typed_count
	`

	if !startDate.IsZero() || !endDate.IsZero() {
		query = "SELECT " + selectFields + `
		FROM visits v
		JOIN urls u ON v.url = u.id
		WHERE v.visit_time > 0
		`

		if !startDate.IsZero() {
			chromeStart := (startDate.Unix() + 11644473600) * 1000000
			query += ` AND v.visit_time >= ?`
			args = append(args, chromeStart)
		}

		if !endDate.IsZero() {
			endTimestamp := endDate.Unix()
			if endDate.Hour() == 0 && endDate.Minute() == 0 && endDate.Second() == 0 {
				endTimestamp += 86400
			}
			chromeEnd := (endTimestamp + 11644473600) * 1000000
			query += ` AND v.visit_time < ?`
			args = append(args, chromeEnd)
		}

		query += ` ORDER BY v.visit_time DESC`
	} else {
		query = "SELECT " + selectFields + `
		FROM visits v
		JOIN urls u ON v.url = u.id
		WHERE v.visit_time > 0
		ORDER BY v.visit_time DESC
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
		var chromeTime int64
		var url, title string
		var visitCount int
		var visitDuration, transition, fromVisit, segmentID, typedCount int64

		if err := rows.Scan(&chromeTime, &url, &title, &visitCount, &visitDuration, &transition, &fromVisit, &segmentID, &typedCount); err != nil {
			return nil, err
		}

		timestamp := ConvertChromeTimestamp(chromeTime)

		entries = append(entries, models.HistoryEntry{
			Timestamp:     timestamp,
			URL:           url,
			Title:         title,
			VisitCount:    visitCount,
			Domain:        ExtractDomain(url),
			Browser:       h.browserName,
			Profile:       h.profile,
			VisitDuration: visitDuration,
			Transition:    transition,
			FromVisit:     fromVisit,
			SegmentID:     segmentID,
			TypedCount:    typedCount,
		})
	}

	return entries, rows.Err()
}
