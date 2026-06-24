package database

import (
	"sort"
	"time"

	"github.com/rzolkos/web-recap/internal/browser"
	"github.com/rzolkos/web-recap/internal/models"
)

// HistoryQuerier defines the interface for querying browser history
type HistoryQuerier interface {
	GetHistory(startDate, endDate time.Time, validOnly bool) ([]models.HistoryEntry, error)
}

// NewQuerier creates a new history querier for the given browser
func NewQuerier(b *browser.Browser) (HistoryQuerier, error) {
	switch b.Type {
	case browser.Chrome, browser.Chromium, browser.Edge, browser.Brave:
		return NewChromeHandler(b.Path, b.Name, b.Profile), nil
	case browser.Firefox:
		return NewFirefoxHandler(b.Path, b.Name, b.Profile), nil
	case browser.Safari:
		return NewSafariHandler(b.Path, b.Name, b.Profile), nil
	default:
		return nil, ErrUnsupportedBrowser
	}
}

// Query retrieves history entries from a specific browser
func Query(b *browser.Browser, startDate, endDate time.Time, validOnly bool) ([]models.HistoryEntry, error) {
	querier, err := NewQuerier(b)
	if err != nil {
		return nil, err
	}

	entries, err := querier.GetHistory(startDate, endDate, validOnly)
	if err != nil {
		return nil, err
	}

	return entries, nil
}


// SortEntriesDescending sorts a slice of HistoryEntry objects descending by timestamp
func SortEntriesDescending(entries []models.HistoryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
}

