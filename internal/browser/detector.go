package browser

import (
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	_ "modernc.org/sqlite"
)

// Detector detects available browsers on the system
type Detector struct {
	HomeDir string
}

// NewDetector creates a new browser detector
func NewDetector() *Detector {
	return &Detector{}
}

// NewDetectorForUser creates a new browser detector for a specific home directory
func NewDetectorForUser(homeDir string) *Detector {
	return &Detector{HomeDir: homeDir}
}

func (d *Detector) getHomeDir() (string, error) {
	if d.HomeDir != "" {
		return d.HomeDir, nil
	}
	return os.UserHomeDir()
}

// Detect returns a list of all detected browser profiles
func (d *Detector) Detect() []Browser {
	var browsers []Browser

	home, err := d.getHomeDir()
	if err != nil {
		return browsers
	}

	// Check each browser type
	for _, bType := range []Type{Chrome, Chromium, Edge, Brave, Firefox, Safari} {
		basePath, err := GetDatabasePath(bType, home)
		if err != nil {
			continue
		}

		// Safari profiles
		if bType == Safari {
			if fileExists(basePath) {
				browsers = append(browsers, Browser{
					Type:    Safari,
					Name:    "Safari",
					Path:    basePath,
					Profile: "Default",
				})
			}

			if runtime.GOOS == "darwin" {
				profilesDir := filepath.Join(home, "Library/Containers/com.apple.Safari/Data/Library/Safari/Profiles")
				tabsDBPath := filepath.Join(home, "Library/Containers/com.apple.Safari/Data/Library/Safari/SafariTabs.db")
				
				profileNames := parseSafariProfiles(tabsDBPath)
				
				entries, err := os.ReadDir(profilesDir)
				if err == nil {
					for _, entry := range entries {
						if entry.IsDir() {
							uuid := entry.Name()
							historyPath := filepath.Join(profilesDir, uuid, "History.db")
							if fileExists(historyPath) {
								profileName := uuid
								if name, ok := profileNames[uuid]; ok && name != "" {
									profileName = name
								}
								browsers = append(browsers, Browser{
									Type:    Safari,
									Name:    "Safari",
									Path:    historyPath,
									Profile: profileName,
								})
							}
						}
					}
				}
			}
			continue
		}

		// Firefox has profile subdirectories containing places.sqlite
		if bType == Firefox {
			searchDir := basePath
			profilesSubdir := filepath.Join(basePath, "Profiles")
			if fileExists(profilesSubdir) {
				searchDir = profilesSubdir
			}

			profileNames := parseFirefoxProfiles(basePath)

			entries, err := os.ReadDir(searchDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						placesPath := filepath.Join(searchDir, entry.Name(), "places.sqlite")
						if fileExists(placesPath) {
							profileName := entry.Name()
							if name, ok := profileNames[entry.Name()]; ok && name != "" {
								profileName = name
							}
							browsers = append(browsers, Browser{
								Type:    Firefox,
								Name:    "Firefox",
								Path:    placesPath,
								Profile: profileName,
							})
						}
					}
				}
			}
			continue
		}

		// Chrome, Chromium, Edge, Brave profiles
		// basePath is UserDataDir/Default/History
		userDataDir := filepath.Dir(filepath.Dir(basePath))
		profileNames := parseLocalState(userDataDir)

		// 1. Check Default profile
		defaultPath := filepath.Join(userDataDir, "Default", "History")
		if fileExists(defaultPath) {
			profileName := "Default"
			if name, ok := profileNames["Default"]; ok && name != "" {
				profileName = name
			}
			browsers = append(browsers, Browser{
				Type:    bType,
				Name:    getBrowserName(bType),
				Path:    defaultPath,
				Profile: profileName,
			})
		}

		// 2. Check other Profile directories (e.g. "Profile 1", "Profile 2")
		entries, err := os.ReadDir(userDataDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					name := entry.Name()
					if strings.HasPrefix(name, "Profile ") {
						profilePath := filepath.Join(userDataDir, name, "History")
						if fileExists(profilePath) {
							profileName := name
							if displayName, ok := profileNames[name]; ok && displayName != "" {
								profileName = displayName
							}
							browsers = append(browsers, Browser{
								Type:    bType,
								Name:    getBrowserName(bType),
								Path:    profilePath,
								Profile: profileName,
							})
						}
					}
				}
			}
		}
	}

	return browsers
}

// GetBrowser returns a specific browser profile, detecting if necessary
func (d *Detector) GetBrowser(browserType Type) (*Browser, error) {
	if browserType == Auto {
		browsers := d.Detect()
		if len(browsers) == 0 {
			return nil, ErrDatabaseNotFound
		}
		return &browsers[0], nil
	}

	// For specific browsers, return all detected profiles of that type
	// If the type is chrome, we find all chrome profiles.
	// Since GetBrowser signature returns (*Browser, error), we return the first detected profile.
	// We will handle query resolution for all profiles in the command runner.
	browsers := d.Detect()
	for _, b := range browsers {
		if b.Type == browserType {
			return &b, nil
		}
	}

	return nil, ErrDatabaseNotFound
}

func getBrowserName(bType Type) string {
	switch bType {
	case Chrome:
		return "Google Chrome"
	case Chromium:
		return "Chromium"
	case Edge:
		return "Microsoft Edge"
	case Brave:
		return "Brave"
	case Firefox:
		return "Firefox"
	case Safari:
		return "Safari"
	default:
		return string(bType)
	}
}

func parseLocalState(userDataDir string) map[string]string {
	names := make(map[string]string)
	localStatePath := filepath.Join(userDataDir, "Local State")
	data, err := os.ReadFile(localStatePath)
	if err != nil {
		return names
	}
	var state struct {
		Profile struct {
			InfoCache map[string]struct {
				Name string `json:"name"`
			} `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &state); err == nil {
		for k, v := range state.Profile.InfoCache {
			if v.Name != "" {
				names[k] = v.Name
			}
		}
	}
	return names
}

func parseFirefoxProfiles(basePath string) map[string]string {
	names := make(map[string]string)
	iniPath := filepath.Join(basePath, "profiles.ini")
	data, err := os.ReadFile(iniPath)
	if err != nil {
		return names
	}

	lines := strings.Split(string(data), "\n")
	var currentName string
	var currentPath string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if currentName != "" && currentPath != "" {
				profileDir := filepath.Base(currentPath)
				names[profileDir] = currentName
			}
			currentName = ""
			currentPath = ""
			continue
		}
		if strings.HasPrefix(line, "Name=") {
			currentName = strings.TrimPrefix(line, "Name=")
		} else if strings.HasPrefix(line, "Path=") {
			currentPath = strings.TrimPrefix(line, "Path=")
		}
	}
	if currentName != "" && currentPath != "" {
		profileDir := filepath.Base(currentPath)
		names[profileDir] = currentName
	}
	return names
}

func parseSafariProfiles(tabsDBPath string) map[string]string {
	names := make(map[string]string)
	
	tempDB, err := copyTempFile(tabsDBPath)
	if err != nil {
		return names
	}
	defer os.Remove(tempDB)

	db, err := sql.Open("sqlite", tempDB)
	if err != nil {
		return names
	}
	defer db.Close()

	rows, err := db.Query("SELECT external_uuid, title FROM bookmarks WHERE external_uuid IS NOT NULL AND external_uuid != '' AND type = 1 AND subtype = 2")
	if err != nil {
		rows, err = db.Query("SELECT external_uuid, title FROM bookmarks WHERE external_uuid IS NOT NULL AND external_uuid != ''")
	}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var uuid, title string
			if err := rows.Scan(&uuid, &title); err == nil && uuid != "" && title != "" {
				names[uuid] = title
			}
		}
	}
	return names
}

func copyTempFile(srcPath string) (string, error) {
	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.CreateTemp("", "web-recap-*.db")
	if err != nil {
		return "", err
	}
	tmpFile := dst.Name()
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		os.Remove(tmpFile)
		return "", err
	}
	return tmpFile, nil
}
