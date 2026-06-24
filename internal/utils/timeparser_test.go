package utils

import (
	"testing"
	"time"
)

func TestParseTimeHelper(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("failed to load location: %v", err)
	}

	// Reference point for "now": 2026-06-20 12:00:00 EST
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, loc)

	tests := []struct {
		input    string
		expected time.Time
		expectErr bool
	}{
		{
			input:    "start",
			expected: time.Unix(0, 0).UTC(),
		},
		{
			input:    "0",
			expected: time.Unix(0, 0).UTC(),
		},
		{
			input:    "now",
			expected: now,
		},
		{
			input:    "today",
			expected: time.Date(2026, 6, 20, 0, 0, 0, 0, loc),
		},
		{
			input:    "yesterday",
			expected: time.Date(2026, 6, 19, 0, 0, 0, 0, loc),
		},
		{
			input:    "3 days",
			expected: now.AddDate(0, 0, -3),
		},
		{
			input:    "5 hours ago",
			expected: now.Add(-5 * time.Hour),
		},
		{
			input:    "30 minutes",
			expected: now.Add(-30 * time.Minute),
		},
		{
			input:    "2026-06-20T10:15:30Z",
			expected: time.Date(2026, 6, 20, 10, 15, 30, 0, time.UTC),
		},
		{
			input:    "2026-06-20T10:15:30-05:00",
			expected: time.Date(2026, 6, 20, 10, 15, 30, 0, time.FixedZone("EST", -5*3600)),
		},
		{
			input:    "2026-06-20T15:00:00", // parses in loc
			expected: time.Date(2026, 6, 20, 15, 0, 0, 0, loc),
		},
		{
			input:    "2026-06-20 15:00:00", // parses in loc
			expected: time.Date(2026, 6, 20, 15, 0, 0, 0, loc),
		},
		{
			input:    "2026-06-20", // parses in loc
			expected: time.Date(2026, 6, 20, 0, 0, 0, 0, loc),
		},
		{
			input:     "invalid-date",
			expectErr: true,
		},
		{
			input:     "999999999999999999999999999999 days",
			expectErr: true,
		},
		{
			input:    "2026-06-20 10:15:30Z",
			expected: time.Date(2026, 6, 20, 10, 15, 30, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			res, err := ParseTimeHelper(tt.input, now, loc)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !res.Equal(tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, res)
			}
		})
	}
}
