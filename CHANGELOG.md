# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Added Vivaldi browser support (Linux, macOS, and Windows) — history is extracted using the same Chromium-based path and profile logic already used for Chrome, Edge, and Brave.
- `web-recap list` now annotates each profile with its access status (`[Readable]`, `[Access Denied (requires Full Disk Access)]`, or `[Not found]`), matching what the README already described (#28).
- Added optional build tags (`nomongo`, `nomysql`, `nopq`, `noingest`) to exclude database output backends and the `ingest` subcommand from the binary, reducing its size by ~46% (from ~16 MB to ~8.7 MB). The new `make build-lean` and `make build-all-lean` Makefile targets apply all four tags; when `noingest` is active the `ingest` subcommand is absent from help output and rejected at runtime. Browser reading is always unaffected since it relies on SQLite unconditionally.

### Changed
- Consolidated the `modernc.org/sqlite` driver registration (previously duplicated across `chrome.go`, `firefox.go`, `safari.go`, and `detector.go`) into single dedicated files (`internal/database/drivers_sqlite.go`, `internal/browser/drivers_sqlite.go`). No behaviour change.
- Moved the MySQL and PostgreSQL blank-import driver registrations out of `ingest_sql.go` into `drivers_mysql.go` and `drivers_postgres.go` respectively, each guarded by the corresponding build tag.
- Replaced `golang.org/x/text/cases` + `golang.org/x/text/language` (imported solely to capitalise browser names in the stats view) with a two-line stdlib helper, eliminating a 317 KB transitive dependency.
- When CGO is available (the default for local `make build` / `make build-lean`), the SQLite driver now uses `mattn/go-sqlite3` instead of `modernc.org/sqlite`. A pair of `//go:build cgo` / `//go:build !cgo` files (`drivers_sqlite_cgo.go` / `drivers_sqlite.go`) select between the two. This avoids pulling in `modernc.org/libc` — a ~5.8 MB port of the C standard library to Go — and shrinks local lean builds from ~8.9 MB to **~6.5 MB**. Cross-compiled (`CGO_ENABLED=0`) dist builds are unchanged and continue to use `modernc.org/sqlite`.
- Vendored `mattn/go-sqlite3` and removed the `SQLITE_ENABLE_FTS3`, `SQLITE_ENABLE_FTS3_PARENTHESIS`, and `SQLITE_ENABLE_RTREE` compile-time flags from its bundled SQLite amalgamation. These features (full-text search and spatial indexing) are compiled in by mattn's defaults but are never used by web-recap; removing them saves a further **~194 KB** from CGO builds. Local and CI builds now use `-mod=vendor`.

### Fixed
- `FormatJSON` now emits `[]` instead of `null` when there are no history entries, preventing downstream consumers from receiving an unexpected null value (#22).
- Summary line now reports the actual system timezone (via `loc.String()`) instead of always printing `UTC` when no `--timezone` flag is provided (#23).
- `help` subcommand no longer appears in `--help` output on Cobra 1.10.x by overriding the usage template to remove its special-case exception (#24).
- Removed `--summary` flag from the `stats` subcommand where it was registered but had no effect; the flag remains on `dump` and `ingest` (#26).
- Renamed stats section heading from "Chrome-only" to "Chromium-based browsers" to accurately reflect that Edge, Brave, and Chromium also contribute visit-duration data (#27).

## [0.4.0] - 2026-06-24

### Fixed
- Fixed nanosecond handling in date range midnight boundary check for Chrome, Firefox, and Safari query handlers, ensuring non-midnight times are not incorrectly shifted.
- Removed redundant sorting step in `database.Query()` since database SQL queries already retrieve entries pre-sorted descending.
- Corrected MongoDB ingestion `insertedCount` calculation to sum `UpsertedCount` and `ModifiedCount` instead of `MatchedCount`, preventing inflated counts of unchanged records.
- Optimized and secured `HasColumn` in SQLite utilities by querying `sqlite_master` and `PRAGMA table_info` instead of using `SELECT *`.
- Captured and propagated errors from closing the output file and compressor (gzip, bzip2, or xz) to prevent silent data corruption.
- Restrained compression flags (`-z`, `-zz`, `-zzz`) to require an output file, preventing compressed output on stdout.
- Fixed `go.mod` warnings by promoting `github.com/spf13/pflag` and `golang.org/x/term` to direct dependencies.

### Changed
- Refactored `ingest.go` by splitting the monolithic ingestion logic into smaller, cohesive files (`ingest_sql.go` and `ingest_mongo.go`).
- Configured `Makefile` to output compiled binaries to the `bin/` directory instead of the project root.
- Documented the dual-packaging directory structure (root-level `debian/` vs `packaging/`) in `README.md`.
- Audited and updated `README.md` examples and ingestion flag details to match the latest CLI syntax.
- Audited and updated `SKILL.md` to align with the current CLI timezone, format, and compression flags, correcting JSON jq patterns.
- Audited and synchronized the man page (`man/web-recap.1`) with recent CLI updates (removed version subcommand, updated conflict flag strategies, and renamed formats).
- Upgraded GitHub Actions CI/CD workflows to use modern, non-deprecated actions (`setup-go@v5`, `cache@v4`, and `action-gh-release@v2`).
- Added `linux-arm64` cross-compilation build target to the `Makefile` and GitHub Actions release workflow.
- Refactored CLI flags to eliminate mutable global variables, moving config parsing to a dedicated `Config` struct.
- Consolidated the duplicate `--user` flag registration into `rootCmd` persistent flags.
- Replaced the `version` subcommand with global `--version` (`-V`) flags on the root command.
- Hid the redundant `help` subcommand from the root command help output.
- Implemented a concise help/usage output on syntax errors or no arguments, keeping full help display only on explicit request.
- Renamed the aligned table output format from "table" to "text" (with default extension `.txt`) and implemented dynamic terminal width detection to automatically scale and truncate columns safely on interactive terminals, while outputting full data to non-terminal outputs like files or pipes.
- Unified JSON output format to write a flat array of entries (removing the `HistoryReport` wrapper structure).
- Configured JSON output to be automatically pretty-printed when outputting to stdout (terminal) and compact/minified when writing to a file.
- Enhanced the stderr summary report to display the total entries, timezone, date range filter info, and a detailed breakdown of entries per browser and profile with counts and percentages.
- Implemented smart output filename parsing to autodetect format and compression from the `-o` file extension (with filename-deduced extensions taking precedence over CLI flags), and automatically autocomplete file extensions when names without extensions are provided.

## [0.3.4] - 2026-06-24

### Added
- Comprehensive test suites for all packages (`internal/database`, `internal/browser`, `internal/output`, `internal/utils`, `cmd/web-recap`, and `scripts`), achieving nearly 100% test coverage across the codebase.

### Fixed
- Fixed SQL injection vulnerability in browser-specific database table names by strictly sanitizing user-supplied browser names.
- Replaced obsolete MD5 hashing algorithm with SHA-256 for generating deterministic MongoDB ObjectIDs.
- Updated GitHub Actions CI workflows to use Go 1.25 and 1.26 to match the version specified in `go.mod`.
- Fixed an inconsistency where history entry browser fields were set to internal type codes (e.g. `chrome`) instead of their human-readable display names (e.g. `Google Chrome`).
- Removed redundant `--no-summary` (`-S`) flag, relying on `--summary=false` instead.
- Fixed Arch Linux PKGBUILD declaring a runtime dependency on glibc when built statically with CGO disabled.
- Removed redundant "flat" option from the `--mode` ingestion flag configuration, consolidating flat structure activation via the `--flat` flag.
- Removed unused exported functions: `NewDetector()`, `GetBrowser()`, and `GetFirefoxProfilePath()`.
- Removed unused sentinel errors: `ErrDatabaseLocked`, `ErrDatabaseError`, and `ErrFirefoxProfileNotFound`.
- Consolidated duplicate file copying functions (`copyTempFile` and `copyFile`) by exporting a shared `CopyFile` helper in the `browser` package.
- Consolidated duplicate `safariEpochDiff` constant declarations to a single package-level constant in `internal/database/util.go`.

## [0.3.3] - 2026-06-24

### Changed
- Refactored `README.md` to extract database schemas, rationale, and contributing guidelines into dedicated files (`docs/SCHEMA.md`, `docs/MIGRATION.md`, `CONTRIBUTING.md`).

## [0.3.2] - 2026-06-24

### Added
- Support for `bzip2` and `xz` compression in `web-recap dump` using the `-z` shorthand flag count (e.g., `-z` for gzip, `-zz` for bzip2, `-zzz` for xz).

### Changed
- Renamed `--tz` flag to `--timezone` and added `-Z` shorthand.
- Renamed `--db` flag to `--database` (keeps `-d` shorthand).
- Added `-x` shorthand for the `--flat` ingestion flag.

## [0.3.1] - 2026-06-24

### Fixed
- Fixed shorthand flag `-z` not working for dump `--compress`.
- Fixed MongoDB ingestion counting bug that double-counted updated records in bulk write mode.
- Removed confusing `keep` alias from ingestion conflict strategy list; added strict validation to reject unknown strategies instead of silently falling back to defaults.

## [0.3.0] - 2026-06-23

### Added
- Pre-flight database permission verification checks for Chrome, Firefox, and Safari, indicating access status (Readable, Access Denied, File Missing).
- Proactive macOS Full Disk Access diagnostics warning on standard error.
- Rich CLI statistics report including active browsing duration analysis (Chrome), unified navigation transition mapping, sessionization (grouping visits by <30min inactivity), and weekly distribution charts.

### Fixed
- Firefox places database query checks for the correct `frecency` column instead of `frequency`.

## [0.2.4] - 2026-06-23

### Fixed
- Firefox Windows profile path detection.
- Propagation of database ingestion errors to parent command.
- Replaced deprecated `strings.Title` with `cases.Title`.

## [0.2.3] - 2026-06-23

### Fixed
- Safari profile detection standard path on macOS.
- Copy WAL/journal auxiliary files alongside databases to capture uncheckpointed database changes.

## [0.2.2] - 2026-06-23

### Fixed
- SQL ingest replace queries.
- Date range filter relative keyword helper and time parser.
- Added Safari permission warning guidance.

## [0.2.1] - 2026-06-23

### Added
- Support for Safari profiles, Chromium Local State mapping, and Firefox `profiles.ini`.
- Differentiate and stamp browser profiles in CSV, CLI tables, and statistics.

## [0.2.0] - 2026-06-20

### Added
- Initial release of the revamped web-recap.
- Added dump, stats, list subcommands.
- Supported relative date expressions (yesterday, today, N days ago).
- Added SQLite, MySQL, PostgreSQL, and MongoDB database ingestion.
- Integrated unit tests with mocked database tests.
