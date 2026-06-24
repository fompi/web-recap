package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
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

	hasCol := func(table, col string) bool {
		found, _ := HasColumn(db, table, col)
		return found
	}

	// Bug fix #1: filter subframe-only URLs that Chrome marks as hidden.
	// Without this filter the caller receives internal redirect artifacts that
	// never correspond to a page the user actually visited.
	hiddenFilter := ""
	if hasCol("urls", "hidden") {
		hiddenFilter = " AND u.hidden = 0"
	}

	// Bug fix #3: resolve the referrer URL by self-joining visits+urls on from_visit.
	// Previously only the raw from_visit integer (a visit row ID) was exposed, which
	// callers cannot use without re-querying the database themselves.
	referrerJoin := `
		LEFT JOIN visits      pv    ON pv.id    = v.from_visit
		LEFT JOIN urls        ref_u ON ref_u.id = pv.url`
	referrerURLExpr := "COALESCE(ref_u.url, '') AS referrer_url"

	// Bug fix #5: capture whether the visit originated from sync or local browsing.
	// The visit_source table is present in Chromium ≥ M29; fall back to 'local'
	// for older profiles or Chromium forks that omit it.
	sourceJoin := ""
	sourceExpr := "'local' AS source"
	if HasTable(db, "visit_source") {
		sourceJoin = "LEFT JOIN visit_source vs ON vs.id = v.id"
		// source = 0 means SYNCED in Chrome's VisitSource enum; anything else is local
		sourceExpr = "CASE WHEN vs.source = 0 THEN 'synced' ELSE 'local' END AS source"
	}

	selectFields := fmt.Sprintf(`
		v.visit_time,
		u.url,
		COALESCE(u.title, '') AS title,
		u.visit_count,
		COALESCE(v.visit_duration, 0) AS visit_duration,
		COALESCE(v.transition, 0) AS transition,
		COALESCE(v.from_visit, 0) AS from_visit,
		COALESCE(v.segment_id, 0) AS segment_id,
		COALESCE(u.typed_count, 0) AS typed_count,
		%s,
		%s
	`, referrerURLExpr, sourceExpr)

	fromClause := fmt.Sprintf(`
		FROM visits v
		JOIN urls u ON v.url = u.id
		%s
		%s
		WHERE v.visit_time > 0%s`,
		referrerJoin, sourceJoin, hiddenFilter)

	var query string
	var args []interface{}

	if !startDate.IsZero() || !endDate.IsZero() {
		query = "SELECT " + selectFields + fromClause

		if !startDate.IsZero() {
			chromeStart := (startDate.Unix() + 11644473600) * 1000000
			query += ` AND v.visit_time >= ?`
			args = append(args, chromeStart)
		}

		if !endDate.IsZero() {
			endTimestamp := endDate.Unix()
			if endDate.Hour() == 0 && endDate.Minute() == 0 && endDate.Second() == 0 && endDate.Nanosecond() == 0 {
				endTimestamp += 86400
			}
			chromeEnd := (endTimestamp + 11644473600) * 1000000
			query += ` AND v.visit_time < ?`
			args = append(args, chromeEnd)
		}

		query += ` ORDER BY v.visit_time DESC`
	} else {
		query = "SELECT " + selectFields + fromClause + ` ORDER BY v.visit_time DESC LIMIT 10000`
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
		var referrerURL, source string

		if err := rows.Scan(
			&chromeTime, &url, &title, &visitCount,
			&visitDuration, &transition, &fromVisit, &segmentID, &typedCount,
			&referrerURL, &source,
		); err != nil {
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
			ReferrerURL:   referrerURL,
			Source:        source,
			// Bug fix #4: decode the transition bitmask to a normalized string so
			// callers can compare visit types across browsers without knowing
			// Chrome's internal encoding.
			VisitTypeLabel: DecodeTransition(transition),
		})
	}

	return entries, rows.Err()
}
