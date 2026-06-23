package output

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// DomainStat stores counts for a domain
type DomainStat struct {
	Domain string
	Count  int
}

// URLStat stores counts for a URL
type URLStat struct {
	URL   string
	Count int
}

// formatDuration formats a duration into a human-readable string (Xh Ym Zs)
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// FormatStats prints a rich statistics analysis of the history to the writer
func FormatStats(w io.Writer, entries []models.HistoryEntry, fromTime, toTime time.Time, loc *time.Location) error {
	totalVisits := len(entries)
	if totalVisits == 0 {
		fmt.Fprintln(w, "No history entries found in the specified range.")
		return nil
	}

	// 1. Gather stats
	domainCounts := make(map[string]int)
	urlCounts := make(map[string]int)
	browserCounts := make(map[string]int)
	hourlyCounts := make([]int, 24)
	dayCounts := make([]int, 7) // 0: Sunday, 1: Monday, ... 6: Saturday
	dailyVisits := make(map[string]int) // key: "YYYY-MM-DD"

	// Time spent (Chrome visit_duration)
	var totalDurationMicro int64
	domainDurations := make(map[string]int64)
	var durationVisitsCount int
	var chromeCount int

	// Navigation modes / Transitions
	navigationCounts := map[string]int{
		"Clicked Link":   0,
		"Typed / Direct": 0,
		"Bookmarks":      0,
		"Reloads":        0,
		"Redirects":      0,
		"Other / System": 0,
	}

	// Safari quality metrics
	var safariCount int
	var safariSuccessCount int
	var safariHTTPNonGETCount int

	for _, entry := range entries {
		domainCounts[entry.Domain]++
		urlCounts[entry.URL]++
		browserKey := fmt.Sprintf("%s (%s)", cases.Title(language.Und).String(entry.Browser), entry.Profile)
		browserCounts[browserKey]++

		// Hour, weekday and date in local timezone
		entryTimeLocal := entry.Timestamp.In(loc)
		hourlyCounts[entryTimeLocal.Hour()]++
		dayCounts[entryTimeLocal.Weekday()]++
		dateStr := entryTimeLocal.Format("2006-01-02")
		dailyVisits[dateStr]++

		// Chrome-specific active browsing time (visit_duration)
		browserLower := strings.ToLower(entry.Browser)
		isChromeFamily := browserLower == "chrome" || browserLower == "chromium" || browserLower == "edge" || browserLower == "brave"

		if isChromeFamily {
			chromeCount++
			if entry.VisitDuration > 0 {
				dur := entry.VisitDuration
				// Cap abnormally large visit durations at 4 hours to avoid sleep mode skewing
				const fourHoursMicro = 4 * 60 * 60 * 1000000
				if dur > fourHoursMicro {
					dur = fourHoursMicro
				}
				totalDurationMicro += dur
				domainDurations[entry.Domain] += dur
				durationVisitsCount++
			}
		}

		// Safari metrics
		if browserLower == "safari" {
			safariCount++
			if entry.LoadSuccessful {
				safariSuccessCount++
			}
			if entry.HTTPNonGET {
				safariHTTPNonGETCount++
			}
		}

		// Navigation mode decoding
		navType := "Other / System"
		if isChromeFamily {
			coreType := entry.Transition & 0xFF
			isClientRedirect := (entry.Transition & 0x10000000) != 0
			if isClientRedirect {
				navType = "Redirects"
			} else {
				switch coreType {
				case 0, 3, 4: // LINK, AUTO_SUBFRAME, MANUAL_SUBFRAME
					navType = "Clicked Link"
				case 1, 5, 9, 10: // TYPED, GENERATED, KEYWORD, KEYWORD_GENERATED
					navType = "Typed / Direct"
				case 2: // AUTO_BOOKMARK
					navType = "Bookmarks"
				case 8: // RELOAD
					navType = "Reloads"
				}
			}
		} else if browserLower == "firefox" {
			// 1: LINK, 2: TYPED, 3: BOOKMARK, 4: EMBED, 5: REDIRECT_TEMP, 6: REDIRECT_PERM, 7: DOWNLOAD, 8: FRAMED_LINK, 9: RELOAD
			switch entry.VisitType {
			case 1, 8:
				navType = "Clicked Link"
			case 2:
				navType = "Typed / Direct"
			case 3:
				navType = "Bookmarks"
			case 5, 6:
				navType = "Redirects"
			case 9:
				navType = "Reloads"
			default:
				if entry.Typed == 1 {
					navType = "Typed / Direct"
				}
			}
		} else if browserLower == "safari" {
			// 0: Standard, 1: Typed, 2: Redirect, 3: Back/Forward, 4: Reload, 5: Synthesized
			switch entry.GenerationType {
			case 0:
				navType = "Clicked Link"
			case 1:
				navType = "Typed / Direct"
			case 2:
				navType = "Redirects"
			case 4:
				navType = "Reloads"
			default:
				if entry.Origin == 1 { // BOOKMARK
					navType = "Bookmarks"
				}
			}
		}
		navigationCounts[navType]++
	}

	// 2. Sort domains
	var sortedDomains []DomainStat
	for d, c := range domainCounts {
		sortedDomains = append(sortedDomains, DomainStat{Domain: d, Count: c})
	}
	sort.Slice(sortedDomains, func(i, j int) bool {
		return sortedDomains[i].Count > sortedDomains[j].Count
	})

	// 3. Sort URLs
	var sortedURLs []URLStat
	for u, c := range urlCounts {
		sortedURLs = append(sortedURLs, URLStat{URL: u, Count: c})
	}
	sort.Slice(sortedURLs, func(i, j int) bool {
		return sortedURLs[i].Count > sortedURLs[j].Count
	})

	// 4. Sessionization
	type Session struct {
		StartTime time.Time
		EndTime   time.Time
		Pages     int
	}
	var sessions []Session
	var currentSession *Session

	// Group visits chronologically
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if currentSession == nil {
			currentSession = &Session{
				StartTime: entry.Timestamp,
				EndTime:   entry.Timestamp,
				Pages:     1,
			}
		} else {
			gap := entry.Timestamp.Sub(currentSession.EndTime)
			if gap < 0 {
				gap = -gap
			}
			if gap > 30*time.Minute {
				sessions = append(sessions, *currentSession)
				currentSession = &Session{
					StartTime: entry.Timestamp,
					EndTime:   entry.Timestamp,
					Pages:     1,
				}
			} else {
				currentSession.EndTime = entry.Timestamp
				currentSession.Pages++
			}
		}
	}
	if currentSession != nil {
		sessions = append(sessions, *currentSession)
	}

	totalSessions := len(sessions)
	var avgPagesPerSession float64
	var avgSessionDuration time.Duration
	var maxSessionPages int
	var maxSessionDay string

	if totalSessions > 0 {
		var sumDuration time.Duration
		var sumPages int
		for _, s := range sessions {
			dur := s.EndTime.Sub(s.StartTime)
			sumDuration += dur
			sumPages += s.Pages

			if s.Pages > maxSessionPages {
				maxSessionPages = s.Pages
				maxSessionDay = s.StartTime.In(loc).Format("2006-01-02")
			}
		}
		avgPagesPerSession = float64(sumPages) / float64(totalSessions)
		avgSessionDuration = sumDuration / time.Duration(totalSessions)
	}

	// 5. Print stats header
	fmt.Fprintln(w, "================================================================================")
	fmt.Fprintln(w, "                            WEB HISTORY STATISTICS")
	fmt.Fprintln(w, "================================================================================")

	fromStr := "Start of log"
	if !fromTime.IsZero() && fromTime.Unix() > 0 {
		fromStr = fromTime.In(loc).Format(time.RFC3339)
	}
	toStr := "Now"
	if !toTime.IsZero() {
		toStr = toTime.In(loc).Format(time.RFC3339)
	}

	fmt.Fprintf(w, "Time Range:     %s to %s\n", fromStr, toStr)
	fmt.Fprintf(w, "Total Visits:    %d\n", totalVisits)
	fmt.Fprintf(w, "Unique Domains:  %d\n", len(domainCounts))
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

	// Print browser breakdown
	fmt.Fprintln(w, "Visits by Browser & Profile:")
	var sortedBrowsers []string
	for b := range browserCounts {
		sortedBrowsers = append(sortedBrowsers, b)
	}
	sort.Strings(sortedBrowsers)

	for _, b := range sortedBrowsers {
		count := browserCounts[b]
		pct := (float64(count) / float64(totalVisits)) * 100
		fmt.Fprintf(w, "  - %-30s %5d (%5.1f%%)\n", b, count, pct)
	}
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

	// Print session analysis
	if totalSessions > 0 {
		fmt.Fprintln(w, "Browsing Sessions (Inactivity threshold: 30 mins):")
		fmt.Fprintf(w, "  - Total Sessions:      %d\n", totalSessions)
		fmt.Fprintf(w, "  - Average Duration:    %s\n", formatDuration(avgSessionDuration))
		fmt.Fprintf(w, "  - Avg Pages/Session:   %.1f\n", avgPagesPerSession)
		if maxSessionPages > 0 {
			fmt.Fprintf(w, "  - Peak Session Size:   %d pages (%s)\n", maxSessionPages, maxSessionDay)
		}
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Print Chrome active time analysis if applicable
	if chromeCount > 0 && durationVisitsCount > 0 {
		totalDuration := time.Duration(totalDurationMicro) * time.Microsecond
		avgDuration := time.Duration(totalDurationMicro/int64(durationVisitsCount)) * time.Microsecond

		type DomainDurationStat struct {
			Domain   string
			Duration time.Duration
		}
		var sortedDomainDurations []DomainDurationStat
		for d, durMicro := range domainDurations {
			dur := time.Duration(durMicro) * time.Microsecond
			sortedDomainDurations = append(sortedDomainDurations, DomainDurationStat{Domain: d, Duration: dur})
		}
		sort.Slice(sortedDomainDurations, func(i, j int) bool {
			return sortedDomainDurations[i].Duration > sortedDomainDurations[j].Duration
		})

		fmt.Fprintln(w, "Browsing Time Analysis (Chrome-only):")
		fmt.Fprintf(w, "  - Total Active Time:   %s\n", formatDuration(totalDuration))
		fmt.Fprintf(w, "  - Average Page Time:   %s\n", formatDuration(avgDuration))
		fmt.Fprintln(w, "  - Top Domains by Duration:")

		limitDurations := 5
		if len(sortedDomainDurations) < limitDurations {
			limitDurations = len(sortedDomainDurations)
		}
		for i := 0; i < limitDurations; i++ {
			sd := sortedDomainDurations[i]
			fmt.Fprintf(w, "    %d. %-35s %s\n", i+1, sd.Domain, formatDuration(sd.Duration))
		}
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Print navigation methods breakdown
	fmt.Fprintln(w, "Navigation Methods Breakdown:")
	navCategories := []string{"Clicked Link", "Typed / Direct", "Bookmarks", "Redirects", "Reloads", "Other / System"}
	for _, cat := range navCategories {
		count := navigationCounts[cat]
		pct := (float64(count) / float64(totalVisits)) * 100
		fmt.Fprintf(w, "  - %-20s %5d (%5.1f%%)\n", cat, count, pct)
	}
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

	// Print Safari metrics if applicable
	if safariCount > 0 {
		fmt.Fprintln(w, "Safari Performance Metrics:")
		successRate := 100.0
		if safariCount > 0 {
			successRate = (float64(safariSuccessCount) / float64(safariCount)) * 100.0
		}
		fmt.Fprintf(w, "  - Page Load Success Rate:   %.1f%% (%d/%d)\n", successRate, safariSuccessCount, safariCount)
		fmt.Fprintf(w, "  - Form Submissions (POST):  %d\n", safariHTTPNonGETCount)
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Print top 10 domains
	fmt.Fprintln(w, "Top 10 Domains:")
	limitDomains := 10
	if len(sortedDomains) < limitDomains {
		limitDomains = len(sortedDomains)
	}
	for i := 0; i < limitDomains; i++ {
		d := sortedDomains[i]
		pct := (float64(d.Count) / float64(totalVisits)) * 100
		fmt.Fprintf(w, "  %2d. %-40s %5d (%5.1f%%)\n", i+1, d.Domain, d.Count, pct)
	}
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

	// Print top 10 pages
	fmt.Fprintln(w, "Top 10 Visited Pages:")
	limitURLs := 10
	if len(sortedURLs) < limitURLs {
		limitURLs = len(sortedURLs)
	}
	for i := 0; i < limitURLs; i++ {
		u := sortedURLs[i]
		displayURL := u.URL
		if len(displayURL) > 60 {
			displayURL = displayURL[:57] + "..."
		}
		fmt.Fprintf(w, "  %2d. %-60s %5d\n", i+1, displayURL, u.Count)
	}
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

	// Print weekly distribution
	fmt.Fprintln(w, "Weekly Activity Distribution:")
	weekdayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	maxDayCount := 0
	for _, count := range dayCounts {
		if count > maxDayCount {
			maxDayCount = count
		}
	}
	order := []int{1, 2, 3, 4, 5, 6, 0} // Mon to Sun
	for _, idx := range order {
		count := dayCounts[idx]
		barWidth := 0
		if maxDayCount > 0 {
			barWidth = (count * 30) / maxDayCount
		}
		bar := strings.Repeat("█", barWidth)
		if count > 0 && barWidth == 0 {
			bar = "▏"
		}
		fmt.Fprintf(w, "  %-10s: %-30s %d\n", weekdayNames[idx], bar, count)
	}
	fmt.Fprintln(w, "")

	weekdayVisits := dayCounts[1] + dayCounts[2] + dayCounts[3] + dayCounts[4] + dayCounts[5]
	weekendVisits := dayCounts[6] + dayCounts[0]
	weekdayPct := (float64(weekdayVisits) / float64(totalVisits)) * 100
	weekendPct := (float64(weekendVisits) / float64(totalVisits)) * 100
	fmt.Fprintf(w, "  Weekdays (Mon-Fri): %d (%.1f%%) | Weekends (Sat-Sun): %d (%.1f%%)\n", weekdayVisits, weekdayPct, weekendVisits, weekendPct)

	var mostActiveDate string
	var mostActiveCount int
	for d, c := range dailyVisits {
		if c > mostActiveCount {
			mostActiveCount = c
			mostActiveDate = d
		}
	}
	if mostActiveCount > 0 {
		fmt.Fprintf(w, "  Most Active Day:    %s (%d visits)\n", mostActiveDate, mostActiveCount)
	}
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

	// Print hourly activity histogram
	fmt.Fprintln(w, "Hourly Activity Histogram:")
	maxHourCount := 0
	for _, c := range hourlyCounts {
		if c > maxHourCount {
			maxHourCount = c
		}
	}

	for hour := 0; hour < 24; hour++ {
		count := hourlyCounts[hour]
		barWidth := 0
		if maxHourCount > 0 {
			barWidth = (count * 40) / maxHourCount
		}
		bar := strings.Repeat("█", barWidth)
		if count > 0 && barWidth == 0 {
			bar = "▏"
		}
		fmt.Fprintf(w, "  %02d:00 - %02d:59: %-40s %d\n", hour, hour, bar, count)
	}
	fmt.Fprintln(w, "================================================================================")

	return nil
}
