package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// durationTolerance is how far, in seconds, the spliced output's
// duration may differ from intro plus trimmed before verification
// fails.
const durationTolerance = 3.0

// TrimFrom stream-copies vodPath from the cut point onward into
// outPath. The seek lands on the keyframe at or before the cut, which
// the cut backoff already allows for.
func TrimFrom(ctx context.Context, cfg Config, vodPath string, cut float64, outPath string) error {
	if vodPath == "" || outPath == "" {
		return errors.New("trim paths missing")
	}
	args := []string{
		"-hide_banner", "-nostats", "-y",
		"-ss", strconv.FormatFloat(cut, 'f', 2, 64),
		"-i", vodPath,
		"-c", "copy",
		"-avoid_negative_ts", "make_zero",
		outPath,
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, resolveFfmpeg(cfg), args...)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("ffmpeg trim: %w: %s", runErr, truncate(string(out), 300))
	}
	return nil
}

// ConcatIntro joins the intro and the trimmed VOD without re-encoding
// using the concat demuxer. Both inputs must share codec, resolution,
// and framerate.
func ConcatIntro(ctx context.Context, cfg Config, introPath, trimmedPath, outPath string) error {
	if introPath == "" || trimmedPath == "" || outPath == "" {
		return errors.New("concat paths missing")
	}
	listPath := filepath.Join(cfg.WorkDir, "concat.txt")
	list := concatEntry(introPath) + concatEntry(trimmedPath)
	if writeErr := os.WriteFile(listPath, []byte(list), 0o600); writeErr != nil {
		return writeErr
	}
	args := []string{
		"-hide_banner", "-nostats", "-y",
		"-f", "concat", "-safe", "0",
		"-i", listPath,
		"-c", "copy",
		outPath,
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, resolveFfmpeg(cfg), args...)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("ffmpeg concat: %w: %s", runErr, truncate(string(out), 300))
	}
	return nil
}

// concatEntry renders one line of a concat demuxer list, escaping
// single quotes in the path.
func concatEntry(path string) string {
	escaped := strings.ReplaceAll(path, "'", `'\''`)
	return "file '" + escaped + "'\n"
}

// MediaDuration returns the container duration of path in seconds.
func MediaDuration(ctx context.Context, cfg Config, path string) (float64, error) {
	if path == "" {
		return 0, errors.New("media path is empty")
	}
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path,
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, resolveFfprobe(cfg), args...)
	out, runErr := cmd.Output()
	if runErr != nil {
		return 0, fmt.Errorf("ffprobe %s: %w", filepath.Base(path), runErr)
	}
	return parseDurationOutput(string(out))
}

// parseDurationOutput converts ffprobe's duration line to seconds.
func parseDurationOutput(out string) (float64, error) {
	trimmed := strings.TrimSpace(out)
	value, parseErr := strconv.ParseFloat(trimmed, 64)
	if parseErr != nil {
		return 0, fmt.Errorf("bad duration %q: %w", trimmed, parseErr)
	}
	if value <= 0 {
		return 0, fmt.Errorf("non-positive duration %q", trimmed)
	}
	return value, nil
}

// VerifyDuration checks that the spliced output is as long as the
// intro plus the trimmed VOD, within tolerance seconds.
func VerifyDuration(introDur, trimmedDur, outDur, tolerance float64) error {
	if introDur <= 0 || trimmedDur <= 0 || outDur <= 0 {
		return errors.New("durations must be positive")
	}
	want := introDur + trimmedDur
	diff := outDur - want
	if diff < 0 {
		diff = -diff
	}
	if diff > tolerance {
		return fmt.Errorf("output is %.1fs, expected %.1fs (difference %.1fs)", outDur, want, diff)
	}
	return nil
}
