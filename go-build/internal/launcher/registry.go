package launcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RegistryRelease represents the upstream npm package document for a specific release.
type RegistryRelease struct {
	Version         string            `json:"version"`
	BinaryChecksums map[string]string `json:"binaryChecksums"`
}

// InstalledMetadata represents ~/.config/manicode/freebuff-metadata.json.
type InstalledMetadata struct {
	Version string `json:"version"`
	Target  string `json:"target"`
}

type cachedRegistryEntry struct {
	Latest    string                  `json:"latest"`
	CheckedAt int64                   `json:"checkedAt"`
	Checksums map[string]string       `json:"binaryChecksums,omitempty"`
}

type notifiedEntry struct {
	Latest string `json:"latest"`
}

// ReadInstalledMetadata reads ~/.config/manicode/freebuff-metadata.json.
func (c *Config) ReadInstalledMetadata() *InstalledMetadata {
	data, err := os.ReadFile(c.MetadataPath)
	if err != nil {
		return nil
	}
	var meta InstalledMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	if !IsStrictReleaseVersion(meta.Version) {
		return nil
	}
	return &meta
}

// GetCurrentVersion returns the installed binary version if both metadata and binary exist
// and the target is compatible with this machine.
func (c *Config) GetCurrentVersion() string {
	meta := c.ReadInstalledMetadata()
	if meta == nil {
		return ""
	}
	st, err := os.Stat(c.BinaryPath)
	if err != nil || !st.Mode().IsRegular() {
		return ""
	}
	target := meta.Target
	if target == "" {
		target = GetPlatformKey()
	}
	if !c.IsTargetAllowedForThisMachine(target) {
		return ""
	}
	return meta.Version
}

// FetchRegistryRelease queries https://registry.npmjs.org/freebuff/<versionOrTag>.
func (c *Config) FetchRegistryRelease(versionOrTag string, timeout time.Duration) (*RegistryRelease, error) {
	if timeout <= 0 {
		timeout = RequestTimeout
	}
	client := NewHTTPClient(timeout)
	endpoint := fmt.Sprintf("%s/%s/%s", NpmRegistryOrigin, PackageName, url.PathEscape(versionOrTag))

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rel RegistryRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	if !IsStrictReleaseVersion(rel.Version) {
		return nil, fmt.Errorf("invalid release version from registry: %q", rel.Version)
	}

	if versionOrTag == "latest" {
		c.WriteRegistryCache(&rel)
	}
	return &rel, nil
}

// ReadRegistryCache reads the cached npm-latest.json if within CacheTTL.
func (c *Config) ReadRegistryCache() *RegistryRelease {
	if c.CacheTTL <= 0 {
		return nil
	}
	data, err := os.ReadFile(c.CacheFile)
	if err != nil {
		return nil
	}
	var entry cachedRegistryEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil
	}
	if !IsStrictReleaseVersion(entry.Latest) || entry.CheckedAt <= 0 {
		return nil
	}
	age := time.Since(time.UnixMilli(entry.CheckedAt))
	if age > c.CacheTTL || age < -time.Hour {
		return nil
	}
	return &RegistryRelease{
		Version:         entry.Latest,
		BinaryChecksums: entry.Checksums,
	}
}

// WriteRegistryCache writes npm-latest.json atomically.
func (c *Config) WriteRegistryCache(rel *RegistryRelease) {
	if c.CacheTTL <= 0 || rel == nil || !IsStrictReleaseVersion(rel.Version) {
		return
	}
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		return
	}
	entry := cachedRegistryEntry{
		Latest:    rel.Version,
		CheckedAt: time.Now().UnixMilli(),
		Checksums: rel.BinaryChecksums,
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return
	}
	tmp := fmt.Sprintf("%s.%d", c.CacheFile, os.Getpid())
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err == nil {
		_ = os.Rename(tmp, c.CacheFile)
	} else {
		_ = os.Remove(tmp)
	}
}

// GetLatestReleaseCached returns the cached latest release or fetches from the registry with a short timeout.
func (c *Config) GetLatestReleaseCached(forceNetwork bool, timeout time.Duration) *RegistryRelease {
	if !forceNetwork {
		if cached := c.ReadRegistryCache(); cached != nil {
			return cached
		}
	}
	rel, err := c.FetchRegistryRelease("latest", timeout)
	if err != nil {
		return nil
	}
	return rel
}

// GetExpectedChecksum resolves the expected SHA-256 hex digest for version and targetKey.
func (c *Config) GetExpectedChecksum(version, targetKey string, provided map[string]string) (string, error) {
	checksums := provided
	if len(checksums) == 0 {
		rel, err := c.FetchRegistryRelease(version, RequestTimeout)
		if err != nil {
			return "", fmt.Errorf("failed to fetch checksums for %s %s: %w", PackageName, version, err)
		}
		checksums = rel.BinaryChecksums
	}
	expected := strings.ToLower(strings.TrimSpace(checksums[targetKey]))
	if !IsSha256Hex(expected) {
		return "", fmt.Errorf("no sha256 checksum is published for %s %s (%s); refusing to install an unverifiable binary", PackageName, version, targetKey)
	}
	return expected, nil
}

// ReadNotifiedVersion reads the last version notified via desktop notification.
func (c *Config) ReadNotifiedVersion() string {
	data, err := os.ReadFile(c.NotifiedFile)
	if err != nil {
		return ""
	}
	var n notifiedEntry
	if err := json.Unmarshal(data, &n); err != nil {
		return ""
	}
	return n.Latest
}

// WriteNotifiedVersion records that a desktop notification was sent for latest.
func (c *Config) WriteNotifiedVersion(latest string) {
	if err := os.MkdirAll(filepath.Dir(c.NotifiedFile), 0o755); err != nil {
		return
	}
	b, err := json.Marshal(notifiedEntry{Latest: latest})
	if err != nil {
		return
	}
	tmp := fmt.Sprintf("%s.%d", c.NotifiedFile, os.Getpid())
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err == nil {
		_ = os.Rename(tmp, c.NotifiedFile)
	}
}

// ClearNotifiedVersion deletes the notified.json marker.
func (c *Config) ClearNotifiedVersion() {
	_ = os.Remove(c.NotifiedFile)
}
