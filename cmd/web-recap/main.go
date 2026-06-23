package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/rzolkos/web-recap/internal/browser"
	"github.com/rzolkos/web-recap/internal/database"
	"github.com/rzolkos/web-recap/internal/models"
	"github.com/rzolkos/web-recap/internal/output"
	"github.com/rzolkos/web-recap/internal/utils"
)

var (
	fromFlag         string
	toFlag           string
	timezone         string
	browserFlag      string
	formatFlag       string
	outputFile       string
	dbFlag           string
	userFlag         string
	summary          bool
	noSummary        bool
	compress         bool
	connectStr       string
	conflictStrategy string
	modeFlag         string
	limitFlag        string
	flatFlag         bool
	version          = "0.2.4"
)



var rootCmd = &cobra.Command{
	Use:   "web-recap",
	Short: "Extract browser history in human-friendly or machine-friendly formats",
	Long: `web-recap extracts browser history from Chrome, Chromium, Firefox, Safari, and Edge.
It supports advanced relative time filters, multiple output formats, and direct database ingestion.`,
}

var dumpCmd = &cobra.Command{
	Use:     "dump [flags]",
	Short:   "Dump raw browser history entries",
	Example: `  web-recap dump
  web-recap dump --browser chrome -f "3 days"
  web-recap dump --from "yesterday" --to "now" --format csv
  web-recap dump -b safari -f "2026-06-20T10:00:00" -o history.json.gz -z`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runQuery(cmd, false, false)
	},
}

var statsCmd = &cobra.Command{
	Use:     "stats [flags]",
	Short:   "Show history statistics and charts",
	Example: `  web-recap stats
  web-recap stats --browser chrome,safari -f "7 days"`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runQuery(cmd, true, false)
	},
}

var ingestCmd = &cobra.Command{
	Use:     "ingest [flags]",
	Short:   "Ingest browser history entries directly into a database",
	Example: `  web-recap ingest -c sqlite://history.db
  web-recap ingest -c postgres://user:pass@localhost/db -M split
  web-recap ingest -c mongodb://localhost/history_db -M both -C replace`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runQuery(cmd, false, true)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List detected browsers and profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		homeDir, err := browser.GetHomeDirForUser(userFlag)
		if err != nil {
			return err
		}
		detector := browser.NewDetectorForUser(homeDir)
		browsers := detector.Detect()

		if len(browsers) == 0 {
			fmt.Println("No browsers detected")
			return nil
		}

		fmt.Println("Detected browsers and profiles:")
		for _, b := range browsers {
			fmt.Printf("  - %s (profile: %s, type: %s): %s\n", b.Name, b.Profile, b.Type, b.Path)
		}

		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("web-recap version %s\n", version)
	},
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Add common filter flags to subcommands that query history
	for _, sub := range []*cobra.Command{dumpCmd, statsCmd, ingestCmd} {
		sub.Flags().StringVarP(&fromFlag, "from", "f", "", "Start date/time (e.g. today, yesterday, '3 days ago', or ISO8601)")
		sub.Flags().StringVarP(&toFlag, "to", "t", "", "End date/time (e.g. now, yesterday, or ISO8601)")
		sub.Flags().StringVar(&timezone, "tz", "", "Timezone name (e.g. America/New_York, UTC, local)")
		sub.Flags().StringVarP(&browserFlag, "browser", "b", "", "Comma-separated list of browsers (defaults to all)")
		sub.Flags().StringVarP(&dbFlag, "db", "d", "", "Custom database paths (e.g. chrome:/path/to/db,safari:/path/to/db)")
		sub.Flags().StringVarP(&userFlag, "user", "u", "", "Retrieve history for another system user")
		sub.Flags().StringVarP(&limitFlag, "limit", "l", "", "Limit max records (e.g. '100' or 'chrome:50,safari:20::100')")
		sub.Flags().BoolVarP(&summary, "summary", "s", true, "Show summary on stderr")
		sub.Flags().BoolVarP(&noSummary, "no-summary", "S", false, "Disable summary on stderr")
	}

	// Dump-specific flags
	dumpCmd.Flags().StringVarP(&formatFlag, "format", "F", "table", "Output format: table, json, jsonl, csv")
	dumpCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file (default: stdout)")
	dumpCmd.Flags().BoolVar(&compress, "compress", false, "Gzip compress output file or stdout")

	// Ingest-specific flags
	ingestCmd.Flags().StringVarP(&connectStr, "connect", "c", "", "Database DSN/connection string for ingestion (required)")
	ingestCmd.Flags().StringVarP(&conflictStrategy, "conflict", "C", "skip", "Ingestion conflict strategy: skip, replace, keep")
	ingestCmd.Flags().StringVarP(&modeFlag, "mode", "M", "merged", "Ingestion mode: merged (only common columns in 'history' table), split (browser-specific tables), both (both merged and split tables)")
	ingestCmd.Flags().BoolVar(&flatFlag, "flat", false, "Create flat tables repeating common data instead of relational schemas")
	_ = ingestCmd.MarkFlagRequired("connect")

	// List-specific flags
	listCmd.Flags().StringVarP(&userFlag, "user", "u", "", "Retrieve history for another system user")

	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(ingestCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(versionCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runQuery(cmd *cobra.Command, statsOnly bool, ingestOnly bool) error {
	// 1. Resolve Timezone
	var loc *time.Location
	if timezone != "" {
		if timezone == "local" {
			loc = time.Local
		} else {
			var err error
			loc, err = time.LoadLocation(timezone)
			if err != nil {
				return fmt.Errorf("invalid timezone %q: %v", timezone, err)
			}
		}
	} else {
		loc = time.Local
	}

	// 2. Parse Date Helpers
	now := time.Now().In(loc)
	var fromVal, toVal time.Time

	if fromFlag == "" && toFlag == "" {
		var err error
		fromVal, err = utils.ParseTimeHelper("today", now, loc)
		if err != nil {
			return err
		}
		toVal = now
	} else {
		if fromFlag != "" {
			var err error
			fromVal, err = utils.ParseTimeHelper(fromFlag, now, loc)
			if err != nil {
				return err
			}
		} else {
			fromVal = time.Unix(0, 0).UTC()
		}

		if toFlag != "" {
			var err error
			toVal, err = utils.ParseTimeHelper(toFlag, now, loc)
			if err != nil {
				return err
			}
		}
	}

	// 3. Resolve Home Directory and Detector
	homeDir, err := browser.GetHomeDirForUser(userFlag)
	if err != nil {
		return err
	}
	detector := browser.NewDetectorForUser(homeDir)

	// 4. Parse DB Paths
	var browsersFlagList []string
	if browserFlag != "" {
		browsersFlagList = strings.Split(browserFlag, ",")
	}
	dbPaths, err := parseDBPaths(dbFlag, browsersFlagList)
	if err != nil {
		return err
	}

	// 5. Resolve Selected Browsers
	browsers, err := resolveBrowsers(browserFlag, detector, dbPaths)
	if err != nil {
		return err
	}

	// 6. Parse Limits
	browserLimits, totalLimit, err := parseLimit(limitFlag)
	if err != nil {
		return err
	}

	// 7. Query entries from selected browsers
	var allEntries []models.HistoryEntry
	var browserNames []string

	for _, b := range browsers {
		browserNames = append(browserNames, fmt.Sprintf("%s (%s)", b.Name, b.Profile))
		entries, err := database.Query(b, fromVal, toVal)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to query %s (profile: %s): %v\n", b.Name, b.Profile, err)
			if browserFlag != "" {
				return fmt.Errorf("failed to query %s (profile: %s): %v", b.Name, b.Profile, err)
			}
			continue
		}

		// Apply browser-specific limit if set
		limitKey := strings.ToLower(b.Name)
		if limitVal, ok := browserLimits[limitKey]; ok && len(entries) > limitVal {
			entries = entries[:limitVal]
		} else if limitVal, ok := browserLimits[string(b.Type)]; ok && len(entries) > limitVal {
			entries = entries[:limitVal]
		}

		allEntries = append(allEntries, entries...)
	}

	// 8. Normalise, Collate, and Sort
	database.SortEntriesDescending(allEntries)

	// Apply total limit if set
	if totalLimit > 0 && len(allEntries) > totalLimit {
		allEntries = allEntries[:totalLimit]
	}

	showSummary := summary && !noSummary

	// If stats subcommand is chosen, display stats
	if statsOnly {
		return output.FormatStats(os.Stdout, allEntries, fromVal, toVal, loc)
	}

	// 9. Ingest directly if connect string is provided (only via ingest subcommand)
	if ingestOnly {
		inserted, err := database.Ingest(connectStr, allEntries, conflictStrategy, modeFlag, flatFlag)
		if err != nil {
			return err
		}
		if showSummary {
			fmt.Fprintf(os.Stderr, "Successfully ingested %d entries into %s using %s mode (flat: %t)\n", inserted, connectStr, modeFlag, flatFlag)
		}
		return nil
	}

	// 10. Write Output
	var out io.Writer = os.Stdout
	var fileToClose *os.File

	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		fileToClose = f
		out = f
	}

	var gzipWriter *gzip.Writer
	if compress {
		gzipWriter = gzip.NewWriter(out)
		out = gzipWriter
	}

	defer func() {
		if gzipWriter != nil {
			gzipWriter.Close()
		}
		if fileToClose != nil {
			fileToClose.Close()
		}
	}()

	// 11. Format output
	formatFlag = strings.ToLower(strings.TrimSpace(formatFlag))
	switch formatFlag {
	case "table":
		err = output.FormatTable(out, allEntries)
	case "json":
		err = output.FormatJSON(out, allEntries, strings.Join(browserNames, ", "), fromVal, toVal, timezone)
	case "jsonl":
		err = output.FormatJSONLines(out, allEntries)
	case "csv":
		err = output.FormatCSV(out, allEntries)
	default:
		return fmt.Errorf("unsupported output format %q", formatFlag)
	}

	if err != nil {
		return fmt.Errorf("failed to format output: %v", err)
	}

	if showSummary {
		fmt.Fprintf(os.Stderr, "Summary: Extracted %d entries from %d browser profile(s)\n", len(allEntries), len(browsers))
	}

	return nil
}

func parseDBPaths(dbFlag string, browsers []string) (map[string]string, error) {
	paths := make(map[string]string)
	if dbFlag == "" {
		return paths, nil
	}

	parts := strings.Split(dbFlag, ",")
	for _, part := range parts {
		if strings.Contains(part, ":") {
			idx := strings.Index(part, ":")
			prefix := part[:idx]
			isBrowser := false
			for _, b := range []string{"chrome", "chromium", "edge", "brave", "firefox", "safari"} {
				if prefix == b {
					isBrowser = true
					break
				}
			}

			if isBrowser {
				paths[prefix] = part[idx+1:]
				continue
			}
		}

		if len(browsers) == 1 {
			paths[browsers[0]] = part
		} else {
			base := filepath.Base(part)
			if base == "History" {
				paths["chrome"] = part
			} else if base == "places.sqlite" {
				paths["firefox"] = part
			} else if base == "History.db" {
				paths["safari"] = part
			} else {
				return nil, fmt.Errorf("ambiguous database path %q: please specify browser type (e.g. chrome:%s)", part, part)
			}
		}
	}
	return paths, nil
}

func resolveBrowsers(browserFlag string, detector *browser.Detector, dbPaths map[string]string) ([]*browser.Browser, error) {
	detected := detector.Detect()
	detectedMap := make(map[string][]*browser.Browser)
	for i := range detected {
		detectedMap[string(detected[i].Type)] = append(detectedMap[string(detected[i].Type)], &detected[i])
	}

	if browserFlag == "" {
		var result []*browser.Browser
		// Add detected profiles
		for i := range detected {
			b := &detected[i]
			if path, ok := dbPaths[string(b.Type)]; ok {
				b.Path = path
			}
			result = append(result, b)
		}
		// Add custom overrides
		for bType, path := range dbPaths {
			found := false
			for _, r := range result {
				if string(r.Type) == bType {
					found = true
					break
				}
			}
			if !found {
				result = append(result, &browser.Browser{
					Type:    browser.Type(bType),
					Name:    bType,
					Path:    path,
					Profile: "Default",
				})
			}
		}
		return result, nil
	}

	var result []*browser.Browser
	parts := strings.Split(browserFlag, ",")
	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		bType := browser.Type(part)

		switch bType {
		case browser.Chrome, browser.Chromium, browser.Edge, browser.Brave, browser.Firefox, browser.Safari:
		default:
			return nil, fmt.Errorf("unsupported browser type %q", part)
		}

		if path, ok := dbPaths[part]; ok {
			result = append(result, &browser.Browser{
				Type:    bType,
				Name:    part,
				Path:    path,
				Profile: "Default",
			})
			continue
		}

		if profiles, ok := detectedMap[part]; ok {
			result = append(result, profiles...)
		} else {
			return nil, fmt.Errorf("browser %q is not installed or detected on this system", part)
		}
	}

	return result, nil
}

func parseLimit(limitStr string) (map[string]int, int, error) {
	limitStr = strings.TrimSpace(limitStr)
	if limitStr == "" {
		return nil, 0, nil
	}

	browserLimits := make(map[string]int)
	var totalLimit int
	var err error

	if strings.Contains(limitStr, "::") {
		parts := strings.Split(limitStr, "::")
		if len(parts) != 2 {
			return nil, 0, fmt.Errorf("invalid limit format: %s", limitStr)
		}
		if parts[0] != "" {
			subparts := strings.Split(parts[0], ",")
			for _, sub := range subparts {
				sub = strings.TrimSpace(sub)
				kv := strings.Split(sub, ":")
				if len(kv) != 2 {
					return nil, 0, fmt.Errorf("invalid browser limit format: %s", sub)
				}
				bName := strings.TrimSpace(kv[0])
				bLim, err := strconv.Atoi(strings.TrimSpace(kv[1]))
				if err != nil || bLim < 0 {
					return nil, 0, fmt.Errorf("invalid limit value: %s", kv[1])
				}
				browserLimits[bName] = bLim
			}
		}
		totalLimit, err = strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || totalLimit < 0 {
			return nil, 0, fmt.Errorf("invalid total limit value: %s", parts[1])
		}
	} else if strings.Contains(limitStr, ":") {
		subparts := strings.Split(limitStr, ",")
		for _, sub := range subparts {
			sub = strings.TrimSpace(sub)
			kv := strings.Split(sub, ":")
			if len(kv) != 2 {
				return nil, 0, fmt.Errorf("invalid browser limit format: %s", sub)
			}
			bName := strings.TrimSpace(kv[0])
			bLim, err := strconv.Atoi(strings.TrimSpace(kv[1]))
			if err != nil || bLim < 0 {
				return nil, 0, fmt.Errorf("invalid limit value: %s", kv[1])
			}
			browserLimits[bName] = bLim
		}
	} else {
		totalLimit, err = strconv.Atoi(limitStr)
		if err != nil || totalLimit < 0 {
			return nil, 0, fmt.Errorf("invalid total limit value: %s", limitStr)
		}
	}

	return browserLimits, totalLimit, nil
}
