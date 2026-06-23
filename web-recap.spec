Name:           web-recap
Version:        0.2.3
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
* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.3-1
- Fix Safari profile detection standard path on macOS

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.2-1
- Fix SQL ingest replace queries, date range filter helper, time parser, and add Safari permission guidance

* Tue Jun 23 2026 Ferran Fontcuberta Figueràs <ferran@fompi.net> - 0.2.1-1
- Add support for Safari profiles, Chromium Local State mapping, and Firefox profiles.ini

* Sat Jun 20 2026 Rob Zolkos <robzolkos@gmail.com> - 0.2.0-1
- Initial Fedora/RedHat package release of revamped web-recap.
