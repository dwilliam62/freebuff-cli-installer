package launcher

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// StagedBinary holds information about a downloaded and verified binary in a temporary directory.
type StagedBinary struct {
	TempBinaryPath string
	Version        string
	TargetKey      string
}

type stagedManifest struct {
	Version string            `json:"version"`
	Target  string            `json:"target"`
	Files   map[string]string `json:"files"`
}

// ComputeFileSha256 computes the lowercase hex SHA-256 of a file on disk.
func ComputeFileSha256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyFileSha256 checks that filePath matches expectedSha256 (fails closed if expectedSha256 is invalid).
func VerifyFileSha256(filePath, expectedSha256 string) error {
	expected := strings.ToLower(strings.TrimSpace(expectedSha256))
	if !IsSha256Hex(expected) {
		return fmt.Errorf("no sha256 checksum is published for this archive")
	}
	actual, err := ComputeFileSha256(filePath)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("sha256 mismatch: expected %s, downloaded %s", expected, actual)
	}
	return nil
}

// IsAllowedArchiveEntry restricts tar extraction strictly to the binary and tree-sitter.wasm at the root.
func (c *Config) IsAllowedArchiveEntry(entryPath string, typeflag byte) (string, bool) {
	if typeflag != tar.TypeReg && typeflag != tar.TypeRegA {
		return "", false
	}
	normalized := strings.TrimPrefix(entryPath, "./")
	for strings.HasPrefix(normalized, "./") {
		normalized = strings.TrimPrefix(normalized, "./")
	}
	if strings.Contains(normalized, "/") || strings.Contains(normalized, "\\") || strings.Contains(normalized, "..") {
		return "", false
	}
	if normalized == c.BinaryName || normalized == "tree-sitter.wasm" {
		return normalized, true
	}
	return "", false
}

// ExtractAllowedTarGz unpacks only allowed root files (freebuff and tree-sitter.wasm) into destDir.
func (c *Config) ExtractAllowedTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		cleanName, ok := c.IsAllowedArchiveEntry(hdr.Name, hdr.Typeflag)
		if !ok {
			continue
		}
		outPath := filepath.Join(destDir, cleanName)
		outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(outFile, tr)
		closeErr := outFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

// FormatBytes formats a byte count into human-readable units.
func FormatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit && exp < 3; n /= unit {
		div *= unit
		exp++
	}
	sizes := []string{"KB", "MB", "GB", "TB"}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), sizes[exp])
}

func renderProgressBar(pct, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := (width * pct) / 100
	empty := width - filled
	return "[" + strings.Repeat("█", filled) + strings.Repeat("░", empty) + "]"
}

func (c *Config) getPartialArchivePath(version, targetKey string) (string, error) {
	if !IsStrictReleaseVersion(version) {
		return "", fmt.Errorf("invalid release version for an archive path: %s", version)
	}
	return filepath.Join(c.ConfigDir, fmt.Sprintf(".%s-%s-%s.tar.gz.part", PackageName, version, targetKey)), nil
}

// StageBinary downloads, verifies (SHA-256), and extracts the target archive into TempDownloadDir.
func (c *Config) StageBinary(version, targetKey string, providedChecksums map[string]string, quiet bool) (*StagedBinary, error) {
	if targetKey == "" {
		targetKey = c.GetDownloadTargetKey()
	}
	fileName := PlatformTargets[targetKey]
	if !IsStrictReleaseVersion(version) {
		return nil, fmt.Errorf("refusing to download %s: %q is not a release version", PackageName, version)
	}
	if fileName == "" {
		return nil, fmt.Errorf("unsupported platform target: %s", targetKey)
	}

	expectedChecksum, err := c.GetExpectedChecksum(version, targetKey, providedChecksums)
	if err != nil {
		return nil, err
	}

	downloadURL := fmt.Sprintf("%s/api/releases/download/%s/%s", c.DownloadOrigin, version, fileName)
	if c.DryRun {
		fmt.Printf("[dry-run] download %s -> %s (expected sha256: %s)\n", downloadURL, c.BinaryPath, expectedChecksum)
		return &StagedBinary{
			TempBinaryPath: filepath.Join(c.TempDownloadDir, c.BinaryName),
			Version:        version,
			TargetKey:      targetKey,
		}, nil
	}

	if err := os.MkdirAll(c.ConfigDir, 0o755); err != nil {
		return nil, err
	}

	partialPath, err := c.getPartialArchivePath(version, targetKey)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= DownloadMaxAttempts; attempt++ {
		_ = os.RemoveAll(c.TempDownloadDir)
		if err := os.MkdirAll(c.TempDownloadDir, 0o755); err != nil {
			return nil, err
		}

		if !quiet {
			fmt.Fprintf(os.Stderr, "\r\x1b[KDownloading %s v%s (%s)...", PackageName, version, targetKey)
		}

		var lastPrint time.Time
		_, dlErr := DownloadFileWithResume(downloadURL, partialPath, c.UserAgent, DownloadTimeout, func(dl, total int64) {
			if quiet {
				return
			}
			now := time.Now()
			if now.Sub(lastPrint) < 100*time.Millisecond && dl != total {
				return
			}
			lastPrint = now
			if total > 0 {
				pct := int((dl * 100) / total)
				fmt.Fprintf(os.Stderr, "\r\x1b[KDownloading... %s %d%% of %s", renderProgressBar(pct, 30), pct, FormatBytes(total))
			} else {
				fmt.Fprintf(os.Stderr, "\r\x1b[KDownloading... %s", FormatBytes(dl))
			}
		})

		if dlErr != nil {
			lastErr = dlErr
			if attempt < DownloadMaxAttempts {
				delay := time.Duration(1<<(attempt-1)) * time.Second
				if !quiet {
					fmt.Fprintf(os.Stderr, "\r\x1b[KDownload interrupted (%v). Retrying in %v (%d/%d)...\n", dlErr, delay, attempt+1, DownloadMaxAttempts)
				}
				time.Sleep(delay)
				continue
			}
			break
		}

		if !quiet {
			fmt.Fprintf(os.Stderr, "\r\x1b[KVerifying SHA-256 checksum...")
		}
		if err := VerifyFileSha256(partialPath, expectedChecksum); err != nil {
			_ = os.Remove(partialPath)
			_ = os.RemoveAll(c.TempDownloadDir)
			return nil, fmt.Errorf("downloaded %s %s (%s) failed verification: %w", PackageName, version, targetKey, err)
		}

		if err := c.ExtractAllowedTarGz(partialPath, c.TempDownloadDir); err != nil {
			_ = os.Remove(partialPath)
			lastErr = err
			break
		}

		tempBin := filepath.Join(c.TempDownloadDir, c.BinaryName)
		if _, err := os.Stat(tempBin); err != nil {
			_ = os.Remove(partialPath)
			_ = os.RemoveAll(c.TempDownloadDir)
			return nil, fmt.Errorf("binary %s not found in extracted archive", c.BinaryName)
		}
		if runtime.GOOS != "windows" {
			_ = os.Chmod(tempBin, 0o755)
		}
		_ = os.Remove(partialPath)

		return &StagedBinary{
			TempBinaryPath: tempBin,
			Version:        version,
			TargetKey:      targetKey,
		}, nil
	}

	_ = os.RemoveAll(c.TempDownloadDir)
	return nil, fmt.Errorf("failed to download %s v%s: %w", PackageName, version, lastErr)
}

type fileReplacement struct {
	backupPath string
	targetPath string
}

func replaceFileWithRollback(sourcePath, targetPath string, replacements *[]fileReplacement) error {
	var backupPath string
	if _, err := os.Stat(targetPath); err == nil {
		backupPath = fmt.Sprintf("%s.old.%d.%d", targetPath, time.Now().UnixMilli(), len(*replacements))
		if err := os.Rename(targetPath, backupPath); err != nil {
			return err
		}
	}
	if err := os.Rename(sourcePath, targetPath); err != nil {
		if backupPath != "" {
			_ = os.Rename(backupPath, targetPath)
		}
		return err
	}
	*replacements = append(*replacements, fileReplacement{backupPath: backupPath, targetPath: targetPath})
	return nil
}

func rollbackReplacements(replacements []fileReplacement) {
	for i := len(replacements) - 1; i >= 0; i-- {
		r := replacements[i]
		_ = os.Remove(r.targetPath)
		if r.backupPath != "" {
			_ = os.Rename(r.backupPath, r.targetPath)
		}
	}
}

func commitReplacements(replacements []fileReplacement) {
	for _, r := range replacements {
		if r.backupPath != "" {
			_ = os.Remove(r.backupPath)
		}
	}
}

// InstallStagedBinary atomically swaps the staged binary, tree-sitter.wasm, and metadata into ~/.config/manicode.
func (c *Config) InstallStagedBinary(staged *StagedBinary, quiet, keepStagingOnFailure bool) error {
	if c.DryRun {
		fmt.Printf("[dry-run] install staged %s v%s (%s) -> %s\n", PackageName, staged.Version, staged.TargetKey, c.BinaryPath)
		return nil
	}

	var replacements []fileReplacement
	installed := false
	metaTemp := fmt.Sprintf("%s.new.%d", c.MetadataPath, os.Getpid())
	stagingDir := filepath.Dir(staged.TempBinaryPath)

	defer func() {
		_ = os.Remove(metaTemp)
		if installed || !keepStagingOnFailure {
			_ = os.RemoveAll(stagingDir)
		}
	}()

	metaBytes, err := json.MarshalIndent(InstalledMetadata{
		Version: staged.Version,
		Target:  staged.TargetKey,
	}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(metaTemp, append(metaBytes, '\n'), 0o644); err != nil {
		return err
	}

	if err := replaceFileWithRollback(staged.TempBinaryPath, c.BinaryPath, &replacements); err != nil {
		rollbackReplacements(replacements)
		return err
	}

	tempWasm := filepath.Join(stagingDir, "tree-sitter.wasm")
	if _, err := os.Stat(tempWasm); err == nil {
		if err := replaceFileWithRollback(tempWasm, c.WasmPath, &replacements); err != nil {
			rollbackReplacements(replacements)
			return err
		}
	}

	if err := replaceFileWithRollback(metaTemp, c.MetadataPath, &replacements); err != nil {
		rollbackReplacements(replacements)
		return err
	}

	commitReplacements(replacements)
	installed = true

	if !quiet {
		fmt.Fprintf(os.Stderr, "\r\x1b[KDownload complete! Installed %s v%s (%s).\n", DisplayName, staged.Version, staged.TargetKey)
	}
	return nil
}

// WriteStagedManifest records staged.json inside a PID staging directory so a future run can adopt it if SIGHUP'd.
func (c *Config) WriteStagedManifest(staged *StagedBinary) {
	stagingDir := filepath.Dir(staged.TempBinaryPath)
	manifestPath := filepath.Join(stagingDir, StagedManifestName)
	tempManifest := manifestPath + ".tmp"

	binHash, err := ComputeFileSha256(staged.TempBinaryPath)
	if err != nil {
		return
	}
	files := map[string]string{
		c.BinaryName: binHash,
	}
	wasmPath := filepath.Join(stagingDir, "tree-sitter.wasm")
	if _, err := os.Stat(wasmPath); err == nil {
		if wasmHash, err := ComputeFileSha256(wasmPath); err == nil {
			files["tree-sitter.wasm"] = wasmHash
		}
	}

	b, err := json.Marshal(stagedManifest{
		Version: staged.Version,
		Target:  staged.TargetKey,
		Files:   files,
	})
	if err != nil {
		return
	}
	if err := os.WriteFile(tempManifest, b, 0o644); err == nil {
		_ = os.Rename(tempManifest, manifestPath)
	} else {
		_ = os.Remove(tempManifest)
	}
}

func isProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func parseStagingDirOwner(name string) (int, bool) {
	prefix := fmt.Sprintf(".%s-download-temp-", PackageName)
	if !strings.HasPrefix(name, prefix) {
		return 0, false
	}
	rest := strings.TrimPrefix(name, prefix)
	rest = strings.TrimSuffix(rest, ".adopting")
	pid, err := strconv.Atoi(rest)
	if err != nil || pid <= 0 {
		return 0, false
	}
	return pid, true
}

func (c *Config) readStagedManifest(stagingDir string) *stagedManifest {
	data, err := os.ReadFile(filepath.Join(stagingDir, StagedManifestName))
	if err != nil {
		return nil
	}
	var m stagedManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	if !IsStrictReleaseVersion(m.Version) || PlatformTargets[m.Target] == "" || len(m.Files) == 0 {
		return nil
	}
	if !IsSha256Hex(m.Files[c.BinaryName]) {
		return nil
	}
	for name, digest := range m.Files {
		if _, ok := c.IsAllowedArchiveEntry(name, tar.TypeReg); !ok || !IsSha256Hex(digest) {
			return nil
		}
	}
	return &m
}

func (c *Config) stagedFilesMatch(stagingDir string, files map[string]string) bool {
	for name, digest := range files {
		fp := filepath.Join(stagingDir, name)
		if err := VerifyFileSha256(fp, digest); err != nil {
			return false
		}
	}
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		n := e.Name()
		if n == StagedManifestName {
			continue
		}
		if _, ok := files[n]; !ok {
			return false
		}
	}
	return true
}

// AdoptOrphanedStagedUpdates adopts any verified update staged by a dead launcher process.
func (c *Config) AdoptOrphanedStagedUpdates() string {
	entries, err := os.ReadDir(c.ConfigDir)
	if err != nil {
		return ""
	}
	installedVersion := c.GetCurrentVersion()
	var adoptedVersion string

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		owner, ok := parseStagingDirOwner(e.Name())
		if !ok {
			continue
		}
		if owner != os.Getpid() && isProcessAlive(owner) {
			continue
		}
		dir := filepath.Join(c.ConfigDir, e.Name())
		m := c.readStagedManifest(dir)
		if adoptedVersion == "" && m != nil &&
			(installedVersion == "" || CompareVersions(installedVersion, m.Version) < 0) &&
			c.IsTargetAllowedForThisMachine(m.Target) {
			claimedDir := c.TempDownloadDir + ".adopting"
			if err := os.Rename(dir, claimedDir); err != nil {
				continue
			}
			if c.stagedFilesMatch(claimedDir, m.Files) {
				err := c.InstallStagedBinary(&StagedBinary{
					TempBinaryPath: filepath.Join(claimedDir, c.BinaryName),
					Version:        m.Version,
					TargetKey:      m.Target,
				}, true, true)
				if err == nil {
					adoptedVersion = m.Version
				}
			} else {
				_ = os.RemoveAll(claimedDir)
			}
			continue
		}
		_ = os.RemoveAll(dir)
	}
	return adoptedVersion
}

// EnsureBinaryReady ensures that ~/.config/manicode/freebuff exists and is ready to run.
func (c *Config) EnsureBinaryReady(forceUpdate bool) error {
	currentVersion := c.GetCurrentVersion()
	if currentVersion != "" && !forceUpdate {
		return nil
	}

	rel, err := c.FetchRegistryRelease("latest", RequestTimeout)
	if err != nil || rel == nil {
		if currentVersion != "" {
			fmt.Fprintf(os.Stderr, "Warning: could not fetch latest release (%v); continuing with cached %s v%s.\n", err, PackageName, currentVersion)
			return nil
		}
		return fmt.Errorf("failed to determine latest %s version from npm registry: %w", PackageName, err)
	}

	if forceUpdate && currentVersion == rel.Version {
		fmt.Printf("%s is already at the latest version (v%s).\n", PackageName, currentVersion)
		return nil
	}

	if forceUpdate && currentVersion != "" {
		fmt.Printf("Updating %s from v%s to v%s...\n", PackageName, currentVersion, rel.Version)
	}

	staged, err := c.StageBinary(rel.Version, c.GetDownloadTargetKey(), rel.BinaryChecksums, false)
	if err != nil {
		if currentVersion != "" && !forceUpdate {
			fmt.Fprintf(os.Stderr, "Warning: %v\nContinuing with cached %s v%s.\n", err, PackageName, currentVersion)
			return nil
		}
		return err
	}

	return c.InstallStagedBinary(staged, false, false)
}
