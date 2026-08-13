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
	cmd := exec.CommandContext(ctx, resolveYtdlp(cfg), ytdlpArgs(cfg, rawDir)...)
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
	args := []string{
		"--download-archive", archivePath(),
		"--simulate",
		"--force-write-download-archive",
		"--no-progress",
		channelVideosURL(cfg),
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, resolveYtdlp(cfg), args...)
	out, runErr := cmd.CombinedOutput()
	logTail(logf, "yt-dlp", out)
	if runErr != nil {
		return fmt.Errorf("yt-dlp archive seed: %w", runErr)
	}
	return nil
}

// DownloadLatest fetches the newest VOD on the channel, ignoring the
// download archive, records it there, and returns the file's path.
func DownloadLatest(ctx context.Context, cfg Config, logf func(string, ...any)) (string, error) {
	if ctx == nil || logf == nil {
		return "", errors.New("missing context or logger")
	}
	rawDir := filepath.Join(cfg.WorkDir, "raw")
	if mkErr := os.MkdirAll(rawDir, 0o750); mkErr != nil {
		return "", mkErr
	}
	args := []string{
		"--playlist-items", "1-3",
		"--match-filter", "!is_live",
		"--max-downloads", "1",
		"--no-progress",
		"-o", filepath.Join(rawDir, "%(upload_date)s-%(id)s.%(ext)s"),
	}
	args = append(args, downloadFormatArgs(cfg)...)
	args = append(args, channelVideosURL(cfg))
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, resolveYtdlp(cfg), args...)
	out, runErr := cmd.CombinedOutput()
	logTail(logf, "yt-dlp", out)
	if runErr != nil && !isMaxDownloadsAbort(runErr) {
		return "", fmt.Errorf("yt-dlp latest: %w", runErr)
	}
	files, listErr := listVideoFiles(rawDir)
	if listErr != nil {
		return "", listErr
	}
	latest := newestFile(files)
	if latest == "" {
		return "", errors.New("no VOD file found after downloading the latest")
	}
	return latest, recordLatestInArchive(ctx, cfg, logf, latest)
}

// recordLatestInArchive marks the downloaded VOD as handled so the
// next scheduled run does not download it again.
func recordLatestInArchive(ctx context.Context, cfg Config, logf func(string, ...any), vodPath string) error {
	id, idErr := vodIDFromFilename(vodPath)
	if idErr != nil {
		return idErr
	}
	args := []string{
		"--download-archive", archivePath(),
		"--simulate",
		"--force-write-download-archive",
		"--no-progress",
		"https://www.twitch.tv/videos/" + id,
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, resolveYtdlp(cfg), args...)
	out, runErr := cmd.CombinedOutput()
	logTail(logf, "yt-dlp", out)
	if runErr != nil {
		return fmt.Errorf("yt-dlp archive record: %w", runErr)
	}
	return nil
}

// isMaxDownloadsAbort reports whether yt-dlp exited with code 101,
// which means --max-downloads stopped it after a successful download.
func isMaxDownloadsAbort(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 101
}

// vodIDFromFilename extracts the VOD id from a raw file named
// %(upload_date)s-%(id)s.%(ext)s.
func vodIDFromFilename(path string) (string, error) {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	_, id, found := strings.Cut(base, "-")
	if !found || id == "" {
		return "", fmt.Errorf("no VOD id in file name %q", path)
	}
	return id, nil
}

// LatestVODID returns the id of the newest VOD on the channel without
// downloading anything.
func LatestVODID(ctx context.Context, cfg Config, logf func(string, ...any)) (string, error) {
	if ctx == nil || logf == nil {
		return "", errors.New("missing context or logger")
	}
	args := []string{
		"--simulate",
		"--print", "id",
		"--playlist-items", "1",
		"--no-progress",
		channelVideosURL(cfg),
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, resolveYtdlp(cfg), args...)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		logTail(logf, "yt-dlp", out)
		return "", fmt.Errorf("yt-dlp latest id: %w", runErr)
	}
	id := strings.TrimSpace(string(out))
	if id == "" {
		return "", errors.New("channel has no VODs")
	}
	return id, nil
}

// IsArchived reports whether the download archive already lists the
// given VOD id. A missing archive means nothing is recorded yet.
func IsArchived(id string) bool {
	if id == "" {
		return false
	}
	// #nosec G304 -- the archive path is built from the per-user config folder
	data, readErr := os.ReadFile(archivePath())
	if readErr != nil {
		return false
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && fields[len(fields)-1] == id {
			return true
		}
	}
	return false
}

// newestFile returns the lexicographically last path, which is the
// newest VOD because file names start with the upload date.
func newestFile(files map[string]bool) string {
	var latest string
	for path := range files {
		if path > latest {
			latest = path
		}
	}
	return latest
}

// archivePath returns the download archive's location in the per-user
// config folder, next to config.json and the log.
func archivePath() string {
	return filepath.Join(configDir(), "archive.txt")
}

func channelVideosURL(cfg Config) string {
	return "https://www.twitch.tv/" + cfg.Channel + "/videos?filter=archives"
}

func ytdlpArgs(cfg Config, rawDir string) []string {
	args := []string{
		"--download-archive", archivePath(),
		"--playlist-items", "1-3",
		"--match-filter", "!is_live",
		"--no-progress",
		"-o", filepath.Join(rawDir, "%(upload_date)s-%(id)s.%(ext)s"),
	}
	args = append(args, downloadFormatArgs(cfg)...)
	return append(args, channelVideosURL(cfg))
}

// downloadFormatArgs pins what yt-dlp delivers: prefer h264 video with
// aac audio and remux into mp4, so the stream-copy pipeline and the
// minimal bundled ffmpeg always receive the codecs they expect. When
// the resolved ffmpeg is a concrete file rather than a PATH name,
// yt-dlp is pointed at the same copy the pipeline uses.
func downloadFormatArgs(cfg Config) []string {
	args := []string{
		"-S", "codec:h264:aac",
		"--remux-video", "mp4",
	}
	if ffmpeg := resolveFfmpeg(cfg); filepath.IsAbs(ffmpeg) {
		args = append(args, "--ffmpeg-location", ffmpeg)
	}
	return args
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
