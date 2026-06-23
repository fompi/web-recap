# web-recap

Extract browser history from Chrome, Chromium, Brave, Firefox, Safari, and Edge browsers and output it in human-friendly or machine-friendly formats, or ingest it directly into relational or document databases.

> **Privacy:** This tool runs entirely on your machine and never transmits data. Your browser history stays local unless you explicitly pipe it to an external service.

---

## Features

- **Multi-profile auto-detection**: Scans and harvests history from all profiles (e.g. Default, Work, Profile 1) of all installed browsers in a single run.
- **Dedicated Subcommands**: Clean separation of concerns with subcommands for dumping history, displaying stats/charts, and database ingestion.
- **Unified Time Filters**: Only two flags (`--from` and `--to`) replace old complex combinations, accepting ISO 8601 dates or human-friendly helpers (`yesterday`, `today`, `now`, `3 days ago`, `5 hours`, etc.).
- **Extended Raw Fields**: Captures all browser-specific parameters (durations, transitions, sessions, redirects, origins, page statuses) without data loss.
- **Direct Database Ingestion**: Copies browser history directly to SQLite, PostgreSQL, MySQL, or MongoDB.
- **Relational & Flat Layouts**: Choose between normalized relational schemas (linked via foreign keys with cascade delete) and denormalized flat repeating data.
- **Minimal dependencies**: Pure Go implementation with no CGO required for simple cross-platform compilation.

---

## Installation

### Download Binary

Download the latest binary from [GitHub Releases](https://github.com/robzolkos/web-recap/releases):

| Platform | Binary |
|----------|--------|
| Linux | `web-recap-linux-amd64` |
| macOS (Intel) | `web-recap-darwin-amd64` |
| macOS (Apple Silicon) | `web-recap-darwin-arm64` |
| Windows | `web-recap-windows-amd64.exe` |

```bash
# macOS (Apple Silicon)
curl -L https://github.com/robzolkos/web-recap/releases/latest/download/web-recap-darwin-arm64 -o ~/.local/bin/web-recap
chmod +x ~/.local/bin/web-recap
```

### macOS Permissions (Full Disk Access)

On macOS 10.14 (Mojave) and later, browser history databases (especially Safari's `History.db`) are protected by system security. If you attempt to query Safari and get a permission error:
```
permission denied reading Safari history database: please grant Full Disk Access...
```

You must manually grant **Full Disk Access** permissions to your terminal emulator (e.g., Terminal, iTerm) or IDE:
1. Open **System Settings** on macOS.
2. Navigate to **Privacy & Security** > **Full Disk Access**.
3. Enable (toggle ON) the checkbox next to your Terminal/iTerm application.
4. Restart your terminal session.

---

### Build from Source

Requires Go 1.21+

```bash
git clone https://github.com/robzolkos/web-recap.git
cd web-recap
make build
./web-recap --help
```

---

## Rationale

The CLI interface and database engine of `web-recap` were overhauled to improve usability, prevent data loss, and maintain a clean separation of concerns:

1. **Parameter Simplification (Unified Date/Time)**
   - *Problem:* Previously, users had to combine 5 different parameters (`--date`, `--start-date`, `--end-date`, `--time`, `--start-time`, `--end-time`) to filter dates and times, which was complex and prone to conflicts.
   - *Solution:* Unified all 5 parameter options into exactly two flags: `--from` / `-f` and `--to` / `-t`. They accept full ISO8601 timestamps, dates, or relative helpers (`start` / `0` for start of log, `today`, `yesterday`, `now`, and offsets like `5 hours ago`, `3 days`, `30 minutes`). By default, it extracts history from the beginning of "today" up to "now".
2. **Separation of Concerns via Subcommands**
   - *Problem:* Commands, printing options, and database connection logic were cluttered together on a single root command.
   - *Solution:* Split actions into discrete subcommands:
     - `dump`: Exports history logs (JSON, JSONL, CSV, Table).
     - `stats`: Analyzes user web activity and outputs summaries and ASCII bar charts.
     - `ingest`: Direct database copy script (keeping connection logic out of the print command).
     - `list`: Helper command to discover active browsers and user profiles.
3. **Multi-Profile Harvesting**
   - *Problem:* Most users have multiple profiles (e.g. Work vs Personal). The tool originally searched only for the "Default" profile.
   - *Solution:* Rewrote browser detection to find all active profiles, extracting and stamping entries with their corresponding `profile` name.
4. **Relational vs Flat layouts**
   - *Problem:* Different browsers record different metrics (e.g., Safari logs redirect origins; Chrome logs transition types and visit durations). Normalizing them into a single table causes data loss or results in a sparse table.
   - *Solution:* Designed two modes via the `--flat` flag:
     - **Relational mode (default / `--flat=false`):** Common columns are stored in a parent `history` table with an auto-incrementing `id` primary key. Browser-specific fields are stored in child tables (e.g. `history_chrome`) linked using `history_id` with `ON DELETE CASCADE`.
     - **Flat mode (`--flat=true`):** Denormalizes everything into flat tables repeating the common fields, avoiding relational constraints.

---

## Migration Guide (Before vs After)

| Goal / Scenario | Before | After |
|---|---|---|
| List profiles | `web-recap list` | `web-recap list` |
| Extract today's logs | `web-recap` | `web-recap dump` |
| Specific browser | `web-recap --browser chrome` | `web-recap dump --browser chrome` |
| Filter by date range | `web-recap --start-date 2025-12-01 --end-date 2025-12-15` | `web-recap dump --from 2025-12-01 --to 2025-12-15` |
| Hours range | `web-recap --date 2025-12-15 --start-time 12:00 --end-time 13:00` | `web-recap dump --from 2025-12-15T12:00:00 --to 2025-12-15T13:00:00` |
| Relative offset | (N/A - manually calculate date strings) | `web-recap dump -f "3 days"` |
| Yesterday to now | (N/A) | `web-recap dump -f yesterday -t now` |
| Show summary charts | (Console summary printed by default) | `web-recap stats` |
| Ingest Chrome history | `web-recap --browser chrome --db-path sqlite://hist.db` | `web-recap ingest -c sqlite://hist.db --browser chrome` |

---

## Usage

### Discover Profiles
```bash
web-recap list
```

### Dump History Entries
```bash
# Dump default browser history from today
web-recap dump

# Export history from last 7 days in JSON Lines format
web-recap dump --from "7 days" --format jsonl

# Export a specific date range from Chrome & Firefox to a compressed CSV
web-recap dump -b chrome,firefox -f 2026-06-01 -t 2026-06-15 -F csv -o history.csv.gz -z
```

### View Statistics
```bash
# Show history stats and domains charts from today
web-recap stats

# Statistics from last 24 hours in America/New_York timezone
web-recap stats --from "24 hours" --tz America/New_York
```

---

## Direct Database Ingestion

You can copy and homogenize history logs directly into local or remote databases.

```bash
web-recap ingest --connect <DSN> [flags]
```

### Supported Databases
- **SQLite**: `sqlite://path/to/database.db` or `sqlite3://path/to/database.db`
- **PostgreSQL**: `postgres://user:password@host:port/dbname?sslmode=disable`
- **MySQL**: `mysql://user:password@tcp(host:port)/dbname`
- **MongoDB**: `mongodb://host:port/dbname`

### Ingestion Flags
- `-c`, `--connect` (Required): Connection DSN string.
- `-C`, `--conflict` (Default `skip`): Conflict resolution strategy (`skip`, `replace`, `keep`).
- `-M`, `--mode` (Default `merged`):
  - `merged`: Single `history` table containing only common columns.
  - `split`: Browser-specific tables containing common + raw columns.
  - `both`: Populates both the merged table and the browser-specific tables.
- `--flat` (Default `false`):
  - If `false` (relational), uses foreign key references (`history_id`) to link child tables (e.g. `history_chrome`) to the parent `history` table.
  - If `true` (flat), denormalizes tables, repeating common columns in child tables.

### Examples

#### 1. Normalized Relational SQLite DB (Default)
Populates parent `history` table and child tables linked via foreign keys (`ON DELETE CASCADE`):
```bash
web-recap ingest -c sqlite://history_relational.db -M both -f "30 days"
```

#### 2. Flat SQLite DB
Populates flat tables repeating columns:
```bash
web-recap ingest -c sqlite://history_flat.db -M both --flat -f "30 days"
```

#### 3. Ingest into Remote PostgreSQL
```bash
web-recap ingest -c "postgres://postgres:secret@localhost:5432/history?sslmode=disable" -M merged
```

#### 4. Ingest into MongoDB
```bash
web-recap ingest -c mongodb://localhost:27017/web_history -M both
```
*Note:* Relational mode (`--flat=false` / default) in MongoDB uses deterministic `ObjectID` mapping between the parent `history` collection and child collections.

---

## Extended Database Schemas

### Relational Schema (`--flat=false`, `-M both` / default)

```mermaid
erDiagram
    history {
        int id PK "Autoincrement"
        string browser
        string profile
        timestamp timestamp
        text url
        text title
        string domain
        int visit_count
    }
    history_chrome {
        int history_id PK, FK "ON DELETE CASCADE"
        int visit_duration
        int transition
        int from_visit
        int segment_id
        int typed_count
    }
    history_firefox {
        int history_id PK, FK "ON DELETE CASCADE"
        int from_visit
        int visit_type
        int session
        int frequency
        int typed
    }
    history_safari {
        int history_id PK, FK "ON DELETE CASCADE"
        int redirect_source
        int redirect_destination
        int origin
        int generation_type
        int load_successful
        int http_non_get
        int synthesized
    }
    history ||--o| history_chrome : references
    history ||--o| history_firefox : references
    history ||--o| history_safari : references
```

---

## Development

```bash
# Run tests
go test -v ./...

# Build binary
make build
```

---

## License

MIT License. See [LICENSE](LICENSE) for details.
