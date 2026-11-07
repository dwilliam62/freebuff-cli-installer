Name:           freebuff-cli
Version:        1.0.0
Release:        1%{?dist}
Summary:        Standalone launcher and updater for the Freebuff AI coding agent
License:        GPL-3.0-or-later
URL:            https://freebuff.com
BuildRequires:  golang >= 1.22
Requires:       glibc
Requires:       ca-certificates
Requires:       kitty
Recommends:     libnotify

%description
Standalone Go launcher and updater for the Freebuff AI coding agent.
Queries the npm registry directly over HTTPS, verifies SHA-256 checksums,
manages ~/.config/manicode/freebuff, and requires zero Node.js or npm runtime.

%build
export CGO_ENABLED=0
go build -trimpath -ldflags="-s -w" -o freebuff-cli ./cmd/freebuff-cli

%install
install -Dpm 0755 freebuff-cli %{buildroot}%{_bindir}/freebuff-cli
install -Dpm 0644 packaging/freebuff-cli.desktop %{buildroot}%{_datadir}/applications/freebuff-cli.desktop

%files
%{_bindir}/freebuff-cli
%{_datadir}/applications/freebuff-cli.desktop
