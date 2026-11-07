package launcher

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	strictReleaseVersionRe = regexp.MustCompile(`^(0|[1-9]\d{0,8})\.(0|[1-9]\d{0,8})\.(0|[1-9]\d{0,8})(?:-(?:0|[1-9]\d{0,8}|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d{0,8}|\d*[A-Za-z-][0-9A-Za-z-]*))*)?$`)
	sha256HexRe            = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
	digitsOnlyRe           = regexp.MustCompile(`^\d+$`)
	leadingZeroDigitRe     = regexp.MustCompile(`^0\d+$`)
)

// IsStrictReleaseVersion validates that version is a safe, strict semver string
// suitable for URLs and file paths (matching upstream launcher.js).
func IsStrictReleaseVersion(v string) bool {
	return len(v) > 0 && len(v) <= 64 && strictReleaseVersionRe.MatchString(v)
}

// IsSha256Hex checks if s is a valid 64-character hex string.
func IsSha256Hex(s string) bool {
	return sha256HexRe.MatchString(s)
}

type parsedSemver struct {
	major      int64
	minor      int64
	patch      int64
	prerelease []string
}

func parseSemver(v string) *parsedSemver {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if plusIdx := strings.IndexByte(v, '+'); plusIdx != -1 {
		v = v[:plusIdx]
	}
	var prerelease []string
	if dashIdx := strings.IndexByte(v, '-'); dashIdx != -1 {
		preStr := v[dashIdx+1:]
		v = v[:dashIdx]
		if preStr == "" {
			return nil
		}
		prerelease = strings.Split(preStr, ".")
		for _, part := range prerelease {
			if part == "" || leadingZeroDigitRe.MatchString(part) {
				return nil
			}
		}
	}

	core := strings.Split(v, ".")
	if len(core) != 3 {
		return nil
	}
	nums := make([]int64, 3)
	for i, part := range core {
		if part == "" || leadingZeroDigitRe.MatchString(part) {
			return nil
		}
		n, err := strconv.ParseInt(part, 10, 64)
		if err != nil || n < 0 {
			return nil
		}
		nums[i] = n
	}

	return &parsedSemver{
		major:      nums[0],
		minor:      nums[1],
		patch:      nums[2],
		prerelease: prerelease,
	}
}

// CompareVersions compares v1 and v2 according to SemVer 2.0 rules.
// Returns -1 if v1 < v2, 0 if v1 == v2, and 1 if v1 > v2.
func CompareVersions(v1, v2 string) int {
	p1 := parseSemver(v1)
	p2 := parseSemver(v2)
	if p1 == nil && p2 == nil {
		return 0
	}
	if p1 == nil {
		return -1
	}
	if p2 == nil {
		return 1
	}

	if p1.major != p2.major {
		if p1.major < p2.major {
			return -1
		}
		return 1
	}
	if p1.minor != p2.minor {
		if p1.minor < p2.minor {
			return -1
		}
		return 1
	}
	if p1.patch != p2.patch {
		if p1.patch < p2.patch {
			return -1
		}
		return 1
	}

	if len(p1.prerelease) == 0 && len(p2.prerelease) == 0 {
		return 0
	}
	if len(p1.prerelease) == 0 {
		return 1
	}
	if len(p2.prerelease) == 0 {
		return -1
	}

	maxLen := len(p1.prerelease)
	if len(p2.prerelease) > maxLen {
		maxLen = len(p2.prerelease)
	}
	for i := 0; i < maxLen; i++ {
		if i >= len(p1.prerelease) {
			return -1
		}
		if i >= len(p2.prerelease) {
			return 1
		}
		id1 := p1.prerelease[i]
		id2 := p2.prerelease[i]
		if id1 == id2 {
			continue
		}
		num1 := digitsOnlyRe.MatchString(id1)
		num2 := digitsOnlyRe.MatchString(id2)
		if num1 && num2 {
			n1, _ := strconv.ParseInt(id1, 10, 64)
			n2, _ := strconv.ParseInt(id2, 10, 64)
			if n1 < n2 {
				return -1
			}
			return 1
		}
		if num1 != num2 {
			if num1 {
				return -1
			}
			return 1
		}
		if id1 < id2 {
			return -1
		}
		return 1
	}
	return 0
}

// IsNewer returns true if candidate > installed.
func IsNewer(candidate, installed string) bool {
	return CompareVersions(candidate, installed) > 0
}
