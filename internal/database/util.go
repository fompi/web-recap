package database

import (
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"
)

// HasColumn checks if a column exists in a given table
func HasColumn(db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.Query("SELECT * FROM " + tableName + " LIMIT 0")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return false, err
	}

	for _, col := range cols {
		if strings.EqualFold(col, columnName) {
			return true, nil
		}
	}
	return false, nil
}


// ConvertChromeTimestamp converts Chrome's timestamp format (microseconds since 1601-01-01) to Unix time
func ConvertChromeTimestamp(chromeTime int64) time.Time {
	// Chrome timestamp is in microseconds since 1601-01-01
	// Unix epoch is 1970-01-01
	// Difference: 11644473600 seconds = 11644473600000000 microseconds
	const chromeEpochDiff = 11644473600

	if chromeTime == 0 {
		return time.Time{}
	}

	unixSeconds := (chromeTime / 1000000) - chromeEpochDiff
	return time.Unix(unixSeconds, 0).UTC()
}

// ConvertFirefoxTimestamp converts Firefox's timestamp format (microseconds since epoch) to Unix time
func ConvertFirefoxTimestamp(firefoxTime int64) time.Time {
	if firefoxTime == 0 {
		return time.Time{}
	}

	// Firefox uses microseconds since Unix epoch
	unixSeconds := firefoxTime / 1000000
	unixNanos := (firefoxTime % 1000000) * 1000
	return time.Unix(unixSeconds, unixNanos).UTC()
}

// ConvertSafariTimestamp converts Safari's timestamp format (seconds since 2001-01-01) to Unix time
func ConvertSafariTimestamp(safariTime float64) time.Time {
	// Safari uses seconds since 2001-01-01
	// Unix epoch is 1970-01-01
	// Difference: 978307200 seconds
	const safariEpochDiff = 978307200

	unixSeconds := int64(safariTime) + safariEpochDiff
	unixNanos := int64((safariTime - float64(int64(safariTime))) * 1e9)
	return time.Unix(unixSeconds, unixNanos).UTC()
}

// ExtractDomain extracts the domain from a URL string
func ExtractDomain(urlStr string) string {
	if urlStr == "" {
		return ""
	}

	// Try to parse as URL
	u, err := url.Parse(urlStr)
	if err != nil {
		// If parsing fails, try to extract domain manually
		if strings.HasPrefix(urlStr, "http://") || strings.HasPrefix(urlStr, "https://") {
			parts := strings.Split(urlStr, "/")
			if len(parts) > 2 {
				return parts[2]
			}
		}
		return urlStr
	}

	if u.Host != "" {
		return u.Host
	}

	return urlStr
}

// CopyDatabaseWithWAL copies the main database file and its auxiliary SQLite files (-wal, -shm, -journal)
// if they exist, to prevent loss of uncheckpointed data from running browsers.
// It returns the temporary database path, a cleanup function, and any error encountered.
func CopyDatabaseWithWAL(srcPath string, prefix string) (string, func(), error) {
	src, err := os.Open(srcPath)
	if err != nil {
		if os.IsPermission(err) || strings.Contains(err.Error(), "operation not permitted") {
			return "", nil, fmt.Errorf("permission denied reading database: please check file permissions or grant Full Disk Access (path: %s)", srcPath)
		}
		return "", nil, err
	}
	defer src.Close()

	dst, err := os.CreateTemp("", prefix+"-*.db")
	if err != nil {
		return "", nil, err
	}
	tmpPath := dst.Name()
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(tmpPath)
		return "", nil, err
	}

	var copiedAuxFiles []string
	cleanup := func() {
		os.Remove(tmpPath)
		for _, f := range copiedAuxFiles {
			os.Remove(f)
		}
	}

	// Try to copy WAL, SHM, and journal files if they exist
	auxSuffixes := []string{"-wal", "-shm", "-journal"}
	for _, suffix := range auxSuffixes {
		auxSrcPath := srcPath + suffix
		if _, err := os.Stat(auxSrcPath); err == nil {
			auxDstPath := tmpPath + suffix
			if err := copyFile(auxSrcPath, auxDstPath); err == nil {
				copiedAuxFiles = append(copiedAuxFiles, auxDstPath)
			}
		}
	}

	return tmpPath, cleanup, nil
}

func copyFile(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}
