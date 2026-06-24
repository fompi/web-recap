# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- Updated GitHub Actions CI workflows to use Go 1.25 and 1.26 to match the version specified in `go.mod`.
- Fixed an inconsistency where history entry browser fields were set to internal type codes (e.g. `chrome`) instead of their human-readable display names (e.g. `Google Chrome`).

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
