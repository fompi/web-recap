---
name: web-recap
description: Extract browser history for finding URLs by topic or getting visit stats. Use when user asks about their browsing history, visited websites, or what they were doing online.
---

# web-recap

Extracts browser history from Chrome, Chromium, Brave, Firefox, Safari, Edge. Run `web-recap --help` for all flags.

## Key Subcommands and Flags

### `dump` (Export history logs)
```
--from, -f string        Start date/time (e.g. today, yesterday, '3 days ago', or ISO8601)
--to, -t string          End date/time (e.g. now, yesterday, or ISO8601)
--browser, -b string     Comma-separated list of browsers (defaults to all)
--format, -F string      Output format: text, json, jsonl, csv (default: text)
--output, -o string      Output file (default: stdout)
--compress, -z           Compress output (requires file output): -z (gzip), -zz (bzip2), -zzz (xz)
```

### `stats` (Show history stats and ascii charts)
```
--from, -f string        Start date/time (e.g. today, yesterday, '3 days ago', or ISO8601)
--to, -t string          End date/time (e.g. now, yesterday, or ISO8601)
--browser, -b string     Comma-separated list of browsers
--timezone, -Z string    Timezone name (e.g. America/New_York, UTC, local)
```

### `list` (Helper command to discover active browsers and profiles)

## Output Format (for `dump -F json`)

JSON is a flat array of entries (e.g., `[]HistoryEntry`). Each entry has: `timestamp`, `url`, `title`, `domain`, `visit_count`, `browser`, `profile`, plus browser-specific fields (e.g., `visit_duration`, `visit_type`, etc.).

## Usage Patterns

**Never dump raw output.** Use jq to reduce tokens.

### Search (find URLs by topic)

```bash
# Find entries matching a topic (searches title, domain, url)
web-recap dump --format json -f "30 days" | jq '[.[] | select(.title + .domain + .url | test("KEYWORD"; "i"))] | unique_by(.url) | map({title, url, domain})'
```

### Stats (visit overview via stats subcommand)

```bash
web-recap stats -f "30 days"
```

### Quick count of total entries

```bash
web-recap dump --format json -f "30 days" | jq 'length'
```
