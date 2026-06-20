package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseTimeHelper parses a string representing a date, time, or a relative duration offset (e.g., "3 days", "today", "yesterday", "now", "start")
// and returns a time.Time object in the specified timezone location (loc).
func ParseTimeHelper(val string, now time.Time, loc *time.Location) (time.Time, error) {
	val = strings.TrimSpace(val)
	lowerVal := strings.ToLower(val)

	if lowerVal == "start" || lowerVal == "0" {
		return time.Unix(0, 0).UTC(), nil
	}
	if lowerVal == "now" {
		return now, nil
	}
	if lowerVal == "today" {
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc), nil
	}
	if lowerVal == "yesterday" {
		y := now.AddDate(0, 0, -1)
		return time.Date(y.Year(), y.Month(), y.Day(), 0, 0, 0, 0, loc), nil
	}

	// Match "N days [ago]", "N hours [ago]", "N minutes [ago]"
	re := regexp.MustCompile(`^(\d+)\s*(day|hour|minute)s?(?:\s+ago)?$`)
	if matches := re.FindStringSubmatch(lowerVal); len(matches) == 3 {
		amount, err := strconv.Atoi(matches[1])
		if err != nil {
			return time.Time{}, err
		}
		unit := matches[2]
		switch unit {
		case "day":
			return now.AddDate(0, 0, -amount), nil
		case "hour":
			return now.Add(-time.Duration(amount) * time.Hour), nil
		case "minute":
			return now.Add(-time.Duration(amount) * time.Minute), nil
		}
	}

	// Try different layouts
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}

	// Replace space with T to handle standard ISO8601 formatting variations
	normalizedVal := strings.ReplaceAll(val, " ", "T")
	// If it doesn't contain a timezone suffix, try replacing lower 't' with upper 'T'
	if !strings.Contains(normalizedVal, "Z") && !strings.Contains(normalizedVal, "+") && strings.Count(normalizedVal, "-") >= 2 {
		normalizedVal = strings.ReplaceAll(val, "t", "T")
	}

	for _, layout := range layouts {
		// If layout has timezone specifiers, parse with Parse
		if strings.Contains(layout, "Z") || strings.Contains(layout, "07") {
			if t, err := time.Parse(layout, val); err == nil {
				return t, nil
			}
			if t, err := time.Parse(layout, normalizedVal); err == nil {
				return t, nil
			}
		} else {
			// Otherwise parse using specified location
			if t, err := time.ParseInLocation(layout, val, loc); err == nil {
				return t, nil
			}
			if t, err := time.ParseInLocation(layout, normalizedVal, loc); err == nil {
				return t, nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse date/time value %q", val)
}
