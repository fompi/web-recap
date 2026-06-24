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

	"github.com/dsnet/compress/bzip2"
	"github.com/ulikunitz/xz"
)

const version = "0.5.1"

var rootCmd = &cobra.Command{
	Use:   "web-recap",
	Short: "Extract browser history in human-friendly or machine-friendly formats",
	Long: `web-recap extracts browser history from Chrome, Chromium, Firefox, Safari, and Edge.
It supports advanced relative time filters, multiple output formats, and direct database ingestion.`,
	Run: func(cmd *cobra.Command, args []string) {
		showVersion, _ := cmd.Flags().GetBool("version")
		if showVersion {
			return
		}
		printShortHelp()
	},
}

var dumpCmd = &cobra.Command{
	Use:     "dump [flags]",
	Short:   "Dump raw browser history entries",
	Example: `  web-recap dump
  web-recap dump --browser chrome -f "3 days"
  web-recap dump --from "yesterday" --to "now" --format csv
  web-recap dump -b safari -f "2026-06-20T10:00:00" -o history.json.gz -z
  web-recap dump -b chrome -o history.csv.bz2 -zz`,
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

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List detected browsers and profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		user, _ := cmd.Flags().GetString("user")
		homeDir, err := browser.GetHomeDirForUser(user)
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
			status := "[Readable]"
			f, err := os.Open(b.Path)
			if err != nil {
				if os.IsPermission(err) {
					status = "[Access Denied (requires Full Disk Access)]"
				} else {
					status = "[Not found]"
				}
			} else {
				f.Close()
			}
			fmt.Printf("  - %s (profile: %s, type: %s): %s %s\n", b.Name, b.Profile, b.Type, b.Path, status)
		}

		return nil
	},
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Persistent flags (available to all commands)
	rootCmd.PersistentFlags().StringP("user", "u", "", "Retrieve history for another system user")
	rootCmd.PersistentFlags().BoolP("version", "V", false, "Show version")

	// Add common filter flags to subcommands that query history
	for _, sub := range append([]*cobra.Command{dumpCmd, statsCmd}, ingestFilterCmds()...) {
		sub.Flags().StringP("from", "f", "", "Start date/time (e.g. today, yesterday, '3 days ago', or ISO8601)")
		sub.Flags().StringP("to", "t", "", "End date/time (e.g. now, yesterday, or ISO8601). If time is exactly midnight, the range is implicitly extended by 24 hours to include the entire day.")
		sub.Flags().StringP("timezone", "Z", "", "Timezone name (e.g. America/New_York, UTC, local)")
		sub.Flags().StringP("browser", "b", "", "Comma-separated list of browsers (defaults to all)")
		sub.Flags().StringP("database", "d", "", "Custom database paths (e.g. chrome:/path/to/db,safari:/path/to/db)")
		sub.Flags().StringP("limit", "l", "", "Limit max records (e.g. '100' or 'chrome:50,safari:20::100')")
		sub.Flags().BoolP("valid-only", "v", false, "Only return successfully loaded, non-hidden visits (filters out failed loads and subframe entries)")
	}
	// --summary controls the one-line stderr report; stats output goes to stdout
	// and IS the report, so the flag does not apply to statsCmd.
	for _, sub := range append([]*cobra.Command{dumpCmd}, ingestSummaryCmds()...) {
		sub.Flags().BoolP("summary", "s", true, "Show summary on stderr")
	}

	// Dump-specific flags
	dumpCmd.Flags().CountP("compress", "z", "Compress output: -z (gzip), -zz (bzip2), -zzz (xz)")
	dumpCmd.Flags().StringP("format", "F", "text", "Output format (text, csv, json, jsonl)")
	dumpCmd.Flags().StringP("output", "o", "", "Output to file path instead of stdout")

	// Ingest-specific flags and command registration (conditional on build tags)
	initIngestCmd()

	// Set custom flag error handler on all commands
	for _, cmd := range []*cobra.Command{rootCmd, dumpCmd, statsCmd, listCmd} {
		cmd.SetFlagErrorFunc(handleFlagError)
	}

	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(listCmd)

	// Hide Cobra's default help command
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "help [command]",
		Short:  "Help about any command",
		Long:   `Help provides help for any command in the application.`,
		Hidden: true,
		Run: func(c *cobra.Command, args []string) {
			cmd, _, e := rootCmd.Find(args)
			if cmd == nil || e != nil {
				c.Printf("Unknown help topic %#q\n", args)
				_ = rootCmd.Usage()
				return
			}
			helpFunc := cmd.HelpFunc()
			helpFunc(cmd, args)
		},
	})
	// Cobra's built-in usage template special-cases the "help" command name so
	// Hidden: true above is ignored. Strip that exception from the template.
	rootCmd.SetUsageTemplate(strings.ReplaceAll(
		rootCmd.UsageTemplate(),
		`(or .IsAvailableCommand (eq .Name "help"))`,
		`.IsAvailableCommand`,
	))

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		showVersion, _ := cmd.Flags().GetBool("version")
		if showVersion {
			fmt.Printf("web-recap version %s\n", version)
			osExit(0)
		}
	}
}

var osExit = os.Exit


func handleFlagError(cmd *cobra.Command, err error) error {
	fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
	printShortHelp()
	osExit(1)
	return nil
}

func main() {
	// Execution without arguments
	if len(os.Args) == 1 {
		printShortHelp()
		osExit(0)
	}

	rootCmd.SilenceUsage = true
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		osExit(1)
	}
}

type Config struct {
	From             string
	To               string
	Timezone         string
	Browser          string
	Format           string
	Output           string
	Database         string
	User             string
	Summary          bool
	Compress         int
	Connect          string
	Conflict         string
	Mode             string
	Limit            string
	Flat             bool
	ValidOnly        bool
}

func parseConfig(cmd *cobra.Command) Config {
	var cfg Config
	
	if cmd.Flags().Lookup("from") != nil {
		cfg.From, _ = cmd.Flags().GetString("from")
	}
	if cmd.Flags().Lookup("to") != nil {
		cfg.To, _ = cmd.Flags().GetString("to")
	}
	if cmd.Flags().Lookup("timezone") != nil {
		cfg.Timezone, _ = cmd.Flags().GetString("timezone")
	}
	if cmd.Flags().Lookup("browser") != nil {
		cfg.Browser, _ = cmd.Flags().GetString("browser")
	}
	if cmd.Flags().Lookup("database") != nil {
		cfg.Database, _ = cmd.Flags().GetString("database")
	}
	if cmd.Flags().Lookup("user") != nil {
		cfg.User, _ = cmd.Flags().GetString("user")
	}
	if cmd.Flags().Lookup("limit") != nil {
		cfg.Limit, _ = cmd.Flags().GetString("limit")
	}
	if cmd.Flags().Lookup("summary") != nil {
		cfg.Summary, _ = cmd.Flags().GetBool("summary")
	}
	if cmd.Flags().Lookup("valid-only") != nil {
		cfg.ValidOnly, _ = cmd.Flags().GetBool("valid-only")
	}
	if cmd.Flags().Lookup("output") != nil {
		cfg.Output, _ = cmd.Flags().GetString("output")
	}
	if cfg.Output != "" {
		// Deduced compression
		deducedCompress := 0
		strippedOut := cfg.Output
		
		if strings.HasSuffix(strings.ToLower(strippedOut), ".gz") {
			deducedCompress = 1
			strippedOut = strippedOut[:len(strippedOut)-3]
		} else if strings.HasSuffix(strings.ToLower(strippedOut), ".bz2") {
			deducedCompress = 2
			strippedOut = strippedOut[:len(strippedOut)-4]
		} else if strings.HasSuffix(strings.ToLower(strippedOut), ".xz") {
			deducedCompress = 3
			strippedOut = strippedOut[:len(strippedOut)-3]
		}

		// Deduced format
		deducedFormat := ""
		if strings.HasSuffix(strings.ToLower(strippedOut), ".json") {
			deducedFormat = "json"
		} else if strings.HasSuffix(strings.ToLower(strippedOut), ".jsonl") {
			deducedFormat = "jsonl"
		} else if strings.HasSuffix(strings.ToLower(strippedOut), ".csv") {
			deducedFormat = "csv"
		} else if strings.HasSuffix(strings.ToLower(strippedOut), ".txt") {
			deducedFormat = "text"
		}

		// Compression logic with precedence
		finalCompress := 0
		if deducedCompress > 0 {
			finalCompress = deducedCompress
		} else if cmd.Flags().Lookup("compress") != nil {
			finalCompress, _ = cmd.Flags().GetCount("compress")
		}
		cfg.Compress = finalCompress

		// Format logic with precedence
		finalFormat := ""
		if deducedFormat != "" {
			finalFormat = deducedFormat
		} else if cmd.Flags().Lookup("format") != nil {
			finalFormat, _ = cmd.Flags().GetString("format")
		}
		if finalFormat == "" {
			finalFormat = "text"
		}
		cfg.Format = finalFormat

		// Autocomplete extensions if output file has no supported suffix
		hasSupportedExtension := false
		lowerFile := strings.ToLower(cfg.Output)
		for _, ext := range []string{".json", ".jsonl", ".csv", ".txt", ".gz", ".bz2", ".xz"} {
			if strings.HasSuffix(lowerFile, ext) {
				hasSupportedExtension = true
				break
			}
		}

		if !hasSupportedExtension {
			// Append format extension first
			switch cfg.Format {
			case "json":
				cfg.Output += ".json"
			case "jsonl":
				cfg.Output += ".jsonl"
			case "csv":
				cfg.Output += ".csv"
			case "text":
				cfg.Output += ".txt"
			}
			// Append compression extension next
			switch cfg.Compress {
			case 1:
				cfg.Output += ".gz"
			case 2:
				cfg.Output += ".bz2"
			case 3:
				cfg.Output += ".xz"
			}
		}
	} else {
		// Output is stdout (empty)
		if cmd.Flags().Lookup("compress") != nil {
			cfg.Compress, _ = cmd.Flags().GetCount("compress")
		}
		if cmd.Flags().Lookup("format") != nil {
			cfg.Format, _ = cmd.Flags().GetString("format")
		}
	}

	if cmd.Flags().Lookup("connect") != nil {
		cfg.Connect, _ = cmd.Flags().GetString("connect")
	}
	if cmd.Flags().Lookup("conflict") != nil {
		cfg.Conflict, _ = cmd.Flags().GetString("conflict")
	}
	if cmd.Flags().Lookup("mode") != nil {
		cfg.Mode, _ = cmd.Flags().GetString("mode")
	}
	if cmd.Flags().Lookup("flat") != nil {
		cfg.Flat, _ = cmd.Flags().GetBool("flat")
	}
	
	return cfg
}

func runQuery(cmd *cobra.Command, statsOnly bool, ingestOnly bool) (err error) {
	cfg := parseConfig(cmd)

	// 1. Resolve Timezone
	var loc *time.Location
	if cfg.Timezone != "" {
		if cfg.Timezone == "local" {
			loc = time.Local
		} else {
			var err error
			loc, err = time.LoadLocation(cfg.Timezone)
			if err != nil {
				return fmt.Errorf("invalid timezone %q: %v", cfg.Timezone, err)
			}
		}
	} else {
		loc = time.Local
	}

	// 2. Parse Date Helpers
	now := time.Now().In(loc)
	var fromVal, toVal time.Time

	if cfg.From == "" && cfg.To == "" {
		var err error
		fromVal, err = utils.ParseTimeHelper("today", now, loc)
		if err != nil {
			return err
		}
		toVal = now
	} else {
		if cfg.From != "" {
			var err error
			fromVal, err = utils.ParseTimeHelper(cfg.From, now, loc)
			if err != nil {
				return err
			}
		} else {
			fromVal = time.Unix(0, 0).UTC()
		}

		if cfg.To != "" {
			var err error
			toVal, err = utils.ParseTimeHelper(cfg.To, now, loc)
			if err != nil {
				return err
			}
		}
	}

	// 3. Resolve Home Directory and Detector
	homeDir, err := browser.GetHomeDirForUser(cfg.User)
	if err != nil {
		return err
	}
	detector := browser.NewDetectorForUser(homeDir)

	// 4. Parse DB Paths
	var browsersFlagList []string
	if cfg.Browser != "" {
		browsersFlagList = strings.Split(cfg.Browser, ",")
	}
	dbPaths, err := parseDBPaths(cfg.Database, browsersFlagList)
	if err != nil {
		return err
	}

	// 5. Resolve Selected Browsers
	browsers, err := resolveBrowsers(cfg.Browser, detector, dbPaths)
	if err != nil {
		return err
	}

	// 6. Parse Limits
	browserLimits, totalLimit, err := parseLimit(cfg.Limit)
	if err != nil {
		return err
	}

	// 7. Query entries from selected browsers
	var allEntries []models.HistoryEntry
	var browserNames []string

	for _, b := range browsers {
		browserNames = append(browserNames, fmt.Sprintf("%s (%s)", b.Name, b.Profile))
		entries, err := database.Query(b, fromVal, toVal, cfg.ValidOnly)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to query %s (profile: %s): %v\n", b.Name, b.Profile, err)
			if cfg.Browser != "" {
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

	showSummary := cfg.Summary

	// If stats subcommand is chosen, display stats
	if statsOnly {
		return output.FormatStats(os.Stdout, allEntries, fromVal, toVal, loc)
	}

	// 9. Ingest directly if connect string is provided (only via ingest subcommand)
	if ingestOnly {
		inserted, err := database.Ingest(cfg.Connect, allEntries, cfg.Conflict, cfg.Mode, cfg.Flat)
		if err != nil {
			return err
		}
		if showSummary {
			fmt.Fprintf(os.Stderr, "Successfully ingested %d entries into %s using %s mode (flat: %t)\n", inserted, cfg.Connect, cfg.Mode, cfg.Flat)
		}
		return nil
	}

	// 10. Write Output
	var out io.Writer = os.Stdout
	var fileToClose *os.File

	if cfg.Output != "" {
		f, err := os.Create(cfg.Output)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		fileToClose = f
		out = f
	}

	if cfg.Compress > 0 && cfg.Output == "" {
		return fmt.Errorf("compression cannot be used when outputting to stdout; please specify an output file using -o/--output")
	}

	var closer io.WriteCloser
	if cfg.Compress > 0 {
		if cfg.Compress == 1 {
			closer = gzip.NewWriter(out)
			out = closer
		} else if cfg.Compress == 2 {
			var err error
			closer, err = bzip2.NewWriter(out, &bzip2.WriterConfig{Level: bzip2.BestCompression})
			if err != nil {
				return fmt.Errorf("failed to create bzip2 writer: %v", err)
			}
			out = closer
		} else {
			var err error
			closer, err = xz.NewWriter(out)
			if err != nil {
				return fmt.Errorf("failed to create xz writer: %v", err)
			}
			out = closer
		}
	}

	defer func() {
		if closer != nil {
			if closeErr := closer.Close(); closeErr != nil {
				if err == nil {
					err = fmt.Errorf("failed to close compressor: %v", closeErr)
				} else {
					fmt.Fprintf(os.Stderr, "Warning: failed to close compressor: %v\n", closeErr)
				}
			}
		}
		if fileToClose != nil {
			if closeErr := fileToClose.Close(); closeErr != nil {
				if err == nil {
					err = fmt.Errorf("failed to close output file: %v", closeErr)
				} else {
					fmt.Fprintf(os.Stderr, "Warning: failed to close output file: %v\n", closeErr)
				}
			}
		}
	}()

	// 11. Format output
	formatVal := strings.ToLower(strings.TrimSpace(cfg.Format))
	switch formatVal {
	case "text":
		err = output.FormatText(out, allEntries)
	case "json":
		err = output.FormatJSON(out, allEntries, cfg.Output == "")
	case "jsonl":
		err = output.FormatJSONLines(out, allEntries)
	case "csv":
		err = output.FormatCSV(out, allEntries)
	default:
		return fmt.Errorf("unsupported output format %q", cfg.Format)
	}

	if err != nil {
		return fmt.Errorf("failed to format output: %v", err)
	}

	if showSummary {
		var rangeStr string
		if !fromVal.IsZero() && !toVal.IsZero() {
			rangeStr = fmt.Sprintf("Range: %s to %s", fromVal.Format(time.RFC3339), toVal.Format(time.RFC3339))
		} else if !fromVal.IsZero() {
			rangeStr = fmt.Sprintf("Range: since %s", fromVal.Format(time.RFC3339))
		} else if !toVal.IsZero() {
			rangeStr = fmt.Sprintf("Range: until %s", toVal.Format(time.RFC3339))
		} else {
			rangeStr = "Range: all time"
		}
		tzStr := cfg.Timezone
		if tzStr == "" {
			tzStr = loc.String()
		}

		type browserProfile struct {
			Name    string
			Profile string
		}
		counts := make(map[browserProfile]int)
		var order []browserProfile
		seen := make(map[browserProfile]bool)

		for _, b := range browsers {
			bp := browserProfile{Name: b.Name, Profile: b.Profile}
			if !seen[bp] {
				seen[bp] = true
				order = append(order, bp)
			}
		}

		for _, entry := range allEntries {
			bp := browserProfile{Name: entry.Browser, Profile: entry.Profile}
			counts[bp]++
			if !seen[bp] {
				seen[bp] = true
				order = append(order, bp)
			}
		}

		fmt.Fprintf(os.Stderr, "Summary: Extracted %d entries in total (%s, Timezone: %s).\n", len(allEntries), rangeStr, tzStr)
		for _, bp := range order {
			count := counts[bp]
			var pct float64
			if len(allEntries) > 0 {
				pct = float64(count) / float64(len(allEntries)) * 100.0
			}
			fmt.Fprintf(os.Stderr, "  - %s (%s): %d entries (%.1f%%)\n", bp.Name, bp.Profile, count, pct)
		}
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
			for _, b := range []string{"chrome", "chromium", "edge", "brave", "vivaldi", "firefox", "safari"} {
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
					Name:    browser.GetBrowserName(browser.Type(bType)),
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
		case browser.Chrome, browser.Chromium, browser.Edge, browser.Brave, browser.Vivaldi, browser.Firefox, browser.Safari:
		default:
			return nil, fmt.Errorf("unsupported browser type %q", part)
		}

		if path, ok := dbPaths[part]; ok {
			result = append(result, &browser.Browser{
				Type:    bType,
				Name:    browser.GetBrowserName(bType),
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
