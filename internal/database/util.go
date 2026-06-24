package database

import (
	"database/sql"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rzolkos/web-recap/internal/browser"
)

const safariEpochDiff = 978307200

// HasTable checks if a table exists in the SQLite database.
func HasTable(db *sql.DB, tableName string) bool {
	for _, r := range tableName {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&name)
	return err == nil
}

// DecodeTransition decodes Chrome's transition bitmask (low byte) into a normalized visit-type string.
// The low byte of the transition field encodes the core type; upper bits are qualifier flags.
func DecodeTransition(transition int64) string {
	switch transition & 0xff {
	case 0:
		return "link"
	case 1:
		return "typed"
	case 2:
		return "bookmark"
	case 5:
		return "reload"
	case 6:
		return "redirect"
	case 7:
		return "download"
	default:
		return "other"
	}
}

// DecodeFirefoxVisitType maps Firefox's integer visit_type enum to a normalized visit-type string.
// Types 5 and 6 are permanent and temporary redirects respectively; both collapse to "redirect".
func DecodeFirefoxVisitType(visitType int64) string {
	switch visitType {
	case 1:
		return "link"
	case 2:
		return "typed"
	case 3:
		return "bookmark"
	case 5, 6:
		return "redirect"
	case 7:
		return "download"
	case 9:
		return "reload"
	default:
		return "other"
	}
}

// DecodeSafariVisitType infers a normalized visit-type string from Safari's boolean visit flags.
// Safari has no visit-type enum; redirect is the only type distinguishable from the schema.
func DecodeSafariVisitType(redirectSource, redirectDestination int64, synthesized, httpNonGET bool) string {
	if redirectSource != 0 || redirectDestination != 0 {
		return "redirect"
	}
	if synthesized || httpNonGET {
		return "other"
	}
	return "link"
}

// HasColumn checks if a column exists in a given table
func HasColumn(db *sql.DB, tableName, columnName string) (bool, error) {
	// Validate table name to avoid SQL injection
	for _, r := range tableName {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return false, fmt.Errorf("invalid table name: %s", tableName)
		}
	}

	// Check if table exists
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", tableName).Scan(&name)
	if err == sql.ErrNoRows {
		return false, fmt.Errorf("table %s does not exist", tableName)
	} else if err != nil {
		return false, err
	}

	// Query column info
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, dType string
		var notnull, pk int
		var dfltVal interface{}
		if err := rows.Scan(&cid, &name, &dType, &notnull, &dfltVal, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, columnName) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
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
			if err := browser.CopyFile(auxSrcPath, auxDstPath); err == nil {
				copiedAuxFiles = append(copiedAuxFiles, auxDstPath)
			}
		}
	}

	return tmpPath, cleanup, nil
}
