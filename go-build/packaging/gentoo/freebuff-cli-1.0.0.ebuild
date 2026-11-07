# Copyright 2026 Gentoo Authors
# Distributed under the terms of the GNU General Public License v3

EAPI=8

inherit desktop go-module

DESCRIPTION="Standalone launcher and updater for the Freebuff AI coding agent"
HOMEPAGE="https://freebuff.com https://github.com/dwilliam62/FreeBuff-cli-installer"
LICENSE="GPL-3+"
SLOT="0"
KEYWORDS="~amd64 ~arm64"

RDEPEND="
	sys-libs/glibc
	app-misc/ca-certificates
	x11-terms/kitty
	x11-libs/libnotify
"
BDEPEND=">=dev-lang/go-1.22"

src_compile() {
	export CGO_ENABLED=0
	ego build -trimpath -ldflags="-s -w" -o freebuff-cli ./cmd/freebuff-cli
}

src_install() {
	dobin freebuff-cli
	domenu packaging/freebuff-cli.desktop
}
