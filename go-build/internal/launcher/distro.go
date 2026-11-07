package launcher

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DistroInfo describes the current Linux distribution and package manager.
type DistroInfo struct {
	ID         string
	IDLike     string
	PrettyName string
	Family     string // nixos | apt | pacman | dnf | emerge | unknown
}

// DetectDistro parses /etc/os-release and maps it to supported distro families.
func DetectDistro() DistroInfo {
	info := DistroInfo{
		PrettyName: "unknown",
		Family:     "unknown",
	}
	f, err := os.Open("/etc/os-release")
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			k := parts[0]
			v := strings.Trim(parts[1], `"'`)
			switch k {
			case "ID":
				info.ID = strings.ToLower(v)
			case "ID_LIKE":
				info.IDLike = strings.ToLower(v)
			case "PRETTY_NAME":
				info.PrettyName = v
			}
		}
	}
	if info.PrettyName == "unknown" && info.ID != "" {
		info.PrettyName = info.ID
	}

	keys := " " + info.ID + " " + info.IDLike + " "
	switch {
	case strings.Contains(keys, "nixos") || fileExists("/etc/NIXOS"):
		info.Family = "nixos"
	case strings.Contains(keys, "debian") || strings.Contains(keys, "ubuntu") || strings.Contains(keys, "mint") || strings.Contains(keys, "pop"):
		info.Family = "apt"
	case strings.Contains(keys, "arch") || strings.Contains(keys, "manjaro") || strings.Contains(keys, "endeavouros"):
		info.Family = "pacman"
	case strings.Contains(keys, "fedora") || strings.Contains(keys, "rhel") || strings.Contains(keys, "centos"):
		info.Family = "dnf"
	case strings.Contains(keys, "gentoo"):
		info.Family = "emerge"
	default:
		if hasCommand("apt-get") {
			info.Family = "apt"
		} else if hasCommand("pacman") {
			info.Family = "pacman"
		} else if hasCommand("dnf") {
			info.Family = "dnf"
		} else if hasCommand("emerge") {
			info.Family = "emerge"
		}
	}
	return info
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func isExecutable(p string) bool {
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return false
	}
	return st.Mode()&0o111 != 0
}

func hasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func getElevationCmd() (string, bool) {
	if os.Geteuid() == 0 {
		return "", true
	}
	if hasCommand("sudo") {
		return "sudo", true
	}
	if hasCommand("doas") {
		return "doas", true
	}
	return "", false
}

// CheckNixLDLoader verifies that NixOS has nix-ld enabled when running on NixOS.
func (c *Config) CheckNixLDLoader() error {
	d := DetectDistro()
	if d.Family != "nixos" {
		return nil
	}
	if isExecutable("/run/current-system/sw/share/nix-ld/lib/ld.so") {
		return nil
	}
	return fmt.Errorf(`freebuff-cli: nix-ld is not enabled on this system.
The freebuff binary is a generic dynamically linked executable,
and NixOS' stub-ld refuses to start those.

Enable it in your NixOS configuration and rebuild:
  programs.nix-ld.enable = true;
  sudo nixos-rebuild switch --flake .`)
}

// CheckCATrust verifies whether standard TLS CA certificate bundles exist on the system.
func CheckCATrust() (string, bool) {
	candidates := []string{
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
		"/etc/ssl/ca-bundle.pem",
		"/etc/ssl/cert.pem",
	}
	for _, p := range candidates {
		if fileExists(p) {
			return p, true
		}
	}
	return "", false
}

// CheckDynamicLinker checks if the x86_64 or aarch64 glibc ELF interpreter exists.
func CheckDynamicLinker() (string, bool) {
	d := DetectDistro()
	if d.Family == "nixos" {
		if isExecutable("/run/current-system/sw/share/nix-ld/lib/ld.so") {
			return "/run/current-system/sw/share/nix-ld/lib/ld.so (nix-ld)", true
		}
		return "/run/current-system/sw/share/nix-ld/lib/ld.so", false
	}
	loaders := []string{
		"/lib64/ld-linux-x86-64.so.2",
		"/lib/x86_64-linux-gnu/ld-linux-x86-64.so.2",
		"/lib/ld-linux-aarch64.so.1",
	}
	for _, l := range loaders {
		if fileExists(l) {
			return l, true
		}
	}
	return loaders[0], false
}

// LegacyNixGenerator describes a detected previous npx-based Nix freebuff-cli generator.
type LegacyNixGenerator struct {
	WrapperBinaryPath string
	TargetStorePath   string
	ModuleFiles       []string
	DefaultNixFiles   []string
	TimerActive       bool
}

// DetectLegacyNixGenerator checks if a previous npx-based Nix generator of freebuff-cli is installed.
func (c *Config) DetectLegacyNixGenerator() *LegacyNixGenerator {
	var found *LegacyNixGenerator

	user := os.Getenv("USER")
	if user == "" {
		user = filepath.Base(c.HomeDir)
	}
	binCandidates := []string{
		filepath.Join("/etc/profiles/per-user", user, "bin", "freebuff-cli"),
		filepath.Join(c.HomeDir, ".nix-profile", "bin", "freebuff-cli"),
		"/run/current-system/sw/bin/freebuff-cli",
	}

	for _, bp := range binCandidates {
		data, err := os.ReadFile(bp)
		if err != nil {
			continue
		}
		content := string(data)
		// Legacy Nix wrapper is a bash script invoking `npx --yes freebuff`
		if strings.HasPrefix(content, "#!") && strings.Contains(content, "npx") && strings.Contains(content, "freebuff") {
			resolved, _ := filepath.EvalSymlinks(bp)
			found = &LegacyNixGenerator{
				WrapperBinaryPath: bp,
				TargetStorePath:   resolved,
			}
			break
		}
	}

	// Search common NixOS config directories for legacy freebuff-cli.nix modules
	configRoots := []string{
		filepath.Join(c.HomeDir, "ddubsos"),
		filepath.Join(c.HomeDir, "NixOS-Hyprland"),
		filepath.Join(c.HomeDir, ".config", "home-manager"),
		filepath.Join(c.HomeDir, ".config", "nixos"),
	}

	var modFiles []string
	var defFiles []string
	for _, root := range configRoots {
		if !fileExists(root) {
			continue
		}
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if d.Name() == ".git" || d.Name() == "result" {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Name() == "freebuff-cli.nix" {
				if b, err := os.ReadFile(p); err == nil && strings.Contains(string(b), "npx") {
					modFiles = append(modFiles, p)
				}
			} else if strings.HasSuffix(d.Name(), ".nix") {
				if b, err := os.ReadFile(p); err == nil {
					s := string(b)
					if strings.Contains(s, "freebuff-cli.nix") || strings.Contains(s, "programs.freebuff-cli.enable") {
						defFiles = append(defFiles, p)
					}
				}
			}
			return nil
		})
	}

	if found == nil && len(modFiles) == 0 {
		return nil
	}
	if found == nil {
		found = &LegacyNixGenerator{}
	}
	found.ModuleFiles = modFiles
	found.DefaultNixFiles = defFiles

	if hasCommand("systemctl") {
		if exec.Command("systemctl", "--user", "is-active", "--quiet", "freebuff-cli-check.timer").Run() == nil {
			found.TimerActive = true
		}
	}
	return found
}

// PromptAndRemoveLegacyNix checks for the previous Nix npx generator and prompts the user to clean it up.
func (c *Config) PromptAndRemoveLegacyNix(autoYes bool) error {
	legacy := c.DetectLegacyNixGenerator()
	if legacy == nil {
		return nil
	}

	fmt.Println("⚠️  Detected previous Nix npx-based freebuff-cli generator:")
	if legacy.WrapperBinaryPath != "" {
		fmt.Printf("  - Active Nix wrapper: %s\n", legacy.WrapperBinaryPath)
		if legacy.TargetStorePath != "" && legacy.TargetStorePath != legacy.WrapperBinaryPath {
			fmt.Printf("    (points to %s)\n", legacy.TargetStorePath)
		}
	}
	for _, m := range legacy.ModuleFiles {
		fmt.Printf("  - Legacy Nix module:  %s\n", m)
	}
	for _, d := range legacy.DefaultNixFiles {
		fmt.Printf("  - Imported/enabled in: %s\n", d)
	}
	if legacy.TimerActive {
		fmt.Println("  - Active systemd timer: freebuff-cli-check.timer")
	}

	proceed := autoYes
	if !proceed && IsTTY(os.Stdin) {
		fmt.Print("\nWould you like to disable/remove the previous Nix npx generator and clean up its npx cache? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		ans = strings.ToLower(strings.TrimSpace(ans))
		proceed = ans == "y" || ans == "yes"
	} else if !proceed {
		fmt.Println("Run 'freebuff-cli --remove-legacy-nix' interactively to clean it up.")
		return nil
	}

	if !proceed {
		fmt.Println("Skipped removing previous Nix generator.")
		return nil
	}

	// 1. Stop systemd user timer if active
	if legacy.TimerActive && hasCommand("systemctl") {
		if c.DryRun {
			fmt.Println("[dry-run] systemctl --user disable --now freebuff-cli-check.timer")
		} else {
			_ = exec.Command("systemctl", "--user", "disable", "--now", "freebuff-cli-check.timer").Run()
			fmt.Println("Stopped freebuff-cli-check.timer.")
		}
	}

	// 2. Clean up stale ~/.npm/_npx/*/node_modules/freebuff caches
	npxBase := filepath.Join(c.HomeDir, ".npm", "_npx")
	if entries, err := os.ReadDir(npxBase); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pkgDir := filepath.Join(npxBase, e.Name(), "node_modules", "freebuff")
			binLink := filepath.Join(npxBase, e.Name(), "node_modules", ".bin", "freebuff")
			if fileExists(pkgDir) {
				if c.DryRun {
					fmt.Printf("[dry-run] rm -rf %s %s\n", pkgDir, binLink)
				} else {
					_ = os.RemoveAll(pkgDir)
					_ = os.Remove(binLink)
					fmt.Printf("Removed cached npx wrapper: %s\n", pkgDir)
				}
			}
		}
	}

	// 3. Comment out legacy import/enable in user Nix config files (with .bak backup) if writable
	for _, df := range legacy.DefaultNixFiles {
		data, err := os.ReadFile(df)
		if err != nil {
			continue
		}
		orig := string(data)
		updated := orig
		updated = strings.ReplaceAll(updated, "imports = [./freebuff-cli.nix];", "# imports = [./freebuff-cli.nix]; # replaced by standalone Go freebuff-cli")
		updated = strings.ReplaceAll(updated, "programs.freebuff-cli.enable = true;", "# programs.freebuff-cli.enable = true; # replaced by standalone Go freebuff-cli")
		if updated != orig {
			if c.DryRun {
				fmt.Printf("[dry-run] comment out legacy freebuff-cli.nix in %s\n", df)
			} else {
				_ = os.WriteFile(df+".bak", data, 0o644)
				if err := os.WriteFile(df, []byte(updated), 0o644); err == nil {
					fmt.Printf("Disabled legacy npx module in %s (backup saved to %s.bak)\n", df, df)
					fmt.Println("Note: Rebuild your NixOS/Home Manager flake (or switch to go-build/nixos/freebuff-cli.nix) to remove the old symlink from /etc/profiles.")
				}
			}
		}
	}

	return nil
}

// LegacyBashScript describes a detected NPM-based freebuff-cli.sh installation and global npm/npx cache.
type LegacyBashScript struct {
	ScriptPaths []string
	GlobalNpm   bool
	NpxDirs     []string
}

// DetectLegacyBashScript checks for installed copies of the NPM-based freebuff-cli.sh script
// as well as global npm freebuff installs and ~/.npm/_npx caches.
func (c *Config) DetectLegacyBashScript() *LegacyBashScript {
	candidates := []string{
		"/usr/local/bin/freebuff-cli.sh",
		filepath.Join(c.HomeDir, ".local", "bin", "freebuff-cli.sh"),
		"/usr/bin/freebuff-cli.sh",
	}
	if p, err := exec.LookPath("freebuff-cli.sh"); err == nil && p != "" {
		candidates = append(candidates, p)
	}

	seen := make(map[string]bool)
	var scripts []string
	for _, p := range candidates {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if fileExists(p) {
			scripts = append(scripts, p)
		}
	}

	var npxDirs []string
	npxBase := filepath.Join(c.HomeDir, ".npm", "_npx")
	if entries, err := os.ReadDir(npxBase); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			pkgDir := filepath.Join(npxBase, e.Name(), "node_modules", "freebuff")
			if fileExists(pkgDir) {
				npxDirs = append(npxDirs, pkgDir)
			}
		}
	}

	globalNpm := false
	if hasCommand("npm") {
		if out, err := exec.Command("npm", "list", "-g", "--depth=0", "freebuff").Output(); err == nil {
			if strings.Contains(string(out), "freebuff@") {
				globalNpm = true
			}
		}
	}

	if len(scripts) == 0 && !globalNpm && len(npxDirs) == 0 {
		return nil
	}
	return &LegacyBashScript{
		ScriptPaths: scripts,
		GlobalNpm:   globalNpm,
		NpxDirs:     npxDirs,
	}
}

// PromptAndRemoveLegacySh checks for the NPM-based freebuff-cli.sh script and npm/npx artifacts and removes them.
func (c *Config) PromptAndRemoveLegacySh(autoYes bool) error {
	legacy := c.DetectLegacyBashScript()
	if legacy == nil {
		fmt.Println("No installed NPM-based freebuff-cli.sh or npm/npx freebuff packages found.")
		return nil
	}

	fmt.Println("⚠️  Detected NPM-based freebuff-cli.sh / npm installation:")
	for _, sp := range legacy.ScriptPaths {
		fmt.Printf("  - Installed script: %s\n", sp)
	}
	if legacy.GlobalNpm {
		fmt.Println("  - Global npm package: freebuff (npm list -g freebuff)")
	}
	for _, nd := range legacy.NpxDirs {
		fmt.Printf("  - Cached npx package: %s\n", nd)
	}

	proceed := autoYes
	if !proceed && IsTTY(os.Stdin) {
		fmt.Print("\nWould you like to remove the NPM-based freebuff-cli.sh script and npm/npx packages? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		ans = strings.ToLower(strings.TrimSpace(ans))
		proceed = ans == "y" || ans == "yes"
	} else if !proceed {
		fmt.Println("Run 'freebuff-cli --remove-sh' interactively (or with -y) to remove it.")
		return nil
	}

	if !proceed {
		fmt.Println("Skipped removing NPM-based freebuff-cli.sh.")
		return nil
	}

	for _, sp := range legacy.ScriptPaths {
		dir := filepath.Dir(sp)
		if isDirWritable(dir) {
			if c.DryRun {
				fmt.Printf("[dry-run] rm -f %s\n", sp)
			} else if err := os.Remove(sp); err == nil {
				fmt.Printf("Removed %s.\n", sp)
			} else {
				fmt.Fprintf(os.Stderr, "Could not remove %s: %v\n", sp, err)
			}
		} else if elevCmd, ok := getElevationCmd(); ok {
			if c.DryRun {
				fmt.Printf("[dry-run] %s rm -f %s\n", elevCmd, sp)
			} else {
				cmd := exec.Command(elevCmd, "rm", "-f", sp)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err == nil {
					fmt.Printf("Removed %s.\n", sp)
				} else {
					fmt.Fprintf(os.Stderr, "Could not remove %s: %v\n", sp, err)
				}
			}
		} else {
			fmt.Fprintf(os.Stderr, "Need root to remove %s and neither sudo nor doas is available.\n", sp)
		}
	}

	if legacy.GlobalNpm && hasCommand("npm") {
		if c.DryRun {
			fmt.Println("[dry-run] npm uninstall -g freebuff")
		} else {
			if exec.Command("npm", "uninstall", "-g", "freebuff").Run() == nil {
				fmt.Println("Uninstalled global npm package 'freebuff'.")
			}
		}
	}

	for _, pkgDir := range legacy.NpxDirs {
		binLink := filepath.Join(filepath.Dir(pkgDir), ".bin", "freebuff")
		if c.DryRun {
			fmt.Printf("[dry-run] rm -rf %s %s\n", pkgDir, binLink)
		} else {
			_ = os.RemoveAll(pkgDir)
			_ = os.Remove(binLink)
			fmt.Printf("Removed cached npx package: %s\n", pkgDir)
		}
	}

	return nil
}

// InstallPrerequisites installs required system packages (kitty, ca-certificates, libnotify, coreutils)
// across Debian/Ubuntu, Arch, Fedora, and Gentoo, or prints NixOS instructions.
func (c *Config) InstallPrerequisites() error {
	d := DetectDistro()
	fmt.Printf("Detected system: %s (family: %s)\n", d.PrettyName, d.Family)

	if d.Family == "nixos" {
		fmt.Println("\nOn NixOS, prerequisites are managed declaratively:")
		fmt.Println("  1. Ensure `programs.nix-ld.enable = true;` is set in your NixOS configuration.")
		fmt.Println("  2. Ensure `pkgs.kitty` and `pkgs.libnotify` are in your packages (or import `go-build/nixos/freebuff-cli.nix`).")
		return nil
	}

	var cmds [][]string
	switch d.Family {
	case "apt":
		cmds = [][]string{
			{"apt-get", "update"},
			{"apt-get", "install", "-y", "kitty", "ca-certificates", "libnotify-bin", "coreutils"},
		}
	case "pacman":
		cmds = [][]string{
			{"pacman", "-S", "--needed", "--noconfirm", "kitty", "ca-certificates", "libnotify", "coreutils"},
		}
	case "dnf":
		cmds = [][]string{
			{"dnf", "install", "-y", "kitty", "ca-certificates", "libnotify", "coreutils"},
		}
	case "emerge":
		cmds = [][]string{
			{"emerge", "--ask=n", "--noreplace", "x11-terms/kitty", "app-misc/ca-certificates", "x11-libs/libnotify", "sys-apps/coreutils"},
		}
	default:
		return fmt.Errorf("automatic prerequisite installation is only supported on Debian/Ubuntu (apt), Arch (pacman), Fedora (dnf), and Gentoo (emerge)")
	}

	needSudo := os.Geteuid() != 0
	elevCmd, canElevate := getElevationCmd()
	if needSudo && !canElevate {
		return fmt.Errorf("root privileges are required to install system packages, and neither sudo nor doas was found")
	}

	for _, args := range cmds {
		fullArgs := args
		if needSudo {
			fullArgs = append([]string{elevCmd}, args...)
		}
		if c.DryRun {
			fmt.Printf("[dry-run] %s\n", strings.Join(fullArgs, " "))
			continue
		}
		fmt.Printf("Running: %s\n", strings.Join(fullArgs, " "))
		cmd := exec.Command(fullArgs[0], fullArgs[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("command %q failed: %w", strings.Join(fullArgs, " "), err)
		}
	}

	fmt.Println("Prerequisites installed successfully.")
	return nil
}
