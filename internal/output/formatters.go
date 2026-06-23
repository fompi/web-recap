package output

import (
	"encoding/csv"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
)

// FormatCSV writes history entries as CSV to the given writer
func FormatCSV(w io.Writer, entries []models.HistoryEntry) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	header := []string{"browser", "profile", "timestamp", "domain", "title", "url", "visit_count"}
	if err := writer.Write(header); err != nil {
		return err
	}

	for _, entry := range entries {
		row := []string{
			entry.Browser,
			entry.Profile,
			entry.Timestamp.Format(time.RFC3339),
			entry.Domain,
			entry.Title,
			entry.URL,
			fmt.Sprintf("%d", entry.VisitCount),
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// FormatTable writes history entries in a human-readable aligned table format
func FormatTable(w io.Writer, entries []models.HistoryEntry) error {
	// tabwriter uses elastic tab stops to align output columns
	tw := tabwriter.NewWriter(w, 4, 4, 2, ' ', 0)
	
	// Write header
	fmt.Fprintln(tw, "BROWSER\tPROFILE\tTIMESTAMP\tDOMAIN\tTITLE\tURL")
	
	for _, entry := range entries {
		// Truncate title for clean display
		title := entry.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		
		// Truncate URL for clean display
		url := entry.URL
		if len(url) > 60 {
			url = url[:57] + "..."
		}
		
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
