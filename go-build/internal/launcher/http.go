package launcher

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var redirectAllowedHosts = map[string]bool{
	"codebuff.com":     true,
	"www.codebuff.com": true,
	"freebuff.com":     true,
	"www.freebuff.com": true,
	"github.com":       true,
}

var redirectAllowedSuffixes = []string{
	".githubusercontent.com",
}

// IsRedirectHostAllowed checks whether a redirect target hostname is permitted.
func IsRedirectHostAllowed(targetHost, originalHost string) bool {
	host := strings.ToLower(targetHost)
	if host == strings.ToLower(originalHost) {
		return true
	}
	if redirectAllowedHosts[host] {
		return true
	}
	for _, suffix := range redirectAllowedSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// EvaluateRedirect checks whether redirecting from currentURL to location is allowed.
func EvaluateRedirect(currentURL, location, originalURL string) (string, error) {
	from, err := url.Parse(currentURL)
	if err != nil {
		return "", fmt.Errorf("invalid source URL: %w", err)
	}
	locURL, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("Redirect to an invalid URL: %s", location)
	}
	target := from.ResolveReference(locURL)
	if target.Scheme != "http" && target.Scheme != "https" {
		return "", fmt.Errorf("Redirect to unsupported protocol: %s:", target.Scheme)
	}
	if from.Scheme == "https" && target.Scheme == "http" {
		return "", fmt.Errorf("Refusing redirect from https to insecure http host %s", target.Hostname())
	}

	origHost := from.Hostname()
	if originalURL != "" {
		if origParsed, err := url.Parse(originalURL); err == nil && origParsed.Hostname() != "" {
			origHost = origParsed.Hostname()
		}
	}
	if !IsRedirectHostAllowed(target.Hostname(), origHost) {
		return "", fmt.Errorf("Refusing redirect to unexpected host %s", target.Hostname())
	}
	return target.String(), nil
}

// NewHTTPClient creates an HTTP client that respects proxy environment variables
// and enforces upstream redirect host security rules.
func NewHTTPClient(timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyFromEnvironment

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("Too many redirects")
			}
			origURL := via[0].URL.String()
			prevURL := via[len(via)-1].URL.String()
			_, err := EvaluateRedirect(prevURL, req.URL.String(), origURL)
			return err
		},
	}
}

var (
	contentRangeCompleteRe    = regexp.MustCompile(`(?i)^bytes (\d+)-(\d+)/(\d+)$`)
	contentRangeUnsatisfiedRe = regexp.MustCompile(`(?i)^bytes \*/(\d+)$`)
)

type contentRangeInfo struct {
	start       int64
	end         int64
	total       int64
	hasRange    bool
	unsatisfied bool
}

func parseContentRange(header string) *contentRangeInfo {
	if header == "" {
		return nil
	}
	if m := contentRangeCompleteRe.FindStringSubmatch(header); m != nil {
		start, _ := strconv.ParseInt(m[1], 10, 64)
		end, _ := strconv.ParseInt(m[2], 10, 64)
		total, _ := strconv.ParseInt(m[3], 10, 64)
		if start > end || end >= total {
			return nil
		}
		return &contentRangeInfo{start: start, end: end, total: total, hasRange: true}
	}
	if m := contentRangeUnsatisfiedRe.FindStringSubmatch(header); m != nil {
		total, _ := strconv.ParseInt(m[1], 10, 64)
		return &contentRangeInfo{total: total, unsatisfied: true}
	}
	return nil
}

// ProgressFunc receives download progress updates.
type ProgressFunc func(downloadedBytes, totalBytes int64)

// DownloadFileWithResume downloads url to destPath, supporting HTTP Range resume
// when a partial file already exists on disk.
func DownloadFileWithResume(urlStr, destPath, userAgent string, timeout time.Duration, onProgress ProgressFunc) (int64, error) {
	var resumedFrom int64
	if st, err := os.Stat(destPath); err == nil {
		resumedFrom = st.Size()
	}

	client := NewHTTPClient(timeout)
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return 0, err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if resumedFrom > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumedFrom))
	}

	resp, err := client.Do(req)
	if err != nil {
		return resumedFrom, err
	}
	defer resp.Body.Close()

	cr := parseContentRange(resp.Header.Get("Content-Range"))

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		if cr != nil && cr.unsatisfied && cr.total == resumedFrom && resumedFrom > 0 {
			return resumedFrom, nil
		}
		_ = os.Remove(destPath)
		return 0, fmt.Errorf("saved partial download is no longer valid (HTTP 416)")
	}

	var flag int
	var totalBytes int64
	var expectedResponseBytes int64 = -1

	switch resp.StatusCode {
	case http.StatusPartialContent:
		if cr == nil || !cr.hasRange || cr.start != resumedFrom {
			_ = os.Remove(destPath)
			return 0, fmt.Errorf("download resume mismatch: expected byte %d", resumedFrom)
		}
		totalBytes = cr.total
		expectedResponseBytes = cr.end - cr.start + 1
		flag = os.O_CREATE | os.O_WRONLY | os.O_APPEND
	case http.StatusOK:
		resumedFrom = 0
		if cl, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err == nil && cl > 0 {
			totalBytes = cl
		}
		flag = os.O_CREATE | os.O_WRONLY | os.O_TRUNC
	default:
		return resumedFrom, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	f, err := os.OpenFile(destPath, flag, 0o644)
	if err != nil {
		return resumedFrom, err
	}
	defer f.Close()

	downloaded := resumedFrom
	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return downloaded, writeErr
			}
			downloaded += int64(n)
			if onProgress != nil {
				onProgress(downloaded, totalBytes)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return downloaded, readErr
		}
	}

	responseBytes := downloaded - resumedFrom
	if expectedResponseBytes >= 0 && responseBytes != expectedResponseBytes {
		if responseBytes > expectedResponseBytes {
			_ = os.Remove(destPath)
		}
		return downloaded, fmt.Errorf("download incomplete: response contained %d of %d bytes", responseBytes, expectedResponseBytes)
	}
	if totalBytes > 0 && downloaded != totalBytes {
		return downloaded, fmt.Errorf("download incomplete: received %d of %d bytes", downloaded, totalBytes)
	}

	return downloaded, nil
}
