package main

import (
	"fmt"
	"os"
	"path/filepath"

	"freebuff-cli/internal/launcher"
)

func printUsage(prog string, cfg *launcher.Config) {
	installPath := launcher.DefaultInstallPath
	if cfg != nil && cfg.InstallPath != "" {
		installPath = cfg.InstallPath
	}
	fmt.Printf(`Usage: %s %s [OPTIONS] [--] [ARGS...]

Standalone cross-distro launcher and updater for the Freebuff AI coding agent.
Queries the npm registry directly over HTTPS, verifies SHA-256 checksums,
manages ~/.config/manicode/freebuff, and requires zero Node.js/npm runtime.

Options:
  -h, --help                Show this help message and exit
  -v, --version             Print launcher and installed freebuff versions and exit
  -c, --check               Check installed vs. latest version and readiness status
  -u, --upgrade, --update   Download/update freebuff to the latest release immediately
  -d, --doctor, --prereqs   Diagnose prerequisites, distro, nix-ld, kitty, and binary cache
      --install-prereqs     Install system prerequisites (kitty, ca-certificates, libnotify)
      --install             Install binary to %s, create Kitty desktop icon, and check legacy Nix wrapper
  -r, --remove, --uninstall Remove installed binary and Kitty desktop icon
      --desktop-entry       Create/update only the Kitty desktop icon (~/.local/share/applications/freebuff-cli.desktop)
      --remove-sh           Remove installed NPM-based freebuff-cli.sh script and global npm/npx freebuff caches
      --remove-legacy-nix   Check for and prompt to remove previous Nix npx-based freebuff-cli generator
      --status              Emit single-line JSON update status for Waybar / Noctalia bar widgets
      --notify              Send a desktop notification (notify-send) if a newer release is available
  -n, --dry-run             Show what would be downloaded/installed/executed without changing anything
      --force               Run even if optional checks warn
  -y, --yes                 Assume yes to interactive prompts (e.g., removing legacy Nix generator)

Any unrecognized arguments (or arguments after --) are forwarded directly to the freebuff binary.
Use '-- --help' or '-- --version' to pass those flags directly to the underlying freebuff agent.

Environment overrides:
  FREEBUFF_INSTALL_PATH           Target binary path for --install/--remove (default %s)
  FREEBUFF_BINARY_TARGET          Override platform target (e.g. linux-x64, linux-x64-baseline, linux-arm64)
  FREEBUFF_CLI_CACHE_TTL          Seconds to cache npm registry lookups (default 21600 / 6h; 0 disables)
  FREEBUFF_CLI_NOTIFY_SEND        Path to notify-send binary for --notify
  NEXT_PUBLIC_CODEBUFF_APP_URL    Release download origin (default https://codebuff.com)
`, prog, launcher.LauncherVersion, installPath, installPath)
}

func main() {
	var (
		showHelp        bool
		showVersion     bool
		doCheck         bool
		doUpgrade       bool
		doDoctor        bool
		doInstallPrereq bool
		doInstall       bool
		doRemove        bool
		doDesktopEntry  bool
		doRemoveSh      bool
		doRemoveNix     bool
		doStatus        bool
		doNotify        bool
		dryRun          bool
		force           bool
		autoYes         bool
		forwardArgs     []string
	)

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			showHelp = true
		case "-v", "-V", "--version":
			showVersion = true
		case "-c", "--check":
			doCheck = true
		case "-u", "--upgrade", "--update":
			doUpgrade = true
		case "-d", "--doctor", "doctor", "--prereqs":
			doDoctor = true
		case "--install-prereqs":
			doInstallPrereq = true
		case "--install":
			doInstall = true
		case "-r", "--remove", "--uninstall":
			doRemove = true
		case "--desktop-entry":
			doDesktopEntry = true
		case "--remove-sh", "--remove-legacy-sh":
			doRemoveSh = true
		case "--remove-legacy-nix":
			doRemoveNix = true
		case "--status":
			doStatus = true
		case "--notify":
			doNotify = true
		case "-n", "--dry-run":
			dryRun = true
		case "--force":
			force = true
		case "-y", "--yes":
			autoYes = true
		case "--":
			forwardArgs = append(forwardArgs, args[i+1:]...)
			i = len(args)
		default:
			forwardArgs = append(forwardArgs, arg)
		}
	}

	cfg, err := launcher.NewConfig(dryRun, force)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	prog := filepath.Base(os.Args[0])

	if showHelp {
		printUsage(prog, cfg)
		os.Exit(0)
	}

	if showVersion {
		cur := cfg.GetCurrentVersion()
		if cur == "" {
			cur = "not downloaded yet"
		} else {
			cur = "v" + cur
		}
		fmt.Printf("freebuff-cli %s (freebuff agent: %s, target: %s)\n", launcher.LauncherVersion, cur, cfg.GetDownloadTargetKey())
		os.Exit(0)
	}

	if doStatus {
		os.Exit(cfg.RunStatusJSON())
	}

	if doNotify {
		os.Exit(cfg.RunNotify())
	}

	if doDoctor {
		os.Exit(cfg.RunDoctor())
	}

	if doCheck {
		os.Exit(cfg.RunCheck())
	}

	if doInstallPrereq {
		if err := cfg.InstallPrerequisites(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if doRemoveSh {
		if err := cfg.PromptAndRemoveLegacySh(autoYes); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if doRemoveNix {
		if err := cfg.PromptAndRemoveLegacyNix(autoYes); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if doDesktopEntry {
		exePath, _ := os.Executable()
		if fileExists := cfg.InstallPath; fileExists != "" {
			if _, err := os.Stat(cfg.InstallPath); err == nil {
				exePath = cfg.InstallPath
			}
		}
		if err := cfg.InstallDesktopEntry(exePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if doInstall {
		if err := cfg.InstallSelf(autoYes); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if doRemove {
		if err := cfg.UninstallSelf(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Check for legacy Nix npx wrapper and prompt on interactive sessions
	if legacy := cfg.DetectLegacyNixGenerator(); legacy != nil {
		_ = cfg.PromptAndRemoveLegacyNix(autoYes)
	}

	if doUpgrade {
		if err := cfg.EnsureBinaryReady(true); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		if len(forwardArgs) == 0 {
			os.Exit(0)
		}
	}

	code, err := cfg.RunBinary(forwardArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
