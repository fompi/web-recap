package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rzolkos/web-recap/internal/models"
)

// Ingest connects to the specified database URL, creates the tables/collections
// dynamically based on the chosen mode, and inserts the entries.
func Ingest(connectStr string, entries []models.HistoryEntry, conflictStrategy string, mode string, flat bool) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conflictStrategy = strings.ToLower(strings.TrimSpace(conflictStrategy))
	if conflictStrategy == "" {
		conflictStrategy = "skip"
	}
	if conflictStrategy != "skip" && conflictStrategy != "replace" {
		return 0, fmt.Errorf("invalid conflict strategy: %s. Must be 'skip' or 'replace'", conflictStrategy)
	}

	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case "split", "merged", "both":
		// Keep as is
	default:
		mode = "merged"
	}

	if strings.HasPrefix(connectStr, "mongodb://") {
		return ingestMongoDB(ctx, connectStr, entries, conflictStrategy, mode, flat)
	}

	var driver string
	var dsn string

	if strings.HasPrefix(connectStr, "sqlite://") {
		driver = "sqlite"
		dsn = strings.TrimPrefix(connectStr, "sqlite://")
	} else if strings.HasPrefix(connectStr, "sqlite3://") {
		driver = "sqlite"
		dsn = strings.TrimPrefix(connectStr, "sqlite3://")
	} else if strings.HasPrefix(connectStr, "postgres://") || strings.HasPrefix(connectStr, "postgresql://") {
		driver = "postgres"
		dsn = connectStr
	} else if strings.HasPrefix(connectStr, "mysql://") {
		driver = "mysql"
		var err error
		dsn, err = parseMySQLDSN(connectStr)
		if err != nil {
			return 0, fmt.Errorf("invalid mysql connection string: %v", err)
		}
	} else {
		driver = "sqlite"
		dsn = connectStr
	}

	return ingestSQL(driver, dsn, entries, conflictStrategy, mode, flat)
}
