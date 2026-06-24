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
- Bump version and document pending issues.
- Added support for bzip2 and xz compression.
- Standardized CLI flags (--timezone, --database, --flat shorthands).
* Wed Jun 24 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.3.1-1
- Fix `-z` compression flag logic
- Fix MongoDB `insertedCount` double counting
- Remove confusing `keep` alias and enforce conflict strategy validation
* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.3.0-1
- Pre-flight database permission verification checks for Chrome, Firefox, and Safari, indicating access status (Readable, Access Denied, File Missing).
- Proactive macOS Full Disk Access diagnostics warning on standard error.
- Rich CLI statistics report including active browsing duration analysis (Chrome), unified navigation transition mapping, sessionization (grouping visits by <30min inactivity), and weekly distribution charts.
- Firefox places database query checks for the correct `frecency` column instead of `frequency`.

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.4-1
- Fix Firefox Windows profile path, add windows path tests, and propagate db ingest errors
- Clean up dead code, synchronize makefile version, and use cases.Title instead of deprecated strings.Title

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.3-1
- Fix Safari profile detection standard path on macOS
- Copy WAL/journal files alongside databases to capture uncommitted history and fix missing entries

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.2-1
- Fix SQL ingest replace queries, date range filter helper, time parser, and add Safari permission guidance

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.1-1
- Add support for Safari profiles, Chromium Local State mapping, and Firefox profiles.ini

* Sat Jun 20 2026 Rob Zolkos <robzolkos@gmail.com> - 0.2.0-1
- Initial Fedora/RedHat package release of revamped web-recap.
