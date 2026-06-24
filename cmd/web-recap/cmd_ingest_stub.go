//go:build noingest

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func ingestFilterCmds() []*cobra.Command  { return nil }
func ingestSummaryCmds() []*cobra.Command { return nil }
func ingestTestCmds() []*cobra.Command    { return nil }
func initIngestCmd()                       {}

func printShortHelp() {
	fmt.Println(`Usage:
  web-recap [command]

Available Commands:
  dump        Dump raw browser history entries
  stats       Show history statistics and charts
  list        List detected browsers and profiles

Examples:
  web-recap dump --browser chrome
  web-recap stats --from "7 days"
  web-recap list

Use "web-recap [command] --help" for more information about a command.`)
}
