# 📜 Changelog

All notable changes to this project are documented in this file.

The version comes from `SCRIPT_VERSION` inside `freebuff-cli.sh`; check yours with
`./freebuff-cli.sh --version` and print the built-in history with `./freebuff-cli.sh --changes`.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [v1.0.0] — 2026-11-07 (`go-build/` Standalone Cross-Distro Edition)

### Added
- 🚀 **Universal Root Installer (`install.sh`)** — a friendly bash script with colored, iconic output supporting NixOS, Debian, Ubuntu, Fedora, Arch Linux, and Gentoo. Installs the stationary Go binary, creates the Kitty desktop entry, and includes `-c/--check`, `-u/--upgrade`, `-r/--remove`, `-d/--dry-run`, `-h/--help`, and `-v/--version`.
- ⚡ **`Quick-Start-Guide.md`** — comprehensive 1-step installation and distro setup guide.
- 📦 **Legacy Relocation (`legacy-shell/`, `legacy-nixos/`)** — moved original `freebuff-cli.sh` and `nixos/freebuff-cli.nix` into dedicated `legacy-` directories in case users still prefer the Node.js/nvm/npx route.
- 🚀 **Standalone Go `freebuff-cli` (`go-build/`)** — a zero-dependency (`CGO_ENABLED=0`, Go `>= 1.22` standard library only) native launcher and updater that replaces the Node.js/npm/npx wrapper chain across **NixOS**, **Debian**, **Ubuntu**, **Gentoo**, **Arch Linux**, and **Fedora**.
- 🔒 **Direct NPM Registry & Release Sync** — queries `https://registry.npmjs.org/freebuff/latest` over HTTPS, validates strict semver, verifies SHA-256 digests against the npm `binaryChecksums` map, enforces redirect host allowlisting (`codebuff.com`, `freebuff.com`, `github.com`, `*.githubusercontent.com`), supports HTTP `Range` download resumption, and extracts `freebuff` and `tree-sitter.wasm` into `~/.config/manicode/`.
- 🧠 **CPU AVX2 Detection & `SIGILL` Fallback** — probes `/proc/cpuinfo` on Linux for `avx2`, manages `~/.config/manicode/cpu-features.json`, and automatically switches to `linux-x64-baseline` if the optimized binary exits with `SIGILL`.
- 🔄 **Deferred Background Updates & Orphan Adoption** — checks for newer releases 100ms after session startup, stages verified updates under `~/.config/manicode/.freebuff-download-temp-<pid>/staged.json`, installs them on exit, and adopts orphaned staged updates on the next launch if the terminal was closed with `SIGHUP`.
- 🧹 **Legacy Nix & Bash Wrapper Detection & Cleanup (`--remove-legacy-nix`, `--remove-sh`)** — detects previous `npx`-based Nix `freebuff-cli` wrappers (`/etc/profiles/per-user/$USER/bin/freebuff-cli`, `~/.nix-profile/bin/freebuff-cli`, `freebuff-cli.nix` modules, and `freebuff-cli-check.timer`) as well as installed NPM-based `freebuff-cli.sh` copies (`/usr/local/bin/freebuff-cli.sh`, `~/.local/bin/freebuff-cli.sh`) and global npm / `~/.npm/_npx/*/node_modules/freebuff` caches, prompting to remove them cleanly.
- 🐱 **Kitty Desktop Entry (`--desktop-entry` / `--install`)** — creates and manages `~/.local/share/applications/freebuff-cli.desktop` (`Exec=kitty --title "Freebuff CLI" --class freebuff-cli -e ...`) to launch Freebuff in a dedicated Kitty terminal window.
- 🩺 **Prerequisite Checker & Auto-Installer (`--doctor` / `--prereqs` / `--install-prereqs`)** — checks `nix-ld` / `glibc` dynamic loader, TLS CA certificates, `kitty`, `notify-send`, `stty`, PATH, and registry health, and can install system prerequisites (`kitty`, `ca-certificates`, `libnotify`, `coreutils`) automatically on Debian/Ubuntu (`apt-get`), Arch (`pacman`), Fedora (`dnf`), and Gentoo (`emerge`).
- 📊 **Status Bar & Notification Flags (`--status` / `--notify`)** — emits single-line JSON for Waybar / Noctalia bar widgets and sends one-shot `notify-send` desktop alerts per upstream release using a 6-hour registry cache (`~/.cache/freebuff-cli/npm-latest.json`).
- 📦 **Cross-Distro Packaging (`go-build/flake.nix`, `go-build/nixos/`, `go-build/packaging/`)** — includes a Nix Flake (`vendorHash = null`) and Home Manager module (`go-build/nixos/freebuff-cli.nix`), plus native packaging recipes for Debian/Ubuntu (`build-deb.sh`), Arch (`PKGBUILD`), Fedora (`freebuff-cli.spec`), and Gentoo (`freebuff-cli-1.0.0.ebuild`).

---

## [v0.7] — 2026-11-05

### Added
- 🩺 **`--doctor`** (`-d`, and the bare word `doctor`) — a read-only diagnosis one step deeper
  than `--check`. It walks `System`, `Downloader`, `Node.js`, `Terminal`, `Self-install` and
  `freebuff`, marking each check `[ok]`, `[warn]` or `[FAIL]` and printing a `fix:` line under
  whatever is wrong: unsupported distro, no curl/wget for the fallbacks, Node below
  `FREEBUFF_MIN_NODE_MAJOR`, missing npm or npx, a non-TTY stdin/stdout, missing `stty`, an
  unset or `dumb` `TERM`, a missing or unwritable install directory, that directory missing
  from `PATH`, an unreachable npm registry, an outdated global freebuff, and the
  `~/.config/manicode` binary cache. Exits `1` only when a check **fails**; warnings alone
  still exit `0`.
- 📖 README documents `--doctor` beside `--check`, including when to reach for each.

### Changed
- 🚦 `--doctor` runs **before** the distro gate, so an unsupported distro is reported together
  with the `--force` / `--nvm` options instead of exiting with `Unsupported distro`.
- 🔢 `SCRIPT_VERSION` bumped to `v0.7`; `--help`, `--changes` and the built-in history list the
  new flag.
- 🎨 `nixos/freebuff-cli.nix` is kept clean under `alejandra` 4.1.0, `statix check` and
  `deadnix`, and the module still evaluates and builds with nix 2.35.4.

### Fixed
- 🐧 `nixos/freebuff-cli.nix` — the wrapper now reads and discards freebuff's pending OSC 11
  background-colour reply after `--version`/`--help`, so bytes like `11;rgb:…` no longer leak
  into the caller's shell. `-V` takes that path too, matching the npm CLI's other `--version`
  alias; previously it fell through to the `exec` path and could leak the reply. The bash
  script needs no equivalent: its `--version` prints locally and its launch goes through
  `exec npx`, which replaces the shell.

---

## [v0.6] — 2026-11-01

### Added
- 📥 **Self-install on a normal run** — `maybe_install_self` copies the script to
  `$INSTALL_PATH` (default `/usr/local/bin/freebuff-cli.sh`) as part of a plain run, so one
  invocation now leaves a copy on `PATH` instead of only doing so with `--install`.
  It is best-effort and never fatal: it is skipped when the running file *is* the installed
  copy or when `$0` is not a real file (`sh freebuff-cli.sh`, `curl … | bash`), and when the
  copy fails (no sudo, missing/unwritable target) it prints
  `Not installed to … — run '… --install'` and launches anyway.

### Changed
- 🧹 The distro gate calls `os_family strict >/dev/null` instead of capturing an unused
  `SUPPORTED_PM`, clearing shellcheck `SC2034`.

### Fixed
- 💬 Running the installer twice could leave nothing in `/usr/local/bin`: a plain run never
  copied the script. It now does, and says so when it cannot.

---

## [v0.5] — 2026-11-01

### Added
- 🧩 **Gentoo support** — `emerge` is now a fourth supported package manager:
  `os_family` maps `ID=gentoo` to `emerge` (and probes for the `emerge` command as a
  fallback), and `install_node_distro` runs
  `emerge --ask=n --noreplace dev-lang/nodejs`, which brings npm and npx with it via the
  default `npm` USE flag and leaves the world file untouched.
- 📖 README gained a Gentoo row in the supported-systems table, an inline explanation of the
  emerge route (no NodeSource repo, root-owned npm prefix) and two Gentoo troubleshooting rows.

### Changed
- 🚫 `install_node_nodesource` short-circuits on `emerge` with
  "NodeSource does not publish packages for Gentoo", so an outdated `dev-lang/nodejs` falls
  through to the nvm route instead of failing.
- 💬 Distro-gate error, `--help` and `--check` messages now list `emerge` alongside
  apt/pacman/dnf.

---

## [v0.4] — 2026-11-01

### Changed
- 🚚 Uninstall moved from `-u/--uninstall` to **`-r/--remove`**; `--uninstall` is still accepted as an alias.
- ↩️ `-u` is short for **`--upgrade`** again (the slot was free after the uninstall move).
- 📄 Script moved into this repository as `freebuff-cli.sh`.

---

## [v0.3] — 2026-11-01

### Added
- 🗑️ Uninstall flag (shipped as `-u/--uninstall`, renamed in v0.4) — removes only the
  installed copy, never the directory itself.

### Changed
- ♻️ **Refactor**, no behaviour change: `detect_pm` and `supported_distro` merged into a
  single `os_family [strict]` helper; one `download <url> <dest>` helper replaces the
  duplicated curl/wget blocks; `as_root` / `pkg_run` replace the repeated
  `"${SUDO[@]+"${SUDO[@]}"}"` expansions; `copy_into` / `elevation_for` collapse
  `install_self`'s two near-identical branches.
- 🩺 `--check` now also reports the script version and install path.

---

## [v0.2] — 2026-11-01

### Added
- ⌨️ `-v` accepted as a lowercase alias for `--version` (previously only `-V`/`--version`).

---

## [v0.1] — 2026-11-01

### Added
- 🏷️ First numbered version, introducing `SCRIPT_VERSION` and the `--version` flag.
- 🕘 `--changes` (`--changelog`): built-in change history.

---

## Earlier, unnumbered work

### Added
- 🧱 **Distro gate** — refuses anything that is not Debian/Ubuntu, Arch or Fedora based,
  exiting with `Unsupported distro` before touching the system.
- 💪 `--force` — escape hatch that bypasses the distro gate for testing.
- 📦 `--install` — copies the script to `FREEBUFF_INSTALL_PATH`
  (default `/usr/local/bin/freebuff-cli.sh`); refuses to create a missing directory.
- 🩺 `--check` — read-only status report for distro, Node, npm, npx and freebuff, with an
  exit code (`0` ready, `1` not ready).
- 🌱 `--nvm` fallback — installs Node.js under `$HOME/.nvm` with no root, `PROFILE=/dev/null`
  so shell rc files are never edited.
- 🔁 **NodeSource fallback** — installs a current Node.js (`FREEBUFF_NODESOURCE_MAJOR`,
  default 22) when `apt`/`dnf` ship something older than the minimum.
- 🐧 **Distro installs** for `apt`, `pacman` and `dnf`.
- 🔢 Version reporting — installed vs. latest published freebuff version, plus `--upgrade`
  (`npm install -g freebuff@latest`).
- 🧪 `--dry-run` — prints every install and launch command instead of running it.
- 🧯 Graceful npm handling — if the global install fails (read-only prefix), it warns and
  continues through `npx --yes`.
- 🚀 **First version** — install `freebuff` globally if it is missing, then launch it via `npx`.

---

## NixOS module

### Added
- 🐧 `nixos/freebuff-cli.nix` — Home Manager module providing a `freebuff-cli` wrapper around
  `npx --yes freebuff`, with Node.js on `PATH` and a fail-fast check for `programs.nix-ld`,
  which the downloaded freebuff binary requires on NixOS.

---

[Unreleased]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v1.0.0]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.7]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.6]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.5]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.4]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.3]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.2]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.1]: https://github.com/dwilliam62/FreeBuff-cli-installer
