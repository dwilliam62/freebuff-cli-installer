package launcher

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	PackageName           = "freebuff"
	DisplayName           = "Freebuff"
	LauncherVersion       = "v1.0.0"
	DefaultDownloadOrigin = "https://codebuff.com"
	NpmRegistryOrigin     = "https://registry.npmjs.org"
	DefaultInstallPath    = "/usr/local/bin/freebuff-cli"
	DefaultCacheTTL       = 6 * time.Hour
	RequestTimeout        = 20 * time.Second
	DownloadTimeout       = 120 * time.Second
	DownloadMaxAttempts   = 3
	StartupCrashWindow    = 10 * time.Second
	StagedManifestName    = "staged.json"
)

// PlatformTargets maps target keys to upstream release tarball names.
var PlatformTargets = map[string]string{
	"linux-x64":           "freebuff-linux-x64.tar.gz",
	"linux-x64-baseline":  "freebuff-linux-x64-baseline.tar.gz",
	"linux-arm64":         "freebuff-linux-arm64.tar.gz",
	"darwin-x64":          "freebuff-darwin-x64.tar.gz",
	"darwin-x64-baseline": "freebuff-darwin-x64-baseline.tar.gz",
	"darwin-arm64":        "freebuff-darwin-arm64.tar.gz",
	"win32-x64":           "freebuff-win32-x64.tar.gz",
	"win32-x64-baseline":  "freebuff-win32-x64-baseline.tar.gz",
}

// BaselineFallbackTargets maps primary targets to their non-AVX2 baseline targets.
var BaselineFallbackTargets = map[string]string{
	"linux-x64":  "linux-x64-baseline",
	"darwin-x64": "darwin-x64-baseline",
	"win32-x64":  "win32-x64-baseline",
}

// Config holds resolved runtime paths and environment overrides.
type Config struct {
	HomeDir          string
	ConfigDir        string
	CacheDir         string
	ApplicationsDir  string
	DesktopEntryPath string
	BinaryName       string
	BinaryPath       string
	WasmPath         string
	MetadataPath     string
	CPUFeaturesPath  string
	TempDownloadDir  string
	CacheFile        string
	NotifiedFile     string
	InstallPath      string
	DownloadOrigin   string
	CacheTTL         time.Duration
	UserAgent        string
	DryRun           bool
	Force            bool
}

// NewConfig initializes a Config using current user directories and environment variables.
func NewConfig(dryRun, force bool) (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = os.Getenv("HOME")
	}
	if homeDir == "" {
		return nil, fmt.Errorf("cannot determine user home directory")
	}

	xdgConfig := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfig == "" {
		xdgConfig = filepath.Join(homeDir, ".config")
	}
	configDir := os.Getenv("FREEBUFF_CONFIG_DIR")
	if configDir == "" {
		configDir = filepath.Join(xdgConfig, "manicode")
	}

	xdgCache := os.Getenv("XDG_CACHE_HOME")
	if xdgCache == "" {
		xdgCache = filepath.Join(homeDir, ".cache")
	}
	cacheDir := filepath.Join(xdgCache, "freebuff-cli")

	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		xdgData = filepath.Join(homeDir, ".local", "share")
	}
	appsDir := filepath.Join(xdgData, "applications")

	binaryName := PackageName
	if runtime.GOOS == "windows" {
		binaryName = PackageName + ".exe"
	}

	installPath := os.Getenv("FREEBUFF_INSTALL_PATH")
	if installPath == "" {
		installPath = DefaultInstallPath
	}

	cacheTTL := DefaultCacheTTL
	if ttlStr := strings.TrimSpace(os.Getenv("FREEBUFF_CLI_CACHE_TTL")); ttlStr != "" {
		if secs, err := strconv.ParseInt(ttlStr, 10, 64); err == nil && secs >= 0 {
			cacheTTL = time.Duration(secs) * time.Second
		}
	}

	origin := ResolveDownloadOrigin(os.Getenv("NEXT_PUBLIC_CODEBUFF_APP_URL"), func(msg string) {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
	})

	return &Config{
		HomeDir:          homeDir,
		ConfigDir:        configDir,
		CacheDir:         cacheDir,
		ApplicationsDir:  appsDir,
		DesktopEntryPath: filepath.Join(appsDir, "freebuff-cli.desktop"),
		BinaryName:       binaryName,
		BinaryPath:       filepath.Join(configDir, binaryName),
		WasmPath:         filepath.Join(configDir, "tree-sitter.wasm"),
		MetadataPath:     filepath.Join(configDir, PackageName+"-metadata.json"),
		CPUFeaturesPath:  filepath.Join(configDir, "cpu-features.json"),
		TempDownloadDir:  filepath.Join(configDir, fmt.Sprintf(".%s-download-temp-%d", PackageName, os.Getpid())),
		CacheFile:        filepath.Join(cacheDir, "npm-latest.json"),
		NotifiedFile:     filepath.Join(cacheDir, "notified.json"),
		InstallPath:      installPath,
		DownloadOrigin:   origin,
		CacheTTL:         cacheTTL,
		UserAgent:        PackageName + "-cli/" + LauncherVersion,
		DryRun:           dryRun,
		Force:            force,
	}, nil
}

// ResolveDownloadOrigin validates NEXT_PUBLIC_CODEBUFF_APP_URL against upstream rules.
func ResolveDownloadOrigin(configured string, warn func(string)) string {
	trimmed := strings.TrimRight(strings.TrimSpace(configured), "/")
	if trimmed == "" {
		return DefaultDownloadOrigin
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if warn != nil {
			warn(fmt.Sprintf("Ignoring NEXT_PUBLIC_CODEBUFF_APP_URL=%s: not a valid URL. Downloading from %s.", configured, DefaultDownloadOrigin))
		}
		return DefaultDownloadOrigin
	}

	host := strings.ToLower(u.Hostname())
	isLocal := host == "localhost" || host == "127.0.0.1" || host == "::1"
	isHTTP := u.Scheme == "http" || u.Scheme == "https"
	isBare := u.User == nil && u.RawQuery == "" && u.Fragment == ""

	if isHTTP && isLocal && isBare {
		return trimmed
	}
	if u.Scheme == "https" && isReleaseOriginHost(host) && isBare {
		return trimmed
	}

	if warn != nil {
		if u.Scheme == "https" {
			warn(fmt.Sprintf("Ignoring NEXT_PUBLIC_CODEBUFF_APP_URL=%s://%s: release downloads come only from codebuff.com or freebuff.com (or localhost). Downloading from %s.", u.Scheme, u.Host, DefaultDownloadOrigin))
		} else {
			warn(fmt.Sprintf("Ignoring NEXT_PUBLIC_CODEBUFF_APP_URL=%s://%s: release downloads must use https (http is allowed only for localhost). Downloading from %s.", u.Scheme, u.Host, DefaultDownloadOrigin))
		}
	}
	return DefaultDownloadOrigin
}

func isReleaseOriginHost(host string) bool {
	host = strings.ToLower(host)
	for _, domain := range []string{"codebuff.com", "freebuff.com"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
