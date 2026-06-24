package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
)

// FirefoxHandler handles Firefox browser history
type FirefoxHandler struct {
	dbPath      string
	browserName string
	profile     string
}

// NewFirefoxHandler creates a new Firefox history handler
func NewFirefoxHandler(dbPath string, browserName string, profile string) *FirefoxHandler {
	if browserName == "" {
		browserName = "firefox"
	}
	return &FirefoxHandler{
		dbPath:      dbPath,
		browserName: browserName,
		profile:     profile,
	}
}

// GetHistory retrieves history entries from Firefox
func (h *FirefoxHandler) GetHistory(startDate, endDate time.Time) ([]models.HistoryEntry, error) {
	// Copy database to temp location to avoid locking issues
	tempDB, cleanup, err := CopyDatabaseWithWAL(h.dbPath, "web-recap-firefox")
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

	titleExpr := "'' as title"
	if hasCol("moz_places", "title") {
		titleExpr = "COALESCE(p.title, '') as title"
	}

	visitCountExpr := "0 as visit_count"
	if hasCol("moz_places", "visit_count") {
		visitCountExpr = "p.visit_count"
	}

	fromVisitExpr := "0 as from_visit"
	if hasCol("moz_historyvisits", "from_visit") {
		fromVisitExpr = "COALESCE(h.from_visit, 0) as from_visit"
	}

	visitTypeExpr := "0 as visit_type"
	if hasCol("moz_historyvisits", "visit_type") {
		visitTypeExpr = "COALESCE(h.visit_type, 0) as visit_type"
	}

	sessionExpr := "0 as session"
	if hasCol("moz_historyvisits", "session") {
		sessionExpr = "COALESCE(h.session, 0) as session"
	}

	frequencyExpr := "0 as frequency"
	if hasCol("moz_places", "frecency") {
		frequencyExpr = "COALESCE(p.frecency, 0) as frequency"
	} else if hasCol("moz_places", "frequency") {
		frequencyExpr = "COALESCE(p.frequency, 0) as frequency"
	}

	typedExpr := "0 as typed"
	if hasCol("moz_places", "typed") {
		typedExpr = "COALESCE(p.typed, 0) as typed"
	}

	// Bug fix #1: filter subframe-only entries that Firefox marks as hidden.
	// moz_places.hidden=1 rows are internal redirect hops and sub-frame URLs
	// that never represent a top-level page navigation by the user.
	hiddenFilter := ""
	if hasCol("moz_places", "hidden") {
		hiddenFilter = " AND p.hidden = 0"
	}

	// Bug fix #3: resolve the referrer URL by self-joining moz_historyvisits on
	// from_visit and then joining moz_places to get the URL string.
	// Previously only the raw from_visit integer was exposed, which callers cannot
	// use without re-querying the database.
	referrerJoin := ""
	referrerURLExpr := "'' AS referrer_url"
	if hasCol("moz_historyvisits", "from_visit") {
		referrerJoin = `
		LEFT JOIN moz_historyvisits pv    ON pv.id    = h.from_visit
		LEFT JOIN moz_places        ref_p ON ref_p.id = pv.place_id`
		referrerURLExpr = "COALESCE(ref_p.url, '') AS referrer_url"
	}

	selectFields := fmt.Sprintf(`
		h.visit_date,
		p.url,
		%s,
		%s,
		%s,
		%s,
		%s,
		%s,
		%s,
		%s
	`, titleExpr, visitCountExpr,
		fromVisitExpr, visitTypeExpr,
		sessionExpr, frequencyExpr, typedExpr,
		referrerURLExpr)

	fromClause := fmt.Sprintf(`
		FROM moz_historyvisits h
		JOIN moz_places p ON h.place_id = p.id
		%s
		WHERE h.visit_date > 0%s`,
		referrerJoin, hiddenFilter)

	var query string
	var args []interface{}

	if !startDate.IsZero() || !endDate.IsZero() {
		query = "SELECT " + selectFields + fromClause

		if !startDate.IsZero() {
			firefoxStart := startDate.Unix() * 1000000
			query += ` AND h.visit_date >= ?`
			args = append(args, firefoxStart)
		}

		if !endDate.IsZero() {
			endTimestamp := endDate.Unix()
			if endDate.Hour() == 0 && endDate.Minute() == 0 && endDate.Second() == 0 && endDate.Nanosecond() == 0 {
				endTimestamp += 86400
			}
			firefoxEnd := endTimestamp * 1000000
			query += ` AND h.visit_date < ?`
			args = append(args, firefoxEnd)
		}

		query += ` ORDER BY h.visit_date DESC`
	} else {
		query = "SELECT " + selectFields + fromClause + ` ORDER BY h.visit_date DESC LIMIT 10000`
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.HistoryEntry

	for rows.Next() {
		var firefoxTime int64
		var url, title string
		var visitCount int
		var fromVisit, visitType, session, frequency, typed int64
		var referrerURL string

		if err := rows.Scan(
			&firefoxTime, &url, &title, &visitCount,
			&fromVisit, &visitType, &session, &frequency, &typed,
			&referrerURL,
		); err != nil {
			return nil, err
		}

		timestamp := ConvertFirefoxTimestamp(firefoxTime)

		entries = append(entries, models.HistoryEntry{
			Timestamp:  timestamp,
			URL:        url,
			Title:      title,
			VisitCount: visitCount,
			Domain:     ExtractDomain(url),
			Browser:    h.browserName,
			Profile:    h.profile,
			FromVisit:  fromVisit,
			VisitType:  visitType,
			Session:    session,
			Frequency:  frequency,
			Typed:      typed,
			ReferrerURL: referrerURL,
			// Bug fix #4: decode Firefox's integer visit_type enum to a normalized
			// string that callers can compare directly against Chrome and Safari entries.
			VisitTypeLabel: DecodeFirefoxVisitType(visitType),
		})
	}

	return entries, rows.Err()
}
