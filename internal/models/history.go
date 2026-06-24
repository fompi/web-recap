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

	// Enriched URL fields
	Scheme      string `json:"scheme,omitempty"`
	Username    string `json:"username,omitempty"`
	FQDN        string `json:"fqdn,omitempty"`
	DomainName  string `json:"domain_name,omitempty"`
	Subdomain   string `json:"subdomain,omitempty"`
	TLD         string `json:"tld,omitempty"`
	Port        string `json:"port,omitempty"`
	Path        string `json:"path,omitempty"`
	QueryParams string `json:"query_params,omitempty"`

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
	LoadSuccessful      *bool `json:"load_successful,omitempty"`
	HTTPNonGET          bool  `json:"http_non_get,omitempty"`
	Synthesized         bool  `json:"synthesized,omitempty"`

	// Normalized cross-browser fields
	ReferrerURL    string `json:"referrer_url,omitempty"`
	VisitTypeLabel string `json:"visit_type_label,omitempty"` // link|typed|bookmark|reload|redirect|download|other
	Source         string `json:"source,omitempty"`           // local|synced
	Hidden         bool   `json:"hidden,omitempty"`
}




