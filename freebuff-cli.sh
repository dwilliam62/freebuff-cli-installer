#!/usr/bin/env bash
#
# freebuff-cli.sh — ensure Node.js/npm and the Freebuff CLI are installed,
# report versions, then launch the CLI.
#
# Supported systems: Debian/Ubuntu, Arch, Fedora and Gentoo based distributions only.
# Anything else (including NixOS) exits with "Unsupported distro" unless --force.
#
set -euo pipefail

PACKAGE="freebuff"
# Version of this script. Bump it on every change (v0.1 was the first).
SCRIPT_VERSION="v0.7"
# Minimum Node.js major version this script will accept.
MIN_NODE_MAJOR="${FREEBUFF_MIN_NODE_MAJOR:-18}"
# Node.js major version installed by the NodeSource fallback.
NODESOURCE_MAJOR="${FREEBUFF_NODESOURCE_MAJOR:-22}"
# nvm version used by the no-root fallback.
NVM_VERSION="${FREEBUFF_NVM_VERSION:-v0.40.1}"
# Where --install copies this script to, and --uninstall removes it from.
INSTALL_PATH="${FREEBUFF_INSTALL_PATH:-/usr/local/bin/freebuff-cli.sh}"

UPGRADE=0
DRY_RUN=0
USE_NVM=0
CHECK=0
DOCTOR=0
INSTALL=0
UNINSTALL=0
FORCE=0
CHANGES=0
FORWARD=()

usage() {
  cat <<EOF
Usage: $(basename "$0") $SCRIPT_VERSION [OPTIONS] [--] [ARGS...]

  -r, --remove    Remove the installed copy at $INSTALL_PATH and exit
  -u, --upgrade   Update freebuff to the latest version before launching
  -n, --dry-run   Show the commands that would run, change nothing
      --nvm       Install Node.js with nvm (no root needed) instead of system packages
  -c, --check     Report the current status and exit; installs and launches nothing
  -d, --doctor    Diagnose the whole toolchain, printing a fix for each problem; exits 1 on a failure
      --install   Copy this script to $INSTALL_PATH and exit
      --force     Run even on an unsupported distro (testing escape hatch)
      --changes   Print what this script does and its change history, then exit
  -v, --version   Print this script's version and exit
  -h, --help      Show this help and exit

Supported systems: Debian/Ubuntu (apt), Arch (pacman), Fedora/RHEL (dnf),
                   Gentoo (emerge).
Any other arguments are passed through to freebuff.

Env overrides:
  FREEBUFF_MIN_NODE_MAJOR      minimum Node.js major version (default $MIN_NODE_MAJOR)
  FREEBUFF_NODESOURCE_MAJOR    Node.js major installed by the NodeSource fallback (default $NODESOURCE_MAJOR)
  FREEBUFF_NVM_VERSION         nvm git tag installed by the fallback (default $NVM_VERSION)
  FREEBUFF_INSTALL_PATH        target for --install/--uninstall (default $INSTALL_PATH)
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    -u|--upgrade)  UPGRADE=1; shift ;;
    -n|--dry-run)  DRY_RUN=1; shift ;;
    --nvm)         USE_NVM=1; shift ;;
    -c|--check)    CHECK=1; shift ;;
    -d|--doctor|doctor) DOCTOR=1; shift ;;
    --install)     INSTALL=1; shift ;;
    -r|--remove|--uninstall) UNINSTALL=1; shift ;;
    --force)       FORCE=1; shift ;;
    --changes|--changelog) CHANGES=1; shift ;;
    -v|-V|--version) printf '%s %s\n' "$(basename "$0")" "$SCRIPT_VERSION"; exit 0 ;;
    -h|--help)     usage; exit 0 ;;
    --)            shift; FORWARD+=("$@"); break ;;
    *)             FORWARD+=("$1"); shift ;;
  esac
done

# ---------------------------------------------------------------- helpers ---
log() { printf '%s\n' "$*"; }
err() { printf 'Error: %s\n' "$*" >&2; }

have() { command -v "$1" >/dev/null 2>&1; }

plan_exit() {
  log "Dry run complete — nothing was installed or launched."
  exit 0
}

# Elevation. SUDO is filled per operation by an explicit branch below, so that
# writable targets never get root-owned files.
SUDO=()
is_root() { [ "${EUID:-$(id -u)}" -eq 0 ]; }
can_elevate() { is_root || have sudo; }

# Run a command, prefixed with sudo only when SUDO is set.
as_root() {
  if [ "${#SUDO[@]}" -gt 0 ]; then
    "${SUDO[@]}" "$@"
  else
    "$@"
  fi
}

# Like as_root, but only prints when --dry-run is set.
pkg_run() {
  local prefix=""
  if [ "${#SUDO[@]}" -gt 0 ]; then
    prefix="${SUDO[*]} "
  fi
  if [ "$DRY_RUN" -eq 1 ]; then
    log "[dry-run] ${prefix}$*"
    return 0
  fi
  as_root "$@"
}

# download <url> <dest>. Returns 1 when no fetcher is available or it fails.
download() {
  local url="$1" dest="$2"
  if have curl; then
    curl -fsSL "$url" -o "$dest" && return 0
    return 1
  fi
  if have wget; then
    wget -qO "$dest" "$url" && return 0
    return 1
  fi
  return 1
}

# Echo the distro package manager: apt | pacman | dnf | emerge.
# With "strict", only /etc/os-release is trusted (no command probing).
os_family() {
  local strict="${1:-}" id="" like="" keys

  if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    id="${ID:-}"
    like="${ID_LIKE:-}"
    keys=" ${id} ${like} "
    case "$keys" in
      *debian*|*ubuntu*|*mint*|*pop*)  echo apt;    return 0 ;;
      *arch*|*manjaro*|*endeavouros*)  echo pacman; return 0 ;;
      *fedora*|*rhel*|*centos*)        echo dnf;    return 0 ;;
      *gentoo*)                        echo emerge; return 0 ;;
    esac
  fi

  if [ "$strict" = "strict" ]; then
    return 1
  fi

  if have apt-get; then echo apt;    return 0; fi
  if have pacman;  then echo pacman; return 0; fi
  if have dnf;     then echo dnf;    return 0; fi
  if have emerge;  then echo emerge; return 0; fi
  return 1
}

# Human-readable distro name, for messages.
distro_label() {
  local pretty=""
  if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    pretty="${PRETTY_NAME:-${ID:-unknown}}"
  fi
  printf '%s' "${pretty:-unknown}"
}

node_major() {
  node --version 2>/dev/null | sed 's/^v//' | cut -d. -f1
}

# npm install -g <spec>. Returns non-zero when npm's global prefix is not
# writable (root-owned prefix, immutable distro), so the caller can fall back.
npm_install_global() {
  if [ "$DRY_RUN" -eq 1 ]; then
    log "[dry-run] npm install -g $1"
    return 0
  fi
  if npm install -g "$1"; then
    return 0
  fi
  log "Warning: the global npm install failed (npm's global prefix is probably read-only)."
  log "Falling back to an on-demand run via npx."
  return 1
}

# ------------------------------------------------------------ Node installs ---
# Install Node.js from the distro repositories. Returns 1 if that is impossible.
install_node_distro() {
  local pm
  if ! pm="$(os_family)"; then
    log "Could not map this system to apt, pacman, dnf or emerge."
    return 1
  fi
  if ! can_elevate; then
    return 1
  fi
  if is_root; then SUDO=(); else SUDO=(sudo); fi

  case "$pm" in
    apt)
      log "Detected a Debian/Ubuntu-based system — installing Node.js with apt..."
      pkg_run apt-get update
      pkg_run apt-get install -y nodejs npm
      ;;
    pacman)
      log "Detected an Arch-based system — installing Node.js with pacman..."
      pkg_run pacman -S --needed --noconfirm nodejs npm
      ;;
    dnf)
      log "Detected a Fedora-based system — installing Node.js with dnf..."
      pkg_run dnf install -y nodejs npm
      ;;
    emerge)
      # Gentoo bundles npm/npx with dev-lang/nodejs (the `npm` USE flag, on by
      # default). --noreplace keeps the world file untouched; --ask=n keeps
      # emerge from prompting. A masked or keyword-gated package makes emerge
      # fail, and ensure_node then falls back to nvm.
      log "Detected a Gentoo-based system — installing Node.js with emerge..."
      pkg_run emerge --ask=n --noreplace dev-lang/nodejs
      ;;
  esac

  if [ "$DRY_RUN" -eq 0 ]; then
    hash -r
  fi
  return 0
}

# Install a current Node.js from the NodeSource repository. Returns 1 on failure.
install_node_nodesource() {
  local pm url script
  if ! pm="$(os_family)"; then
    return 1
  fi
  case "$pm" in
    apt) url="https://deb.nodesource.com/setup_${NODESOURCE_MAJOR}.x" ;;
    dnf) url="https://rpm.nodesource.com/setup_${NODESOURCE_MAJOR}.x" ;;
    pacman)
      log "NodeSource does not publish packages for Arch-based systems."
      return 1
      ;;
    emerge)
      log "NodeSource does not publish packages for Gentoo."
      return 1
      ;;
  esac
  if ! can_elevate; then
    return 1
  fi
  if is_root; then SUDO=(); else SUDO=(sudo); fi

  log "Node.js v${NODESOURCE_MAJOR} is not in your distro — using the NodeSource repository..."

  if [ "$DRY_RUN" -eq 1 ]; then
    local prefix=""
    if [ "${#SUDO[@]}" -gt 0 ]; then prefix="${SUDO[*]} "; fi
    log "[dry-run] download $url"
    log "[dry-run] ${prefix}bash nodesource-setup.sh"
    log "[dry-run] ${prefix}$pm install -y nodejs"
    return 0
  fi

  script="$(mktemp "${TMPDIR:-/tmp}/nodesource_setup.XXXXXX.sh")"
  # shellcheck disable=SC2064
  trap "rm -f '$script'" EXIT

  if ! download "$url" "$script"; then
    log "Failed to download the NodeSource setup script from $url"
    return 1
  fi
  if ! as_root bash "$script"; then
    log "The NodeSource setup script failed."
    return 1
  fi

  case "$pm" in
    apt)
      if ! as_root apt-get install -y nodejs; then
        log "Installing nodejs from the NodeSource repository failed."
        return 1
      fi
      ;;
    dnf)
      if ! as_root dnf install -y nodejs; then
        log "Installing nodejs from the NodeSource repository failed."
        return 1
      fi
      ;;
  esac

  hash -r
  return 0
}

# Install Node.js with nvm into $HOME. Needs no root, so it works when the distro
# has no current Node.js or the user cannot sudo. Returns 1 on failure.
NVM_DIR="${NVM_DIR:-$HOME/.nvm}"

load_nvm() {
  if [ -s "$NVM_DIR/nvm.sh" ]; then
    # shellcheck disable=SC1091
    . "$NVM_DIR/nvm.sh"
    return 0
  fi
  return 1
}

install_node_nvm() {
  local url script major

  export NVM_DIR
  url="https://raw.githubusercontent.com/nvm-sh/nvm/${NVM_VERSION}/install.sh"

  if [ "$DRY_RUN" -eq 1 ]; then
    if ! load_nvm; then
      log "[dry-run] download $url"
      log "[dry-run] PROFILE=/dev/null bash nvm-install.sh"
    fi
    log "[dry-run] nvm install --lts && nvm use --lts"
    return 0
  fi

  if ! load_nvm; then
    log "Installing nvm into $NVM_DIR (no root needed)..."
    script="$(mktemp "${TMPDIR:-/tmp}/nvm_install.XXXXXX.sh")"
    # shellcheck disable=SC2064
    trap "rm -f '$script'" EXIT

    if ! download "$url" "$script"; then
      log "Failed to download the nvm install script from $url"
      return 1
    fi
    # PROFILE=/dev/null keeps nvm from editing your shell rc files.
    if ! PROFILE=/dev/null bash "$script"; then
      log "The nvm install script failed."
      return 1
    fi
    if ! load_nvm; then
      log "nvm installed but $NVM_DIR/nvm.sh is missing."
      return 1
    fi
  fi

  log "Installing Node.js LTS with nvm..."
  if ! nvm install --lts; then
    log "nvm install --lts failed."
    return 1
  fi
  nvm use --lts >/dev/null

  major="$(node_major)"
  if [ -z "$major" ] || [ "$major" -lt "$MIN_NODE_MAJOR" ]; then
    log "The current LTS (v${major:-unknown}) is older than v${MIN_NODE_MAJOR}; installing the newest release..."
    if ! nvm install node; then
      log "nvm install node failed."
      return 1
    fi
    nvm use node >/dev/null
  fi

  hash -r
  log "Using Node.js v$(node_major) from nvm ($NVM_DIR)."
  return 0
}

# Make sure node and npm exist and are new enough.
ensure_node() {
  local major

  # An explicit --nvm request skips the system package routes entirely.
  if [ "$USE_NVM" -eq 1 ]; then
    if ! install_node_nvm; then
      err "could not install Node.js with nvm (see the messages above)."
      exit 1
    fi
    if [ "$DRY_RUN" -eq 1 ]; then plan_exit; fi
  fi

  if ! have node || ! have npm; then
    log "Node.js/npm was not found."
    if ! install_node_distro; then
      if can_elevate; then
        log "System packages did not provide Node.js — falling back to nvm."
      else
        log "No root or sudo available — installing Node.js with nvm instead."
      fi
      if ! install_node_nvm; then
        err "could not install Node.js (system packages and nvm both failed)."
        err "Install Node.js (>= ${MIN_NODE_MAJOR}) manually: https://nodejs.org/en/download"
        exit 1
      fi
    fi
    if [ "$DRY_RUN" -eq 1 ]; then plan_exit; fi
    if ! have node || ! have npm; then
      err "Node.js/npm installation did not succeed. Install it manually: https://nodejs.org/en/download"
      exit 1
    fi
  fi

  major="$(node_major)"
  if [ -z "$major" ] || [ "$major" -lt "$MIN_NODE_MAJOR" ]; then
    log "Node.js v${major:-unknown} is older than the required v${MIN_NODE_MAJOR}."
    if ! install_node_nodesource; then
      log "NodeSource is not usable here — falling back to nvm."
      if ! install_node_nvm; then
        err "Node.js v${MIN_NODE_MAJOR}+ could not be installed via NodeSource or nvm."
        exit 1
      fi
    fi
    if [ "$DRY_RUN" -eq 1 ]; then plan_exit; fi

    major="$(node_major)"
    if [ -z "$major" ] || [ "$major" -lt "$MIN_NODE_MAJOR" ]; then
      err "Node.js is still v${major:-unknown} (v${MIN_NODE_MAJOR}+ required)."
      exit 1
    fi
    log "Now using Node.js v${major}."
  fi
}

# Installed version of the global package, or empty string if absent.
installed_version() {
  npm list -g --depth=0 "$PACKAGE" 2>/dev/null \
    | sed -n "s/.*${PACKAGE}@\([^ ]*\).*/\1/p" \
    | head -n 1
}

# Latest version published to the npm registry (empty on network failure).
latest_version() {
  npm view "$PACKAGE" version 2>/dev/null || true
}

# ---------------------------------------------------------------- changes ---
show_changes() {
  cat <<EOF
freebuff-cli.sh $SCRIPT_VERSION — what it does and how it changed

Flags:
  -r, --remove    remove the installed copy at $INSTALL_PATH
  -u, --upgrade   update freebuff to the latest published version
  -n, --dry-run   print every install and launch command without running it
      --nvm       install Node.js under \$HOME/.nvm with no root
  -c, --check     read-only report: distro, Node, npm, npx, freebuff versions
  -d, --doctor    full diagnosis: every prerequisite, with a fix beside each problem
      --install   copy this script to $INSTALL_PATH (never creates the directory)
      --force     run on distros this script was not written for
      --changes   print this list
  -v, --version   print this script's version
  -h, --help      usage

Change history (newest first):
  * v0.7 — --doctor: a deeper read-only diagnosis than --check, with the fix
    printed beside each problem (distro, downloader, Node, npm, npx, TTY,
    install path, registry, binary cache)
  * v0.6 — a plain run now copies the script to the install path when that is
    possible, and says how to do it when it is not; removed an unused variable
    reported by shellcheck
  * v0.5 — Gentoo support (emerge); NodeSource is skipped there, nvm is the
    fallback for an outdated dev-lang/nodejs
  * v0.4 — uninstall moved to -r/--remove; -u is short for --upgrade again
  * v0.3 — -u/--uninstall; internal refactor: one distro-family helper, one
    download helper, one sudo wrapper, one copy helper
  * v0.2 — accept lowercase -v as an alias for --version
  * v0.1 — first numbered version, adding the --version flag; everything below
  * --force and --changes, plus the distro gate that refuses systems that are
    not Debian/Ubuntu, Arch, Fedora or Gentoo based
  * --install, which refuses to create a missing target directory
  * --check, a read-only status report that exits 1 when something is missing
  * --nvm fallback for machines without root or with an outdated distro Node
  * NodeSource fallback when apt/dnf ship a Node older than the minimum
  * distro installs for apt, pacman, dnf and emerge
  * freebuff version reporting and the --upgrade self-update
  * --dry-run, so every action can be previewed first
  * first version: install freebuff globally if missing, then launch it via npx
EOF
}

if [ "$CHANGES" -eq 1 ]; then
  show_changes
  exit 0
fi

# ------------------------------------------------------------------ doctor ---
# Read-only diagnosis, one step deeper than --check: every prerequisite the
# launch depends on, with the fix printed next to the problem. Runs before the
# distro gate, because an unsupported distro is exactly what doctor should
# report rather than refuse.
DOC_FAIL=0
DOC_WARN=0

doc_ok()   { log "  [ok]   $*"; }
doc_warn() { log "  [warn] $*"; DOC_WARN=$((DOC_WARN + 1)); }
doc_fail() { log "  [FAIL] $*"; DOC_FAIL=$((DOC_FAIL + 1)); }
doc_fix()  { log "         fix: $*"; }

doctor() {
  local pm="" major="" current="" latest="" dir="" cache=""

  log "$(basename "$0") $SCRIPT_VERSION — doctor"
  log ""

  log "System"
  log "  $(distro_label)"
  if pm="$(os_family strict)"; then
    doc_ok "supported distro — package manager: $pm"
  elif pm="$(os_family || true)" && [ -n "$pm" ]; then
    doc_warn "unsupported distro — $pm is installed, but only Debian/Ubuntu, Arch, Fedora and Gentoo are supported"
    doc_fix "use --force to try anyway, or --nvm to skip system packages"
  else
    doc_warn "unsupported distro — none of apt, pacman, dnf or emerge found"
    doc_fix "install Node.js manually, or re-run with --nvm (needs curl or wget)"
  fi

  log "Downloader"
  if have curl; then
    doc_ok "curl present (used by the NodeSource and nvm fallbacks)"
  elif have wget; then
    doc_ok "wget present (used by the NodeSource and nvm fallbacks)"
  else
    doc_warn "neither curl nor wget is installed"
    doc_fix "install one of them, or the fallbacks cannot download anything"
  fi

  log "Node.js"
  if have node; then
    major="$(node_major)"
    if [ -n "$major" ] && [ "$major" -ge "$MIN_NODE_MAJOR" ]; then
      doc_ok "$(node --version) at $(command -v node) — meets the v$MIN_NODE_MAJOR minimum"
    else
      doc_fail "$(node --version 2>/dev/null || echo node) is older than the required v$MIN_NODE_MAJOR"
      doc_fix "run without --doctor and let NodeSource/nvm upgrade it, or use --nvm"
    fi
  else
    doc_fail "node not found"
    doc_fix "run without --doctor to install it, or install Node.js v$MIN_NODE_MAJOR+ yourself"
  fi

  if have npm; then
    doc_ok "npm $(npm --version 2>/dev/null)"
  else
    doc_fail "npm not found"
    doc_fix "it ships with Node.js — reinstall Node.js v$MIN_NODE_MAJOR+"
  fi

  if have npx; then
    doc_ok "npx present"
  else
    doc_fail "npx not found — $PACKAGE is launched through npx"
    doc_fix "it ships with npm — reinstall Node.js v$MIN_NODE_MAJOR+"
  fi

  log "Terminal"
  if [ -t 0 ] && [ -t 1 ]; then
    doc_ok "attached to a terminal"
  else
    doc_warn "stdin or stdout is not a terminal"
    doc_fix "freebuff is an interactive agent — run it from a normal shell, not from a pipe"
  fi
  if have stty; then
    doc_ok "stty present"
  else
    doc_warn "stty not found"
    doc_fix "install util-linux/coreutils; some terminals need it for sane line handling"
  fi
  if [ -n "${TERM:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
    doc_ok "TERM=$TERM"
  else
    doc_warn "TERM is unset or 'dumb'"
    doc_fix "export TERM=xterm-256color"
  fi

  log "Self-install"
  dir="$(dirname "$INSTALL_PATH")"
  if [ ! -d "$dir" ]; then
    doc_warn "$dir does not exist — --install refuses to create it"
    doc_fix "create it, or point FREEBUFF_INSTALL_PATH at an existing directory"
  elif [ -w "$dir" ] || is_root || can_elevate; then
    doc_ok "$dir is writable (directly or through sudo)"
  else
    doc_warn "cannot write $dir and sudo is not available"
    doc_fix "set FREEBUFF_INSTALL_PATH=\$HOME/.local/bin/freebuff-cli.sh"
  fi
  case ":${PATH}:" in
    *":${dir}:"*) doc_ok "$dir is on PATH" ;;
    *) doc_warn "$dir is not on PATH"
       doc_fix "add it to PATH, or call the script by its full path" ;;
  esac

  log "freebuff"
  if have npm; then
    current="$(installed_version || true)"
    if [ -n "$current" ]; then
      doc_ok "installed globally: v$current"
    else
      doc_ok "no global install — npx fetches it into the npm cache on launch"
    fi

    latest="$(latest_version)"
    if [ -z "$latest" ]; then
      doc_warn "the npm registry could not be reached (offline or blocked)"
      doc_fix "launches still work from the npx cache; check network or proxy settings"
    elif [ -n "$current" ] && [ "$current" != "$latest" ]; then
      doc_warn "v$current is outdated — latest is v$latest"
      doc_fix "re-run with --upgrade"
    else
      doc_ok "npm registry reachable — latest is v$latest"
    fi
  fi

  cache="${XDG_CONFIG_HOME:-$HOME/.config}/manicode"
  if [ -d "$cache" ]; then
    doc_ok "native binary cache: $cache"
  else
    doc_ok "native binary cache not created yet — the first launch downloads it"
  fi

  log ""
  if [ "$DOC_FAIL" -eq 0 ] && [ "$DOC_WARN" -eq 0 ]; then
    log "All checks passed — freebuff should launch."
    return 0
  fi
  log "$DOC_FAIL failure(s), $DOC_WARN warning(s)."
  if [ "$DOC_FAIL" -gt 0 ]; then
    log "Not ready — run without --doctor to install what is missing."
    return 1
  fi
  log "Nothing blocking — freebuff should launch."
  return 0
}

if [ "$DOCTOR" -eq 1 ]; then
  if doctor; then
    exit 0
  fi
  exit 1
fi

# ------------------------------------------------------------------ distro ---
# Refuse to run anywhere this script cannot install Node.js reliably.
if ! os_family strict >/dev/null; then
  if [ "$FORCE" -eq 1 ]; then
    log "Warning: unsupported distro ($(distro_label)) — continuing because of --force."
    log "Installs may fail here; --force is meant for testing, not daily use."
  else
    err "Unsupported distro: $(distro_label)"
    err "This script supports Debian/Ubuntu (apt), Arch (pacman), Fedora/RHEL (dnf)"
    err "and Gentoo (emerge) based systems only."
    err "Use --force to attempt it anyway (not recommended)."
    exit 1
  fi
fi

# -------------------------------------------------------------- status check ---
# Read-only: reports what is present and what is missing, changes nothing.
check_status() {
  local problems=0 major="" current="" latest="" pm=""

  log "System: $(distro_label)"
  if pm="$(os_family)"; then
    log "Package manager: $pm"
  else
    log "Package manager: none of apt/pacman/dnf/emerge detected (nvm route only)"
  fi

  if have node; then
    major="$(node_major)"
    if [ -n "$major" ] && [ "$major" -ge "$MIN_NODE_MAJOR" ]; then
      log "Node.js: $(node --version) — ok (needs >= v$MIN_NODE_MAJOR)"
    else
      log "Node.js: $(node --version) — too old (needs >= v$MIN_NODE_MAJOR)"
      problems=1
    fi
  else
    log "Node.js: not found (needs >= v$MIN_NODE_MAJOR)"
    problems=1
  fi

  if have npm; then
    log "npm: $(npm --version)"
  else
    log "npm: not found"
    problems=1
  fi

  if have npx; then
    log "npx: present"
  else
    log "npx: not found (needed to launch $PACKAGE)"
    problems=1
  fi

  log "Script: $SCRIPT_VERSION at $INSTALL_PATH"

  if have npm; then
    current="$(installed_version || true)"
    if [ -n "$current" ]; then
      log "$PACKAGE: v$current installed globally"
    else
      log "$PACKAGE: not installed globally (would be fetched by npx on launch)"
    fi

    latest="$(latest_version)"
    if [ -z "$latest" ]; then
      log "$PACKAGE latest: unknown (offline or registry unavailable)"
    elif [ -z "$current" ]; then
      log "$PACKAGE latest: v$latest"
    elif [ "$current" = "$latest" ]; then
      log "$PACKAGE latest: v$latest — up to date"
    else
      log "$PACKAGE latest: v$latest — outdated, re-run with --upgrade to update"
    fi
  fi

  if [ "$problems" -eq 0 ]; then
    log "Status: ready to run."
    return 0
  fi
  log "Status: not ready — run without --check to install what is missing."
  return 1
}

if [ "$CHECK" -eq 1 ]; then
  if check_status; then
    exit 0
  fi
  exit 1
fi

# ------------------------------------------------- install / uninstall self ---
# copy_into <src> <dest> — honours SUDO, falls back to cp when install is absent.
copy_into() {
  if have install; then
    as_root install -m 0755 "$1" "$2"
  else
    as_root cp "$1" "$2" && as_root chmod 0755 "$2"
  fi
}

# Pick the elevation needed for an operation on $INSTALL_PATH's directory.
# Returns 1 when privileges are needed but unavailable.
elevation_for() {
  local dir="$1"
  if [ -w "$dir" ] || is_root; then
    SUDO=()
    return 0
  fi
  if ! can_elevate; then
    return 1
  fi
  SUDO=(sudo)
  return 0
}

install_self() {
  local src dir
  src="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
  dir="$(dirname "$INSTALL_PATH")"

  if [ "$src" = "$INSTALL_PATH" ]; then
    log "Already installed at $INSTALL_PATH."
    return 0
  fi

  if [ ! -d "$dir" ]; then
    err "$dir does not exist — refusing to create it."
    err "Point FREEBUFF_INSTALL_PATH at an existing directory instead."
    return 1
  fi

  if ! elevation_for "$dir"; then
    err "need root to write $dir, and sudo is not available."
    err "Re-run as root, or set FREEBUFF_INSTALL_PATH to a writable directory."
    return 1
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    if [ "${#SUDO[@]}" -gt 0 ]; then
      log "[dry-run] ${SUDO[*]} install -m 0755 $src $INSTALL_PATH"
    else
      log "[dry-run] install -m 0755 $src $INSTALL_PATH"
    fi
    return 0
  fi

  if ! copy_into "$src" "$INSTALL_PATH"; then
    err "could not copy to $INSTALL_PATH"
    return 1
  fi
  if [ ! -x "$INSTALL_PATH" ]; then
    err "installation failed — $INSTALL_PATH is missing or not executable."
    return 1
  fi
  log "Installed to $INSTALL_PATH."

  case ":${PATH}:" in
    *":${dir}:"*) log "Run it with: $(basename "$INSTALL_PATH")" ;;
    *) log "Note: $dir is not in your PATH — add it, or run $INSTALL_PATH directly." ;;
  esac
  return 0
}

uninstall_self() {
  local dir
  dir="$(dirname "$INSTALL_PATH")"

  if [ ! -e "$INSTALL_PATH" ]; then
    log "No installed copy at $INSTALL_PATH — nothing to remove."
    return 0
  fi

  if ! elevation_for "$dir"; then
    err "need root to remove $INSTALL_PATH, and sudo is not available."
    return 1
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    if [ "${#SUDO[@]}" -gt 0 ]; then
      log "[dry-run] ${SUDO[*]} rm -f $INSTALL_PATH"
    else
      log "[dry-run] rm -f $INSTALL_PATH"
    fi
    return 0
  fi

  if ! as_root rm -f "$INSTALL_PATH"; then
    err "could not remove $INSTALL_PATH"
    return 1
  fi
  log "Removed $INSTALL_PATH. The directory itself was left in place."
  return 0
}

# Best-effort install on a normal run: put this script where --install would,
# so a single plain run leaves a copy on PATH. Never fatal and never noisy — the
# launch still happens when the copy is not possible (no sudo, piped via curl,
# or INSTALL_DIR not writable).
maybe_install_self() {
  local src
  src="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"

  # Nothing to install when we are already the installed copy, or when $0 is
  # not a real file (sh freebuff-cli.sh / curl ... | bash).
  if [ "$src" = "$INSTALL_PATH" ] || [ ! -f "$src" ]; then
    return 0
  fi

  if install_self >/dev/null 2>&1; then
    log "Installed this script to $INSTALL_PATH."
    return 0
  fi

  log "Not installed to $INSTALL_PATH — run '$(basename "$0") --install' to put it on your PATH."
  return 0
}

if [ "$INSTALL" -eq 1 ]; then
  if install_self; then
    exit 0
  fi
  exit 1
fi

if [ "$UNINSTALL" -eq 1 ]; then
  if uninstall_self; then
    exit 0
  fi
  exit 1
fi

# ------------------------------------------------------------------ Node.js ---
ensure_node
log "Node.js $(node --version) / npm $(npm --version)"

# ------------------------------------------------------------------ freebuff ---
CURRENT="$(installed_version || true)"

if [ "$UPGRADE" -eq 1 ]; then
  if [ -n "$CURRENT" ]; then
    log "Updating $PACKAGE (currently v$CURRENT) to the latest version..."
  else
    log "$PACKAGE is not installed. Installing the latest version..."
  fi
  npm_install_global "$PACKAGE@latest" || true
elif [ -n "$CURRENT" ]; then
  log "$PACKAGE is already installed."
else
  log "$PACKAGE is not installed. Installing via npm..."
  npm_install_global "$PACKAGE" || true
fi

if [ "$DRY_RUN" -eq 1 ]; then
  plan_exit
fi

CURRENT="$(installed_version || true)"
LATEST="$(latest_version)"

if [ -n "$CURRENT" ]; then
  log "Installed $PACKAGE: v$CURRENT"
else
  log "No readable global $PACKAGE install (npm's global prefix may be read-only on this system)."
fi

if [ -z "$LATEST" ]; then
  log "Could not check the latest published version (offline or registry unavailable)."
elif [ -z "$CURRENT" ]; then
  log "Latest published $PACKAGE: v$LATEST"
elif [ "$CURRENT" = "$LATEST" ]; then
  log "Latest published $PACKAGE: v$LATEST (up to date)"
else
  log "Latest published $PACKAGE: v$LATEST (outdated — re-run with --upgrade to update)"
fi

# --------------------------------------------------------------- self-install ---
maybe_install_self

# ------------------------------------------------------------------- launch ---
log "Starting $PACKAGE..."
exec npx --yes "$PACKAGE" ${FORWARD[@]+"${FORWARD[@]}"}
