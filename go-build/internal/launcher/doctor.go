package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type statusWidgetOutput struct {
	Visible bool       `json:"visible"`
	Glyph   string     `json:"glyph,omitempty"`
	Text    string     `json:"text,omitempty"`
	Tips    [][]string `json:"tips,omitempty"`
}

// WarnIfOutdated checks the 6-hour registry cache (or 1s bounded lookup) and warns on stderr if behind.
func (c *Config) WarnIfOutdated() {
	current := c.GetCurrentVersion()
	if current == "" {
		return
	}
	rel := c.GetLatestReleaseCached(false, 1*time.Second)
	if rel == nil || rel.Version == "" {
		return
	}
	if IsNewer(rel.Version, current) {
		fmt.Fprintf(os.Stderr, "freebuff-cli: freebuff %s is behind npm latest %s.\nIt updates when a session exits, or refresh now with: freebuff-cli --upgrade\n", current, rel.Version)
	}
}

// RunStatusJSON prints the single-line JSON contract for Waybar / Noctalia bar widgets.
func (c *Config) RunStatusJSON() int {
	current := c.GetCurrentVersion()
	rel := c.GetLatestReleaseCached(false, 2*time.Second)
	if current == "" || rel == nil || !IsNewer(rel.Version, current) {
		fmt.Println(`{"visible":false}`)
		return 0
	}
	out := statusWidgetOutput{
		Visible: true,
		Glyph:   "arrow-up",
		Text:    rel.Version,
		Tips: [][]string{
			{"Installed", current},
			{"Latest", rel.Version},
			{"Update", "freebuff-cli --upgrade"},
		},
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
	return 0
}

// RunNotify sends a desktop notification via notify-send once per published release when an update is available.
func (c *Config) RunNotify() int {
	current := c.GetCurrentVersion()
	rel := c.GetLatestReleaseCached(false, 5*time.Second)
	if current == "" || rel == nil || !IsNewer(rel.Version, current) {
		c.ClearNotifiedVersion()
		return 0
	}
	if c.ReadNotifiedVersion() == rel.Version {
		return 0
	}
	notifyBin := os.Getenv("FREEBUFF_CLI_NOTIFY_SEND")
	if notifyBin == "" {
		notifyBin = "notify-send"
	}
	path, err := exec.LookPath(notifyBin)
	if err != nil {
		return 0
	}
	cmd := exec.Command(path,
		"--app-name=freebuff-cli",
		"--urgency=normal",
		"--icon=software-update-available",
		"freebuff update available",
		fmt.Sprintf("Installed %s, npm latest %s. Run freebuff-cli --upgrade to refresh.", current, rel.Version),
	)
	if cmd.Run() == nil {
		c.WriteNotifiedVersion(rel.Version)
	}
	return 0
}

// RunCheck prints a concise read-only status & update report (-c / --check).
// Returns 0 if ready and up-to-date (or ready to launch), 1 if prerequisites are missing.
func (c *Config) RunCheck() int {
	distro := DetectDistro()
	problems := 0

	fmt.Printf("Launcher:        freebuff-cli %s (standalone Go %s, zero Node/npm dependencies)\n", LauncherVersion, runtime.Version())
	fmt.Printf("System:          %s (family: %s)\n", distro.PrettyName, distro.Family)
	fmt.Printf("Platform target: %s (AVX2: %s)\n", c.GetDownloadTargetKey(), c.DescribeAvx2Support())

	loader, loaderOK := CheckDynamicLinker()
	if loaderOK {
		fmt.Printf("Dynamic loader:  %s — ok\n", loader)
	} else {
		fmt.Printf("Dynamic loader:  %s — MISSING (required to start downloaded binary)\n", loader)
		problems++
	}

	if kittyPath, err := exec.LookPath("kitty"); err == nil {
		fmt.Printf("Kitty terminal:  %s — ok\n", kittyPath)
	} else {
		fmt.Printf("Kitty terminal:  not found (required for desktop launcher; run --install-prereqs)\n")
	}

	if fileExists(c.DesktopEntryPath) {
		fmt.Printf("Desktop entry:   %s — installed\n", c.DesktopEntryPath)
	} else {
		fmt.Printf("Desktop entry:   not installed (run 'freebuff-cli --install' to create)\n")
	}

	if legacy := c.DetectLegacyNixGenerator(); legacy != nil {
		fmt.Printf("Legacy Nix wrap: DETECTED (%s) — run 'freebuff-cli --remove-legacy-nix' to clean up\n", legacy.WrapperBinaryPath)
	}
	if legacySh := c.DetectLegacyBashScript(); legacySh != nil && len(legacySh.ScriptPaths) > 0 {
		fmt.Printf("Legacy .sh wrap: DETECTED (%s) — run 'freebuff-cli --remove-sh' to clean up\n", strings.Join(legacySh.ScriptPaths, ", "))
	}

	current := c.GetCurrentVersion()
	meta := c.ReadInstalledMetadata()
	if current != "" && meta != nil {
		fmt.Printf("Installed agent: v%s (%s) at %s\n", current, meta.Target, c.BinaryPath)
	} else {
		fmt.Printf("Installed agent: none yet (will be downloaded on first launch to %s)\n", c.BinaryPath)
	}

	rel, err := c.FetchRegistryRelease("latest", 5*time.Second)
	if err != nil || rel == nil {
		fmt.Printf("Latest upstream: unknown (offline or registry unreachable: %v)\n", err)
	} else if current == "" {
		fmt.Printf("Latest upstream: v%s (run 'freebuff-cli' or 'freebuff-cli --upgrade' to download)\n", rel.Version)
	} else if IsNewer(rel.Version, current) {
		fmt.Printf("Latest upstream: v%s — UPDATE AVAILABLE (run 'freebuff-cli --upgrade')\n", rel.Version)
	} else {
		fmt.Printf("Latest upstream: v%s — up to date\n", rel.Version)
	}

	if problems == 0 {
		fmt.Println("Status:          ready to run.")
		return 0
	}
	fmt.Println("Status:          not ready — fix missing prerequisites above or run --doctor.")
	return 1
}

// RunDoctor performs a comprehensive diagnostic walkthrough (-d / --doctor / --prereqs).
func (c *Config) RunDoctor() int {
	failures := 0
	warnings := 0

	ok := func(msg string, args ...any) {
		fmt.Printf("  [ok]   "+msg+"\n", args...)
	}
	warn := func(msg string, args ...any) {
		fmt.Printf("  [warn] "+msg+"\n", args...)
		warnings++
	}
	fail := func(msg string, args ...any) {
		fmt.Printf("  [FAIL] "+msg+"\n", args...)
		failures++
	}
	fix := func(msg string, args ...any) {
		fmt.Printf("         fix: "+msg+"\n", args...)
	}

	fmt.Printf("freebuff-cli %s — doctor & prerequisite check\n\n", LauncherVersion)

	// 1. System & Distro
	distro := DetectDistro()
	fmt.Println("System & Architecture")
	fmt.Printf("  %s (%s/%s)\n", distro.PrettyName, runtime.GOOS, runtime.GOARCH)
	switch distro.Family {
	case "nixos", "apt", "pacman", "dnf", "emerge":
		ok("supported Linux distribution family: %s", distro.Family)
	default:
		warn("unrecognized distribution family (%s) — binary still runs if glibc is present", distro.PrettyName)
	}

	target := c.GetDownloadTargetKey()
	if PlatformTargets[target] != "" {
		ok("resolved binary target: %s (AVX2: %s)", target, c.DescribeAvx2Support())
	} else {
		fail("unsupported platform/architecture target: %s", target)
	}

	// 2. Runtime Prerequisites (Dynamic Linker, CA certs, Kitty, notify-send)
	fmt.Println("Prerequisites")
	loader, loaderOK := CheckDynamicLinker()
	if loaderOK {
		ok("ELF dynamic linker present: %s", loader)
	} else {
		fail("ELF dynamic linker missing: %s", loader)
		if distro.Family == "nixos" {
			fix("enable `programs.nix-ld.enable = true;` in your NixOS config and rebuild")
		} else {
			fix("install glibc / libc6 for your distribution")
		}
	}

	if caPath, caOK := CheckCATrust(); caOK {
		ok("TLS CA certificate bundle: %s", caPath)
	} else {
		fail("no TLS CA certificate bundle found in /etc/ssl or /etc/pki")
		fix("run 'freebuff-cli --install-prereqs' to install ca-certificates")
	}

	if kittyPath, err := exec.LookPath("kitty"); err == nil {
		ok("kitty terminal present: %s", kittyPath)
	} else {
		warn("kitty terminal not found on PATH (required for the desktop launcher icon)")
		fix("run 'freebuff-cli --install-prereqs' to install kitty")
	}

	if notifyPath, err := exec.LookPath("notify-send"); err == nil {
		ok("notify-send present: %s (used by --notify)", notifyPath)
	} else {
		warn("notify-send not found (optional, used for desktop update notifications)")
		fix("run 'freebuff-cli --install-prereqs' to install libnotify")
	}

	// 3. Legacy Wrapper Check (Nix & freebuff-cli.sh)
	fmt.Println("Legacy Wrapper Check")
	if legacy := c.DetectLegacyNixGenerator(); legacy != nil {
		warn("previous Nix npx-based freebuff-cli generator detected")
		if legacy.WrapperBinaryPath != "" {
			fmt.Printf("         wrapper: %s\n", legacy.WrapperBinaryPath)
		}
		for _, m := range legacy.ModuleFiles {
			fmt.Printf("         module:  %s\n", m)
		}
		fix("run 'freebuff-cli --remove-legacy-nix' (or 'freebuff-cli --install') to disable/remove it")
	} else {
		ok("no conflicting legacy npx-based Nix generator detected")
	}
	if legacySh := c.DetectLegacyBashScript(); legacySh != nil && len(legacySh.ScriptPaths) > 0 {
		warn("installed NPM-based freebuff-cli.sh script detected: %s", strings.Join(legacySh.ScriptPaths, ", "))
		fix("run 'freebuff-cli --remove-sh' to remove the old bash script and npm/npx cache")
	} else {
		ok("no conflicting installed freebuff-cli.sh script on PATH")
	}

	// 4. Terminal
	fmt.Println("Terminal")
	if IsTTY(os.Stdin) && IsTTY(os.Stdout) {
		ok("attached to an interactive terminal")
	} else {
		warn("stdin or stdout is not a TTY")
		fix("run freebuff-cli from an interactive terminal (such as kitty)")
	}
	if hasCommand("stty") {
		ok("stty present (used for OSC 11 reply drain)")
	} else {
		warn("stty not found (falling back to non-blocking syscall drain)")
	}
	termEnv := os.Getenv("TERM")
	if termEnv != "" && termEnv != "dumb" {
		ok("TERM=%s", termEnv)
	} else {
		warn("TERM is unset or 'dumb'")
		fix("export TERM=xterm-kitty or xterm-256color")
	}

	// 5. Self-install & Desktop Entry
	fmt.Println("Installation & Desktop Icon")
	installDir := filepath.Dir(c.InstallPath)
	if st, err := os.Stat(installDir); err == nil && st.IsDir() {
		if isDirWritable(installDir) || hasCommand("sudo") {
			ok("install directory %s exists and is writable (directly or via sudo)", installDir)
		} else {
			warn("cannot write %s and sudo is unavailable", installDir)
			fix("set FREEBUFF_INSTALL_PATH=$HOME/.local/bin/freebuff-cli")
		}
	} else {
		warn("%s does not exist", installDir)
		fix("set FREEBUFF_INSTALL_PATH=$HOME/.local/bin/freebuff-cli")
	}
	pathEnv := ":" + os.Getenv("PATH") + ":"
	if strings.Contains(pathEnv, ":"+installDir+":") {
		ok("%s is on PATH", installDir)
	} else {
		warn("%s is not on PATH", installDir)
		fix("add %s to PATH", installDir)
	}
	if fileExists(c.DesktopEntryPath) {
		ok("Kitty desktop entry installed: %s", c.DesktopEntryPath)
	} else {
		warn("Kitty desktop entry not installed yet (%s)", c.DesktopEntryPath)
		fix("run 'freebuff-cli --install' to install binary and Kitty desktop icon")
	}

	// 6. Upstream Registry & Native Binary Cache
	fmt.Println("Freebuff Binary & Registry")
	current := c.GetCurrentVersion()
	if current != "" {
		st, _ := os.Stat(c.BinaryPath)
		sizeStr := "unknown size"
		if st != nil {
			sizeStr = FormatBytes(st.Size())
		}
		ok("installed binary: v%s (%s) at %s", current, sizeStr, c.BinaryPath)
	} else {
		ok("binary not downloaded yet — first launch or '--upgrade' will download it to %s", c.BinaryPath)
	}
	if fileExists(c.WasmPath) {
		ok("tree-sitter.wasm present at %s", c.WasmPath)
	}

	rel, err := c.FetchRegistryRelease("latest", 5*time.Second)
	if err != nil || rel == nil {
		warn("npm registry could not be reached (%v)", err)
		fix("check network or HTTP_PROXY/HTTPS_PROXY settings; cached binary still launches offline")
	} else {
		ok("npm registry reachable — latest release is v%s", rel.Version)
		if current != "" && IsNewer(rel.Version, current) {
			warn("installed v%s is outdated (latest is v%s)", current, rel.Version)
			fix("run 'freebuff-cli --upgrade'")
		}
	}

	fmt.Println()
	if failures == 0 && warnings == 0 {
		fmt.Println("All checks passed — freebuff-cli is ready.")
		return 0
	}
	fmt.Printf("%d failure(s), %d warning(s).\n", failures, warnings)
	if failures > 0 {
		fmt.Println("Not ready — resolve the [FAIL] items above or run 'freebuff-cli --install-prereqs'.")
		return 1
	}
	fmt.Println("Nothing blocking — freebuff-cli should launch.")
	return 0
}
