//go:build !noingest

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

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

// ingestFilterCmds returns the ingest command so the shared init loop can add
// common filter flags (--from, --to, etc.) to it alongside dump and stats.
func ingestFilterCmds() []*cobra.Command { return []*cobra.Command{ingestCmd} }

// ingestSummaryCmds returns the ingest command so the shared init loop can add
// --summary to it alongside dump.
func ingestSummaryCmds() []*cobra.Command { return []*cobra.Command{ingestCmd} }

// ingestTestCmds returns the ingest command for test helpers that iterate over
// all registered commands.
func ingestTestCmds() []*cobra.Command { return []*cobra.Command{ingestCmd} }

func printShortHelp() {
	fmt.Println(`Usage:
  web-recap [command]

Available Commands:
  dump        Dump raw browser history entries
  stats       Show history statistics and charts
  ingest      Ingest browser history entries directly into a database
  list        List detected browsers and profiles

Examples:
  web-recap dump --browser chrome
  web-recap stats --from "7 days"
  web-recap list

Use "web-recap [command] --help" for more information about a command.`)
}

func initIngestCmd() {
	ingestCmd.Flags().StringP("connect", "c", "", "Database connection string (e.g. mysql://user:pass@host/db) (Required)")
	ingestCmd.Flags().StringP("conflict", "C", "skip", "Ingestion conflict strategy: skip, replace")
	ingestCmd.Flags().StringP("mode", "M", "merged", "Ingestion mode: merged (only common columns in 'history' table), split (browser-specific tables), both (both merged and split tables)")
	ingestCmd.Flags().BoolP("flat", "x", false, "Create flat tables repeating common data instead of relational schemas")
	_ = ingestCmd.MarkFlagRequired("connect")
	ingestCmd.SetFlagErrorFunc(handleFlagError)
	rootCmd.AddCommand(ingestCmd)
}
