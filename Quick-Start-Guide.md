# ⚡ Freebuff CLI Quick Start Guide

Get the [Freebuff](https://freebuff.com) AI coding agent installed, configured, and running on your Linux system in minutes.

---

## 🎯 Quick Installation (1 Step)

Clone the repository and run `install.sh`:

```bash
git clone https://github.com/dwilliam62/FreeBuff-cli-installer.git
cd FreeBuff-cli-installer
chmod +x install.sh
./install.sh
```

The installer will:
1. Detect your Linux distribution (**NixOS**, **Debian**, **Ubuntu**, **Fedora**, **Arch Linux**, **Gentoo**, etc.).
2. Check for any legacy NPM/shell-based versions and prompt to remove them cleanly.
3. Build or deploy the static Go binary (`freebuff-cli`) directly to your user or system PATH.
4. Install the **Kitty** desktop application launcher (`freebuff-cli.desktop`).

---

## 📋 Runtime Prerequisites

- **Kitty Terminal (`kitty`) — Recommended**: The desktop launcher automatically opens Freebuff in a dedicated Kitty window for full keyboard protocol, bracketed paste, and truecolor support.
- **Dynamic ELF Loader (`glibc` / `nix-ld`)**: Freebuff downloads a high-performance native ELF binary on startup.
  - **Debian / Ubuntu / Fedora / Arch / Gentoo**: Provided out of the box by `glibc`.
  - **NixOS**: Requires `programs.nix-ld.enable = true;` in your NixOS configuration so the binary can run.
- **TLS CA Certificates (`ca-certificates`)**: Standard root certificates for downloading release archives from `registry.npmjs.org` and `codebuff.com`.

Run the built-in diagnostic at any time to verify your system:
```bash
./install.sh --check
```
Or directly via the binary:
```bash
freebuff-cli --doctor
```

---

## 🚀 Running Freebuff

### 1. From Terminal
Launch Freebuff from any terminal:
```bash
freebuff-cli
```
*On first launch, the native binary (~138MB) and `tree-sitter.wasm` are automatically downloaded into `~/.config/manicode/freebuff` and verified via SHA-256.*

### 2. From Your Desktop Launcher
Search for **Freebuff CLI** in your application launcher (Rofi, Wofi, GNOME, KDE, Noctalia, etc.) to start Freebuff inside a dedicated Kitty window.

---

## 🛠️ Handy Commands

| Command | Action |
| :--- | :--- |
| `freebuff-cli -v` | Print launcher & installed agent versions |
| `freebuff-cli -c` | Check for newer upstream npm releases and status |
| `freebuff-cli -u` | Immediately download and install the latest upstream release |
| `freebuff-cli --doctor` | Run full prerequisite, dynamic loader, and environment diagnosis |
| `freebuff-cli --status` | Output single-line JSON status for Waybar or Noctalia widgets |
| `freebuff-cli --notify` | Send desktop notification (`notify-send`) when a new release lands |
| `./install.sh -d` | Dry-run preview of installation commands |
| `./install.sh -r` | Cleanly remove the binary and Kitty desktop launcher |

---

## 🐧 Distro-Specific Setup

### NixOS
1. **Enable `nix-ld`** in your system configuration (e.g. `/etc/nixos/configuration.nix` or your flake):
   ```nix
   programs.nix-ld.enable = true;
   ```
2. **Declarative Home Manager Module**:
   Import `go-build/nixos/freebuff-cli.nix` into your Home Manager flake:
   ```nix
   {
     imports = [ ./go-build/nixos/freebuff-cli.nix ];
     programs.freebuff-cli.enable = true;
   }
   ```
   This automatically manages the standalone Go package, Kitty terminal, desktop icon, and background update-check timer.

### Debian / Ubuntu
Build or install the standalone `.deb`:
```bash
./go-build/packaging/debian/build-deb.sh
sudo apt install ./go-build/freebuff-cli_1.0.0_amd64.deb
```

### Arch Linux / Manjaro
Build and install via `PKGBUILD`:
```bash
cd go-build/packaging/arch
makepkg -si
```

### Fedora / RHEL
Build and install using the RPM spec:
```bash
rpmbuild -bb go-build/packaging/fedora/freebuff-cli.spec
```

### Gentoo
Use the ebuild in `go-build/packaging/gentoo/freebuff-cli-1.0.0.ebuild`:
```bash
sudo emerge -av app-misc/freebuff-cli
```

---

## 📁 Repository Layout

- `install.sh` — Universal bash installer for all distros (Go binary install, NixOS support, legacy shell cleanup).
- `go-build/` — Standalone Go implementation (`freebuff-cli`) with zero external module dependencies.
  - `flake.nix` — Nix Flake definition (`vendorHash = null`).
  - `nixos/freebuff-cli.nix` — Modern Home Manager module for NixOS.
  - `packaging/` — Native packaging files (`.deb`, `PKGBUILD`, `.spec`, `.ebuild`).
- `legacy-shell/` — Contains original `freebuff-cli.sh` bash script for users who prefer the legacy Node.js/nvm path.
- `legacy-nixos/` — Contains original `nixos/freebuff-cli.nix` legacy wrapper module.
