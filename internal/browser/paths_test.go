package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetLinuxPath(t *testing.T) {
	tests := []struct {
		name      string
		browser   Type
		expectErr bool
		contains  string
	}{
		{
			name:     "Chrome",
			browser:  Chrome,
			contains: ".config/google-chrome",
		},
		{
			name:     "Chromium",
			browser:  Chromium,
			contains: ".config/chromium",
		},
		{
			name:     "Firefox",
			browser:  Firefox,
			contains: ".mozilla/firefox",
		},
		{
			name:      "Safari not available",
			browser:   Safari,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := "/home/testuser"
			path, err := getLinuxPath(home, tt.browser)

			if tt.expectErr && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectErr && !strings.Contains(filepath.ToSlash(path), tt.contains) {
				t.Errorf("expected path to contain %q, got %q", tt.contains, path)
			}
		})
	}
}

func TestGetDarwinPath(t *testing.T) {
	tests := []struct {
		name      string
		browser   Type
		expectErr bool
		contains  string
	}{
		{
			name:     "Chrome",
			browser:  Chrome,
			contains: "Library/Application Support/Google/Chrome",
		},
		{
			name:     "Firefox",
			browser:  Firefox,
			contains: "Library/Application Support/Firefox",
		},
		{
			name:     "Safari",
			browser:  Safari,
			contains: "Library/Safari/History.db",
		},
		{
			name:      "Edge",
			browser:   Edge,
			contains:  "Library/Application Support/Microsoft Edge",
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := "/Users/testuser"
			path, err := getDarwinPath(home, tt.browser)

			if tt.expectErr && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectErr && !strings.Contains(filepath.ToSlash(path), tt.contains) {
				t.Errorf("expected path to contain %q, got %q", tt.contains, path)
			}
		})
	}
}

func TestGetWindowsPath(t *testing.T) {
	tests := []struct {
		name      string
		browser   Type
		expectErr bool
		contains  string
	}{
		{
			name:     "Chrome",
			browser:  Chrome,
			contains: "AppData/Local/Google/Chrome",
		},
		{
			name:     "Chromium",
			browser:  Chromium,
			contains: "AppData/Local/Chromium",
		},
		{
			name:     "Firefox",
			browser:  Firefox,
			contains: "AppData/Roaming/Mozilla/Firefox",
		},
		{
			name:      "Safari not available",
			browser:   Safari,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := "/Users/testuser"
			path, err := getWindowsPath(home, tt.browser)

			if tt.expectErr && err == nil {
				t.Errorf("expected error but got none")
			}

			if !tt.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if !tt.expectErr && !strings.Contains(filepath.ToSlash(path), tt.contains) {
				t.Errorf("expected path to contain %q, got %q", tt.contains, path)
			}
		})
	}
}

func TestGetWindowsPath_EdgeCases(t *testing.T) {
	// Backup env
	oldLocal := os.Getenv("LOCALAPPDATA")
	oldApp := os.Getenv("APPDATA")
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")

	defer func() {
		os.Setenv("LOCALAPPDATA", oldLocal)
		os.Setenv("APPDATA", oldApp)
		os.Setenv("HOME", oldHome)
		os.Setenv("USERPROFILE", oldUserProfile)
	}()

	// 1. home is empty, LOCALAPPDATA and APPDATA are set
	os.Setenv("LOCALAPPDATA", "/local")
	os.Setenv("APPDATA", "/roaming")
	p, err := getWindowsPath("", Chrome)
	if err != nil || !strings.Contains(filepath.ToSlash(p), "/local/Google/Chrome") {
		t.Errorf("expected local path, got %q, error: %v", p, err)
	}

	// 2. home is empty, LOCALAPPDATA and APPDATA are empty, HOME is empty (so os.UserHomeDir fails)
	os.Setenv("LOCALAPPDATA", "")
	os.Setenv("APPDATA", "")
	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")
	_, err = getWindowsPath("", Chrome)
	if err == nil {
		t.Errorf("expected error when home dir cannot be resolved, got nil")
	}

	// 3. home is empty, LOCALAPPDATA and APPDATA are empty, but HOME is set (so os.UserHomeDir succeeds)
	os.Setenv("HOME", "/mockhome")
	p, err = getWindowsPath("", Chrome)
	if err != nil || !strings.Contains(filepath.ToSlash(p), "/mockhome/AppData/Local/Google/Chrome") {
		t.Errorf("expected mock home path, got %q, error: %v", p, err)
	}
}

func TestGetDatabasePath_AllOS(t *testing.T) {
	oldOS := currentOS
	defer func() { currentOS = oldOS }()

	// Test case 1: empty home
	currentOS = "darwin"
	_, err := GetDatabasePath(Chrome, "")
	if err != nil {
		// Might not have user home in some test envs, but should not crash
		t.Logf("GetDatabasePath empty home returned error: %v", err)
	}

	// Test empty home with unset environment variables (HOME and USERPROFILE)
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")
	_, homeErr := GetDatabasePath(Chrome, "")
	if homeErr == nil {
		t.Errorf("expected error when HOME and USERPROFILE are unset, got nil")
	}
	os.Setenv("HOME", oldHome)
	os.Setenv("USERPROFILE", oldUserProfile)

	// Test case 2: darwin
	currentOS = "darwin"
	p, err := GetDatabasePath(Chrome, "/Users/test")
	if err != nil || !strings.Contains(filepath.ToSlash(p), "Library/Application Support/Google/Chrome") {
		t.Errorf("expected darwin Chrome path, got %q, error: %v", p, err)
	}
	// Darwin auto
	p, _ = GetDatabasePath(Auto, "/Users/test")
	if p != "" {
		t.Errorf("expected empty path for Auto, got %q", p)
	}
	// Darwin unknown
	_, err = GetDatabasePath(Type("unknown"), "/Users/test")
	if err != ErrUnknownBrowser {
		t.Errorf("expected ErrUnknownBrowser, got %v", err)
	}

	// Test case 3: linux
	currentOS = "linux"
	p, err = GetDatabasePath(Chrome, "/home/test")
	if err != nil || !strings.Contains(filepath.ToSlash(p), ".config/google-chrome") {
		t.Errorf("expected linux Chrome path, got %q, error: %v", p, err)
	}
	// Linux Chromium
	p, _ = GetDatabasePath(Chromium, "/home/test")
	if !strings.Contains(filepath.ToSlash(p), ".config/chromium") {
		t.Errorf("expected linux Chromium path, got %q", p)
	}
	// Linux Edge
	p, _ = GetDatabasePath(Edge, "/home/test")
	if !strings.Contains(filepath.ToSlash(p), ".config/microsoft-edge") {
		t.Errorf("expected linux Edge path, got %q", p)
	}
	// Linux Brave
	p, _ = GetDatabasePath(Brave, "/home/test")
	if !strings.Contains(filepath.ToSlash(p), ".config/BraveSoftware") {
		t.Errorf("expected linux Brave path, got %q", p)
	}
	// Linux Safari (not available)
	_, err = GetDatabasePath(Safari, "/home/test")
	if err != ErrBrowserNotAvailable {
		t.Errorf("expected ErrBrowserNotAvailable, got %v", err)
	}
	// Linux Auto
	p, _ = GetDatabasePath(Auto, "/home/test")
	if p != "" {
		t.Errorf("expected empty path, got %q", p)
	}
	// Linux unknown
	_, err = GetDatabasePath(Type("unknown"), "/home/test")
	if err != ErrUnknownBrowser {
		t.Errorf("expected ErrUnknownBrowser, got %v", err)
	}

	// Test case 4: windows
	currentOS = "windows"
	p, err = GetDatabasePath(Chrome, "/Users/test")
	if err != nil || !strings.Contains(filepath.ToSlash(p), "AppData/Local/Google/Chrome") {
		t.Errorf("expected windows Chrome path, got %q, error: %v", p, err)
	}
	// Windows Chromium
	p, _ = GetDatabasePath(Chromium, "/Users/test")
	if !strings.Contains(filepath.ToSlash(p), "AppData/Local/Chromium") {
		t.Errorf("expected windows Chromium path, got %q", p)
	}
	// Windows Edge
	p, _ = GetDatabasePath(Edge, "/Users/test")
	if !strings.Contains(filepath.ToSlash(p), "AppData/Local/Microsoft") {
		t.Errorf("expected windows Edge path, got %q", p)
	}
	// Windows Brave
	p, _ = GetDatabasePath(Brave, "/Users/test")
	if !strings.Contains(filepath.ToSlash(p), "AppData/Local/BraveSoftware") {
		t.Errorf("expected windows Brave path, got %q", p)
	}
	// Windows Safari
	_, err = GetDatabasePath(Safari, "/Users/test")
	if err != ErrBrowserNotAvailable {
		t.Errorf("expected ErrBrowserNotAvailable for Safari on Windows, got %v", err)
	}
	// Windows Auto
	p, _ = GetDatabasePath(Auto, "/Users/test")
	if p != "" {
		t.Errorf("expected empty path, got %q", p)
	}
	// Windows unknown
	_, err = GetDatabasePath(Type("unknown"), "/Users/test")
	if err != ErrUnknownBrowser {
		t.Errorf("expected ErrUnknownBrowser, got %v", err)
	}
	// Windows fallback env check
	oldLocal := os.Getenv("LOCALAPPDATA")
	oldApp := os.Getenv("APPDATA")
	os.Setenv("LOCALAPPDATA", "")
	os.Setenv("APPDATA", "")
	p, _ = GetDatabasePath(Chrome, "")
	if p != "" {
		t.Logf("GetDatabasePath with empty envs returned %q", p)
	}
	os.Setenv("LOCALAPPDATA", oldLocal)
	os.Setenv("APPDATA", oldApp)

	// Test case 5: unsupported OS
	currentOS = "freebsd"
	_, err = GetDatabasePath(Chrome, "/Users/test")
	if err != ErrUnsupportedPlatform {
		t.Errorf("expected ErrUnsupportedPlatform, got %v", err)
	}
}

func TestGetHomeDirForUser_AllOS(t *testing.T) {
	oldOS := currentOS
	defer func() { currentOS = oldOS }()

	// Test empty username
	_, err := GetHomeDirForUser("")
	if err != nil {
		t.Logf("GetHomeDirForUser empty username error: %v", err)
	}

	// Darwin
	currentOS = "darwin"
	h, err := GetHomeDirForUser("testuser")
	if err != nil || h != "/Users/testuser" {
		t.Errorf("expected /Users/testuser, got %q, error: %v", h, err)
	}

	// Linux
	currentOS = "linux"
	h, err = GetHomeDirForUser("testuser")
	if err != nil || h != "/home/testuser" {
		t.Errorf("expected /home/testuser, got %q, error: %v", h, err)
	}

	// Windows
	currentOS = "windows"
	os.Setenv("SystemDrive", "D:")
	h, err = GetHomeDirForUser("testuser")
	expectedD := filepath.Join("D:", "Users", "testuser")
	if err != nil || h != expectedD {
		t.Errorf("expected %s, got %q, error: %v", expectedD, h, err)
	}

	// Windows empty SystemDrive
	os.Setenv("SystemDrive", "")
	h, err = GetHomeDirForUser("testuser")
	expectedC := filepath.Join("C:", "Users", "testuser")
	if err != nil || h != expectedC {
		t.Errorf("expected %s, got %q, error: %v", expectedC, h, err)
	}

	// Unsupported OS
	currentOS = "freebsd"
	_, err = GetHomeDirForUser("testuser")
	if err != ErrUnsupportedPlatform {
		t.Errorf("expected ErrUnsupportedPlatform, got %v", err)
	}
}

func TestCopyFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "copy-file-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srcPath := filepath.Join(tempDir, "src.txt")
	dstPath := filepath.Join(tempDir, "dst.txt")

	// Src doesn't exist
	err = CopyFile(srcPath, dstPath)
	if err == nil {
		t.Errorf("expected error copying non-existent file, got nil")
	}

	// Write data to src
	data := []byte("hello copy")
	if err := os.WriteFile(srcPath, data, 0644); err != nil {
		t.Fatalf("failed to write src: %v", err)
	}

	// Destination directory doesn't exist/invalid path
	invalidDst := filepath.Join(tempDir, "nonexistent-dir", "dst.txt")
	err = CopyFile(srcPath, invalidDst)
	if err == nil {
		t.Errorf("expected error copying to invalid destination, got nil")
	}

	// Copy successfully
	err = CopyFile(srcPath, dstPath)
	if err != nil {
		t.Errorf("unexpected error copying: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("failed to read dst: %v", err)
	}
	if string(got) != "hello copy" {
		t.Errorf("expected 'hello copy', got %q", string(got))
	}
}
