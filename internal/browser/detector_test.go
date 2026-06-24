package browser

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDetector_Detect(t *testing.T) {
	// Temporarily override platform controls
	oldOS := currentOS
	oldIsDarwin := isDarwinOS
	currentOS = "darwin"
	isDarwinOS = true
	defer func() {
		currentOS = oldOS
		isDarwinOS = oldIsDarwin
	}()

	tempDir, err := os.MkdirTemp("", "detector-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create structures for Chrome, Firefox, and Safari
	chromeUserDataDir := filepath.Join(tempDir, "Library/Application Support/Google/Chrome")
	firefoxUserDataDir := filepath.Join(tempDir, "Library/Application Support/Firefox")
	safariDir := filepath.Join(tempDir, "Library/Safari")

	// 1. Setup Mock Chrome
	chromeDefaultHistory := filepath.Join(chromeUserDataDir, "Default", "History")
	chromeProfile1History := filepath.Join(chromeUserDataDir, "Profile 1", "History")
	if err := os.MkdirAll(filepath.Dir(chromeDefaultHistory), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(chromeProfile1History), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chromeDefaultHistory, []byte("chrome default history"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(chromeProfile1History, []byte("chrome profile 1 history"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write Chrome Local State JSON
	localStateData := map[string]interface{}{
		"profile": map[string]interface{}{
			"info_cache": map[string]interface{}{
				"Default": map[string]interface{}{
					"name": "Work",
				},
				"Profile 1": map[string]interface{}{
					"name": "Personal",
				},
			},
		},
	}
	localStateBytes, err := json.Marshal(localStateData)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chromeUserDataDir, "Local State"), localStateBytes, 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Setup Mock Firefox
	firefoxProfileDir := filepath.Join(firefoxUserDataDir, "Profiles", "abc.default")
	if err := os.MkdirAll(firefoxProfileDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(firefoxProfileDir, "places.sqlite"), []byte("firefox places"), 0644); err != nil {
		t.Fatal(err)
	}

	// Write profiles.ini
	profilesIniContent := `
[Profile0]
Name=default-release
IsRelative=1
Path=Profiles/abc.default

[Profile1]
Name=other-profile
IsRelative=1
Path=Profiles/xyz.other
`
	if err := os.WriteFile(filepath.Join(firefoxUserDataDir, "profiles.ini"), []byte(profilesIniContent), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Setup Mock Safari
	if err := os.MkdirAll(safariDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(safariDir, "History.db"), []byte("safari history"), 0644); err != nil {
		t.Fatal(err)
	}

	// Safari profiles dir
	safariProfilesDir := filepath.Join(safariDir, "Profiles", "uuid123")
	if err := os.MkdirAll(safariProfilesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(safariProfilesDir, "History.db"), []byte("safari profile history"), 0644); err != nil {
		t.Fatal(err)
	}

	// SafariTabs.db
	tabsDBPath := filepath.Join(safariDir, "SafariTabs.db")
	db, err := sql.Open("sqlite", tabsDBPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE bookmarks (
			external_uuid TEXT,
			title TEXT,
			type INTEGER,
			subtype INTEGER
		);
		INSERT INTO bookmarks (external_uuid, title, type, subtype) VALUES ('uuid123', 'My Custom Safari Profile', 1, 2);
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	// Run detection
	detector := NewDetectorForUser(tempDir)
	browsers := detector.Detect()

	// Verify we detected:
	// - Chrome Default (named "Work")
	// - Chrome Profile 1 (named "Personal")
	// - Firefox default-release (abc.default)
	// - Safari Default
	// - Safari Profile (named "My Custom Safari Profile")
	
	detectedMap := make(map[string]Browser)
	for _, b := range browsers {
		detectedMap[string(b.Type)+":"+b.Profile] = b
	}

	// Chrome
	if b, ok := detectedMap["chrome:Work"]; !ok || b.Name != "Google Chrome" {
		t.Errorf("missing or incorrect Chrome default profile: %+v", browsers)
	}
	if b, ok := detectedMap["chrome:Personal"]; !ok || b.Name != "Google Chrome" {
		t.Errorf("missing or incorrect Chrome Profile 1: %+v", browsers)
	}

	// Firefox
	if b, ok := detectedMap["firefox:default-release"]; !ok || b.Name != "Firefox" {
		t.Errorf("missing or incorrect Firefox default-release: %+v", browsers)
	}

	// Safari
	if b, ok := detectedMap["safari:Default"]; !ok || b.Name != "Safari" {
		t.Errorf("missing or incorrect Safari Default: %+v", browsers)
	}
	if b, ok := detectedMap["safari:My Custom Safari Profile"]; !ok || b.Name != "Safari" {
		t.Errorf("missing or incorrect Safari Custom Profile: %+v", browsers)
	}
}

func TestDetector_EmptyHomeDirError(t *testing.T) {
	// Unset HOME and USERPROFILE, then call Detect() to ensure getHomeDir() returns an error and Detect returns empty slice.
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")
	defer func() {
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
	}()

	detector := Detector{}
	browsers := detector.Detect()
	if len(browsers) != 0 {
		t.Errorf("expected no browsers detected on error, got %d", len(browsers))
	}
}

func TestDetector_GetBrowserName(t *testing.T) {
	tests := []struct {
		bType    Type
		expected string
	}{
		{Chrome, "Google Chrome"},
		{Chromium, "Chromium"},
		{Edge, "Microsoft Edge"},
		{Brave, "Brave"},
		{Firefox, "Firefox"},
		{Safari, "Safari"},
		{Auto, "auto"},
		{Type("custom"), "custom"},
	}

	for _, tc := range tests {
		got := GetBrowserName(tc.bType)
		if got != tc.expected {
			t.Errorf("GetBrowserName(%q) = %q; expected %q", tc.bType, got, tc.expected)
		}
	}
}

func TestDetector_Detect_LinuxFallback(t *testing.T) {
	// Set currentOS to "linux" and run Detect to cover Safari ErrBrowserNotAvailable / continue path
	oldOS := currentOS
	currentOS = "linux"
	defer func() { currentOS = oldOS }()

	tempDir, err := os.MkdirTemp("", "detector-linux-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	detector := NewDetectorForUser(tempDir)
	detector.Detect() // Should execute without issues and ignore Safari
}

func TestDetector_Detect_SafariFallback(t *testing.T) {
	oldOS := currentOS
	oldIsDarwin := isDarwinOS
	currentOS = "darwin"
	isDarwinOS = true
	defer func() {
		currentOS = oldOS
		isDarwinOS = oldIsDarwin
	}()

	tempDir, err := os.MkdirTemp("", "detector-safari-fallback-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create Safari profiles dir inside Containers path
	safariContainerDir := filepath.Join(tempDir, "Library/Containers/com.apple.Safari/Data/Library/Safari")
	safariProfilesDir := filepath.Join(safariContainerDir, "Profiles", "uuid456")
	if err := os.MkdirAll(safariProfilesDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(safariProfilesDir, "History.db"), []byte("container safari profile"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create container tabs DB but without columns type/subtype to force query error and fallback
	tabsDBPath := filepath.Join(safariContainerDir, "SafariTabs.db")
	db, err := sql.Open("sqlite", tabsDBPath)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		CREATE TABLE bookmarks (
			external_uuid TEXT,
			title TEXT
		);
		INSERT INTO bookmarks (external_uuid, title) VALUES ('uuid456', 'Fallback Safari Profile');
	`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	detector := NewDetectorForUser(tempDir)
	browsers := detector.Detect()

	found := false
	for _, b := range browsers {
		if b.Profile == "Fallback Safari Profile" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected to detect fallback Safari profile, but got: %+v", browsers)
	}
}

func TestDetector_ParseFirefoxProfiles_Error(t *testing.T) {
	// Call parseFirefoxProfiles with invalid base path to trigger error
	names := parseFirefoxProfiles("/invalid/path")
	if len(names) != 0 {
		t.Errorf("expected empty names on parse error, got %+v", names)
	}
}

func TestDetector_ParseSafariProfiles_Error(t *testing.T) {
	// Call parseSafariProfiles with invalid path to trigger error
	names := parseSafariProfiles("/invalid/path")
	if len(names) != 0 {
		t.Errorf("expected empty names on parse error, got %+v", names)
	}
}

func TestDetector_CopyTempFile_Error(t *testing.T) {
	// Call copyTempFile with non-existent source to trigger copy error
	_, err := copyTempFile("/invalid/source/file.db")
	if err == nil {
		t.Errorf("expected error copying non-existent file, got nil")
	}
}

func TestDetector_ParseSafariProfiles_SqlOpenError(t *testing.T) {
	oldSqlOpen := sqlOpen
	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		return nil, errors.New("mock sql open error")
	}
	defer func() { sqlOpen = oldSqlOpen }()

	// We need a dummy SafariTabs.db file that copyTempFile succeeds on
	tempDir, err := os.MkdirTemp("", "safari-sqlopen-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dummyDB := filepath.Join(tempDir, "SafariTabs.db")
	if err := os.WriteFile(dummyDB, []byte("dummy db content"), 0644); err != nil {
		t.Fatal(err)
	}

	names := parseSafariProfiles(dummyDB)
	if len(names) != 0 {
		t.Errorf("expected empty names on sql open error, got %+v", names)
	}
}

func TestDetector_CopyTempFile_CreateTempError(t *testing.T) {
	// Temporarily point TMPDIR/TMP/TEMP to a non-existent path to trigger CreateTemp error
	oldTmpDir := os.Getenv("TMPDIR")
	oldTmp := os.Getenv("TMP")
	oldTemp := os.Getenv("TEMP")

	os.Setenv("TMPDIR", "/nonexistent-directory-for-test-xyz")
	os.Setenv("TMP", "/nonexistent-directory-for-test-xyz")
	os.Setenv("TEMP", "/nonexistent-directory-for-test-xyz")

	defer func() {
		os.Setenv("TMPDIR", oldTmpDir)
		os.Setenv("TMP", oldTmp)
		os.Setenv("TEMP", oldTemp)
	}()

	_, err := copyTempFile("/invalid/source/file.db")
	if err == nil {
		t.Errorf("expected error when os.CreateTemp fails, got nil")
	}
}
