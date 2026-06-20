package database

import (
	"database/sql"
	"io"
	"os"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	_ "modernc.org/sqlite"
)

// FirefoxHandler handles Firefox browser history
type FirefoxHandler struct {
	dbPath  string
	profile string
}

// NewFirefoxHandler creates a new Firefox history handler
func NewFirefoxHandler(dbPath string, profile string) *FirefoxHandler {
	return &FirefoxHandler{
		dbPath:  dbPath,
		profile: profile,
	}
}

// GetHistory retrieves history entries from Firefox
func (h *FirefoxHandler) GetHistory(startDate, endDate time.Time) ([]models.HistoryEntry, error) {
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
		h.visit_date,
		p.url,
		COALESCE(p.title, '') as title,
		p.visit_count,
		COALESCE(h.from_visit, 0) as from_visit,
		COALESCE(h.visit_type, 0) as visit_type,
		COALESCE(h.session, 0) as session,
		COALESCE(p.frequency, 0) as frequency,
		COALESCE(p.typed, 0) as typed
	`

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
			Browser:    "firefox",
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

// copyDatabase copies the Firefox database to a temporary file
func (h *FirefoxHandler) copyDatabase() (string, error) {
	src, err := os.Open(h.dbPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.CreateTemp("", "web-recap-firefox-*.db")
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
