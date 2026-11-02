# 📜 Changelog

All notable changes to this project are documented in this file.

The version comes from `SCRIPT_VERSION` inside `freebuff-cli.sh`; check yours with
`./freebuff-cli.sh --version` and print the built-in history with `./freebuff-cli.sh --changes`.
Format loosely follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

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
[v0.6]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.5]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.4]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.3]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.2]: https://github.com/dwilliam62/FreeBuff-cli-installer
[v0.1]: https://github.com/dwilliam62/FreeBuff-cli-installer
