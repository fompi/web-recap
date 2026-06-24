package output

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	"github.com/rzolkos/web-recap/internal/utils"
)

// titleCase uppercases the first rune of s; browser names are ASCII so this is safe.
func titleCase(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

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

// navCategoryFromLabel maps a normalized VisitTypeLabel to a display category.
// Falls back to "Other / System" for unknown or empty labels.
func navCategoryFromLabel(label string) string {
	switch label {
	case "link":
		return "Clicked Link"
	case "typed":
		return "Typed / Direct"
	case "bookmark":
		return "Bookmarks"
	case "reload":
		return "Reloads"
	case "redirect":
		return "Redirects"
	case "download":
		return "Other / System"
	default:
		return "Other / System"
	}
}

// referrerDomain extracts the hostname from a referrer URL, or "" if unparseable/empty.
func referrerDomain(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	host := u.Hostname()
	// Strip leading "www."
	host = strings.TrimPrefix(host, "www.")
	return host
}

// extractSearchQuery extracts search queries from popular search engines.
func extractSearchQuery(urlStr string) string {
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return ""
	}
	qParams := u.Query()
	var query string
	if strings.Contains(host, "google.") || strings.Contains(host, "duckduckgo.") || strings.Contains(host, "bing.") {
		query = qParams.Get("q")
	} else if strings.Contains(host, "youtube.com") || strings.Contains(host, "youtu.be") {
		query = qParams.Get("search_query")
	} else if strings.Contains(host, "search.yahoo.") {
		query = qParams.Get("p")
	}
	return strings.TrimSpace(query)
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

	// Navigation modes
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

	// Source breakdown (local vs synced)
	sourceCounts := map[string]int{"local": 0, "synced": 0}

	// Referrer insights
	referrerDomainCounts := make(map[string]int)
	var referredCount int

	// New advanced statistical fields
	secureCount := 0
	insecureCount := 0
	localNetworkCount := 0
	externalNetworkCount := 0
	tldTypeCounts := make(map[string]int)
	continentCounts := make(map[string]int)
	portCounts := make(map[string]int)
	searchQueryCounts := make(map[string]int)
	basicAuthPlaintextCount := 0

	for _, entry := range entries {
		domainCounts[entry.Domain]++
		urlCounts[entry.URL]++
		browserKey := fmt.Sprintf("%s (%s)", titleCase(entry.Browser), entry.Profile)
		browserCounts[browserKey]++

		// Hour, weekday and date in local timezone
		entryTimeLocal := entry.Timestamp.In(loc)
		hourlyCounts[entryTimeLocal.Hour()]++
		dayCounts[entryTimeLocal.Weekday()]++
		dateStr := entryTimeLocal.Format("2006-01-02")
		dailyVisits[dateStr]++

		// Source tracking
		switch entry.Source {
		case "synced":
			sourceCounts["synced"]++
		default:
			sourceCounts["local"]++
		}

		// Referrer tracking
		if entry.ReferrerURL != "" {
			referredCount++
			if rd := referrerDomain(entry.ReferrerURL); rd != "" {
				referrerDomainCounts[rd]++
			}
		}

		// Security (HTTPS vs HTTP/Other)
		if strings.ToLower(entry.Scheme) == "https" {
			secureCount++
		} else if strings.ToLower(entry.Scheme) == "http" || strings.ToLower(entry.Scheme) == "ftp" {
			insecureCount++
		} else if entry.Scheme != "" {
			insecureCount++
		}

		// Network Scope (Local vs External)
		if utils.IsLocal(entry.FQDN) {
			localNetworkCount++
		} else {
			externalNetworkCount++
		}

		// TLD Type & Continent breakdown
		if entry.TLD != "" {
			tldType := utils.GetTLDType(entry.TLD)
			tldTypeCounts[tldType]++

			if !utils.IsLocal(entry.FQDN) {
				continent := utils.GetContinent(entry.TLD)
				continentCounts[continent]++
			}
		}

		// Ports
		if entry.Port != "" {
			portCounts[entry.Port]++
		}

		// Searches
		if q := extractSearchQuery(entry.URL); q != "" {
			searchQueryCounts[q]++
		}

		// Plaintext credentials count
		u, err := url.Parse(entry.URL)
		if err == nil && u.User != nil {
			if pass, ok := u.User.Password(); ok && pass != "***" && pass != "" {
				basicAuthPlaintextCount++
			}
		}

		// Chrome-specific active browsing time (visit_duration)
		browserLower := strings.ToLower(entry.Browser)
		isChromeFamily := browserLower == "chrome" || browserLower == "google chrome" || browserLower == "chromium" || browserLower == "edge" || browserLower == "microsoft edge" || browserLower == "brave"

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
			if entry.LoadSuccessful != nil && *entry.LoadSuccessful {
				safariSuccessCount++
			}
			if entry.HTTPNonGET {
				safariHTTPNonGETCount++
			}
		}

		// Navigation mode: prefer the normalized label; fall back to raw fields.
		navType := ""
		if entry.VisitTypeLabel != "" {
			navType = navCategoryFromLabel(entry.VisitTypeLabel)
		}
		if navType == "" {
			// Legacy fallback for entries that predate label normalization
			if isChromeFamily {
				coreType := entry.Transition & 0xFF
				isClientRedirect := (entry.Transition & 0x10000000) != 0
				if isClientRedirect {
					navType = "Redirects"
				} else {
					switch coreType {
					case 0, 3, 4:
						navType = "Clicked Link"
					case 1, 5, 9, 10:
						navType = "Typed / Direct"
					case 2:
						navType = "Bookmarks"
					case 8:
						navType = "Reloads"
					default:
						navType = "Other / System"
					}
				}
			} else if browserLower == "firefox" {
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
					} else {
						navType = "Other / System"
					}
				}
			} else if browserLower == "safari" {
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
					if entry.Origin == 1 {
						navType = "Bookmarks"
					} else {
						navType = "Other / System"
					}
				}
			} else {
				navType = "Other / System"
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

	// 4. Sort referrer domains
	var sortedReferrerDomains []DomainStat
	for d, c := range referrerDomainCounts {
		sortedReferrerDomains = append(sortedReferrerDomains, DomainStat{Domain: d, Count: c})
	}
	sort.Slice(sortedReferrerDomains, func(i, j int) bool {
		return sortedReferrerDomains[i].Count > sortedReferrerDomains[j].Count
	})

	// 5. Domain loyalty
	var oneTimeDomains, returnDomains int
	for _, c := range domainCounts {
		if c == 1 {
			oneTimeDomains++
		} else {
			returnDomains++
		}
	}

	// 6. Sessionization
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

	// 7. Print stats header
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

	// Visits by browser
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

	// Source breakdown (only shown when synced data exists)
	if sourceCounts["synced"] > 0 {
		fmt.Fprintln(w, "Source Breakdown:")
		localCount := sourceCounts["local"]
		syncedCount := sourceCounts["synced"]
		fmt.Fprintf(w, "  - %-30s %5d (%5.1f%%)\n", "Local (this device)", localCount, float64(localCount)/float64(totalVisits)*100)
		fmt.Fprintf(w, "  - %-30s %5d (%5.1f%%)\n", "Synced (other devices)", syncedCount, float64(syncedCount)/float64(totalVisits)*100)
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Session analysis
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

	// Chrome active time analysis
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

		fmt.Fprintln(w, "Browsing Time Analysis (Chromium-based browsers):")
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

	// Navigation methods breakdown
	fmt.Fprintln(w, "Navigation Methods Breakdown:")
	navCategories := []string{"Clicked Link", "Typed / Direct", "Bookmarks", "Redirects", "Reloads", "Other / System"}
	for _, cat := range navCategories {
		count := navigationCounts[cat]
		pct := (float64(count) / float64(totalVisits)) * 100
		fmt.Fprintf(w, "  - %-20s %5d (%5.1f%%)\n", cat, count, pct)
	}
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

	// Referrer insights (only when referrer data is present)
	if referredCount > 0 || len(referrerDomainCounts) > 0 {
		directCount := totalVisits - referredCount
		fmt.Fprintln(w, "Referrer Insights:")
		fmt.Fprintf(w, "  - Direct visits (no referrer):  %5d (%5.1f%%)\n", directCount, float64(directCount)/float64(totalVisits)*100)
		fmt.Fprintf(w, "  - Referred visits:              %5d (%5.1f%%)\n", referredCount, float64(referredCount)/float64(totalVisits)*100)
		if len(sortedReferrerDomains) > 0 {
			fmt.Fprintln(w, "  - Top entry-point domains:")
			limit := 5
			if len(sortedReferrerDomains) < limit {
				limit = len(sortedReferrerDomains)
			}
			for i := 0; i < limit; i++ {
				rd := sortedReferrerDomains[i]
				fmt.Fprintf(w, "      %d. %-35s %d\n", i+1, rd.Domain, rd.Count)
			}
		}
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Safari metrics
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

	// Security Protocol Ratio
	if secureCount+insecureCount > 0 {
		fmt.Fprintln(w, "Security Analysis (Protocol Ratio):")
		tot := secureCount + insecureCount
		fmt.Fprintf(w, "  - Secure (HTTPS):        %d (%.1f%%)\n", secureCount, float64(secureCount)/float64(tot)*100)
		fmt.Fprintf(w, "  - Insecure (HTTP/Other): %d (%.1f%%)\n", insecureCount, float64(insecureCount)/float64(tot)*100)
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Network Scope Analysis
	fmt.Fprintln(w, "Network Scope Analysis:")
	fmt.Fprintf(w, "  - Local / Intranet:      %d (%.1f%%)\n", localNetworkCount, float64(localNetworkCount)/float64(totalVisits)*100)
	fmt.Fprintf(w, "  - External Internet:     %d (%.1f%%)\n", externalNetworkCount, float64(externalNetworkCount)/float64(totalVisits)*100)
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

	// TLD Type Classification
	if len(tldTypeCounts) > 0 {
		fmt.Fprintln(w, "TLD Type Distribution:")
		var sortedTLDTypes []string
		var totalTLDs int
		for tldType, count := range tldTypeCounts {
			sortedTLDTypes = append(sortedTLDTypes, tldType)
			totalTLDs += count
		}
		sort.Slice(sortedTLDTypes, func(i, j int) bool {
			return tldTypeCounts[sortedTLDTypes[i]] > tldTypeCounts[sortedTLDTypes[j]]
		})
		for _, tldType := range sortedTLDTypes {
			count := tldTypeCounts[tldType]
			fmt.Fprintf(w, "  - %-20s %5d (%.1f%%)\n", tldType, count, float64(count)/float64(totalTLDs)*100)
		}
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Continent Distribution (ccTLDs only)
	if len(continentCounts) > 0 {
		fmt.Fprintln(w, "Geographical ccTLD Continent Distribution:")
		var sortedContinents []string
		var totalContinents int
		for cont, count := range continentCounts {
			sortedContinents = append(sortedContinents, cont)
			totalContinents += count
		}
		sort.Slice(sortedContinents, func(i, j int) bool {
			return continentCounts[sortedContinents[i]] > continentCounts[sortedContinents[j]]
		})
		for _, cont := range sortedContinents {
			count := continentCounts[cont]
			fmt.Fprintf(w, "  - %-20s %5d (%.1f%%)\n", cont, count, float64(count)/float64(totalContinents)*100)
		}
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Port Breakdown
	if len(portCounts) > 0 {
		fmt.Fprintln(w, "Explicit Port Usage (Top Ports):")
		type PortStat struct {
			Port  string
			Count int
		}
		var sortedPorts []PortStat
		for p, c := range portCounts {
			sortedPorts = append(sortedPorts, PortStat{Port: p, Count: c})
		}
		sort.Slice(sortedPorts, func(i, j int) bool {
			return sortedPorts[i].Count > sortedPorts[j].Count
		})
		limit := 5
		if len(sortedPorts) < limit {
			limit = len(sortedPorts)
		}
		for i := 0; i < limit; i++ {
			ps := sortedPorts[i]
			fmt.Fprintf(w, "  - Port :%-10s      %5d\n", ps.Port, ps.Count)
		}
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Top Search Queries
	if len(searchQueryCounts) > 0 {
		fmt.Fprintln(w, "Top Search Queries:")
		type QueryStat struct {
			Query string
			Count int
		}
		var sortedQueries []QueryStat
		for q, c := range searchQueryCounts {
			sortedQueries = append(sortedQueries, QueryStat{Query: q, Count: c})
		}
		sort.Slice(sortedQueries, func(i, j int) bool {
			return sortedQueries[i].Count > sortedQueries[j].Count
		})
		limit := 10
		if len(sortedQueries) < limit {
			limit = len(sortedQueries)
		}
		for i := 0; i < limit; i++ {
			qs := sortedQueries[i]
			displayQuery := qs.Query
			if len(displayQuery) > 50 {
				displayQuery = displayQuery[:47] + "..."
			}
			fmt.Fprintf(w, "  %2d. %-50s %d\n", i+1, displayQuery, qs.Count)
		}
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Basic Auth Warning
	if basicAuthPlaintextCount > 0 {
		fmt.Fprintln(w, "!!! SECURITY ALERT !!!")
		fmt.Fprintf(w, "  Detected %d history entries with cleartext Basic Auth credentials (passwords).\n", basicAuthPlaintextCount)
		fmt.Fprintln(w, "  Consider using the --censor / -x flag to redact passwords during dump/ingest.")
		fmt.Fprintln(w, "--------------------------------------------------------------------------------")
	}

	// Top 10 domains
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
	totalDomains := len(domainCounts)
	fmt.Fprintf(w, "  One-time domains (visited once):   %d / %d (%.1f%%)\n", oneTimeDomains, totalDomains, float64(oneTimeDomains)/float64(totalDomains)*100)
	fmt.Fprintf(w, "  Return domains (visited 2+ times): %d / %d (%.1f%%)\n", returnDomains, totalDomains, float64(returnDomains)/float64(totalDomains)*100)
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

	// Top 10 pages
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

	// Weekly distribution
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

	// Hourly activity histogram
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
	fmt.Fprintln(w, "")

	// Time-of-day segments
	nightVisits := 0
	for h := 0; h < 6; h++ {
		nightVisits += hourlyCounts[h]
	}
	morningVisits := 0
	for h := 6; h < 12; h++ {
		morningVisits += hourlyCounts[h]
	}
	afternoonVisits := 0
	for h := 12; h < 18; h++ {
		afternoonVisits += hourlyCounts[h]
	}
	eveningVisits := 0
	for h := 18; h < 24; h++ {
		eveningVisits += hourlyCounts[h]
	}
	fmt.Fprintf(w, "  Night     (00–05): %5d visits (%5.1f%%)\n", nightVisits, float64(nightVisits)/float64(totalVisits)*100)
	fmt.Fprintf(w, "  Morning   (06–11): %5d visits (%5.1f%%)\n", morningVisits, float64(morningVisits)/float64(totalVisits)*100)
	fmt.Fprintf(w, "  Afternoon (12–17): %5d visits (%5.1f%%)\n", afternoonVisits, float64(afternoonVisits)/float64(totalVisits)*100)
	fmt.Fprintf(w, "  Evening   (18–23): %5d visits (%5.1f%%)\n", eveningVisits, float64(eveningVisits)/float64(totalVisits)*100)
	fmt.Fprintln(w, "================================================================================")

	return nil
}

// FormatStatsHTML writes history statistics as a premium, self-contained HTML Dashboard
func FormatStatsHTML(w io.Writer, entries []models.HistoryEntry, fromTime, toTime time.Time, loc *time.Location) error {
	totalVisits := len(entries)
	if totalVisits == 0 {
		_, err := fmt.Fprintln(w, "<html><body>No entries found</body></html>")
		return err
	}

	// 1. Basic Stats Calculation
	domainCounts := make(map[string]int)
	browserCounts := make(map[string]int)
	hourlyCounts := make([]int, 24)
	dayCounts := make([]int, 7)
	dailyVisits := make(map[string]int)

	secureCount := 0
	insecureCount := 0
	localNetworkCount := 0
	externalNetworkCount := 0
	tldTypeCounts := make(map[string]int)
	continentCounts := make(map[string]int)
	portCounts := make(map[string]int)
	searchQueryCounts := make(map[string]int)
	basicAuthPlaintextCount := 0
	navigationCounts := make(map[string]int)

	for _, entry := range entries {
		domainCounts[entry.Domain]++
		browserKey := fmt.Sprintf("%s (%s)", titleCase(entry.Browser), entry.Profile)
		browserCounts[browserKey]++

		entryTimeLocal := entry.Timestamp.In(loc)
		hourlyCounts[entryTimeLocal.Hour()]++
		dayCounts[entryTimeLocal.Weekday()]++
		dailyVisits[entryTimeLocal.Format("2006-01-02")]++

		if strings.ToLower(entry.Scheme) == "https" {
			secureCount++
		} else if entry.Scheme != "" {
			insecureCount++
		}

		if utils.IsLocal(entry.FQDN) {
			localNetworkCount++
		} else {
			externalNetworkCount++
		}

		if entry.TLD != "" {
			tldType := utils.GetTLDType(entry.TLD)
			tldTypeCounts[tldType]++
			if !utils.IsLocal(entry.FQDN) {
				continent := utils.GetContinent(entry.TLD)
				continentCounts[continent]++
			}
		}

		if entry.Port != "" {
			portCounts[entry.Port]++
		}

		if q := extractSearchQuery(entry.URL); q != "" {
			searchQueryCounts[q]++
		}

		u, err := url.Parse(entry.URL)
		if err == nil && u.User != nil {
			if pass, ok := u.User.Password(); ok && pass != "***" && pass != "" {
				basicAuthPlaintextCount++
			}
		}

		navLabel := entry.VisitTypeLabel
		if navLabel == "" {
			navLabel = "other"
		}
		navigationCounts[navLabel]++
	}

	// Sort and format datasets for JS injection
	// Top Domains
	type DomainStat struct {
		Domain string `json:"domain"`
		Count  int    `json:"count"`
	}
	var sortedDomains []DomainStat
	for d, c := range domainCounts {
		sortedDomains = append(sortedDomains, DomainStat{Domain: d, Count: c})
	}
	sort.Slice(sortedDomains, func(i, j int) bool {
		return sortedDomains[i].Count > sortedDomains[j].Count
	})
	topDomainsLimit := 10
	if len(sortedDomains) < topDomainsLimit {
		topDomainsLimit = len(sortedDomains)
	}
	top10Domains := sortedDomains[:topDomainsLimit]

	// Top Queries
	type QueryStat struct {
		Query string `json:"query"`
		Count int    `json:"count"`
	}
	var sortedQueries []QueryStat
	for q, c := range searchQueryCounts {
		sortedQueries = append(sortedQueries, QueryStat{Query: q, Count: c})
	}
	sort.Slice(sortedQueries, func(i, j int) bool {
		return sortedQueries[i].Count > sortedQueries[j].Count
	})
	topQueriesLimit := 10
	if len(sortedQueries) < topQueriesLimit {
		topQueriesLimit = len(sortedQueries)
	}
	top10Queries := sortedQueries[:topQueriesLimit]

	// Daily Visits Timeline (Sort keys chronologically)
	var dates []string
	var dateVisits []int
	for d := range dailyVisits {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	for _, d := range dates {
		dateVisits = append(dateVisits, dailyVisits[d])
	}

	// JSON encode all maps/slices to inject cleanly in JS
	datesJSON, _ := json.Marshal(dates)
	dateVisitsJSON, _ := json.Marshal(dateVisits)
	hourlyJSON, _ := json.Marshal(hourlyCounts)


	tldTypeKeys := []string{}
	tldTypeVals := []int{}
	for k, v := range tldTypeCounts {
		tldTypeKeys = append(tldTypeKeys, k)
		tldTypeVals = append(tldTypeVals, v)
	}
	tldTypeKeysJSON, _ := json.Marshal(tldTypeKeys)
	tldTypeValsJSON, _ := json.Marshal(tldTypeVals)

	continentKeys := []string{}
	continentVals := []int{}
	for k, v := range continentCounts {
		continentKeys = append(continentKeys, k)
		continentVals = append(continentVals, v)
	}
	continentKeysJSON, _ := json.Marshal(continentKeys)
	continentValsJSON, _ := json.Marshal(continentVals)

	browserKeys := []string{}
	browserVals := []int{}
	for k, v := range browserCounts {
		browserKeys = append(browserKeys, k)
		browserVals = append(browserVals, v)
	}
	browserKeysJSON, _ := json.Marshal(browserKeys)
	browserValsJSON, _ := json.Marshal(browserVals)

	navKeys := []string{}
	navVals := []int{}
	for k, v := range navigationCounts {
		navKeys = append(navKeys, k)
		navVals = append(navVals, v)
	}
	navKeysJSON, _ := json.Marshal(navKeys)
	navValsJSON, _ := json.Marshal(navVals)

	fromStr := "Start of log"
	if !fromTime.IsZero() && fromTime.Unix() > 0 {
		fromStr = fromTime.In(loc).Format("2006-01-02 15:04:05")
	}
	toStr := "Now"
	if !toTime.IsZero() {
		toStr = toTime.In(loc).Format("2006-01-02 15:04:05")
	}

	htmlTemplate := `<!DOCTYPE html>
<html lang="en" class="dark">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Web History Recap Dashboard</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;600;700&display=swap" rel="stylesheet">
    <script>
        tailwind.config = {
            darkMode: 'class',
            theme: {
                extend: {
                    fontFamily: {
                        sans: ['Outfit', 'sans-serif'],
                    }
                }
            }
        }
    </script>
    <style>
        body {
            background-color: #0b0f19;
            color: #f3f4f6;
        }
        .glass-card {
            background: rgba(17, 24, 39, 0.7);
            backdrop-filter: blur(12px);
            border: 1px solid rgba(255, 255, 255, 0.05);
            border-radius: 1rem;
        }
    </style>
</head>
<body class="p-6 font-sans">
    <div class="max-w-7xl mx-auto space-y-6">
        <!-- Header -->
        <header class="flex flex-col md:flex-row justify-between items-start md:items-center p-6 glass-card shadow-2xl">
            <div>
                <h1 class="text-3xl font-bold bg-gradient-to-r from-blue-400 to-indigo-500 bg-clip-text text-transparent">Web Recap Dashboard</h1>
                <p class="text-gray-400 text-sm mt-1">Enterprise-grade statistics and browser history analytics</p>
            </div>
            <div class="mt-4 md:mt-0 text-right md:text-right">
                <span class="text-xs font-semibold px-2.5 py-0.5 rounded bg-indigo-900/50 text-indigo-300 border border-indigo-500/30">Local Execution</span>
                <p class="text-xs text-gray-500 mt-2">Range: <span class="text-gray-300 font-mono">%[1]s</span> to <span class="text-gray-300 font-mono">%[2]s</span></p>
            </div>
        </header>

        <!-- Security Warning Alert -->
        %[3]s

        <!-- Summary KPI Grid -->
        <section class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-6">
            <div class="p-6 glass-card shadow-lg flex flex-col justify-between">
                <span class="text-gray-400 text-sm font-semibold uppercase tracking-wider">Total Visits</span>
                <span class="text-4xl font-extrabold text-blue-400 mt-2">%[4]d</span>
                <span class="text-xs text-gray-500 mt-2">Individual visit events</span>
            </div>
            <div class="p-6 glass-card shadow-lg flex flex-col justify-between">
                <span class="text-gray-400 text-sm font-semibold uppercase tracking-wider">Unique Domains</span>
                <span class="text-4xl font-extrabold text-indigo-400 mt-2">%[5]d</span>
                <span class="text-xs text-gray-500 mt-2">Distinct root domains visited</span>
            </div>
            <div class="p-6 glass-card shadow-lg flex flex-col justify-between">
                <span class="text-gray-400 text-sm font-semibold uppercase tracking-wider">HTTPS Ratio</span>
                <span class="text-4xl font-extrabold text-green-400 mt-2">%[6].1f%%</span>
                <span class="text-xs text-gray-500 mt-2">%[7]d secure / %[8]d insecure</span>
            </div>
            <div class="p-6 glass-card shadow-lg flex flex-col justify-between">
                <span class="text-gray-400 text-sm font-semibold uppercase tracking-wider">Network Scope</span>
                <span class="text-4xl font-extrabold text-purple-400 mt-2">%[9].1f%% Ext</span>
                <span class="text-xs text-gray-500 mt-2">%[10]d local / %[11]d external</span>
            </div>
        </section>

        <!-- Main Charts Section -->
        <section class="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <!-- Timeline Chart -->
            <div class="p-6 glass-card lg:col-span-2">
                <h3 class="text-lg font-bold text-gray-200 mb-4">Visits Timeline</h3>
                <div class="h-80">
                    <canvas id="timelineChart"></canvas>
                </div>
            </div>
            <!-- Protocol Security & Network Scope Donut charts -->
            <div class="p-6 glass-card flex flex-col justify-between">
                <div>
                    <h3 class="text-lg font-bold text-gray-200 mb-4">Security & Scope Summary</h3>
                </div>
                <div class="h-64 flex justify-center items-center relative">
                    <canvas id="securityChart"></canvas>
                </div>
                <div class="text-xs text-center text-gray-400 mt-2">
                    Encryption ratio based on parsed HTTP scheme
                </div>
            </div>
        </section>

        <section class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            <!-- TLD Type pie -->
            <div class="p-6 glass-card">
                <h3 class="text-lg font-bold text-gray-200 mb-4">TLD Type Classification</h3>
                <div class="h-60 flex justify-center items-center">
                    <canvas id="tldChart"></canvas>
                </div>
            </div>
            <!-- Geographic ccTLDs pie -->
            <div class="p-6 glass-card">
                <h3 class="text-lg font-bold text-gray-200 mb-4">Geographic Continent Distribution</h3>
                <div class="h-60 flex justify-center items-center">
                    <canvas id="continentChart"></canvas>
                </div>
            </div>
            <!-- Navigation modes -->
            <div class="p-6 glass-card">
                <h3 class="text-lg font-bold text-gray-200 mb-4">Navigation Methods</h3>
                <div class="h-60 flex justify-center items-center">
                    <canvas id="navChart"></canvas>
                </div>
            </div>
        </section>

        <!-- Hourly & Browser breakdown -->
        <section class="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <div class="p-6 glass-card lg:col-span-2">
                <h3 class="text-lg font-bold text-gray-200 mb-4">Hourly Activity Histogram</h3>
                <div class="h-64">
                    <canvas id="hourlyChart"></canvas>
                </div>
            </div>
            <div class="p-6 glass-card">
                <h3 class="text-lg font-bold text-gray-200 mb-4">Browsers & Profiles</h3>
                <div class="h-64">
                    <canvas id="browserChart"></canvas>
                </div>
            </div>
        </section>

        <!-- Tables section -->
        <section class="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <!-- Top Domains table -->
            <div class="p-6 glass-card">
                <h3 class="text-lg font-bold text-gray-200 mb-4">Top 10 Domains</h3>
                <div class="overflow-x-auto">
                    <table class="min-w-full divide-y divide-gray-800">
                        <thead>
                            <tr>
                                <th class="px-4 py-2 text-left text-xs font-semibold text-gray-400 uppercase">Domain</th>
                                <th class="px-4 py-2 text-right text-xs font-semibold text-gray-400 uppercase">Visits</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-gray-800/50">
                            %[12]s
                        </tbody>
                    </table>
                </div>
            </div>

            <!-- Top Search Queries table -->
            <div class="p-6 glass-card">
                <h3 class="text-lg font-bold text-gray-200 mb-4">Top Search Queries</h3>
                <div class="overflow-x-auto">
                    <table class="min-w-full divide-y divide-gray-800">
                        <thead>
                            <tr>
                                <th class="px-4 py-2 text-left text-xs font-semibold text-gray-400 uppercase">Query</th>
                                <th class="px-4 py-2 text-right text-xs font-semibold text-gray-400 uppercase">Count</th>
                            </tr>
                        </thead>
                        <tbody class="divide-y divide-gray-800/50">
                            %[13]s
                        </tbody>
                    </table>
                </div>
            </div>
        </section>
    </div>

    <!-- Chart Scripts Injection -->
    <script>
        const chartOptions = {
            responsive: true,
            maintainAspectRatio: false,
            plugins: {
                legend: {
                    labels: { color: '#9ca3af', font: { family: 'Outfit' } }
                }
            }
        };

        // Timeline
        new Chart(document.getElementById('timelineChart').getContext('2d'), {
            type: 'line',
            data: {
                labels: %[14]s,
                datasets: [{
                    label: 'Visits',
                    data: %[15]s,
                    borderColor: '#60a5fa',
                    backgroundColor: 'rgba(96, 165, 250, 0.1)',
                    fill: true,
                    tension: 0.3
                }]
            },
            options: {
                ...chartOptions,
                scales: {
                    x: { ticks: { color: '#9ca3af' }, grid: { color: 'rgba(255, 255, 255, 0.05)' } },
                    y: { ticks: { color: '#9ca3af' }, grid: { color: 'rgba(255, 255, 255, 0.05)' } }
                }
            }
        });

        // Hourly
        new Chart(document.getElementById('hourlyChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: ['00', '01', '02', '03', '04', '05', '06', '07', '08', '09', '10', '11', '12', '13', '14', '15', '16', '17', '18', '19', '20', '21', '22', '23'],
                datasets: [{
                    label: 'Visits',
                    data: %[16]s,
                    backgroundColor: '#818cf8',
                    borderRadius: 4
                }]
            },
            options: {
                ...chartOptions,
                scales: {
                    x: { ticks: { color: '#9ca3af' }, grid: { color: 'rgba(255, 255, 255, 0.05)' } },
                    y: { ticks: { color: '#9ca3af' }, grid: { color: 'rgba(255, 255, 255, 0.05)' } }
                }
            }
        });

        // Security Donut
        new Chart(document.getElementById('securityChart').getContext('2d'), {
            type: 'doughnut',
            data: {
                labels: ['Secure (HTTPS)', 'Insecure (HTTP)'],
                datasets: [{
                    data: [%[7]d, %[8]d],
                    backgroundColor: ['#10b981', '#ef4444'],
                    borderWidth: 0
                }]
            },
            options: chartOptions
        });

        // TLDs
        new Chart(document.getElementById('tldChart').getContext('2d'), {
            type: 'pie',
            data: {
                labels: %[17]s,
                datasets: [{
                    data: %[18]s,
                    backgroundColor: ['#6366f1', '#a855f7', '#ec4899', '#3b82f6', '#14b8a6'],
                    borderWidth: 0
                }]
            },
            options: chartOptions
        });

        // Continent
        new Chart(document.getElementById('continentChart').getContext('2d'), {
            type: 'doughnut',
            data: {
                labels: %[19]s,
                datasets: [{
                    data: %[20]s,
                    backgroundColor: ['#f59e0b', '#10b981', '#3b82f6', '#ec4899', '#8b5cf6', '#06b6d4'],
                    borderWidth: 0
                }]
            },
            options: chartOptions
        });

        // Navigation Mode
        new Chart(document.getElementById('navChart').getContext('2d'), {
            type: 'polarArea',
            data: {
                labels: %[21]s,
                datasets: [{
                    data: %[22]s,
                    backgroundColor: ['rgba(99, 102, 241, 0.6)', 'rgba(168, 85, 247, 0.6)', 'rgba(236, 72, 153, 0.6)', 'rgba(59, 130, 246, 0.6)', 'rgba(20, 184, 166, 0.6)'],
                    borderWidth: 0
                }]
            },
            options: chartOptions
        });

        // Browsers
        new Chart(document.getElementById('browserChart').getContext('2d'), {
            type: 'bar',
            data: {
                labels: %[23]s,
                datasets: [{
                    label: 'Visits',
                    data: %[24]s,
                    backgroundColor: '#a855f7',
                    borderRadius: 4
                }]
            },
            options: {
                ...chartOptions,
                indexAxis: 'y'
            }
        });
    </script>
</body>
</html>`

	// Build rows for domains
	domainRows := ""
	for _, d := range top10Domains {
		domainRows += fmt.Sprintf(`<tr>
            <td class="px-4 py-2 font-medium text-gray-300">%[1]s</td>
            <td class="px-4 py-2 text-right font-mono text-blue-400">%[2]d</td>
        </tr>`, d.Domain, d.Count)
	}
	if len(top10Domains) == 0 {
		domainRows = `<tr><td colspan="2" class="px-4 py-2 text-gray-500 text-center">No domains found</td></tr>`
	}

	// Build rows for queries
	queryRows := ""
	for _, q := range top10Queries {
		displayQ := q.Query
		if len(displayQ) > 50 {
			displayQ = displayQ[:47] + "..."
		}
		queryRows += fmt.Sprintf(`<tr>
            <td class="px-4 py-2 font-medium text-gray-300">%[1]s</td>
            <td class="px-4 py-2 text-right font-mono text-indigo-400">%[2]d</td>
        </tr>`, htmlEscape(displayQ), q.Count)
	}
	if len(top10Queries) == 0 {
		queryRows = `<tr><td colspan="2" class="px-4 py-2 text-gray-500 text-center">No search queries found</td></tr>`
	}

	// Security Warning banner if any plaintext password exists
	alertBanner := ""
	if basicAuthPlaintextCount > 0 {
		alertBanner = fmt.Sprintf(`
        <div class="flex items-center p-4 mb-4 rounded-lg bg-red-900/30 text-red-400 border border-red-500/20" role="alert">
            <svg class="flex-shrink-0 w-5 h-5 mr-3" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" fill="currentColor" viewBox="0 0 20 20">
                <path d="M10 .5a9.5 9.5 0 1 0 9.5 9.5A9.51 9.51 0 0 0 10 .5ZM10 15a1 1 0 1 1 0-2 1 1 0 0 1 0 2Zm1-4a1 1 0 0 1-2 0V6a1 1 0 0 1 2 0v5Z"/>
            </svg>
            <div>
                <span class="font-bold">Security Alert!</span> Plaintext Basic Auth credentials (passwords) were detected in <span class="font-bold font-mono">%d</span> history entries. Consider using the <code class="font-mono bg-red-950 px-1 py-0.5 rounded">--censor</code> / <code class="font-mono bg-red-950 px-1 py-0.5 rounded">-x</code> flag to redact credentials.
            </div>
        </div>`, basicAuthPlaintextCount)
	}

	httpsPct := 0.0
	if secureCount+insecureCount > 0 {
		httpsPct = float64(secureCount) / float64(secureCount+insecureCount) * 100
	}

	extPct := 0.0
	if totalVisits > 0 {
		extPct = float64(externalNetworkCount) / float64(totalVisits) * 100
	}

	// Execute template
	_, err := fmt.Fprintf(w, htmlTemplate,
		fromStr,                  // 1
		toStr,                    // 2
		alertBanner,              // 3
		totalVisits,              // 4
		len(domainCounts),        // 5
		httpsPct,                 // 6
		secureCount,              // 7
		insecureCount,            // 8
		extPct,                   // 9
		localNetworkCount,        // 10
		externalNetworkCount,     // 11
		domainRows,               // 12
		queryRows,                // 13
		datesJSON,                // 14
		dateVisitsJSON,           // 15
		hourlyJSON,               // 16
		tldTypeKeysJSON,          // 17
		tldTypeValsJSON,          // 18
		continentKeysJSON,        // 19
		continentValsJSON,        // 20
		navKeysJSON,              // 21
		navValsJSON,              // 22
		browserKeysJSON,          // 23
		browserValsJSON,          // 24
	)
	return err
}

func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
