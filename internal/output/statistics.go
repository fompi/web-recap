package output

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
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

	for _, entry := range entries {
		domainCounts[entry.Domain]++
		urlCounts[entry.URL]++
		browserCounts[entry.Browser]++

		// Hour in specified location
		entryTimeLocal := entry.Timestamp.In(loc)
		hourlyCounts[entryTimeLocal.Hour()]++
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

	// 4. Print stats header
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
	fmt.Fprintln(w, "Visits by Browser:")
	for b, count := range browserCounts {
		pct := (float64(count) / float64(totalVisits)) * 100
		fmt.Fprintf(w, "  - %-12s %5d (%5.1f%%)\n", strings.Title(b), count, pct)
	}
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

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
		// Truncate URL
		displayURL := u.URL
		if len(displayURL) > 60 {
			displayURL = displayURL[:57] + "..."
		}
		fmt.Fprintf(w, "  %2d. %-60s %5d\n", i+1, displayURL, u.Count)
	}
	fmt.Fprintln(w, "--------------------------------------------------------------------------------")

	// Print hourly activity histogram
	fmt.Fprintln(w, "Hourly Activity Histogram:")
	// Find max hourly count to scale the bar chart (max width: 40 blocks)
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
