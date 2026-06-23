package database

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	_ "modernc.org/sqlite"
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

	db, err := sql.Open("sqlite", tempDB)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var query string
	var args []interface{}

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

	selectFields := fmt.Sprintf(`
		h.visit_date,
		p.url,
		%s,
		%s,
		%s,
		%s,
		%s,
		%s,
		%s
	`, titleExpr, visitCountExpr, fromVisitExpr, visitTypeExpr, sessionExpr, frequencyExpr, typedExpr)

	if !startDate.IsZero() || !endDate.IsZero() {
		query = "SELECT " + selectFields + `
		FROM moz_historyvisits h
		JOIN moz_places p ON h.place_id = p.id
		WHERE h.visit_date > 0
		`

		if !startDate.IsZero() {
			firefoxStart := startDate.Unix() * 1000000
			query += ` AND h.visit_date >= ?`
			args = append(args, firefoxStart)
		}

		if !endDate.IsZero() {
			endTimestamp := endDate.Unix()
			if endDate.Hour() == 0 && endDate.Minute() == 0 && endDate.Second() == 0 {
				endTimestamp += 86400
			}
			firefoxEnd := endTimestamp * 1000000
			query += ` AND h.visit_date < ?`
			args = append(args, firefoxEnd)
		}

		query += ` ORDER BY h.visit_date DESC`
	} else {
		query = "SELECT " + selectFields + `
		FROM moz_historyvisits h
		JOIN moz_places p ON h.place_id = p.id
		WHERE h.visit_date > 0
		ORDER BY h.visit_date DESC
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
		var firefoxTime int64
		var url, title string
		var visitCount int
		var fromVisit, visitType, session, frequency, typed int64

		if err := rows.Scan(&firefoxTime, &url, &title, &visitCount, &fromVisit, &visitType, &session, &frequency, &typed); err != nil {
			return nil, err
		}

		timestamp := ConvertFirefoxTimestamp(firefoxTime)
		if timestamp.IsZero() {
			continue
		}

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
		})
	}

	return entries, rows.Err()
}
