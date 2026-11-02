# 📜 Changelog

All notable changes to this project are documented in this file.

The version comes from `SCRIPT_VERSION` inside `freebuff-cli.sh`; check yours with
`./freebuff-cli.sh --version` and print the built-in history with `./freebuff-cli.sh --changes`.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
[v0.4]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.3]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.2]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.1]: https://github.com/dwilliam62/FreeBuff-cli-installer
