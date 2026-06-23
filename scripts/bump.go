package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run scripts/bump.go <new_version>")
		os.Exit(1)
	}

	newVersion := strings.TrimSpace(os.Args[1])
	if newVersion == "" {
		fmt.Println("Error: new version cannot be empty")
		os.Exit(1)
	}

	// 1. Read old version from VERSION file
	oldVersionBytes, err := os.ReadFile("VERSION")
	if err != nil {
		fmt.Printf("Error reading VERSION file: %v\n", err)
		os.Exit(1)
	}
	oldVersion := strings.TrimSpace(string(oldVersionBytes))
	fmt.Printf("Bumping version from %s to %s...\n", oldVersion, newVersion)

	// 2. Process CHANGELOG.md
	changelogBytes, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		fmt.Printf("Error reading CHANGELOG.md: %v\n", err)
		os.Exit(1)
	}
	changelog := string(changelogBytes)

	// Check if ## [Unreleased] exists
	if !strings.Contains(changelog, "## [Unreleased]") {
		fmt.Println("Error: CHANGELOG.md does not contain '## [Unreleased]' section")
		os.Exit(1)
	}

	today := time.Now().Format("2006-01-02")
	releaseHeader := fmt.Sprintf("## [Unreleased]\n\n## [%s] - %s", newVersion, today)
	newChangelog := strings.Replace(changelog, "## [Unreleased]", releaseHeader, 1)

	// Save CHANGELOG.md
	if err := os.WriteFile("CHANGELOG.md", []byte(newChangelog), 0644); err != nil {
		fmt.Printf("Error writing CHANGELOG.md: %v\n", err)
		os.Exit(1)
	}

	// 3. Extract release notes from the newly created section in CHANGELOG.md
	startIdx := strings.Index(newChangelog, "## ["+newVersion+"]")
	if startIdx == -1 {
		fmt.Println("Error finding new release section in CHANGELOG.md")
		os.Exit(1)
	}

	headerLineEnd := strings.Index(newChangelog[startIdx:], "\n")
	if headerLineEnd == -1 {
		fmt.Println("Error parsing header line end in CHANGELOG.md")
		os.Exit(1)
	}
	searchStart := startIdx + headerLineEnd + 1
	var notesBuilder strings.Builder
	lines := strings.Split(newChangelog[searchStart:], "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## [") {
			break
		}
		notesBuilder.WriteString(line)
		notesBuilder.WriteByte('\n')
	}
	notes := notesBuilder.String()

	// Extract bullet points
	var bulletPoints []string
	for _, line := range strings.Split(notes, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			bulletPoints = append(bulletPoints, strings.TrimPrefix(trimmed, "- "))
		} else if strings.HasPrefix(trimmed, "* ") {
			bulletPoints = append(bulletPoints, strings.TrimPrefix(trimmed, "* "))
		}
	}

	if len(bulletPoints) == 0 {
		fmt.Println("Warning: No bullet points found in the release notes. Using default changelog entry.")
		bulletPoints = append(bulletPoints, "Release version "+newVersion)
	}

	// 4. Overwrite VERSION file
	if err := os.WriteFile("VERSION", []byte(newVersion+"\n"), 0644); err != nil {
		fmt.Printf("Error writing VERSION file: %v\n", err)
		os.Exit(1)
	}

	// 5. Update cmd/web-recap/main.go
	if err := replaceInFile(
		filepath.Join("cmd", "web-recap", "main.go"),
		`version          = "`+oldVersion+`"`,
		`version          = "`+newVersion+`"`,
	); err != nil {
		fmt.Printf("Error updating main.go: %v\n", err)
		os.Exit(1)
	}

	// 6. Update Makefile
	if err := replaceInFile(
		"Makefile",
		"VERSION ?= "+oldVersion,
		"VERSION ?= "+newVersion,
	); err != nil {
		fmt.Printf("Error updating Makefile: %v\n", err)
		os.Exit(1)
	}

	// 7. Update packaging/arch/PKGBUILD
	if err := replaceInFile(
		filepath.Join("packaging", "arch", "PKGBUILD"),
		"pkgver="+oldVersion,
		"pkgver="+newVersion,
	); err != nil {
		fmt.Printf("Error updating PKGBUILD: %v\n", err)
		os.Exit(1)
	}

	// 8. Update man/web-recap.1
	manBytes, err := os.ReadFile(filepath.Join("man", "web-recap.1"))
	if err != nil {
		fmt.Printf("Error reading man page: %v\n", err)
		os.Exit(1)
	}
	manContent := string(manBytes)
	manRegex := regexp.MustCompile(`\.TH WEB-RECAP 1 "[0-9-]+" "web-recap [0-9.]+"`)
	updatedManHeader := fmt.Sprintf(`.TH WEB-RECAP 1 "%s" "web-recap %s"`, today, newVersion)
	newManContent := manRegex.ReplaceAllString(manContent, updatedManHeader)
	if err := os.WriteFile(filepath.Join("man", "web-recap.1"), []byte(newManContent), 0644); err != nil {
		fmt.Printf("Error writing man page: %v\n", err)
		os.Exit(1)
	}

	// 9. Update debian/changelog
	changelogPath := filepath.Join("debian", "changelog")
	debChangelogBytes, err := os.ReadFile(changelogPath)
	if err != nil {
		fmt.Printf("Error reading debian/changelog: %v\n", err)
		os.Exit(1)
	}
	debChangelog := string(debChangelogBytes)

	// Format debian entry
	rfcDate := time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700")
	var debBullets strings.Builder
	for _, bp := range bulletPoints {
		debBullets.WriteString("  * ")
		debBullets.WriteString(bp)
		debBullets.WriteByte('\n')
	}

	debEntry := fmt.Sprintf(
		"web-recap (%s-1) unstable; urgency=medium\n\n%s\n -- Ferran Fontcuberta Figueràs <ferran@fompi.net>  %s\n\n",
		newVersion,
		debBullets.String(),
		rfcDate,
	)

	if err := os.WriteFile(changelogPath, []byte(debEntry+debChangelog), 0644); err != nil {
		fmt.Printf("Error writing debian/changelog: %v\n", err)
		os.Exit(1)
	}

	// 10. Update packaging/fedora/web-recap.spec
	specPath := filepath.Join("packaging", "fedora", "web-recap.spec")
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Printf("Error reading web-recap.spec: %v\n", err)
		os.Exit(1)
	}
	specContent := string(specBytes)

	// Replace version
	specContent = strings.Replace(specContent, "Version:        "+oldVersion, "Version:        "+newVersion, 1)

	// Format Fedora changelog entry
	fedoraDate := time.Now().Format("Mon Jan 02 2006")
	var fedoraBullets strings.Builder
	for _, bp := range bulletPoints {
		fedoraBullets.WriteString("- ")
		fedoraBullets.WriteString(bp)
		fedoraBullets.WriteByte('\n')
	}

	fedoraChangelogEntry := fmt.Sprintf(
		"* %s Ferran Fontcuberta Figueràs <ferran@fompi.net> - %s-1\n%s\n",
		fedoraDate,
		newVersion,
		fedoraBullets.String(),
	)

	// Inject into spec %changelog section
	specContent = strings.Replace(specContent, "%changelog\n", "%changelog\n"+fedoraChangelogEntry, 1)

	if err := os.WriteFile(specPath, []byte(specContent), 0644); err != nil {
		fmt.Printf("Error writing web-recap.spec: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Successfully synchronized all files!")
}

func replaceInFile(path, oldStr, newStr string) error {
	contentBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(contentBytes)
	if !strings.Contains(content, oldStr) {
		return fmt.Errorf("could not find target content %q in %s", oldStr, path)
	}
	updated := strings.Replace(content, oldStr, newStr, 1)
	return os.WriteFile(path, []byte(updated), 0644)
}
