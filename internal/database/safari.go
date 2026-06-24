package database

import (
	"database/sql"
	"fmt"
	"runtime"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	_ "modernc.org/sqlite"
)

// SafariHandler handles Safari browser history
type SafariHandler struct {
	dbPath      string
	browserName string
	profile     string
}

var isDarwinOS = (runtime.GOOS == "darwin")

// NewSafariHandler creates a new Safari history handler
func NewSafariHandler(dbPath string, browserName string, profile string) *SafariHandler {
	if browserName == "" {
		browserName = "safari"
	}
	return &SafariHandler{
		dbPath:      dbPath,
		browserName: browserName,
		profile:     profile,
	}
}

// GetHistory retrieves history entries from Safari
func (h *SafariHandler) GetHistory(startDate, endDate time.Time) ([]models.HistoryEntry, error) {
	if !isDarwinOS {
		return nil, ErrSafariNotAvailable
	}

	// Copy database to temp location to avoid locking issues
	tempDB, cleanup, err := CopyDatabaseWithWAL(h.dbPath, "web-recap-safari")
	if err != nil {
		return nil, err
	}
	defer cleanup()

	db, _ := sql.Open("sqlite", tempDB)
	defer db.Close()

	var query string
	var args []interface{}

	hasCol := func(table, col string) bool {
		found, _ := HasColumn(db, table, col)
		return found
	}

	titleExpr := "hi.url as title"
	if hasCol("history_visits", "title") {
		titleExpr = "COALESCE(hv.title, hi.url) as title"
	} else if hasCol("history_items", "title") {
		titleExpr = "COALESCE(hi.title, hi.url) as title"
	}

	redirectSourceExpr := "0 as redirect_source"
	if hasCol("history_visits", "redirect_source") {
		redirectSourceExpr = "COALESCE(hv.redirect_source, 0) as redirect_source"
	}

	redirectDestExpr := "0 as redirect_destination"
	if hasCol("history_visits", "redirect_destination") {
		redirectDestExpr = "COALESCE(hv.redirect_destination, 0) as redirect_destination"
	}

	originExpr := "0 as origin"
	if hasCol("history_visits", "origin") {
		originExpr = "COALESCE(hv.origin, 0) as origin"
	}

	genTypeExpr := "0 as generation_type"
	if hasCol("history_visits", "generation_type") {
		genTypeExpr = "COALESCE(hv.generation_type, 0) as generation_type"
	}

	loadSuccExpr := "1 as load_successful"
	if hasCol("history_visits", "load_successful") {
		loadSuccExpr = "COALESCE(hv.load_successful, 1) as load_successful"
	}

	httpNonGetExpr := "0 as http_non_get"
	if hasCol("history_visits", "http_non_get") {
		httpNonGetExpr = "COALESCE(hv.http_non_get, 0) as http_non_get"
	}

	synthesizedExpr := "0 as synthesized"
	if hasCol("history_visits", "synthesized") {
		synthesizedExpr = "COALESCE(hv.synthesized, 0) as synthesized"
	}

	selectFields := fmt.Sprintf(`
		hv.visit_time,
		hi.url,
		%s,
		hi.visit_count,
		%s,
		%s,
		%s,
		%s,
		%s,
		%s,
		%s
	`, titleExpr, redirectSourceExpr, redirectDestExpr, originExpr, genTypeExpr, loadSuccExpr, httpNonGetExpr, synthesizedExpr)

	if !startDate.IsZero() || !endDate.IsZero() {
		query = "SELECT " + selectFields + `
		FROM history_visits hv
		JOIN history_items hi ON hv.history_item = hi.id
		WHERE hv.visit_time > 0
		`

		if !startDate.IsZero() {
			safariStart := startDate.Unix() - safariEpochDiff
			query += ` AND hv.visit_time >= ?`
			args = append(args, safariStart)
		}

		if !endDate.IsZero() {
			endTimestamp := endDate.Unix()
			if endDate.Hour() == 0 && endDate.Minute() == 0 && endDate.Second() == 0 && endDate.Nanosecond() == 0 {
				endTimestamp += 86400
			}
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

		entries = append(entries, models.HistoryEntry{
			Timestamp:           timestamp,
			URL:                 url,
			Title:               title,
			VisitCount:          visitCount,
			Domain:              ExtractDomain(url),
			Browser:             h.browserName,
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
