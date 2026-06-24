package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
)

func TestFormatStats_Empty(t *testing.T) {
	var buf bytes.Buffer
	err := FormatStats(&buf, nil, time.Time{}, time.Time{}, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No history entries found in the specified range.") {
		t.Errorf("expected no entries message, got: %q", buf.String())
	}
}

func TestFormatStats_Comprehensive(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("failed to load timezone: %v", err)
	}

	fromTime := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	toTime := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)

	entries := []models.HistoryEntry{
		// 1. Chrome entry with standard duration
		{
			Browser:       "Chrome",
			Profile:       "Default",
			Timestamp:     time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC),
			Domain:        "google.com",
			URL:           "https://google.com",
			Title:         "Google Search",
			VisitCount:    1,
			VisitDuration: 60 * 1000000, // 1 minute
			Transition:    0,           // LINK (Clicked Link)
		},
		// 2. Chrome entry with capped duration
		{
			Browser:       "google chrome",
			Profile:       "Profile 1",
			Timestamp:     time.Date(2026, 6, 20, 10, 15, 0, 0, time.UTC), // <= 30 mins gap
			Domain:        "youtube.com",
			URL:           "https://youtube.com/watch",
			Title:         "YouTube video",
			VisitCount:    2,
			VisitDuration: 6 * 3600 * 1000000, // 6 hours (should be capped at 4 hours)
			Transition:    1,                  // TYPED (Typed / Direct)
		},
		// 3. Chrome entry with bookmark transition
		{
			Browser:       "chromium",
			Profile:       "Default",
			Timestamp:     time.Date(2026, 6, 20, 11, 0, 0, 0, time.UTC), // > 30 mins gap
			Domain:        "github.com",
			URL:           "https://github.com",
			Title:         "GitHub",
			VisitCount:    5,
			VisitDuration: 10 * 1000000, // 10 seconds
			Transition:    2,            // AUTO_BOOKMARK (Bookmarks)
		},
		// 4. Chrome entry with reload transition
		{
			Browser:       "edge",
			Profile:       "Default",
			Timestamp:     time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
			Domain:        "reddit.com",
			URL:           "https://reddit.com",
			Title:         "Reddit",
			VisitCount:    1,
			VisitDuration: 5 * 1000000,
			Transition:    8, // RELOAD
		},
		// 5. Chrome entry with redirect transition
		{
			Browser:       "brave",
			Profile:       "Default",
			Timestamp:     time.Date(2026, 6, 20, 13, 0, 0, 0, time.UTC),
			Domain:        "twitter.com",
			URL:           "https://twitter.com",
			Title:         "Twitter",
			VisitCount:    1,
			VisitDuration: 2 * 1000000,
			Transition:    0x10000000, // Client redirect flag
		},
		// 6. Firefox entry with transition link
		{
			Browser:    "Firefox",
			Profile:    "default-release",
			Timestamp:  time.Date(2026, 6, 20, 14, 0, 0, 0, time.UTC),
			Domain:     "mozilla.org",
			URL:        "https://mozilla.org",
			Title:      "Mozilla",
			VisitCount: 1,
			VisitType:  1, // LINK
		},
		// 7. Firefox entry with transition typed
		{
			Browser:    "Firefox",
			Profile:    "default-release",
			Timestamp:  time.Date(2026, 6, 20, 15, 0, 0, 0, time.UTC),
			Domain:     "wikipedia.org",
			URL:        "https://wikipedia.org",
			Title:      "Wikipedia",
			VisitCount: 1,
			VisitType:  2, // TYPED
		},
		// 8. Firefox entry with transition bookmark
		{
			Browser:    "Firefox",
			Profile:    "default-release",
			Timestamp:  time.Date(2026, 6, 20, 16, 0, 0, 0, time.UTC),
			Domain:     "amazon.com",
			URL:        "https://amazon.com",
			Title:      "Amazon",
			VisitCount: 1,
			VisitType:  3, // BOOKMARK
		},
		// 9. Firefox entry with transition redirect
		{
			Browser:    "Firefox",
			Profile:    "default-release",
			Timestamp:  time.Date(2026, 6, 20, 17, 0, 0, 0, time.UTC),
			Domain:     "yahoo.com",
			URL:        "https://yahoo.com",
			Title:      "Yahoo",
			VisitCount: 1,
			VisitType:  5, // REDIRECT_TEMP
		},
		// 10. Firefox entry with transition reload
		{
			Browser:    "Firefox",
			Profile:    "default-release",
			Timestamp:  time.Date(2026, 6, 20, 18, 0, 0, 0, time.UTC),
			Domain:     "bing.com",
			URL:        "https://bing.com",
			Title:      "Bing",
			VisitCount: 1,
			VisitType:  9, // RELOAD
		},
		// 11. Firefox entry with typed fallback
		{
			Browser:    "Firefox",
			Profile:    "default-release",
			Timestamp:  time.Date(2026, 6, 20, 19, 0, 0, 0, time.UTC),
			Domain:     "duckduckgo.com",
			URL:        "https://duckduckgo.com",
			Title:      "DuckDuckGo",
			VisitCount: 1,
			VisitType:  0, // Other
			Typed:      1,
		},
		// 12. Safari entry with clicked link
		{
			Browser:        "Safari",
			Profile:        "Personal",
			Timestamp:      time.Date(2026, 6, 20, 20, 0, 0, 0, time.UTC),
			Domain:         "apple.com",
			URL:            "https://apple.com",
			Title:            "Apple",
			VisitCount:     1,
			GenerationType: 0, // Clicked Link
			LoadSuccessful: func() *bool { b := true; return &b }(),
		},
		// 13. Safari entry with typed
		{
			Browser:        "Safari",
			Profile:        "Personal",
			Timestamp:      time.Date(2026, 6, 20, 21, 0, 0, 0, time.UTC),
			Domain:         "icloud.com",
			URL:            "https://icloud.com",
			Title:            "iCloud",
			VisitCount:     1,
			GenerationType: 1, // Typed
			LoadSuccessful: func() *bool { b := false; return &b }(),
			HTTPNonGET:     true,
		},
		// 14. Safari entry with redirect
		{
			Browser:        "Safari",
			Profile:        "Personal",
			Timestamp:      time.Date(2026, 6, 20, 22, 0, 0, 0, time.UTC),
			Domain:         "news.ycombinator.com",
			URL:            "https://news.ycombinator.com",
			Title:            "Hacker News",
			VisitCount:     1,
			GenerationType: 2, // Redirect
		},
		// 15. Safari entry with reload
		{
			Browser:        "Safari",
			Profile:        "Personal",
			Timestamp:      time.Date(2026, 6, 20, 23, 0, 0, 0, time.UTC),
			Domain:         "stackoverflow.com",
			URL:            "https://stackoverflow.com",
			Title:            "Stack Overflow",
			VisitCount:     1,
			GenerationType: 4, // Reload
		},
		// 16. Safari entry with bookmark origin
		{
			Browser:        "Safari",
			Profile:        "Personal",
			Timestamp:      time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC), // Sunday visit
			Domain:         "nytimes.com",
			URL:            "https://nytimes.com",
			Title:            "NY Times",
			VisitCount:     1,
			GenerationType: 9, // Other
			Origin:         1, // Bookmark
		},
	}

	var buf bytes.Buffer
	err = FormatStats(&buf, entries, fromTime, toTime, loc)
	if err != nil {
		t.Fatalf("unexpected error formatting stats: %v", err)
	}

	output := buf.String()

	// Verify header and basic stats are printed
	if !strings.Contains(output, "WEB HISTORY STATISTICS") {
		t.Errorf("missing stats title: %q", output)
	}
	if !strings.Contains(output, "Total Visits:    16") {
		t.Errorf("incorrect total visits count: %q", output)
	}
	if !strings.Contains(output, "Unique Domains:  16") {
		t.Errorf("incorrect unique domains count: %q", output)
	}

	// Verify browser breakdown list
	if !strings.Contains(output, "Chrome (Default)") || !strings.Contains(output, "Safari (Personal)") || !strings.Contains(output, "Firefox (default-release)") {
		t.Errorf("missing browser profile breakdown: %q", output)
	}

	// Verify session analysis
	if !strings.Contains(output, "Browsing Sessions (Inactivity threshold: 30 mins):") {
		t.Errorf("missing session analysis section: %q", output)
	}

	// Verify Chrome time analysis
	if !strings.Contains(output, "Browsing Time Analysis (Chromium-based browsers):") {
		t.Errorf("missing Chromium browsing time analysis: %q", output)
	}
	if !strings.Contains(output, "Total Active Time:") {
		t.Errorf("missing Chrome total active time: %q", output)
	}

	// Verify navigation breakdown
	if !strings.Contains(output, "Navigation Methods Breakdown:") {
		t.Errorf("missing navigation breakdown: %q", output)
	}
	if !strings.Contains(output, "Clicked Link") || !strings.Contains(output, "Typed / Direct") || !strings.Contains(output, "Bookmarks") {
		t.Errorf("missing navigation category labels: %q", output)
	}

	// Verify Safari performance metrics
	if !strings.Contains(output, "Safari Performance Metrics:") {
		t.Errorf("missing Safari performance metrics section: %q", output)
	}
	if !strings.Contains(output, "Page Load Success Rate:") {
		t.Errorf("missing page load success rate: %q", output)
	}

	// Verify top lists
	if !strings.Contains(output, "Top 10 Domains:") || !strings.Contains(output, "Top 10 Visited Pages:") {
		t.Errorf("missing top lists: %q", output)
	}

	// Verify distributions
	if !strings.Contains(output, "Weekly Activity Distribution:") || !strings.Contains(output, "Hourly Activity Histogram:") {
		t.Errorf("missing distribution sections: %q", output)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d        time.Duration
		expected string
	}{
		{2 * time.Hour + 30 * time.Minute + 15 * time.Second, "2h 30m 15s"},
		{45 * time.Minute + 5 * time.Second, "45m 5s"},
		{12 * time.Second, "12s"},
	}

	for _, tc := range tests {
		actual := formatDuration(tc.d)
		if actual != tc.expected {
			t.Errorf("formatDuration(%v) = %q; expected %q", tc.d, actual, tc.expected)
		}
	}
}

func TestFormatStats_EdgeCases(t *testing.T) {
	// Construct a case that triggers:
	// 1. sortedDomainDurations count < 5
	// 2. sortedDomains count < 10
	// 3. sortedURLs count < 10
	// 4. long URL to exceed 60 characters
	// 5. count > 0 && barWidth == 0 for weekly activity distribution
	// 6. count > 0 && barWidth == 0 for hourly activity histogram
	
	// Create 100 entries on Monday at 12:00 to scale the max count to 100,
	// and 1 entry on Saturday at 13:00 to test the low-count bar character "▏".
	entries := make([]models.HistoryEntry, 0, 102)
	
	// Monday is Weekday 1. Saturday is Weekday 6.
	// 2026-06-22 is a Monday.
	// 2026-06-27 is a Saturday.
	mondayBase := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 100; i++ {
		entries = append(entries, models.HistoryEntry{
			Browser:       "Chrome",
			Profile:       "Default",
			Timestamp:     mondayBase.Add(time.Duration(i) * time.Second),
			Domain:        "google.com",
			URL:           "https://google.com",
			Title:         "Google",
			VisitCount:    1,
			VisitDuration: 10 * 1000000, // 10s
		})
	}
	
	// Saturday visit
	saturdayBase := time.Date(2026, 6, 27, 13, 0, 0, 0, time.UTC)
	entries = append(entries, models.HistoryEntry{
		Browser:       "Chrome",
		Profile:       "Default",
		Timestamp:     saturdayBase,
		Domain:        "somelongdomainname.com",
		URL:           "https://somelongdomainname.com/very/long/url/path/that/exceeds/sixty/characters/limit/to/verify/truncation/works/correctly",
		Title:         "Long URL Title",
		VisitCount:    1,
		VisitDuration: 5 * 1000000, // 5s
	})
	
	var buf bytes.Buffer
	err := FormatStats(&buf, entries, time.Time{}, time.Time{}, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error formatting stats: %v", err)
	}

	output := buf.String()

	// Verify the "▏" bar is present for Saturday/13:00 which has only 1 visit vs 100 visits max
	if !strings.Contains(output, "Saturday  :") && !strings.Contains(output, "▏") {
		t.Errorf("expected ▏ character to be used in low-count bars: %q", output)
	}
}
