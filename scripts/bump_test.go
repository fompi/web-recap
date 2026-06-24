package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestEnvironment(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "bump_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create required files with mock content
	files := map[string]string{
		"VERSION":      "1.0.0\n",
		"CHANGELOG.md": "## [Unreleased]\n- Added feature X\n* Fixed bug Y\n\n## [1.0.0] - 2023-01-01\n- Initial release",
		"Makefile":     "VERSION ?= 1.0.0\n",
		filepath.Join("cmd", "web-recap", "main.go"):   "package main\nvar version          = \"1.0.0\"\n",
		filepath.Join("packaging", "arch", "PKGBUILD"): "pkgver=1.0.0\n",
		filepath.Join("man", "web-recap.1"):            ".TH WEB-RECAP 1 \"2023-01-01\" \"web-recap 1.0.0\"\n",
		filepath.Join("debian", "changelog"):           "web-recap (1.0.0-1) unstable; urgency=medium\n\n  * Initial release\n\n -- Author <author@example.com>  Mon, 01 Jan 2023 00:00:00 +0000\n",
		filepath.Join("packaging", "fedora", "web-recap.spec"): "Name: web-recap\nVersion:        1.0.0\n\n%changelog\n* Mon Jan 01 2023 Author - 1.0.0-1\n- Initial release\n",
	}

	for path, content := range files {
		fullPath := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create dir %s: %v", filepath.Dir(fullPath), err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file %s: %v", fullPath, err)
		}
	}

	return dir
}

func runMainCapture(args []string) (string, int) {
	oldArgs := osArgs
	oldExit := osExit
	defer func() {
		osArgs = oldArgs
		osExit = oldExit
	}()

	osArgs = args
	exitCode := 0
	osExit = func(code int) {
		exitCode = code
		panic(fmt.Sprintf("exit %d", code))
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
	}()

	func() {
		defer func() {
			if r := recover(); r != nil {
				// panic occurred, check if it's from our osExit
				if msg, ok := r.(string); ok && strings.HasPrefix(msg, "exit ") {
					return
				}
				panic(r)
			}
		}()
		main()
	}()

	w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)

	return buf.String(), exitCode
}

func TestBump_Success(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)

	// Change working directory to temp dir
	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})

	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(out, "Successfully synchronized all files!") {
		t.Errorf("Expected success message, got: %s", out)
	}

	// Verify VERSION file was updated
	versionBytes, _ := os.ReadFile("VERSION")
	if strings.TrimSpace(string(versionBytes)) != "1.1.0" {
		t.Errorf("Expected VERSION to be 1.1.0, got: %s", string(versionBytes))
	}

	// Verify main.go was updated
	mainBytes, _ := os.ReadFile(filepath.Join("cmd", "web-recap", "main.go"))
	if !strings.Contains(string(mainBytes), "version          = \"1.1.0\"") {
		t.Errorf("cmd/web-recap/main.go not updated correctly")
	}
}

func TestBump_NoArgs(t *testing.T) {
	out, exitCode := runMainCapture([]string{"bump.go"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Usage: go run") {
		t.Errorf("Expected usage message, got: %s", out)
	}
}

func TestBump_EmptyVersion(t *testing.T) {
	out, exitCode := runMainCapture([]string{"bump.go", "  "})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "new version cannot be empty") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_MissingVersionFile(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Remove(filepath.Join(dir, "VERSION"))

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error reading VERSION file") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_MissingChangelogFile(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Remove(filepath.Join(dir, "CHANGELOG.md"))

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error reading CHANGELOG.md") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_InvalidChangelog(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("No unreleased section here"), 0644)

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "does not contain '## [Unreleased]'") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_NoBulletsInChangelog(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	
	// Create changelog with no bullet points under unreleased
	changelogContent := "## [Unreleased]\nSome text here\n\n## [1.0.0] - 2023-01-01\n- Initial release"
	os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(changelogContent), 0644)

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if !strings.Contains(out, "Warning: No bullet points found") {
		t.Errorf("Expected warning message, got: %s", out)
	}
}

func TestBump_MissingMainGo(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Remove(filepath.Join(dir, "cmd", "web-recap", "main.go"))

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error updating main.go") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_ReplaceTargetNotFound(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	
	// Replace the version string in main.go so replaceInFile cannot find the old string
	os.WriteFile(filepath.Join(dir, "cmd", "web-recap", "main.go"), []byte("package main\nvar version = \"wrong\"\n"), 0644)

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "could not find target content") {
		t.Errorf("Expected error message about missing target content, got: %s", out)
	}
}

func TestBump_MissingMakefile(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Remove(filepath.Join(dir, "Makefile"))

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error updating Makefile") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_MissingPKGBUILD(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Remove(filepath.Join(dir, "packaging", "arch", "PKGBUILD"))
	
	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error updating PKGBUILD") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_MissingManPage(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Remove(filepath.Join(dir, "man", "web-recap.1"))

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error reading man page") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_MissingDebianChangelog(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Remove(filepath.Join(dir, "debian", "changelog"))

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error reading debian/changelog") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_MissingFedoraSpec(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Remove(filepath.Join(dir, "packaging", "fedora", "web-recap.spec"))

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error reading web-recap.spec") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_WriteChangelogError(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Chmod(filepath.Join(dir, "CHANGELOG.md"), 0400)

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error writing CHANGELOG.md") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_WriteVersionError(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Chmod(filepath.Join(dir, "VERSION"), 0400)

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error writing VERSION file") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_WriteManPageError(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Chmod(filepath.Join(dir, "man", "web-recap.1"), 0400)

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error writing man page") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_WriteDebianChangelogError(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Chmod(filepath.Join(dir, "debian", "changelog"), 0400)

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error writing debian/changelog") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_WriteFedoraSpecError(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.Chmod(filepath.Join(dir, "packaging", "fedora", "web-recap.spec"), 0400)

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error writing web-recap.spec") {
		t.Errorf("Expected error message, got: %s", out)
	}
}

func TestBump_HeaderLineEndNotFound(t *testing.T) {
	dir := setupTestEnvironment(t)
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte("## [Unreleased]"), 0644)

	originalDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(originalDir)

	out, exitCode := runMainCapture([]string{"bump.go", "1.1.0"})
	if exitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", exitCode)
	}
	if !strings.Contains(out, "Error parsing header line end") {
		t.Errorf("Expected error message, got: %s", out)
	}
}
