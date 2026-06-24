package utils

import (
	"net"
	"net/url"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// URLParts represents the decomposed components of a parsed URL
type URLParts struct {
	Scheme      string
	Username    string
	Password    string
	FQDN        string
	DomainName  string
	Subdomain   string
	TLD         string
	Port        string
	Path        string
	QueryParams string
}

// IsLocal returns true if the host corresponds to localhost, a private network IP,
// or a local domain name without dots or ending in .local/.lan.
func IsLocal(host string) bool {
	h := host
	if sh, _, err := net.SplitHostPort(host); err == nil {
		h = sh
	}
	h = strings.ToLower(strings.TrimSpace(h))
	if h == "" {
		return false
	}
	if h == "localhost" || h == "::1" || strings.HasSuffix(h, ".local") || strings.HasSuffix(h, ".lan") {
		return true
	}
	// If no dots, it's a local hostname (e.g., router, nas, homeassistant)
	if !strings.Contains(h, ".") {
		return true
	}
	// Check if it is an IP address
	if ip := net.ParseIP(h); ip != nil {
		return ip.IsLoopback() || ip.IsPrivate()
	}
	return false
}

// GetTLDType classifies a TLD into ccTLD, gTLD, or modern/new gTLD
func GetTLDType(tld string) string {
	tld = strings.ToLower(strings.TrimPrefix(tld, "."))
	if tld == "" {
		return "other"
	}
	parts := strings.Split(tld, ".")
	lastPart := parts[len(parts)-1]

	// Legacy generic TLDs
	legacyGTLDs := map[string]bool{
		"com": true, "net": true, "org": true, "edu": true,
		"gov": true, "mil": true, "int": true, "info": true,
		"biz": true,
	}
	if legacyGTLDs[tld] || legacyGTLDs[lastPart] {
		return "gTLD"
	}

	// Overrides for common ccTLDs used as generic/modern tech domains
	modernOverrides := map[string]bool{
		"co": true, "me": true, "tv": true, "cc": true, "fm": true, "io": true, "ai": true,
	}
	if modernOverrides[tld] || modernOverrides[lastPart] {
		return "modern"
	}

	// Country Code TLDs are exactly 2 letters
	if len(lastPart) == 2 {
		return "ccTLD"
	}

	return "modern"
}

var continentMap = map[string]string{
	// Europe (EU)
	"es": "Europe", "fr": "Europe", "it": "Europe", "de": "Europe",
	"uk": "Europe", "nl": "Europe", "be": "Europe", "ch": "Europe",
	"at": "Europe", "dk": "Europe", "se": "Europe", "no": "Europe",
	"fi": "Europe", "ie": "Europe", "pl": "Europe", "pt": "Europe",
	"gr": "Europe", "cz": "Europe", "hu": "Europe", "ro": "Europe",
	"bg": "Europe", "sk": "Europe", "hr": "Europe", "si": "Europe",
	"lt": "Europe", "lv": "Europe", "ee": "Europe", "is": "Europe",
	"lu": "Europe", "ru": "Europe", "ua": "Europe", "by": "Europe",
	"md": "Europe", "al": "Europe", "ba": "Europe", "me": "Europe",
	"mk": "Europe", "rs": "Europe", "eu": "Europe", "gg": "Europe",
	"je": "Europe", "im": "Europe",

	// North America (NA)
	"us": "North America", "ca": "North America", "mx": "North America",
	"gl": "North America", "pr": "North America", "cr": "North America",
	"pa": "North America", "ni": "North America", "gt": "North America",
	"hn": "North America", "sv": "North America", "cu": "North America",
	"jm": "North America", "bs": "North America",

	// South America (SA)
	"br": "South America", "ar": "South America", "cl": "South America",
	"co": "South America", "pe": "South America", "ve": "South America",
	"ec": "South America", "bo": "South America", "py": "South America",
	"uy": "South America",

	// Asia (AS)
	"cn": "Asia", "jp": "Asia", "kr": "Asia", "in": "Asia",
	"id": "Asia", "my": "Asia", "sg": "Asia", "th": "Asia",
	"vn": "Asia", "ph": "Asia", "tw": "Asia", "hk": "Asia",
	"pk": "Asia", "bd": "Asia", "lk": "Asia", "ir": "Asia",
	"tr": "Asia", "il": "Asia", "sa": "Asia", "ae": "Asia",
	"kp": "Asia", "kh": "Asia", "la": "Asia", "mm": "Asia",
	"jo": "Asia", "lb": "Asia", "sy": "Asia", "ye": "Asia",
	"om": "Asia", "qa": "Asia", "kw": "Asia", "bh": "Asia",
	"asia": "Asia",

	// Africa (AF)
	"za": "Africa", "ke": "Africa", "ng": "Africa", "eg": "Africa",
	"ma": "Africa", "dz": "Africa", "tn": "Africa", "ly": "Africa",
	"sd": "Africa", "et": "Africa", "tz": "Africa", "ug": "Africa",
	"gh": "Africa", "ci": "Africa", "sn": "Africa", "mu": "Africa",
	"mg": "Africa", "so": "Africa", "ao": "Africa",

	// Oceania (OC)
	"au": "Oceania", "nz": "Oceania", "fj": "Oceania", "pg": "Oceania",
	"vu": "Oceania", "ws": "Oceania", "to": "Oceania", "tv": "Oceania",
	"cc": "Oceania", "cx": "Oceania", "nf": "Oceania",
}

// GetContinent maps a TLD (especially a ccTLD) to its geographic continent.
// Returns "Global / Generic" if it is not a geographic TLD or is not mapped.
func GetContinent(tld string) string {
	tld = strings.ToLower(strings.TrimPrefix(tld, "."))
	if tld == "" {
		return "Global / Generic"
	}
	parts := strings.Split(tld, ".")
	lastPart := parts[len(parts)-1]

	if continent, ok := continentMap[lastPart]; ok {
		return continent
	}
	return "Global / Generic"
}

// DeconstructURL parses a URL and decomposes it into metadata fields.
func DeconstructURL(urlStr string) URLParts {
	u, err := url.Parse(urlStr)
	if err != nil {
		return URLParts{}
	}

	parts := URLParts{
		Scheme:      u.Scheme,
		Path:        u.Path,
		QueryParams: u.RawQuery,
	}

	if u.User != nil {
		parts.Username = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			parts.Password = pass
		}
	}

	host := u.Host
	var port string
	if sh, sp, err := net.SplitHostPort(u.Host); err == nil {
		host = sh
		port = sp
	}
	parts.FQDN = host
	parts.Port = port

	if host == "" {
		return parts
	}

	if IsLocal(host) {
		parts.DomainName = host
		return parts
	}

	// Try to get registered domain (eTLD+1)
	baseDomain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err == nil {
		parts.DomainName = baseDomain
		tld, _ := publicsuffix.PublicSuffix(host)
		parts.TLD = tld
		if len(host) > len(baseDomain) {
			parts.Subdomain = host[:len(host)-len(baseDomain)-1]
		}
	} else {
		// Fallback
		parts.DomainName = host
		if idx := strings.LastIndex(host, "."); idx != -1 {
			parts.TLD = host[idx+1:]
		}
	}

	return parts
}
