package models

import "time"

// HistoryEntry represents a single browser history entry containing standard and raw metadata
type HistoryEntry struct {
	Timestamp  time.Time `json:"timestamp"`
	URL        string    `json:"url"`
	Title      string    `json:"title"`
	VisitCount int       `json:"visit_count"`
	Domain     string    `json:"domain"`
	Browser    string    `json:"browser"`
	Profile    string    `json:"profile"`

	// Chrome-specific fields
	VisitDuration int64 `json:"visit_duration,omitempty"`
	Transition    int64 `json:"transition,omitempty"`
	FromVisit     int64 `json:"from_visit,omitempty"`
	SegmentID     int64 `json:"segment_id,omitempty"`
	TypedCount    int64 `json:"typed_count,omitempty"`

	// Firefox-specific fields
	VisitType int64 `json:"visit_type,omitempty"`
	Session   int64 `json:"session,omitempty"`
	Frequency int64 `json:"frequency,omitempty"`
	Typed     int64 `json:"typed,omitempty"`

	// Safari-specific fields
	RedirectSource      int64 `json:"redirect_source,omitempty"`
	RedirectDestination int64 `json:"redirect_destination,omitempty"`
	Origin              int64 `json:"origin,omitempty"`
	GenerationType      int64 `json:"generation_type,omitempty"`
	LoadSuccessful      bool  `json:"load_successful,omitempty"`
	HTTPNonGET          bool  `json:"http_non_get,omitempty"`
	Synthesized         bool  `json:"synthesized,omitempty"`
}

// HistoryReport represents a collection of history entries for a specific time period
type HistoryReport struct {
	Browser      string          `json:"browser"`
	StartDate    time.Time       `json:"start_date"`
	EndDate      time.Time       `json:"end_date"`
	Timezone     string          `json:"timezone"`
	TotalEntries int             `json:"total_entries"`
	Entries      []HistoryEntry  `json:"entries"`
}

// BrowserType represents the type of browser
type BrowserType string

const (
	BrowserChrome    BrowserType = "chrome"
	BrowserChromium  BrowserType = "chromium"
	BrowserEdge      BrowserType = "edge"
	BrowserFirefox   BrowserType = "firefox"
	BrowserSafari    BrowserType = "safari"
	BrowserUnknown   BrowserType = "unknown"
)

func (b BrowserType) String() string {
	return string(b)
}
