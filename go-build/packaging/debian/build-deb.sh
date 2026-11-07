#!/usr/bin/env bash
# Build a standalone Debian/Ubuntu .deb package for freebuff-cli.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"

VERSION="1.0.0"
ARCH="${DEB_ARCH:-amd64}"
GOARCH_MAP="amd64"
if [ "$ARCH" = "arm64" ]; then
  GOARCH_MAP="arm64"
fi

PKG_NAME="freebuff-cli_${VERSION}_${ARCH}"
STAGE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PKG_NAME}.XXXXXX")"
trap 'rm -rf "$STAGE_DIR"' EXIT

mkdir -p "${STAGE_DIR}/DEBIAN"
mkdir -p "${STAGE_DIR}/usr/bin"
mkdir -p "${STAGE_DIR}/usr/share/applications"

echo "Building static Go binary for linux/${GOARCH_MAP}..."
(
  cd "$ROOT_DIR"
  CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH_MAP" \
    go build -trimpath -ldflags="-s -w" -o "${STAGE_DIR}/usr/bin/freebuff-cli" ./cmd/freebuff-cli
)
chmod 0755 "${STAGE_DIR}/usr/bin/freebuff-cli"

install -m 0644 "${ROOT_DIR}/packaging/freebuff-cli.desktop" \
  "${STAGE_DIR}/usr/share/applications/freebuff-cli.desktop"

cat > "${STAGE_DIR}/DEBIAN/control" <<EOF
Package: freebuff-cli
Version: ${VERSION}
Section: devel
Priority: optional
Architecture: ${ARCH}
Depends: libc6, ca-certificates, kitty
Recommends: libnotify-bin
Maintainer: ddubs <https://github.com/dwilliam62/FreeBuff-cli-installer>
Description: Standalone launcher and updater for the Freebuff AI coding agent
 Queries the npm registry directly over HTTPS, verifies SHA-256 checksums,
 manages ~/.config/manicode/freebuff, and launches in Kitty with zero
 Node.js or npm dependencies.
EOF

OUT_DEB="${ROOT_DIR}/${PKG_NAME}.deb"
if command -v dpkg-deb >/dev/null 2>&1; then
  dpkg-deb --build --root-owner-group "$STAGE_DIR" "$OUT_DEB"
else
  # Portable .deb builder using standard ar + tar + gzip (works on NixOS, Arch, Fedora, Gentoo, etc.)
  WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/${PKG_NAME}-ar.XXXXXX")"
  trap 'rm -rf "$STAGE_DIR" "$WORK_DIR"' EXIT
  printf '2.0\n' > "${WORK_DIR}/debian-binary"
  (
    cd "${STAGE_DIR}/DEBIAN"
    tar --owner=0 --group=0 -czf "${WORK_DIR}/control.tar.gz" .
  )
  (
    cd "${STAGE_DIR}"
    tar --owner=0 --group=0 --exclude='./DEBIAN' -czf "${WORK_DIR}/data.tar.gz" .
  )
  rm -f "$OUT_DEB"
  (
    cd "$WORK_DIR"
    ar r "$OUT_DEB" debian-binary control.tar.gz data.tar.gz >/dev/null 2>&1
  )
fi
echo "Built Debian/Ubuntu package: $OUT_DEB"
