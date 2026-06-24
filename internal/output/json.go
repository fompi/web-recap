package output

import (
	"encoding/json"
	"io"

	"github.com/rzolkos/web-recap/internal/models"
)

// FormatJSON writes history entries as JSON to the given writer
func FormatJSON(w io.Writer, entries []models.HistoryEntry, pretty bool) error {
	encoder := json.NewEncoder(w)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	encoder.SetEscapeHTML(false)

	return encoder.Encode(entries)
}

// FormatJSONLines writes history entries as JSON lines (one per line) to the given writer
func FormatJSONLines(w io.Writer, entries []models.HistoryEntry) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)

	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return err
		}
	}

	return nil
}
