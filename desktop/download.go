package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// DownloadNew fetches any VODs not yet recorded in the download
// archive and returns the paths of the new files, oldest first.
func DownloadNew(ctx context.Context, cfg Config, logf func(string, ...any)) ([]string, error) {
	if ctx == nil || logf == nil {
		return nil, errors.New("missing context or logger")
	}
	rawDir := filepath.Join(cfg.WorkDir, "raw")
	if mkErr := os.MkdirAll(rawDir, 0o750); mkErr != nil {
		return nil, mkErr
	}
	before, beforeErr := listVideoFiles(rawDir)
	if beforeErr != nil {
		return nil, beforeErr
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, cfg.YtdlpPath, ytdlpArgs(cfg, rawDir)...)
	out, runErr := cmd.CombinedOutput()
	logTail(logf, "yt-dlp", out)
	if runErr != nil {
		return nil, fmt.Errorf("yt-dlp: %w", runErr)
	}
	after, afterErr := listVideoFiles(rawDir)
	if afterErr != nil {
		return nil, afterErr
	}
	return newFiles(before, after), nil
}

// SeedArchive marks every VOD currently on the channel as already
// downloaded, without downloading anything. It exists for first-time
// setup, so old VODs are never pulled down.
func SeedArchive(ctx context.Context, cfg Config, logf func(string, ...any)) error {
	if ctx == nil || logf == nil {
		return errors.New("missing context or logger")
	}
	if mkErr := os.MkdirAll(cfg.WorkDir, 0o750); mkErr != nil {
		return mkErr
	}
	args := []string{
		"--download-archive", archivePath(cfg),
		"--simulate",
		"--force-write-download-archive",
		"--no-progress",
		channelVideosURL(cfg),
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, cfg.YtdlpPath, args...)
	out, runErr := cmd.CombinedOutput()
	logTail(logf, "yt-dlp", out)
	if runErr != nil {
		return fmt.Errorf("yt-dlp archive seed: %w", runErr)
	}
	return nil
}

func archivePath(cfg Config) string {
	return filepath.Join(cfg.WorkDir, "archive.txt")
}

func channelVideosURL(cfg Config) string {
	return "https://www.twitch.tv/" + cfg.Channel + "/videos?filter=archives"
}

func ytdlpArgs(cfg Config, rawDir string) []string {
	return []string{
		"--download-archive", archivePath(cfg),
		"--playlist-items", "1-3",
		"--match-filter", "!is_live",
		"--no-progress",
		"-o", filepath.Join(rawDir, "%(upload_date)s-%(id)s.%(ext)s"),
		channelVideosURL(cfg),
	}
}

// listVideoFiles returns the finished (non-partial) files in dir,
// keyed by full path.
func listVideoFiles(dir string) (map[string]bool, error) {
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return nil, readErr
	}
	files := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".ytdl") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		files[filepath.Join(dir, name)] = true
	}
	return files, nil
}

// newFiles returns paths present in after but not in before, sorted.
func newFiles(before, after map[string]bool) []string {
	var fresh []string
	for path := range after {
		if !before[path] {
			fresh = append(fresh, path)
		}
	}
	sort.Strings(fresh)
	return fresh
}

// logTail records the last few lines of a tool's output.
func logTail(logf func(string, ...any), name string, out []byte) {
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	start := 0
	if len(lines) > 10 {
		start = len(lines) - 10
	}
	for _, line := range lines[start:] {
		if line != "" {
			logf("%s: %s", name, line)
		}
	}
}
