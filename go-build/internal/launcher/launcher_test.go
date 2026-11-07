package launcher

import (
	"archive/tar"
	"testing"
)

func TestIsStrictReleaseVersion(t *testing.T) {
	valid := []string{"0.2.36", "1.0.0", "0.0.1-alpha.1", "2.10.3-rc-1"}
	for _, v := range valid {
		if !IsStrictReleaseVersion(v) {
			t.Errorf("expected %q to be valid strict release version", v)
		}
	}

	invalid := []string{"", "v0.2.36", "1.2", "1.2.3+build1", "../0.2.36", "01.2.3", "0.2.36/../../etc/passwd"}
	for _, v := range invalid {
		if IsStrictReleaseVersion(v) {
			t.Errorf("expected %q to be rejected by IsStrictReleaseVersion", v)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		v1   string
		v2   string
		want int
	}{
		{"0.2.36", "0.2.36", 0},
		{"0.2.37", "0.2.36", 1},
		{"0.2.35", "0.2.36", -1},
		{"1.0.0", "1.0.0-alpha.1", 1},
		{"1.0.0-alpha.1", "1.0.0-alpha.2", -1},
	}
	for _, tc := range cases {
		got := CompareVersions(tc.v1, tc.v2)
		if got != tc.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.v1, tc.v2, got, tc.want)
		}
	}
}

func TestEvaluateRedirect(t *testing.T) {
	u, err := EvaluateRedirect(
		"https://codebuff.com/api/releases/download/0.2.36/freebuff-linux-x64.tar.gz",
		"https://objects.githubusercontent.com/github-production-release-asset/12345",
		"https://codebuff.com/api/releases/download/0.2.36/freebuff-linux-x64.tar.gz",
	)
	if err != nil || u == "" {
		t.Fatalf("expected githubusercontent redirect to be allowed, got err=%v", err)
	}

	_, err = EvaluateRedirect(
		"https://codebuff.com/api/releases/download/0.2.36/freebuff-linux-x64.tar.gz",
		"https://evil.example.com/freebuff-linux-x64.tar.gz",
		"https://codebuff.com/api/releases/download/0.2.36/freebuff-linux-x64.tar.gz",
	)
	if err == nil {
		t.Fatalf("expected untrusted redirect host to be refused")
	}

	_, err = EvaluateRedirect(
		"https://codebuff.com/api/releases/download/0.2.36/freebuff-linux-x64.tar.gz",
		"http://codebuff.com/insecure.tar.gz",
		"https://codebuff.com/api/releases/download/0.2.36/freebuff-linux-x64.tar.gz",
	)
	if err == nil {
		t.Fatalf("expected https->http downgrade redirect to be refused")
	}
}

func TestParseCpuinfoAvx2(t *testing.T) {
	withAvx2 := "processor\t: 0\nflags\t\t: fpu vme de pse tsc msr pae mce cx8 apic sep mtrr pge mca cmov pat pse36 clflush mmx fxsr sse sse2 ht syscall nx rdtscp lm constant_tsc rep_good nopl xtopology nonstop_tsc cpuid extd_apicid aperfmperf rapl pni pclmulqdq monitor ssse3 fma cx16 sse4_1 sse4_2 movbe popcnt aes xsave avx f16c rdrand lahf_lm cmp_legacy svm extapic cr8_legacy abm sse4a misalignsse 3dnowprefetch osvw ibs xop skinit wdt lwp fma4 tce nodeid_msr tbm topoext perfctr_core perfctr_nb bpext ptsc mwaitx cpb hw_pstate ssbd vmmcall fsgsbase bmi1 avx2 smep bmi2\n"
	res := ParseCpuinfoAvx2(withAvx2)
	if res == nil || !*res {
		t.Fatalf("expected ParseCpuinfoAvx2 to return true")
	}

	withoutAvx2 := "processor\t: 0\nflags\t\t: fpu sse sse2 avx\n"
	res2 := ParseCpuinfoAvx2(withoutAvx2)
	if res2 == nil || *res2 {
		t.Fatalf("expected ParseCpuinfoAvx2 to return false")
	}
}

func TestIsAllowedArchiveEntry(t *testing.T) {
	cfg := &Config{BinaryName: "freebuff"}
	if name, ok := cfg.IsAllowedArchiveEntry("./freebuff", tar.TypeReg); !ok || name != "freebuff" {
		t.Errorf("expected ./freebuff to be allowed, got %q %v", name, ok)
	}
	if name, ok := cfg.IsAllowedArchiveEntry("tree-sitter.wasm", tar.TypeReg); !ok || name != "tree-sitter.wasm" {
		t.Errorf("expected tree-sitter.wasm to be allowed, got %q %v", name, ok)
	}
	if _, ok := cfg.IsAllowedArchiveEntry("../freebuff", tar.TypeReg); ok {
		t.Errorf("expected path traversal entry to be rejected")
	}
	if _, ok := cfg.IsAllowedArchiveEntry("freebuff", tar.TypeSymlink); ok {
		t.Errorf("expected symlink entry to be rejected")
	}
}
