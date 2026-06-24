package utils

import (
	"testing"
)

func TestIsLocal(t *testing.T) {
	tests := []struct {
		host     string
		expected bool
	}{
		{"localhost", true},
		{"localhost:8080", true},
		{"127.0.0.1", true},
		{"127.0.0.1:3000", true},
		{"::1", true},
		{"192.168.1.1", true},
		{"10.0.0.5", true},
		{"172.16.0.10", true},
		{"nas.local", true},
		{"router.lan", true},
		{"router", true},
		{"google.com", false},
		{"sub.google.com", false},
		{"8.8.8.8", false},
	}

	for _, tc := range tests {
		got := IsLocal(tc.host)
		if got != tc.expected {
			t.Errorf("IsLocal(%q) = %v; want %v", tc.host, got, tc.expected)
		}
	}
}

func TestGetTLDType(t *testing.T) {
	tests := []struct {
		tld      string
		expected string
	}{
		{"com", "gTLD"},
		{".org", "gTLD"},
		{"es", "ccTLD"},
		{"co.uk", "ccTLD"},
		{"io", "modern"}, // overridden or ccTLD? We set modernOverrides for "io", "ai", "co", "me", "tv", "cc", "fm"
		{"dev", "modern"},
		{"tech", "modern"},
		{"xyz", "modern"},
	}

	for _, tc := range tests {
		got := GetTLDType(tc.tld)
		if got != tc.expected {
			t.Errorf("GetTLDType(%q) = %q; want %q", tc.tld, got, tc.expected)
		}
	}
}

func TestGetContinent(t *testing.T) {
	tests := []struct {
		tld      string
		expected string
	}{
		{"es", "Europe"},
		{"co.uk", "Europe"},
		{"us", "North America"},
		{"br", "South America"},
		{"cn", "Asia"},
		{"za", "Africa"},
		{"au", "Oceania"},
		{"com", "Global / Generic"},
		{"io", "Global / Generic"}, // "io" is not mapped to continent in continentMap
	}

	for _, tc := range tests {
		got := GetContinent(tc.tld)
		if got != tc.expected {
			t.Errorf("GetContinent(%q) = %q; want %q", tc.tld, got, tc.expected)
		}
	}
}

func TestDeconstructURL(t *testing.T) {
	tests := []struct {
		url      string
		expected URLParts
	}{
		{
			"https://user:password@sub.example.co.uk:8080/path/to/page?q=test#anchor",
			URLParts{
				Scheme:     "https",
				Username:   "user",
				Password:   "password",
				FQDN:       "sub.example.co.uk",
				DomainName: "example.co.uk",
				Subdomain:  "sub",
				TLD:        "co.uk",
				Port:       "8080",
				Path:       "/path/to/page",
			},
		},
		{
			"http://localhost:3000/dev",
			URLParts{
				Scheme:     "http",
				FQDN:       "localhost",
				DomainName: "localhost",
				Port:       "3000",
				Path:       "/dev",
			},
		},
	}

	for _, tc := range tests {
		got := DeconstructURL(tc.url)
		if got.Scheme != tc.expected.Scheme ||
			got.Username != tc.expected.Username ||
			got.Password != tc.expected.Password ||
			got.FQDN != tc.expected.FQDN ||
			got.DomainName != tc.expected.DomainName ||
			got.Subdomain != tc.expected.Subdomain ||
			got.TLD != tc.expected.TLD ||
			got.Port != tc.expected.Port ||
			got.Path != tc.expected.Path {
			t.Errorf("DeconstructURL(%q) = %+v; want %+v", tc.url, got, tc.expected)
		}
	}
}
