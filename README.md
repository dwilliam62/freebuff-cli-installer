<div align="center">

# 🚀 Freebuff CLI Installer

**One command to get [Freebuff](https://freebuff.com) — the AI coding agent — running on your Linux box.**

<p>
  <img alt="go version" src="https://img.shields.io/badge/standalone_go-v1.0.0-00ADD8?style=for-the-badge&logo=go&logoColor=white">
  <img alt="script version" src="https://img.shields.io/badge/installer-v1.0.0-2ea043?style=for-the-badge&logo=gnubash&logoColor=white">
  <img alt="platforms" src="https://img.shields.io/badge/platform-NixOS%20%7C%20Debian%20%7C%20Ubuntu%20%7C%20Arch%20%7C%20Fedora%20%7C%20Gentoo-1f6feb?style=for-the-badge&logo=linux&logoColor=white">
  <img alt="shell" src="https://img.shields.io/badge/shell-bash%20%2F%20go-4eaa25?style=for-the-badge&logo=gnubash&logoColor=white">
  <img alt="license" src="https://img.shields.io/badge/license-GPL--3.0-blueviolet?style=for-the-badge&logo=opensourceinitiative&logoColor=white">
</p>

<p>
  <a href="Quick-Start-Guide.md"><img alt="Quick Start Guide" src="https://img.shields.io/badge/Docs-Quick--Start--Guide.md-orange?style=flat-square&logo=gitbook&logoColor=white"></a>
  <a href="https://freebuff.com"><img alt="Website" src="https://img.shields.io/badge/Website-freebuff.com-8a2be2?style=flat-square&logo=googlechrome&logoColor=white"></a>
  <a href="https://discord.com/invite/yXG3w7wxfs"><img alt="Discord" src="https://img.shields.io/badge/Discord-Join%20the%20community-5865f2?style=flat-square&logo=discord&logoColor=white"></a>
  <a href="https://github.com/dwilliam62/FreeBuff-cli-installer"><img alt="GitHub" src="https://img.shields.io/badge/GitHub-FreeBuff--cli--installer-181717?style=flat-square&logo=github&logoColor=white"></a>
</p>

</div>

---

## ⚡ Quick Start

Clone the repository and run the unified installer (`install.sh`):

```bash
git clone https://github.com/dwilliam62/FreeBuff-cli-installer.git
cd FreeBuff-cli-installer
chmod +x install.sh
./install.sh
```

The installer detects your Linux distribution (**NixOS**, **Debian**, **Ubuntu**, **Fedora**, **Arch Linux**, **Gentoo**, etc.), checks for legacy shell/NPM versions and prompts to clean them up, deploys the standalone Go binary (`freebuff-cli`) to your PATH, and sets up the **Kitty** desktop launcher.

For a fast step-by-step walkthrough and distro tips, see the **[Quick Start Guide](Quick-Start-Guide.md)**.

---

## 🎯 Purpose & Editions

Freebuff upstream publishes its coding agent as a native ELF binary verified against an npm package manifest. This repository provides two editions:

1. **🚀 Standalone Go Edition (`install.sh` + `go-build/`) — Recommended**:
   - A single, static native binary (`freebuff-cli`, `CGO_ENABLED=0`, Go standard library only).
   - **Zero Node.js / npm / npx / nvm dependencies**: Connects directly to `registry.npmjs.org` over HTTPS.
   - Verifies SHA-256 checksums, supports resumable downloads, and manages `~/.config/manicode/freebuff`.
   - Automatically handles AVX2 CPU detection with `-baseline` fallback on `SIGILL`.
   - Includes **Kitty** desktop integration (`freebuff-cli.desktop`).
2. **📜 Legacy Shell & NPM Edition (`legacy-shell/`, `legacy-nixos/`)**:
   - Preserved for users who require the original Node.js/nvm bootstrapping bash script (`legacy-shell/freebuff-cli.sh`) or the legacy Home Manager wrapper (`legacy-nixos/nixos/freebuff-cli.nix`).

---

## 🎛️ Installer Usage (`install.sh`)

```console
Usage: ./install.sh [OPTIONS]
```

<div align="left">

| Flag | Description |
|:--|:--|
| *(none)* | Full guided install: detects legacy shell versions, installs binary & Kitty desktop icon |
| `-c`, `--check` | Status check: verifies installed version, detects binary vs. shell version, checks deps |
| `-u`, `--upgrade` | Rebuilds/installs the standalone binary and updates the upstream Freebuff agent |
| `-r`, `--remove` | Uninstalls the stationary binary and removes the Kitty desktop launcher |
| `-d`, `-n`, `--dry-run` | Prints every command that would run without modifying the system |
| `-y`, `--yes` | Non-interactive mode (assumes yes to prompts, e.g. removing legacy shell versions) |
| `-v`, `--version` | Prints installer version |
| `-h`, `--help` | Displays help message and environment overrides |

</div>

### Examples

```bash
./install.sh                # Install binary to PATH and create Kitty desktop launcher
./install.sh --check        # Inspect environment, detect legacy wrappers, verify prerequisites
./install.sh --dry-run      # Preview all changes first
./install.sh --upgrade      # Upgrade freebuff-cli binary and fetch latest upstream agent
./install.sh --remove       # Uninstall binary and desktop icon
```

---

## 📁 Repository Directory Structure

```text
FreeBuff-cli-installer/
├── install.sh                  # 🚀 Universal root installer for all Linux distributions
├── Quick-Start-Guide.md        # ⚡ Step-by-step setup and distro guide
├── README.md                   # 📖 Project documentation
├── CHANGELOG.md                # 📜 Project changelog
├── go-build/                   # 🐹 Standalone Go binary source & packaging
│   ├── cmd/freebuff-cli/       # CLI entrypoint (main.go)
│   ├── internal/launcher/      # Core launcher, HTTP resume, semver, CPU AVX2, and doctor logic
│   ├── flake.nix               # Nix Flake definition (vendorHash = null)
│   ├── nixos/                  # Modern NixOS Home Manager module (freebuff-cli.nix)
│   ├── packaging/              # Native packaging recipes (.deb, PKGBUILD, .spec, .ebuild)
│   ├── go.mod                  # Zero-dependency Go module (Go >= 1.22)
│   └── README.md               # Go build and packaging documentation
├── legacy-shell/               # 📜 Original NPM-based bash installer
│   └── freebuff-cli.sh         # Legacy Node.js/NodeSource/nvm bootstrapping script
└── legacy-nixos/               # 📜 Original NixOS npx wrapper
    └── nixos/
        └── freebuff-cli.nix    # Legacy Home Manager module invoking npx
```

---

## 📋 Runtime Prerequisites

- **Kitty Terminal (`kitty`) — Recommended**: The desktop launcher (`freebuff-cli.desktop`) opens Freebuff inside Kitty for complete keyboard protocol, mouse event reporting, and truecolor support.
- **Dynamic ELF Loader (`glibc` / `nix-ld`)**: The downloaded agent binary is a 64-bit dynamically linked ELF executable.
  - **Debian / Ubuntu / Fedora / Arch / Gentoo**: Provided natively by `glibc` / `libc6`.
  - **NixOS**: Requires `programs.nix-ld.enable = true;` in your system configuration.
- **TLS CA Certificates (`ca-certificates`)**: System root certificates for TLS connections to `registry.npmjs.org` and `codebuff.com`.

---

## 🐧 Distro Integration

### 1. NixOS
- **Via Flake & Home Manager**:
  Import `go-build/nixos/freebuff-cli.nix` in your configuration:
  ```nix
  {
    imports = [ ./go-build/nixos/freebuff-cli.nix ];
    programs.freebuff-cli.enable = true;
  }
  ```
- **Via `install.sh`**:
  Running `./install.sh` on NixOS automatically targets `~/.local/bin/freebuff-cli` (no root required) and installs the Kitty desktop entry.

### 2. Debian / Ubuntu
Build or install the standalone `.deb`:
```bash
./go-build/packaging/debian/build-deb.sh
sudo apt install ./go-build/freebuff-cli_1.0.0_amd64.deb
```

### 3. Arch Linux / Manjaro
```bash
cd go-build/packaging/arch
makepkg -si
```

### 4. Fedora / RHEL
```bash
rpmbuild -bb go-build/packaging/fedora/freebuff-cli.spec
```

### 5. Gentoo
Use the Portage ebuild in `go-build/packaging/gentoo/freebuff-cli-1.0.0.ebuild`:
```bash
sudo emerge -av app-misc/freebuff-cli
```

---

## 🩺 Troubleshooting

<div align="left">

| Symptom | Cause & Fix |
|:--|:--|
| `Could not start dynamically linked executable` (NixOS) | Enable `programs.nix-ld.enable = true;` in your NixOS config and rebuild (`nixos-rebuild switch`). |
| `Legacy shell/NPM version detected` | Run `./install.sh` and answer `Y` when prompted to remove the old shell script and stale npx cache. |
| `Kitty terminal not found` | Install `kitty` via your distro package manager (`apt install kitty`, `pacman -S kitty`, `dnf install kitty`, or `emerge x11-terms/kitty`). |
| `... is not in your PATH` | Add `~/.local/bin` or `/usr/local/bin` to PATH in `~/.bashrc` or `~/.zshrc`. |
| `npm registry unreachable` | Check internet or proxy settings (`HTTP_PROXY`, `HTTPS_PROXY`); cached binary runs offline. |

</div>

---

## 🤝 Community & Links

<div align="center">

|  |  |
|:--|:--|
| 🌐 **Website** | <https://freebuff.com> |
| 💬 **Discord** | <https://discord.com/invite/yXG3w7wxfs> |
| 🐙 **Repository** | <https://github.com/dwilliam62/FreeBuff-cli-installer> |
| 📜 **Changelog** | [CHANGELOG.md](CHANGELOG.md) |
| ⚡ **Quick Start Guide** | [Quick-Start-Guide.md](Quick-Start-Guide.md) |

</div>

---

## 📄 License

GNU General Public License v3.0 — see [LICENSE](LICENSE).

<div align="center">

**Happy coding with Freebuff!** ✨

</div>
