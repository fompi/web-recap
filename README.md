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

On macOS 10.14 (Mojave) and later, browser history databases (especially Safari's `History.db`) are protected by system security.

The `web-recap list` command automatically runs checks on each profile's database file and annotates its state (e.g. `[Readable]` or `[Access Denied (requires Full Disk Access)]`) to help you diagnose access status.

If you attempt to query Safari and get a permission error:
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

See the [CONTRIBUTING.md](CONTRIBUTING.md) guide for build instructions and development setup.

---

## Rationale & Migration

If you are upgrading from `v0.1.x` or want to understand the architectural design behind the CLI flags and history processing, please check the [Rationale & Migration Guide](docs/MIGRATION.md).

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

# Compress output using bzip2 (-zz) or xz (-zzz)
web-recap dump -o history.json.bz2 -zz
web-recap dump -o history.json.xz -zzz
```

### View Statistics
```bash
# Show history stats and domains charts from today
web-recap stats

# Statistics from last 24 hours in America/New_York timezone
web-recap stats --from "24 hours" --timezone America/New_York
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
- `-x`, `--flat` (Default `false`):
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
web-recap ingest -c sqlite://history_flat.db -M both -x -f "30 days"
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

If you need detailed information about the tables structure for database ingestion (including relational ER diagrams), please see the [Database Schemas Documentation](docs/SCHEMA.md).

---

## Packaging Structure

This repository uses a dual-packaging structure:
- The `debian/` directory is located directly in the root of the project. This is a technical requirement of Debian packaging tools (like `dpkg-buildpackage`), which expect the `debian` configuration directory to be at the root of the source tree.
- Packaging configurations for other distributions (such as Arch Linux and Fedora) are isolated under the `packaging/` directory.

---

## Contributing & Development

We welcome issues and pull requests! The codebase maintains near 100% test coverage across all core packages. Please see [CONTRIBUTING.md](CONTRIBUTING.md) for details on how to set up your environment, run tests, and execute the automated release process.

---

## License

MIT License. See [LICENSE](LICENSE) for details.
