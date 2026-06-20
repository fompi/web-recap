package browser

import (
	"os"
	"path/filepath"
	"strings"
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

		// Safari has only one default profile
		if bType == Safari {
			if fileExists(basePath) {
				browsers = append(browsers, Browser{
					Type:    Safari,
					Name:    "Safari",
					Path:    basePath,
					Profile: "Default",
				})
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

			entries, err := os.ReadDir(searchDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						placesPath := filepath.Join(searchDir, entry.Name(), "places.sqlite")
						if fileExists(placesPath) {
							browsers = append(browsers, Browser{
								Type:    Firefox,
								Name:    "Firefox",
								Path:    placesPath,
								Profile: entry.Name(),
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

		// 1. Check Default profile
		defaultPath := filepath.Join(userDataDir, "Default", "History")
		if fileExists(defaultPath) {
			browsers = append(browsers, Browser{
				Type:    bType,
				Name:    getBrowserName(bType),
				Path:    defaultPath,
				Profile: "Default",
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
							browsers = append(browsers, Browser{
								Type:    bType,
								Name:    getBrowserName(bType),
								Path:    profilePath,
								Profile: name,
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
