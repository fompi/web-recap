Name:           web-recap
Version:        0.3.4
Release:        1%{?dist}
Summary:        Extract browser history in human-friendly or machine-friendly formats

License:        MIT
URL:            https://github.com/rzolkos/web-recap
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  golang

%description
web-recap extracts browser history from Chrome, Firefox, Safari, Edge, and Brave
and outputs it in structured formats (Table, CSV, JSON, and JSONLines) suitable for LLMs.
It also supports direct database ingestion.

%prep
%autosetup

%build
go build -ldflags="-s -w" -o %{name} ./cmd/web-recap

%install
install -D -p -m 0755 %{name} %{buildroot}%{_bindir}/%{name}
install -D -p -m 0644 man/%{name}.1 %{buildroot}%{_mandir}/man1/%{name}.1

%files
%license LICENSE
%doc README.md
%{_bindir}/%{name}
%{_mandir}/man1/%{name}.1.gz

%changelog
* Wed Jun 24 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.3.4-1
- Comprehensive test suites for all packages (`internal/database`, `internal/browser`, `internal/output`, `internal/utils`, `cmd/web-recap`, and `scripts`), achieving nearly 100% test coverage across the codebase.
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

* Wed Jun 24 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.3.3-1
- Refactored `README.md` to extract database schemas, rationale, and contributing guidelines into dedicated files (`docs/SCHEMA.md`, `docs/MIGRATION.md`, `CONTRIBUTING.md`).

* Wed Jun 24 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.3.2-1
- Support for `bzip2` and `xz` compression in `web-recap dump` using the `-z` shorthand flag count (e.g., `-z` for gzip, `-zz` for bzip2, `-zzz` for xz).
- Renamed `--tz` flag to `--timezone` and added `-Z` shorthand.
- Renamed `--db` flag to `--database` (keeps `-d` shorthand).
- Added `-x` shorthand for the `--flat` ingestion flag.

* Wed Jun 24 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.3.1-1
- Fixed shorthand flag `-z` not working for dump `--compress`.
- Fixed MongoDB ingestion counting bug that double-counted updated records in bulk write mode.
- Removed confusing `keep` alias from ingestion conflict strategy list; added strict validation to reject unknown strategies instead of silently falling back to defaults.

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.3.0-1
- Pre-flight database permission verification checks for Chrome, Firefox, and Safari, indicating access status (Readable, Access Denied, File Missing).
- Proactive macOS Full Disk Access diagnostics warning on standard error.
- Rich CLI statistics report including active browsing duration analysis (Chrome), unified navigation transition mapping, sessionization (grouping visits by <30min inactivity), and weekly distribution charts.
- Firefox places database query checks for the correct `frecency` column instead of `frequency`.

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.4-1
- Firefox Windows profile path detection.
- Propagation of database ingestion errors to parent command.
- Replaced deprecated `strings.Title` with `cases.Title`.

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.3-1
- Safari profile detection standard path on macOS.
- Copy WAL/journal auxiliary files alongside databases to capture uncheckpointed database changes.

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.2-1
- SQL ingest replace queries.
- Date range filter relative keyword helper and time parser.
- Added Safari permission warning guidance.

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.1-1
- Support for Safari profiles, Chromium Local State mapping, and Firefox `profiles.ini`.
- Differentiate and stamp browser profiles in CSV, CLI tables, and statistics.

* Sat Jun 20 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.0-1
- Initial release of the revamped web-recap.
- Added dump, stats, list subcommands.
- Supported relative date expressions (yesterday, today, N days ago).
- Added SQLite, MySQL, PostgreSQL, and MongoDB database ingestion.
- Integrated unit tests with mocked database tests.

