#!/usr/bin/env bash
# ==============================================================================
#  🚀 Freebuff CLI Installer
#  Installs the standalone, zero-dependency native Go binary for Freebuff CLI
#  Supports NixOS, Debian, Ubuntu, Fedora, Arch Linux, Gentoo, and generic Linux.
# ==============================================================================
set -euo pipefail

SCRIPT_VERSION="v1.0.0"
INSTALL_SCRIPT_NAME="$(basename "$0")"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GO_BUILD_DIR="${SCRIPT_DIR}/go-build"

# ------------------------------------------------------------------------------
# Colors and formatting
# ------------------------------------------------------------------------------
if [ -t 1 ]; then
  BOLD="$(printf '\033[1m')"
  DIM="$(printf '\033[2m')"
  GREEN="$(printf '\033[32m')"
  CYAN="$(printf '\033[36m')"
  YELLOW="$(printf '\033[33m')"
  RED="$(printf '\033[31m')"
  MAGENTA="$(printf '\033[35m')"
  BLUE="$(printf '\033[34m')"
  RESET="$(printf '\033[0m')"
else
  BOLD=""
  DIM=""
  GREEN=""
  CYAN=""
  YELLOW=""
  RED=""
  MAGENTA=""
  BLUE=""
  RESET=""
fi

info()    { printf "${CYAN}ℹ %s${RESET}\n" "$*"; }
success() { printf "${GREEN}✔ %s${RESET}\n" "$*"; }
warn()    { printf "${YELLOW}⚠ %s${RESET}\n" "$*"; }
error()   { printf "${RED}✖ %s${RESET}\n" "$*" >&2; }
header()  { printf "\n${BOLD}${MAGENTA}==> %s${RESET}\n" "$*"; }

# ------------------------------------------------------------------------------
# Defaults & Options
# ------------------------------------------------------------------------------
MODE="install"     # install | check | upgrade | remove
DRY_RUN=0
AUTO_YES=0

usage() {
  cat <<EOF
${BOLD}🚀 Freebuff CLI Installer (${SCRIPT_VERSION})${RESET}
Installs the standalone, zero-dependency native Go binary for Freebuff CLI.
Works across NixOS, Debian, Ubuntu, Fedora, Arch Linux, Gentoo, and other distros.

${BOLD}USAGE:${RESET}
  ./${INSTALL_SCRIPT_NAME} [OPTIONS]

${BOLD}OPTIONS:${RESET}
  -c, --check       Check installed versions, detect binary vs. shell version, verify deps
  -u, --upgrade     Update/rebuild the standalone binary and upstream Freebuff agent
  -r, --remove      Uninstall freebuff-cli binary and the Kitty desktop launcher
  -d, -n, --dry-run Show what would be done without making system modifications
  -y, --yes         Non-interactive mode (assume yes to prompts)
  -v, --version     Show installer version
  -h, --help        Show this help message

${BOLD}ENVIRONMENT OVERRIDES:${RESET}
  FREEBUFF_INSTALL_PATH   Target destination for the binary (default: /usr/local/bin/freebuff-cli)
                          (On NixOS with absent /usr/local/bin, defaults to ~/.local/bin/freebuff-cli)
EOF
}

# ------------------------------------------------------------------------------
# Parse CLI arguments
# ------------------------------------------------------------------------------
while [ "$#" -gt 0 ]; do
  case "$1" in
    -c|--check)
      MODE="check"
      shift
      ;;
    -u|--upgrade|--update)
      MODE="upgrade"
      shift
      ;;
    -r|--remove|--uninstall)
      MODE="remove"
      shift
      ;;
    -d|-n|--dry-run)
      DRY_RUN=1
      shift
      ;;
    -y|--yes)
      AUTO_YES=1
      shift
      ;;
    -v|--version)
      printf "%s %s\n" "${INSTALL_SCRIPT_NAME}" "${SCRIPT_VERSION}"
      exit 0
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      error "Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

# ------------------------------------------------------------------------------
# Helpers: Distro & Command Detection
# ------------------------------------------------------------------------------
have() { command -v "$1" >/dev/null 2>&1; }

detect_distro() {
  local id="" id_like="" pretty="Unknown Linux"
  if [ -r /etc/os-release ]; then
    # shellcheck disable=SC1091
    . /etc/os-release
    id="${ID:-}"
    id_like="${ID_LIKE:-}"
    pretty="${PRETTY_NAME:-$id}"
  fi

  local keys=" ${id} ${id_like} "
  case "$keys" in
    *nixos*|*NixOS*)
      printf "%s|%s\n" "nixos" "$pretty"
      ;;
    *debian*|*ubuntu*|*mint*|*pop*)
      printf "%s|%s\n" "apt" "$pretty"
      ;;
    *arch*|*manjaro*|*endeavouros*)
      printf "%s|%s\n" "pacman" "$pretty"
      ;;
    *fedora*|*rhel*|*centos*)
      printf "%s|%s\n" "dnf" "$pretty"
      ;;
    *gentoo*)
      printf "%s|%s\n" "emerge" "$pretty"
      ;;
    *)
      if [ -e /etc/NIXOS ]; then
        printf "%s|%s\n" "nixos" "$pretty"
      elif have apt-get; then
        printf "%s|%s\n" "apt" "$pretty"
      elif have pacman; then
        printf "%s|%s\n" "pacman" "$pretty"
      elif have dnf; then
        printf "%s|%s\n" "dnf" "$pretty"
      elif have emerge; then
        printf "%s|%s\n" "emerge" "$pretty"
      else
        printf "%s|%s\n" "generic" "$pretty"
      fi
      ;;
  esac
}

DISTRO_INFO="$(detect_distro)"
DISTRO_FAMILY="${DISTRO_INFO%%|*}"
DISTRO_NAME="${DISTRO_INFO#*|}"

# Determine target binary path
resolve_target_path() {
  if [ -n "${FREEBUFF_INSTALL_PATH:-}" ]; then
    printf '%s' "$FREEBUFF_INSTALL_PATH"
    return 0
  fi

  # Prefer ~/.local/bin on NixOS or whenever ~/.local/bin is in PATH and /usr/local/bin is not
  local in_path_local=0
  local in_path_usr=0
  case ":${PATH}:" in
    *":${HOME}/.local/bin:"*|*":${HOME}/.local/bin/:"*) in_path_local=1 ;;
  esac
  case ":${PATH}:" in
    *":/usr/local/bin:"*|*":/usr/local/bin/:"*) in_path_usr=1 ;;
  esac

  if [ "$DISTRO_FAMILY" = "nixos" ] || { [ "$in_path_local" -eq 1 ] && [ "$in_path_usr" -eq 0 ]; }; then
    printf '%s' "${HOME}/.local/bin/freebuff-cli"
  else
    printf '%s' "/usr/local/bin/freebuff-cli"
  fi
}

TARGET_BIN="$(resolve_target_path)"
TARGET_DIR="$(dirname "$TARGET_BIN")"
DESKTOP_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/applications"
DESKTOP_FILE="${DESKTOP_DIR}/freebuff-cli.desktop"

is_dir_writable() {
  local dir="$1"
  [ -w "$dir" ] || [ "${EUID:-$(id -u)}" -eq 0 ]
}

# Elevation helper: checks sudo first, doas as fallback
get_elevation_cmd() {
  if [ "${EUID:-$(id -u)}" -eq 0 ]; then
    return 0
  fi
  if have sudo; then
    printf "sudo\n"
    return 0
  elif have doas; then
    printf "doas\n"
    return 0
  else
    return 1
  fi
}

elevated_rm() {
  local target="$1"
  if [ ! -e "$target" ] && [ ! -L "$target" ]; then
    return 0
  fi

  local dir
  dir="$(dirname "$target")"

  if [ "${EUID:-$(id -u)}" -eq 0 ] || { [ -w "$dir" ] && { [ ! -e "$target" ] || [ -w "$target" ]; }; }; then
    rm -f "$target"
    return $?
  fi

  local elev
  if elev="$(get_elevation_cmd)"; then
    "$elev" rm -f "$target"
    return $?
  else
    error "Cannot remove $target: write permission denied and neither 'sudo' nor 'doas' was found."
    return 1
  fi
}

# Check if 'install' is an alias or shell function and flag the issue
check_install_alias() {
  local is_alias=0
  local is_func=0
  if alias install >/dev/null 2>&1; then
    is_alias=1
  fi
  if [ "$(type -t install 2>/dev/null)" = "function" ]; then
    is_func=1
  fi
  if [ "$is_alias" -eq 1 ] || [ "$is_func" -eq 1 ]; then
    warn "'install' is shadowed by a shell alias or function in your environment!"
    info "Flagged issue: bypassing alias/function and invoking coreutils 'install' directly."
  fi
}

resolve_install_cmd() {
  check_install_alias
  if have install; then
    command -v install
  elif [ -x /usr/bin/install ]; then
    printf '/usr/bin/install\n'
  elif [ -x /bin/install ]; then
    printf '/bin/install\n'
  else
    printf 'install\n'
  fi
}

# ------------------------------------------------------------------------------
# Check for legacy shell / npm installations
# ------------------------------------------------------------------------------
detect_legacy_shell() {
  local found=()

  # 1. Shell script in common locations
  local candidates=(
    "/usr/local/bin/freebuff-cli.sh"
    "${HOME}/.local/bin/freebuff-cli.sh"
    "/usr/bin/freebuff-cli.sh"
  )
  if have freebuff-cli.sh; then
    candidates+=("$(command -v freebuff-cli.sh)")
  fi

  for p in "${candidates[@]}"; do
    if [ -f "$p" ]; then
      found+=("script:$p")
    fi
  done

  # 2. Check if a legacy bash script is installed as `freebuff-cli` (not a binary)
  if have freebuff-cli; then
    local bin_path
    bin_path="$(command -v freebuff-cli)"
    if [ -f "$bin_path" ] && [ ! -d "$bin_path" ]; then
      # Check if it's a shell script (starts with #!)
      if head -n 1 "$bin_path" 2>/dev/null | grep -q '^#!'; then
        # Check if it calls npx or freebuff-cli.sh
        if grep -q -E '(npx.*freebuff|freebuff-cli\.sh)' "$bin_path" 2>/dev/null; then
          found+=("legacy_wrapper:$bin_path")
        fi
      fi
    fi
  fi

  # 3. Check for global npm package
  if have npm; then
    if npm list -g --depth=0 freebuff 2>/dev/null | grep -q 'freebuff@'; then
      found+=("npm_global:freebuff")
    fi
  fi

  printf '%s\n' "${found[@]+"${found[@]}"}"
}

prompt_uninstall_legacy_shell() {
  local items=()
  while IFS= read -r line; do
    [ -n "$line" ] && items+=("$line")
  done < <(detect_legacy_shell)

  if [ "${#items[@]}" -eq 0 ]; then
    return 0
  fi

  warn "Detected legacy NPM/shell-based Freebuff installation:"
  for it in "${items[@]}"; do
    case "$it" in
      script:*)
        printf "    ${YELLOW}• Old shell script:${RESET} %s\n" "${it#script:}"
        ;;
      legacy_wrapper:*)
        printf "    ${YELLOW}• Old npx wrapper script:${RESET} %s\n" "${it#legacy_wrapper:}"
        ;;
      npm_global:*)
        printf "    ${YELLOW}• Global npm package:${RESET} freebuff\n"
        ;;
    esac
  done

  local do_clean=0
  if [ "$AUTO_YES" -eq 1 ]; then
    do_clean=1
  elif [ -t 0 ]; then
    printf "\n${BOLD}Would you like to remove the legacy NPM/shell installation? [Y/n]: ${RESET}"
    read -r resp
    resp="${resp:-y}"
    if [[ "$resp" =~ ^[Yy]$ ]]; then
      do_clean=1
    fi
  fi

  if [ "$do_clean" -eq 1 ]; then
    header "Removing legacy NPM/shell installation..."
    for it in "${items[@]}"; do
      case "$it" in
        script:*)
          local sfile="${it#script:}"
          if [ "$DRY_RUN" -eq 1 ]; then
            info "[dry-run] rm -f $sfile (elevated with sudo/doas if needed)"
          else
            if elevated_rm "$sfile"; then
              success "Removed old script: $sfile"
            else
              warn "Could not remove $sfile (check permissions)"
            fi
          fi
          ;;
        legacy_wrapper:*)
          local wfile="${it#legacy_wrapper:}"
          # If it's in the nix store, mention nix rebuild
          if [[ "$wfile" == /nix/store/* ]] || [[ "$wfile" == /etc/profiles/* ]]; then
            info "Legacy Nix wrapper $wfile is managed by Home Manager / Nix profile."
            info "Update your configuration to import go-build/nixos/freebuff-cli.nix to supersede it."
          else
            if [ "$DRY_RUN" -eq 1 ]; then
              info "[dry-run] rm -f $wfile (elevated with sudo/doas if needed)"
            else
              if elevated_rm "$wfile"; then
                success "Removed old wrapper: $wfile"
              fi
            fi
          fi
          ;;
        npm_global:*)
          if [ "$DRY_RUN" -eq 1 ]; then
            info "[dry-run] npm uninstall -g freebuff"
          else
            if npm uninstall -g freebuff 2>/dev/null; then
              success "Uninstalled global npm package 'freebuff'"
            fi
          fi
          ;;
      esac
    done

    # Clean npx cache
    local npx_dir="${HOME}/.npm/_npx"
    if [ -d "$npx_dir" ]; then
      for cdir in "$npx_dir"/*/node_modules/freebuff; do
        if [ -d "$cdir" ]; then
          if [ "$DRY_RUN" -eq 1 ]; then
            info "[dry-run] rm -rf $cdir"
          else
            rm -rf "$cdir" "$(dirname "$cdir")/.bin/freebuff" 2>/dev/null || true
            success "Cleaned stale npx cache in $cdir"
          fi
        fi
      done
    fi
  else
    info "Skipped removing legacy shell installation."
  fi
}

# ------------------------------------------------------------------------------
# Check Command (-c / --check)
# ------------------------------------------------------------------------------
do_check() {
  header "System & Freebuff Status Check"
  printf "${BOLD}Operating System:${RESET}  %s (%s)\n" "$DISTRO_NAME" "$DISTRO_FAMILY"
  printf "${BOLD}Target Install Path:${RESET} %s\n" "$TARGET_BIN"

  # 1. Check if binary is installed
  if [ -x "$TARGET_BIN" ]; then
    if file "$TARGET_BIN" 2>/dev/null | grep -q 'ELF'; then
      local v_out
      v_out="$("$TARGET_BIN" --version 2>/dev/null || echo "unknown")"
      success "Installed binary at $TARGET_BIN: $v_out"
    else
      warn "File at $TARGET_BIN is not an ELF binary (possibly a script)"
    fi
  else
    info "No stationary Go binary installed at $TARGET_BIN"
  fi

  # 2. Check for legacy shell / npm version
  local legacy_items=()
  while IFS= read -r line; do
    [ -n "$line" ] && legacy_items+=("$line")
  done < <(detect_legacy_shell)

  if [ "${#legacy_items[@]}" -gt 0 ]; then
    warn "Legacy shell/NPM version detected (${#legacy_items[@]} items) — run './install.sh' to replace it"
  else
    success "No legacy shell/NPM freebuff installation found"
  fi

  # 3. Dynamic linker / Nix-LD check
  if [ "$DISTRO_FAMILY" = "nixos" ]; then
    if [ -x "/run/current-system/sw/share/nix-ld/lib/ld.so" ]; then
      success "Nix-LD dynamic loader is enabled and active"
    else
      warn "Nix-LD dynamic loader not found (required for the downloaded Freebuff ELF binary)"
      printf "    ${DIM}Add 'programs.nix-ld.enable = true;' to your NixOS configuration and rebuild.${RESET}\n"
    fi
  else
    if [ -e "/lib64/ld-linux-x86-64.so.2" ] || [ -e "/lib/ld-linux-aarch64.so.1" ]; then
      success "Standard glibc dynamic linker present"
    else
      warn "Standard glibc 64-bit dynamic linker not found"
    fi
  fi

  # 4. Kitty terminal
  if have kitty; then
    success "Kitty terminal present at $(command -v kitty)"
  else
    warn "Kitty terminal not found on PATH (required for desktop launcher)"
    printf "    ${DIM}Install kitty via your distro package manager.${RESET}\n"
  fi

  # 5. Desktop launcher entry
  if [ -f "$DESKTOP_FILE" ]; then
    success "Kitty desktop icon entry installed: $DESKTOP_FILE"
  else
    info "Kitty desktop icon entry not installed ($DESKTOP_FILE)"
  fi

  # 6. Upstream registry reachability & cached binary
  local cache_bin="${HOME}/.config/manicode/freebuff"
  local meta_file="${HOME}/.config/manicode/freebuff-metadata.json"
  if [ -x "$cache_bin" ] && [ -f "$meta_file" ]; then
    local installed_v
    installed_v="$(sed -n 's/.*"version": *"\([^"]*\)".*/\1/p' "$meta_file" | head -n 1)"
    success "Downloaded agent in ~/.config/manicode/freebuff (v${installed_v:-unknown})"
  else
    info "No agent cached yet in ~/.config/manicode/freebuff (downloaded on first run)"
  fi
}

# ------------------------------------------------------------------------------
# Build the stationary Go binary
# ------------------------------------------------------------------------------
build_binary_if_needed() {
  local bin="${GO_BUILD_DIR}/freebuff-cli"
  if [ -f "$bin" ] && [ -x "$bin" ]; then
    return 0
  fi

  header "Building standalone freebuff-cli binary..."
  if ! have go; then
    error "Go compiler is not installed! (Go >= 1.22 required to build)"
    exit 1
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    info "[dry-run] (cd $GO_BUILD_DIR && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o freebuff-cli ./cmd/freebuff-cli)"
    return 0
  fi

  (
    cd "$GO_BUILD_DIR"
    export CGO_ENABLED=0
    go build -trimpath -ldflags="-s -w" -o freebuff-cli ./cmd/freebuff-cli
  )
  success "Built stationary Go binary: ${GO_BUILD_DIR}/freebuff-cli"
}

# ------------------------------------------------------------------------------
# Install Desktop Entry
# ------------------------------------------------------------------------------
install_desktop_entry() {
  local icon_path="${HOME}/AppImages/.icons/freebuff"
  if [ ! -f "$icon_path" ]; then
    icon_path="utilities-terminal"
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    info "[dry-run] write desktop launcher to $DESKTOP_FILE"
    return 0
  fi

  mkdir -p "$DESKTOP_DIR"
  cat > "$DESKTOP_FILE" <<EOF
[Desktop Entry]
Type=Application
Name=Freebuff CLI
GenericName=AI Coding Agent
Comment=Freebuff AI Coding Agent (CLI in Kitty)
Icon=${icon_path}
TryExec=kitty
Exec=kitty --title "Freebuff CLI" --class freebuff-cli -e ${TARGET_BIN}
Terminal=false
Categories=Development;Utility;ConsoleOnly;
Keywords=ai;coding;agent;freebuff;cli;terminal;
StartupWMClass=freebuff-cli
EOF
  chmod 0644 "$DESKTOP_FILE"

  if have update-desktop-database; then
    update-desktop-database "$DESKTOP_DIR" >/dev/null 2>&1 || true
  fi
  success "Installed Kitty desktop entry: $DESKTOP_FILE"
}

# ------------------------------------------------------------------------------
# Install Binary
# ------------------------------------------------------------------------------
do_install() {
  prompt_uninstall_legacy_shell

  header "Installing standalone Go freebuff-cli binary..."
  build_binary_if_needed

  local src_bin="${GO_BUILD_DIR}/freebuff-cli"
  if [ ! -f "$src_bin" ] && [ "$DRY_RUN" -eq 0 ]; then
    error "Binary not found at $src_bin!"
    exit 1
  fi

  local inst_cmd
  inst_cmd="$(resolve_install_cmd)"

  if [ ! -d "$TARGET_DIR" ]; then
    if is_dir_writable "$(dirname "$TARGET_DIR")"; then
      mkdir -p "$TARGET_DIR"
    else
      local elev
      if elev="$(get_elevation_cmd)"; then
        "$elev" mkdir -p "$TARGET_DIR"
      else
        error "Need root to create $TARGET_DIR, and neither sudo nor doas is available."
        exit 1
      fi
    fi
  fi

  if [ "$DRY_RUN" -eq 1 ]; then
    if is_dir_writable "$TARGET_DIR"; then
      info "[dry-run] $inst_cmd -m 0755 $src_bin $TARGET_BIN"
    else
      local elev="sudo"
      have doas && ! have sudo && elev="doas"
      info "[dry-run] $elev $inst_cmd -m 0755 $src_bin $TARGET_BIN"
    fi
  else
    if is_dir_writable "$TARGET_DIR"; then
      "$inst_cmd" -m 0755 "$src_bin" "$TARGET_BIN"
    else
      local elev
      if elev="$(get_elevation_cmd)"; then
        "$elev" "$inst_cmd" -m 0755 "$src_bin" "$TARGET_BIN"
      else
        error "Need root to write $TARGET_DIR, and neither sudo nor doas is available."
        exit 1
      fi
    fi
    success "Installed binary to $TARGET_BIN"
  fi

  install_desktop_entry

  # Check PATH
  case ":${PATH}:" in
    *":${TARGET_DIR}:"*|*":${TARGET_DIR}/:"*) ;;
    *)
      warn "$TARGET_DIR is not currently in your PATH!"
      printf "    ${DIM}Add 'export PATH=\"%s:\$PATH\"' to your ~/.bashrc or ~/.zshrc.${RESET}\n" "$TARGET_DIR"
      ;;
  esac

  header "Installation Complete! 🚀"
  printf "You can now run Freebuff directly from your terminal:\n"
  printf "  ${BOLD}freebuff-cli${RESET}\n"
  printf "Or launch it from your desktop application menu via Kitty!\n"
}

# ------------------------------------------------------------------------------
# Upgrade Binary
# ------------------------------------------------------------------------------
do_upgrade() {
  header "Upgrading freebuff-cli..."
  (
    cd "$GO_BUILD_DIR"
    export CGO_ENABLED=0
    go build -trimpath -ldflags="-s -w" -o freebuff-cli ./cmd/freebuff-cli
  )
  do_install

  if [ -x "$TARGET_BIN" ]; then
    info "Refreshing upstream Freebuff native agent binary..."
    "$TARGET_BIN" --upgrade
  fi
}

# ------------------------------------------------------------------------------
# Remove Binary
# ------------------------------------------------------------------------------
do_remove() {
  header "Uninstalling freebuff-cli..."

  if [ -e "$TARGET_BIN" ] || [ -L "$TARGET_BIN" ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
      info "[dry-run] rm -f $TARGET_BIN (elevated with sudo/doas if needed)"
    else
      if elevated_rm "$TARGET_BIN"; then
        success "Removed binary: $TARGET_BIN"
      else
        warn "Could not remove $TARGET_BIN"
      fi
    fi
  else
    info "No binary found at $TARGET_BIN"
  fi

  if [ -f "$DESKTOP_FILE" ]; then
    if [ "$DRY_RUN" -eq 1 ]; then
      info "[dry-run] rm -f $DESKTOP_FILE"
    else
      rm -f "$DESKTOP_FILE"
      if have update-desktop-database; then
        update-desktop-database "$DESKTOP_DIR" >/dev/null 2>&1 || true
      fi
      success "Removed desktop entry: $DESKTOP_FILE"
    fi
  fi

  success "Uninstallation complete."
}

# ------------------------------------------------------------------------------
# Main Dispatch
# ------------------------------------------------------------------------------
case "$MODE" in
  check)
    do_check
    ;;
  install)
    do_install
    ;;
  upgrade)
    do_upgrade
    ;;
  remove)
    do_remove
    ;;
esac
