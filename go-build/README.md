# `freebuff-cli` (Standalone Go Build)

A standalone, zero-dependency cross-distro launcher, updater, and desktop integration tool for the [Freebuff](https://freebuff.com) AI coding agent — supporting **NixOS**, **Debian**, **Ubuntu**, **Gentoo**, **Arch Linux**, and **Fedora**.

---

## Why a Standalone Go Binary?

Upstream publishes `freebuff` to npm as a thin Node.js wrapper (`index.js`, `launcher.js`, `http.js`) that does **not** contain the application itself. Instead, it downloads a standalone native ELF executable (`~/.config/manicode/freebuff`) and `tree-sitter.wasm` from `https://codebuff.com/api/releases/download/<version>/...` and verifies its SHA-256 against the `binaryChecksums` map published in `https://registry.npmjs.org/freebuff/latest`.

By implementing this launcher directly in Go (`CGO_ENABLED=0`) using **only the Go standard library**:
- **Zero Node.js / npm / npx / NodeSource / nvm dependency**: Eliminates installing or maintaining a Node.js toolchain across distros.
- **Instant NPM Release Parity**: Queries `https://registry.npmjs.org/freebuff/latest` directly over HTTPS, verifies the SHA-256 checksum from the npm manifest, supports HTTP `Range` download resumption, and stages background updates (`deferUpdatesUntilExit`) identically to upstream `launcher.js`.
- **No Two-Layer Cache Desync**: Eliminates the `~/.npm/_npx` cache layer completely.
- **Legacy Nix Wrapper Detection**: Automatically detects previous `npx`-based Nix `freebuff-cli` wrappers (`/etc/profiles/per-user/$USER/bin/freebuff-cli`, `freebuff-cli.nix`, `freebuff-cli-check.timer`) and prompts to disable/remove them cleanly.
- **Kitty Desktop Launcher**: Creates `~/.local/share/applications/freebuff-cli.desktop` to launch `freebuff-cli` directly in a dedicated **Kitty** terminal window.

---

## Dependencies & Requirements

### 1. Build-Time Dependencies
- **Go**: **`>= 1.22`** (tested with Go `1.26.x`).
- **External Go Modules**: **None** (`go.mod` uses 100% Go standard library; `vendorHash = null` in Nix, no network needed during `go build`).

### 2. Runtime Requirements
- **Kitty Terminal (`kitty`) — Required**: Used by the desktop launcher (`freebuff-cli.desktop`) and recommended for full keyboard protocol and truecolor support.
- **`glibc` / Dynamic ELF Loader — Required**: The downloaded upstream `~/.config/manicode/freebuff` binary is a dynamically linked 64-bit ELF executable (`/lib64/ld-linux-x86-64.so.2`).
  - On **Debian, Ubuntu, Arch, Fedora, Gentoo**: Provided out of the box by `glibc` / `libc6`.
  - On **NixOS**: Requires `programs.nix-ld.enable = true;` in your system configuration so `/run/current-system/sw/share/nix-ld/lib/ld.so` can start foreign ELF binaries.
- **`ca-certificates` — Required**: Standard system TLS root certificates for HTTPS requests to `registry.npmjs.org` and `codebuff.com`.
- **`libnotify` (`notify-send`) — Optional**: Used by `freebuff-cli --notify` for desktop notifications when a new upstream release is published.
- **`coreutils` (`stty`) — Optional**: Used to drain pending OSC 11 terminal background-color replies on short-lived flags (`--version`, `--help`), with a built-in Go syscall fallback if absent.

You can verify all prerequisites at any time with:
```bash
./freebuff-cli --doctor
```
And install missing system packages automatically on Debian/Ubuntu, Arch, Fedora, or Gentoo with:
```bash
./freebuff-cli --install-prereqs
```

---

## How to Build

From inside the `go-build` directory:

```bash
cd go-build

# Build a static, stripped binary (CGO_ENABLED=0)
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o freebuff-cli ./cmd/freebuff-cli
```

Cross-compile for ARM64 (`linux-arm64`) if needed:
```bash
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o freebuff-cli ./cmd/freebuff-cli
```

Run unit tests:
```bash
go test -v ./...
```

---

## CLI Usage & Flags

```text
Usage: freebuff-cli v1.0.0 [OPTIONS] [--] [ARGS...]

Options:
  -h, --help                Show this help message and exit
  -v, --version             Print launcher and installed freebuff versions and exit
  -c, --check               Check installed vs. latest version and readiness status
  -u, --upgrade, --update   Download/update freebuff to the latest release immediately
  -d, --doctor, --prereqs   Diagnose prerequisites, distro, nix-ld, kitty, and binary cache
      --install-prereqs     Install system prerequisites (kitty, ca-certificates, libnotify)
      --install             Install binary to /usr/local/bin/freebuff-cli, create Kitty desktop icon, and check legacy Nix wrapper
  -r, --remove, --uninstall Remove installed binary and Kitty desktop icon
      --desktop-entry       Create/update only the Kitty desktop icon (~/.local/share/applications/freebuff-cli.desktop)
      --remove-sh           Remove installed NPM-based freebuff-cli.sh script and global npm/npx freebuff caches
      --remove-legacy-nix   Check for and prompt to remove previous Nix npx-based freebuff-cli generator
      --status              Emit single-line JSON update status for Waybar / Noctalia bar widgets
      --notify              Send a desktop notification (notify-send) if a newer release is available
  -n, --dry-run             Show what would be downloaded/installed/executed without changing anything
      --force               Run even if optional checks warn
  -y, --yes                 Assume yes to interactive prompts (e.g., removing legacy Nix generator)
```

### Common Commands

```bash
# Check version
./freebuff-cli -v

# Check for updates and readiness status
./freebuff-cli -c

# Full prerequisite & toolchain diagnostic
./freebuff-cli --doctor

# Install system prerequisites (kitty, ca-certificates, libnotify)
./freebuff-cli --install-prereqs

# Download or upgrade the upstream freebuff binary to latest
./freebuff-cli --upgrade

# Install freebuff-cli binary to PATH + create Kitty desktop entry + prompt to remove old Nix/bash wrappers
./freebuff-cli --install

# Remove the old NPM-based freebuff-cli.sh script and npm/npx cache
./freebuff-cli --remove-sh

# Launch Freebuff
freebuff-cli
```

---

## Distro Installation & Packaging

### 1. Quick Local Install (Any Distro)
```bash
# Installs to /usr/local/bin/freebuff-cli (or ~/.local/bin/freebuff-cli on NixOS)
# Also creates ~/.local/share/applications/freebuff-cli.desktop for Kitty
./freebuff-cli --install
```
Or install to `~/.local/bin/freebuff-cli` without root:
```bash
FREEBUFF_INSTALL_PATH="$HOME/.local/bin/freebuff-cli" ./freebuff-cli --install
```

### 2. Nix / NixOS (`flake.nix` & `nixos/freebuff-cli.nix`)
1. Ensure `programs.nix-ld.enable = true;` is set in your NixOS configuration.
2. Build or run directly with Nix Flakes:
   ```bash
   nix build ./go-build
   ./result/bin/freebuff-cli --check
   ```
3. Or import `go-build/nixos/freebuff-cli.nix` into Home Manager (replaces the old `npx`-based `freebuff-cli.nix` module and automatically installs `freebuff-cli`, `kitty`, `libnotify`, the Kitty `.desktop` entry, and the 4-hour systemd update check timer):
   ```nix
   imports = [ ./go-build/nixos/freebuff-cli.nix ];
   programs.freebuff-cli.enable = true;
   ```
4. If migrating from the previous `npx`-based Nix wrapper, run:
   ```bash
   ./freebuff-cli --remove-legacy-nix
   ```

### 3. Debian / Ubuntu (`.deb`)
```bash
./packaging/debian/build-deb.sh
sudo apt install ./freebuff-cli_1.0.0_amd64.deb
```

### 4. Arch Linux / Manjaro / EndeavourOS (`PKGBUILD`)
```bash
cd packaging/arch
makepkg -si
```

### 5. Fedora / RHEL (`RPM spec`)
```bash
rpmbuild -bb packaging/fedora/freebuff-cli.spec
```

### 6. Gentoo (`ebuild`)
Copy `packaging/gentoo/freebuff-cli-1.0.0.ebuild` to your local overlay under `app-misc/freebuff-cli/` and run:
```bash
sudo emerge -av app-misc/freebuff-cli
```
