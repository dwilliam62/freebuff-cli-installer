package launcher

import (
	"encoding/json"
	"os"
	"regexp"
	"runtime"
	"strings"
)

// CPUFeatureRecord represents ~/.config/manicode/cpu-features.json.
type CPUFeatureRecord struct {
	Avx2     bool   `json:"avx2"`
	Evidence string `json:"evidence,omitempty"`
}

var (
	cpuFlagsLineRe = regexp.MustCompile(`(?m)^flags\s*:`)
	cpuAvx2FlagRe  = regexp.MustCompile(`(?m)^flags\s*:.*\bavx2\b`)
)

// GetPlatformKey maps Go's runtime.GOOS and runtime.GOARCH to upstream npm platform keys.
func GetPlatformKey() string {
	osName := runtime.GOOS
	if osName == "windows" {
		osName = "win32"
	}
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x64"
	case "arm64":
		arch = "arm64"
	}
	return osName + "-" + arch
}

// GetTargetOverride checks FREEBUFF_BINARY_TARGET, CODEBUFF_BINARY_TARGET, and CLI_BINARY_TARGET.
func GetTargetOverride() string {
	for _, env := range []string{"FREEBUFF_BINARY_TARGET", "CODEBUFF_BINARY_TARGET", "CLI_BINARY_TARGET"} {
		val := strings.TrimSpace(os.Getenv(env))
		if val != "" && PlatformTargets[val] != "" {
			return val
		}
	}
	return ""
}

// ParseCpuinfoAvx2 checks /proc/cpuinfo text for the avx2 flag.
// Returns nil if flags lines are not present, or a pointer to true/false.
func ParseCpuinfoAvx2(text string) *bool {
	if !cpuFlagsLineRe.MatchString(text) {
		return nil
	}
	has := cpuAvx2FlagRe.MatchString(text)
	return &has
}

// ProbeCPUAvx2 inspects the OS for AVX2 support.
func ProbeCPUAvx2() *bool {
	if runtime.GOARCH != "amd64" {
		return nil
	}
	if runtime.GOOS == "linux" {
		data, err := os.ReadFile("/proc/cpuinfo")
		if err != nil {
			return nil
		}
		return ParseCpuinfoAvx2(string(data))
	}
	return nil
}

// ReadCPUFeatureRecord reads ~/.config/manicode/cpu-features.json.
func (c *Config) ReadCPUFeatureRecord() *CPUFeatureRecord {
	data, err := os.ReadFile(c.CPUFeaturesPath)
	if err != nil {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	avxVal, ok := raw["avx2"].(bool)
	if !ok {
		return nil
	}
	ev, _ := raw["evidence"].(string)
	return &CPUFeatureRecord{
		Avx2:     avxVal,
		Evidence: ev,
	}
}

// WriteCPUFeatureRecord persists ~/.config/manicode/cpu-features.json.
func (c *Config) WriteCPUFeatureRecord(avx2 bool, evidence string) {
	_ = os.MkdirAll(c.ConfigDir, 0o755)
	b, err := json.Marshal(CPUFeatureRecord{Avx2: avx2, Evidence: evidence})
	if err == nil {
		_ = os.WriteFile(c.CPUFeaturesPath, b, 0o644)
	}
}

// ClearCPUFeatureRecord removes ~/.config/manicode/cpu-features.json.
func (c *Config) ClearCPUFeatureRecord() {
	_ = os.Remove(c.CPUFeaturesPath)
}

// MachineHasAvx2 determines whether this machine supports AVX2.
func (c *Config) MachineHasAvx2() bool {
	if runtime.GOARCH != "amd64" {
		return true
	}
	rec := c.ReadCPUFeatureRecord()
	if rec != nil && (rec.Avx2 || rec.Evidence != "") {
		return rec.Avx2
	}
	if rec != nil {
		probed := ProbeCPUAvx2()
		if probed == nil || !*probed {
			ev := "crash"
			if probed != nil && !*probed {
				ev = "cpuinfo"
			}
			c.WriteCPUFeatureRecord(false, ev)
			return false
		}
		c.ClearCPUFeatureRecord()
	}
	if runtime.GOOS == "linux" {
		if probed := ProbeCPUAvx2(); probed != nil {
			return *probed
		}
	}
	return true
}

// KnownToHaveAvx2 returns true when positive evidence confirms AVX2 support.
func (c *Config) KnownToHaveAvx2() bool {
	if !c.MachineHasAvx2() {
		return false
	}
	rec := c.ReadCPUFeatureRecord()
	if rec != nil && rec.Avx2 && rec.Evidence != "" {
		return true
	}
	probed := ProbeCPUAvx2()
	return probed != nil && *probed
}

// DescribeAvx2Support returns a human-readable summary of AVX2 status.
func (c *Config) DescribeAvx2Support() string {
	rec := c.ReadCPUFeatureRecord()
	if rec != nil && !rec.Avx2 {
		return "no (recorded crash)"
	}
	if runtime.GOOS == "linux" {
		if c.MachineHasAvx2() {
			return "yes"
		}
		return "no"
	}
	return "not checked (assumed present)"
}

// GetBaselineFallbackTargetKey returns the baseline target for this platform, or empty string.
func GetBaselineFallbackTargetKey() string {
	return BaselineFallbackTargets[GetPlatformKey()]
}

// GetDefaultTargetKey returns the default target key for this machine.
func (c *Config) GetDefaultTargetKey() string {
	if override := GetTargetOverride(); override != "" {
		return override
	}
	pk := GetPlatformKey()
	if fallback := BaselineFallbackTargets[pk]; fallback != "" && !c.MachineHasAvx2() {
		return fallback
	}
	return pk
}

// IsTargetAllowedForThisMachine checks if an installed target is valid for this machine.
func (c *Config) IsTargetAllowedForThisMachine(target string) bool {
	if override := GetTargetOverride(); override != "" {
		return target == override
	}
	return target == GetBaselineFallbackTargetKey() || target == c.GetDefaultTargetKey()
}

// GetDownloadTargetKey determines which target key to download next.
func (c *Config) GetDownloadTargetKey() string {
	if override := GetTargetOverride(); override != "" {
		return override
	}
	meta := c.ReadInstalledMetadata()
	if meta != nil && meta.Target != "" && c.IsTargetAllowedForThisMachine(meta.Target) {
		if meta.Target == GetBaselineFallbackTargetKey() && c.KnownToHaveAvx2() {
			return c.GetDefaultTargetKey()
		}
		return meta.Target
	}
	return c.GetDefaultTargetKey()
}
