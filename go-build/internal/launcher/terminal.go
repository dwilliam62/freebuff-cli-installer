package launcher

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	exitAlternateScreenSeq = "\x1b[?1049l"
	safeTerminalResetSeq   = "\x1b[?1000l" + // Disable X10 mouse mode
		"\x1b[?1002l" + // Disable button event mouse mode
		"\x1b[?1003l" + // Disable any-event mouse mode
		"\x1b[?1006l" + // Disable SGR extended mouse mode
		"\x1b[?1004l" + // Disable focus reporting
		"\x1b[?2004l" + // Disable bracketed paste mode
		"\x1b[<u" + // Pop kitty keyboard protocol flags
		"\x1b[>4;0m" + // Reset modifyOtherKeys
		"\x1b[?25h" // Show cursor
)

// IsTTY returns true if f is a character device terminal.
func IsTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return (st.Mode() & os.ModeCharDevice) != 0
}

// ResetTerminal emits ANSI sequences to restore terminal state after child exit/crash.
func ResetTerminal(exitAlternateScreen bool) {
	if !IsTTY(os.Stdout) {
		return
	}
	if exitAlternateScreen {
		_, _ = os.Stdout.WriteString(exitAlternateScreenSeq + safeTerminalResetSeq)
	} else {
		_, _ = os.Stdout.WriteString(safeTerminalResetSeq)
	}
}

// DrainTerminalReply reads and discards pending OSC 11 background-color replies
// after short-lived flags (--version / --help) so "11;rgb:..." never leaks to the shell.
func DrainTerminalReply() {
	if !IsTTY(os.Stdin) {
		return
	}
	sttyPath, err := exec.LookPath("stty")
	if err == nil && sttyPath != "" {
		saveCmd := exec.Command(sttyPath, "-g")
		saveCmd.Stdin = os.Stdin
		savedBytes, err := saveCmd.Output()
		if err == nil {
			saved := string(savedBytes)
			rawCmd := exec.Command(sttyPath, "-icanon", "min", "0", "time", "2")
			rawCmd.Stdin = os.Stdin
			if rawCmd.Run() == nil {
				buf := make([]byte, 256)
				_, _ = os.Stdin.Read(buf)
				restoreCmd := exec.Command(sttyPath, saved)
				restoreCmd.Stdin = os.Stdin
				_ = restoreCmd.Run()
				return
			}
		}
	}

	// Fallback non-blocking read using syscall.SetNonblock
	fd := int(os.Stdin.Fd())
	_ = syscall.SetNonblock(fd, true)
	defer func() { _ = syscall.SetNonblock(fd, false) }()

	deadline := time.Now().Add(150 * time.Millisecond)
	buf := make([]byte, 256)
	for time.Now().Before(deadline) {
		n, err := syscall.Read(fd, buf)
		if n > 0 {
			break
		}
		if err != nil {
			time.Sleep(15 * time.Millisecond)
		}
	}
}

func isShortLivedArg(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "-v", "-V", "--version", "-h", "--help":
		return true
	}
	return false
}

// PrintSystemInfo prints diagnostic system info on crash/spawn failure.
func (c *Config) PrintSystemInfo() {
	meta := c.ReadInstalledMetadata()
	target := c.GetDefaultTargetKey()
	if meta != nil && meta.Target != "" {
		target = meta.Target
	}
	fmt.Fprintln(os.Stderr, "System info:")
	fmt.Fprintf(os.Stderr, "  Platform: %s %s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(os.Stderr, "  Launcher: freebuff-cli %s (Go %s)\n", LauncherVersion, runtime.Version())
	if runtime.GOARCH == "amd64" {
		fmt.Fprintf(os.Stderr, "  AVX2:     %s\n", c.DescribeAvx2Support())
	}
	fmt.Fprintf(os.Stderr, "  Target:   %s\n", target)
	fmt.Fprintf(os.Stderr, "  Binary:   %s\n", c.BinaryPath)
}

func (c *Config) printBaselineOverrideHint() {
	fb := GetBaselineFallbackTargetKey()
	if fb == "" {
		return
	}
	fmt.Fprintln(os.Stderr, "To force the baseline (non-AVX2) build, set:")
	fmt.Fprintf(os.Stderr, "  FREEBUFF_BINARY_TARGET=%s\n\n", fb)
}

// RunBinary executes ~/.config/manicode/freebuff with args, background update staging,
// SIGILL fallback to -baseline, terminal cleanup, and OSC 11 reply draining.
func (c *Config) RunBinary(forwardArgs []string) (int, error) {
	if err := c.CheckNixLDLoader(); err != nil {
		return 1, err
	}

	c.AdoptOrphanedStagedUpdates()

	if err := c.EnsureBinaryReady(false); err != nil {
		return 1, err
	}

	if c.DryRun {
		fmt.Printf("[dry-run] exec %s %v\n", c.BinaryPath, forwardArgs)
		fmt.Println("Dry run complete — nothing was installed or launched.")
		return 0, nil
	}

	shortLived := isShortLivedArg(forwardArgs)
	if !shortLived {
		c.WarnIfOutdated()
	}

	return c.spawnAndMonitor(forwardArgs, true, shortLived)
}

func (c *Config) spawnAndMonitor(forwardArgs []string, allowBaselineFallback, shortLived bool) (int, error) {
	if _, err := os.Stat(c.BinaryPath); err != nil {
		_ = os.Remove(c.MetadataPath)
		c.PrintSystemInfo()
		return 1, fmt.Errorf("downloaded binary is missing at %s", c.BinaryPath)
	}

	cmd := exec.Command(c.BinaryPath, forwardArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"CODEBUFF_LAUNCHER_PID="+strconv.Itoa(os.Getpid()),
		"CODEBUFF_LAUNCHER_VERSION="+LauncherVersion,
	)

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		ResetTerminal(false)
		c.PrintSystemInfo()
		return 1, fmt.Errorf("failed to start %s: %w", PackageName, err)
	}

	// Forward termination signals to child
	sigChan := make(chan os.Signal, 4)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigChan)

	doneChan := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-sigChan:
				if cmd.Process != nil {
					_ = cmd.Process.Signal(sig)
				}
			case <-doneChan:
				return
			}
		}
	}()

	var (
		stagedMu     sync.Mutex
		stagedUpdate *StagedBinary
	)

	// Background deferred update check (100ms after launch, matching launcher.js)
	if !shortLived {
		go func() {
			select {
			case <-time.After(100 * time.Millisecond):
			case <-doneChan:
				return
			}
			rel, err := c.FetchRegistryRelease("latest", RequestTimeout)
			if err != nil || rel == nil {
				return
			}
			select {
			case <-doneChan:
				return
			default:
			}
			cur := c.GetCurrentVersion()
			if cur == "" || CompareVersions(cur, rel.Version) < 0 {
				sb, err := c.StageBinary(rel.Version, c.GetDownloadTargetKey(), rel.BinaryChecksums, true)
				if err != nil || sb == nil {
					return
				}
				select {
				case <-doneChan:
					_ = os.RemoveAll(c.TempDownloadDir)
					return
				default:
				}
				c.WriteStagedManifest(sb)
				stagedMu.Lock()
				stagedUpdate = sb
				stagedMu.Unlock()
			}
		}()
	}

	waitErr := cmd.Wait()
	close(doneChan)
	aliveDuration := time.Since(startTime)

	if shortLived {
		DrainTerminalReply()
	}

	var exitCode int
	var exitSignal syscall.Signal
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
			if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				exitSignal = ws.Signal()
			}
		} else {
			exitCode = 1
		}
	}

	// Check for SIGILL / illegal instruction fallback to -baseline
	if allowBaselineFallback && exitSignal == syscall.SIGILL {
		if ok, code, err := c.tryFallbackToBaseline(forwardArgs, shortLived); ok {
			return code, err
		}
	}

	if !shortLived {
		ResetTerminal(exitSignal != 0)
	}

	if exitSignal != 0 {
		c.printCrashDiagnostics(exitCode, exitSignal, aliveDuration)
	}

	// Install deferred update on exit if one was staged during the session
	stagedMu.Lock()
	pending := stagedUpdate
	stagedMu.Unlock()
	if pending != nil {
		cur := c.GetCurrentVersion()
		if cur != "" && CompareVersions(cur, pending.Version) >= 0 {
			_ = os.RemoveAll(c.TempDownloadDir)
		} else {
			if err := c.InstallStagedBinary(pending, true, true); err == nil {
				fmt.Printf("Updated %s to v%s. Ready for your next launch.\n", DisplayName, pending.Version)
			} else {
				fmt.Fprintf(os.Stderr, "Could not install %s update: %v\n", DisplayName, err)
			}
		}
	}

	if exitSignal != 0 {
		return 1, nil
	}
	return exitCode, nil
}

func (c *Config) tryFallbackToBaseline(forwardArgs []string, shortLived bool) (bool, int, error) {
	fallbackTarget := GetBaselineFallbackTargetKey()
	if fallbackTarget == "" || GetTargetOverride() != "" {
		return false, 1, nil
	}
	meta := c.ReadInstalledMetadata()
	currentTarget := c.GetDefaultTargetKey()
	if meta != nil && meta.Target != "" {
		currentTarget = meta.Target
	}
	if currentTarget == fallbackTarget {
		return false, 1, nil
	}

	probed := ProbeCPUAvx2()
	cpuHasAvx2 := probed != nil && *probed
	if cpuHasAvx2 {
		c.WriteCPUFeatureRecord(true, "cpuinfo")
	} else if probed != nil && !*probed {
		c.WriteCPUFeatureRecord(false, "cpuinfo")
	} else {
		c.WriteCPUFeatureRecord(false, "crash")
	}

	version := ""
	var checksums map[string]string
	if meta != nil && meta.Version != "" {
		version = meta.Version
	} else if rel, err := c.FetchRegistryRelease("latest", RequestTimeout); err == nil && rel != nil {
		version = rel.Version
		checksums = rel.BinaryChecksums
	}
	if version == "" {
		return false, 1, nil
	}

	ResetTerminal(true)
	fmt.Fprintln(os.Stderr)
	if cpuHasAvx2 {
		fmt.Fprintf(os.Stderr, "%s crashed, but this CPU supports AVX2; using the older-CPU binary until the next update.\n", PackageName)
	} else {
		fmt.Fprintf(os.Stderr, "%s is switching to the older-CPU (%s) binary for this machine.\n", PackageName, fallbackTarget)
	}

	staged, err := c.StageBinary(version, fallbackTarget, checksums, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to download %s: %v\n", fallbackTarget, err)
		return false, 1, nil
	}
	if err := c.InstallStagedBinary(staged, false, false); err != nil {
		return false, 1, nil
	}

	code, err := c.spawnAndMonitor(forwardArgs, false, shortLived)
	return true, code, err
}

func (c *Config) printCrashDiagnostics(code int, sig syscall.Signal, alive time.Duration) {
	_ = alive
	if sig != syscall.SIGILL && sig != syscall.SIGSEGV && sig != syscall.SIGBUS && sig != syscall.SIGABRT {
		return
	}
	fmt.Fprintf(os.Stderr, "\nError: %s exited immediately (signal %s)\n\n", PackageName, sig.String())
	switch sig {
	case syscall.SIGILL:
		fmt.Fprintln(os.Stderr, "Your CPU may not support the required instruction set (AVX2).")
		fmt.Fprintln(os.Stderr, "This typically affects CPUs from before 2013.")
		fmt.Fprintln(os.Stderr)
		if meta := c.ReadInstalledMetadata(); meta != nil && meta.Target == GetBaselineFallbackTargetKey() {
			fmt.Fprintln(os.Stderr, "This is already the older-CPU (baseline) build — please include the output below in your report.")
			fmt.Fprintln(os.Stderr)
		} else {
			c.printBaselineOverrideHint()
		}
	case syscall.SIGSEGV:
		fmt.Fprintln(os.Stderr, "The binary crashed with a segmentation fault (SIGSEGV).")
		fmt.Fprintln(os.Stderr)
	case syscall.SIGBUS:
		fmt.Fprintln(os.Stderr, "The binary crashed with a bus error (SIGBUS).")
		fmt.Fprintln(os.Stderr)
	case syscall.SIGABRT:
		fmt.Fprintln(os.Stderr, "The binary crashed with an abort signal (SIGABRT).")
		fmt.Fprintln(os.Stderr)
	}
	c.PrintSystemInfo()
	fmt.Fprintln(os.Stderr, "\nPlease report this issue at:")
	fmt.Fprintln(os.Stderr, "  https://github.com/CodebuffAI/codebuff/issues")
}
