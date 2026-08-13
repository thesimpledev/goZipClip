package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var ptsTimePattern = regexp.MustCompile(`pts_time:([0-9]+(?:\.[0-9]+)?)`)

// DetectCut scans the opening of the VOD for the first frame whose
// scene-change score exceeds the configured threshold and returns
// that timestamp minus the cut backoff, in seconds.
func DetectCut(ctx context.Context, cfg Config, vodPath string) (float64, error) {
	if vodPath == "" {
		return 0, errors.New("vod path is empty")
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, resolveFfmpeg(cfg), sceneScanArgs(cfg, vodPath)...)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return 0, fmt.Errorf("ffmpeg scene scan: %w: %s", runErr, truncate(string(out), 300))
	}
	ts, found := firstPtsTime(bytes.NewReader(out))
	if !found {
		return 0, fmt.Errorf("no scene change above %.2f in the first %d minutes",
			cfg.SceneThreshold, cfg.ScanWindowMinutes)
	}
	cut := ts - float64(cfg.CutBackoffSeconds)
	if cut < 0 {
		cut = 0
	}
	return cut, nil
}

func sceneScanArgs(cfg Config, vodPath string) []string {
	filter := fmt.Sprintf("scale=320:-1,select='gt(scene,%s)',showinfo",
		strconv.FormatFloat(cfg.SceneThreshold, 'f', -1, 64))
	return []string{
		"-hide_banner", "-nostats",
		"-i", vodPath,
		"-t", strconv.Itoa(cfg.ScanWindowMinutes * 60),
		"-vf", filter,
		"-f", "null", "-",
	}
}

// firstPtsTime scans ffmpeg showinfo output for the first frame
// timestamp and reports whether one was found.
func firstPtsTime(r io.Reader) (float64, bool) {
	if r == nil {
		return 0, false
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "Parsed_showinfo") {
			continue
		}
		match := ptsTimePattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		value, parseErr := strconv.ParseFloat(match[1], 64)
		if parseErr != nil {
			continue
		}
		return value, true
	}
	return 0, false
}

// ExtractPreview writes a single frame at the given timestamp to
// outPath as a JPEG.
func ExtractPreview(ctx context.Context, cfg Config, vodPath string, at float64, outPath string) error {
	if vodPath == "" || outPath == "" {
		return errors.New("preview paths missing")
	}
	args := []string{
		"-hide_banner", "-nostats", "-y",
		"-ss", strconv.FormatFloat(at, 'f', 2, 64),
		"-i", vodPath,
		"-frames:v", "1",
		"-q:v", "3",
		outPath,
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, resolveFfmpeg(cfg), args...)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("ffmpeg preview: %w: %s", runErr, truncate(string(out), 300))
	}
	return nil
}

// parseTimestamp accepts "SS", "MM:SS", or "HH:MM:SS" (seconds may
// have a fraction) and returns total seconds.
func parseTimestamp(s string) (float64, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, errors.New("timestamp is empty")
	}
	parts := strings.Split(trimmed, ":")
	if len(parts) > 3 {
		return 0, fmt.Errorf("bad timestamp %q", s)
	}
	total := 0.0
	for _, part := range parts {
		value, parseErr := strconv.ParseFloat(part, 64)
		if parseErr != nil || value < 0 {
			return 0, fmt.Errorf("bad timestamp %q", s)
		}
		total = total*60 + value
	}
	return total, nil
}

// formatTimestamp renders seconds as HH:MM:SS.s for display.
func formatTimestamp(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	rest := seconds - float64(hours*3600+minutes*60)
	return fmt.Sprintf("%02d:%02d:%04.1f", hours, minutes, rest)
}
