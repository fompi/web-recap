package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
	"golang.org/x/term"
)

// FormatCSV writes history entries as CSV to the given writer
func FormatCSV(w io.Writer, entries []models.HistoryEntry) error {
	writer := csv.NewWriter(w)

	// Write header
	header := []string{
		"browser", "profile", "timestamp", "domain",
		"scheme", "username", "fqdn", "domain_name", "subdomain", "tld", "port", "path", "query_params",
		"title", "url", "visit_count",
	}
	_ = writer.Write(header)

	for _, entry := range entries {
		row := []string{
			entry.Browser,
			entry.Profile,
			entry.Timestamp.Format(time.RFC3339),
			entry.Domain,
			entry.Scheme,
			entry.Username,
			entry.FQDN,
			entry.DomainName,
			entry.Subdomain,
			entry.TLD,
			entry.Port,
			entry.Path,
			entry.QueryParams,
			entry.Title,
			entry.URL,
			fmt.Sprintf("%d", entry.VisitCount),
		}
		_ = writer.Write(row)
	}
	writer.Flush()
	return writer.Error()
}

// FormatText writes history entries in a human-readable aligned table format
func FormatText(w io.Writer, entries []models.HistoryEntry) error {
	// Detect terminal width if writing to a terminal
	terminalWidth := -1
	type fdReader interface {
		Fd() uintptr
	}
	if f, ok := w.(fdReader); ok && term.IsTerminal(int(f.Fd())) {
		if width, _, err := term.GetSize(int(f.Fd())); err == nil {
			terminalWidth = width
		}
	}

	maxBrowser := 7 // len("BROWSER")
	maxProfile := 7 // len("PROFILE")
	maxDomain := 6  // len("DOMAIN")

	for _, entry := range entries {
		if len(entry.Browser) > maxBrowser {
			maxBrowser = len(entry.Browser)
		}
		if len(entry.Profile) > maxProfile {
			maxProfile = len(entry.Profile)
		}
		if len(entry.Domain) > maxDomain {
			maxDomain = len(entry.Domain)
		}
	}

	// Calculate truncation limits if we are writing to a terminal
	maxTitleWidth := -1
	maxURLWidth := -1
	if terminalWidth > 0 {
		// Budget estimate for fixed columns (browser, profile, timestamp, domain) plus spacing
		fixedWidth := maxBrowser + maxProfile + 19 + maxDomain + 20
		remaining := terminalWidth - fixedWidth
		if remaining < 30 {
			// Keep small minimum limits if terminal is very narrow
			maxTitleWidth = 15
			maxURLWidth = 20
		} else {
			// Distribute remaining space: 40% to title, 60% to URL
			maxTitleWidth = remaining * 4 / 10
			maxURLWidth = remaining * 6 / 10
			if maxTitleWidth < 15 {
				maxTitleWidth = 15
			}
			if maxURLWidth < 20 {
				maxURLWidth = 20
			}
		}
	}

	// tabwriter uses elastic tab stops to align output columns
	tw := tabwriter.NewWriter(w, 4, 4, 2, ' ', 0)
	
	// Write header
	fmt.Fprintln(tw, "BROWSER\tPROFILE\tTIMESTAMP\tDOMAIN\tTITLE\tURL")
	
	for _, entry := range entries {
		title := truncateString(entry.Title, maxTitleWidth)
		url := truncateString(entry.URL, maxURLWidth)
		
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.Browser,
			entry.Profile,
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			entry.Domain,
			title,
			url,
		)
	}
	
	return tw.Flush()
}

// truncateString truncates a string safely considering UTF-8 runes
func truncateString(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

