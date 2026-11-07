package launcher

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ResolveIconPath finds the best available icon for the desktop entry.
func (c *Config) ResolveIconPath() string {
	candidates := []string{
		filepath.Join(c.HomeDir, "AppImages", ".icons", "freebuff"),
		filepath.Join(c.HomeDir, ".local", "share", "icons", "freebuff.png"),
		"/usr/share/pixmaps/freebuff.png",
	}
	for _, p := range candidates {
		if fileExists(p) {
			return p
		}
	}
	return "utilities-terminal"
}

// DesktopEntryContent generates the XDG desktop entry that launches freebuff-cli inside kitty.
func (c *Config) DesktopEntryContent(execBinaryPath string) string {
	if execBinaryPath == "" {
		execBinaryPath = c.InstallPath
	}
	icon := c.ResolveIconPath()
	return fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Freebuff CLI
GenericName=AI Coding Agent
Comment=Freebuff AI Coding Agent (CLI in Kitty)
Icon=%s
TryExec=kitty
Exec=kitty --title "Freebuff CLI" --class freebuff-cli -e %s
Terminal=false
Categories=Development;Utility;ConsoleOnly;
Keywords=ai;coding;agent;freebuff;cli;terminal;
StartupWMClass=freebuff-cli
`, icon, execBinaryPath)
}

// InstallDesktopEntry creates ~/.local/share/applications/freebuff-cli.desktop.
func (c *Config) InstallDesktopEntry(execBinaryPath string) error {
	content := c.DesktopEntryContent(execBinaryPath)
	if c.DryRun {
		fmt.Printf("[dry-run] write desktop entry to %s (Exec=kitty ... -e %s)\n", c.DesktopEntryPath, execBinaryPath)
		return nil
	}
	if err := os.MkdirAll(c.ApplicationsDir, 0o755); err != nil {
		return fmt.Errorf("could not create applications directory %s: %w", c.ApplicationsDir, err)
	}
	if err := os.WriteFile(c.DesktopEntryPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("could not write desktop entry %s: %w", c.DesktopEntryPath, err)
	}
	if hasCommand("update-desktop-database") {
		_ = exec.Command("update-desktop-database", c.ApplicationsDir).Run()
	}
	fmt.Printf("Installed Kitty desktop entry to %s.\n", c.DesktopEntryPath)
	return nil
}

// RemoveDesktopEntry removes ~/.local/share/applications/freebuff-cli.desktop.
func (c *Config) RemoveDesktopEntry() error {
	if !fileExists(c.DesktopEntryPath) {
		return nil
	}
	if c.DryRun {
		fmt.Printf("[dry-run] rm -f %s\n", c.DesktopEntryPath)
		return nil
	}
	if err := os.Remove(c.DesktopEntryPath); err != nil {
		return err
	}
	fmt.Printf("Removed desktop entry %s.\n", c.DesktopEntryPath)
	return nil
}

func isDirWritable(dir string) bool {
	if os.Geteuid() == 0 {
		return true
	}
	testFile := filepath.Join(dir, fmt.Sprintf(".freebuff-write-test-%d", os.Getpid()))
	f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(testFile)
	return true
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := fmt.Sprintf("%s.tmp.%d", dst, os.Getpid())
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// InstallSelf copies the compiled freebuff-cli binary to c.InstallPath (falling back to ~/.local/bin/freebuff-cli on NixOS if /usr/local/bin is missing)
// and installs the Kitty .desktop entry, plus checks for legacy Nix generators.
func (c *Config) InstallSelf(autoYes bool) error {
	if err := c.PromptAndRemoveLegacyNix(autoYes); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}
	if legacySh := c.DetectLegacyBashScript(); legacySh != nil && len(legacySh.ScriptPaths) > 0 {
		_ = c.PromptAndRemoveLegacySh(autoYes)
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot resolve current executable path: %w", err)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)

	targetPath := c.InstallPath
	targetDir := filepath.Dir(targetPath)

	// On NixOS, /usr/local/bin often does not exist; default to ~/.local/bin/freebuff-cli if targetDir is absent
	if _, err := os.Stat(targetDir); err != nil {
		distro := DetectDistro()
		if distro.Family == "nixos" && targetPath == DefaultInstallPath {
			localBin := filepath.Join(c.HomeDir, ".local", "bin")
			_ = os.MkdirAll(localBin, 0o755)
			targetPath = filepath.Join(localBin, "freebuff-cli")
			targetDir = localBin
			fmt.Printf("NixOS detected (/usr/local/bin absent) — installing to %s instead.\n", targetPath)
		} else {
			return fmt.Errorf("%s does not exist — refusing to create it.\nPoint FREEBUFF_INSTALL_PATH at an existing directory (e.g. $HOME/.local/bin/freebuff-cli)", targetDir)
		}
	}

	if exePath == targetPath {
		fmt.Printf("Binary already installed at %s.\n", targetPath)
	} else if isDirWritable(targetDir) {
		if c.DryRun {
			fmt.Printf("[dry-run] install -m 0755 %s %s\n", exePath, targetPath)
		} else {
			if err := copyFile(exePath, targetPath, 0o755); err != nil {
				return fmt.Errorf("could not copy binary to %s: %w", targetPath, err)
			}
			fmt.Printf("Installed binary to %s.\n", targetPath)
		}
	} else {
		elevCmd, ok := getElevationCmd()
		if !ok {
			return fmt.Errorf("need root to write %s, and neither sudo nor doas is available.\nSet FREEBUFF_INSTALL_PATH=$HOME/.local/bin/freebuff-cli", targetDir)
		}
		if c.DryRun {
			fmt.Printf("[dry-run] %s install -m 0755 %s %s\n", elevCmd, exePath, targetPath)
		} else {
			cmd := exec.Command(elevCmd, "install", "-m", "0755", exePath, targetPath)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("%s install to %s failed: %w", elevCmd, targetPath, err)
			}
			fmt.Printf("Installed binary to %s.\n", targetPath)
		}
	}

	if err := c.InstallDesktopEntry(targetPath); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not install desktop entry: %v\n", err)
	}

	pathEnv := ":" + os.Getenv("PATH") + ":"
	if !strings.Contains(pathEnv, ":"+targetDir+":") {
		fmt.Printf("Note: %s is not in your PATH — add it, or run %s directly.\n", targetDir, targetPath)
	}
	return nil
}

// UninstallSelf removes the installed binary and the Kitty .desktop entry.
func (c *Config) UninstallSelf() error {
	targetPath := c.InstallPath
	if !fileExists(targetPath) {
		localAlt := filepath.Join(c.HomeDir, ".local", "bin", "freebuff-cli")
		if targetPath == DefaultInstallPath && fileExists(localAlt) {
			targetPath = localAlt
		}
	}

	if !fileExists(targetPath) {
		fmt.Printf("No installed binary at %s — nothing to remove.\n", targetPath)
	} else {
		targetDir := filepath.Dir(targetPath)
		if isDirWritable(targetDir) {
			if c.DryRun {
				fmt.Printf("[dry-run] rm -f %s\n", targetPath)
			} else {
				if err := os.Remove(targetPath); err != nil {
					return fmt.Errorf("could not remove %s: %w", targetPath, err)
				}
				fmt.Printf("Removed %s.\n", targetPath)
			}
		} else {
			elevCmd, ok := getElevationCmd()
			if !ok {
				return fmt.Errorf("need root to remove %s, and neither sudo nor doas is available", targetPath)
			}
			if c.DryRun {
				fmt.Printf("[dry-run] %s rm -f %s\n", elevCmd, targetPath)
			} else {
				cmd := exec.Command(elevCmd, "rm", "-f", targetPath)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("%s rm of %s failed: %w", elevCmd, targetPath, err)
				}
				fmt.Printf("Removed %s.\n", targetPath)
			}
		}
	}

	if err := c.RemoveDesktopEntry(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not remove desktop entry: %v\n", err)
	}
	return nil
}
