package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The official standalone yt-dlp build for Windows and its published
// checksum list, both served from the project's latest GitHub release.
const (
	ytdlpExeURL  = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
	ytdlpSumsURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/SHA2-256SUMS"
)

// ytdlpSetupTimeout bounds the whole launch-time install or update.
const ytdlpSetupTimeout = 10 * time.Minute

// ytdlpMaxDownload caps how much of a release download is read, so a
// misbehaving server cannot fill the disk. The real binary is ~20MB.
const ytdlpMaxDownload = 200 << 20

// EnsureYtdlp installs or updates the yt-dlp copy ZipClip runs. It
// runs once at launch, in the background; the pipeline waits for it before
// its first download so a run never races the binary being replaced.
// It only acts on Windows, and only while the configured path is still
// the default: a user-managed yt-dlp is left alone. Failure is logged,
// not fatal, because a PATH copy may still work.
func EnsureYtdlp(ctx context.Context, cfg Config, logf func(string, ...any)) {
	if ctx == nil || logf == nil {
		return
	}
	if runtime.GOOS != "windows" {
		return
	}
	if cfg.YtdlpPath != "" && cfg.YtdlpPath != "yt-dlp" {
		return
	}
	target := ytdlpSetupTarget()
	if target == "" {
		return
	}
	setupCtx, cancel := context.WithTimeout(ctx, ytdlpSetupTimeout)
	defer cancel()
	if fileExists(target) {
		if updateErr := updateYtdlp(setupCtx, target, logf); updateErr != nil {
			logf("yt-dlp update failed (keeping current copy): %v", updateErr)
		}
		return
	}
	if installErr := installYtdlp(setupCtx, target, logf); installErr != nil {
		logf("yt-dlp install failed (falling back to PATH): %v", installErr)
	}
}

// ytdlpSetupTarget picks which yt-dlp copy launch-time setup manages:
// the copy shipped next to the executable when present (zip installs,
// where the folder is writable), otherwise the managed copy in the
// per-user folder (MSIX installs, where the package is read-only).
func ytdlpSetupTarget() string {
	if bundled := bundledToolPath("yt-dlp"); fileExists(bundled) {
		return bundled
	}
	return managedYtdlpPath()
}

// updateYtdlp runs the managed copy's built-in self-updater.
func updateYtdlp(ctx context.Context, managed string, logf func(string, ...any)) error {
	// #nosec G204 -- the managed path is built from the per-user cache folder
	cmd := exec.CommandContext(ctx, managed, "-U")
	out, runErr := cmd.CombinedOutput()
	logTail(logf, "yt-dlp -U", out)
	if runErr != nil {
		return fmt.Errorf("yt-dlp -U: %w", runErr)
	}
	return nil
}

// installYtdlp downloads the official yt-dlp.exe into the managed
// folder, verifying it against the release's published SHA-256 sum
// before moving it into place.
func installYtdlp(ctx context.Context, dest string, logf func(string, ...any)) error {
	logf("downloading yt-dlp from %s", ytdlpExeURL)
	want, sumErr := fetchYtdlpSum(ctx)
	if sumErr != nil {
		return sumErr
	}
	if mkErr := os.MkdirAll(filepath.Dir(dest), 0o750); mkErr != nil {
		return mkErr
	}
	partial := dest + ".download"
	got, dlErr := downloadToFile(ctx, ytdlpExeURL, partial)
	if dlErr != nil {
		return dlErr
	}
	if got != want {
		// The partial file failed verification; removal is best effort
		// because a leftover .download file is never executed.
		_ = os.Remove(partial)
		return fmt.Errorf("yt-dlp checksum mismatch: got %s want %s", got, want)
	}
	if renameErr := os.Rename(partial, dest); renameErr != nil {
		return renameErr
	}
	logf("installed yt-dlp at %s", dest)
	return nil
}

// fetchYtdlpSum returns the published SHA-256 sum of yt-dlp.exe from
// the latest release's checksum list.
func fetchYtdlpSum(ctx context.Context) (string, error) {
	resp, getErr := httpGet(ctx, ytdlpSumsURL)
	if getErr != nil {
		return "", getErr
	}
	defer func() {
		// Nothing actionable in a failed close of a fully read response.
		_ = resp.Body.Close()
	}()
	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return "", readErr
	}
	return findSum(data, "yt-dlp.exe")
}

// findSum extracts the hex digest for name from a standard checksum
// list of "digest filename" lines.
func findSum(data []byte, name string) (string, error) {
	if name == "" {
		return "", errors.New("checksum name is empty")
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum for %s in release checksum list", name)
}

// httpGet issues a GET and fails on any non-OK status.
func httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if reqErr != nil {
		return nil, reqErr
	}
	resp, doErr := http.DefaultClient.Do(req)
	if doErr != nil {
		return nil, doErr
	}
	if resp.StatusCode != http.StatusOK {
		// The body of a rejected response carries nothing actionable.
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp, nil
}

// downloadToFile streams url into path and returns the hex SHA-256 of
// what was written.
func downloadToFile(ctx context.Context, url, path string) (string, error) {
	resp, getErr := httpGet(ctx, url)
	if getErr != nil {
		return "", getErr
	}
	defer func() {
		// Nothing actionable in a failed close of a read-only body.
		_ = resp.Body.Close()
	}()
	// #nosec G304 G302 -- the path is built from the per-user cache folder, not external
	// input, and the download is an executable so it needs the (owner-only) execute bit
	file, createErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o700)
	if createErr != nil {
		return "", createErr
	}
	digest := sha256.New()
	_, copyErr := io.Copy(file, io.TeeReader(io.LimitReader(resp.Body, ytdlpMaxDownload), digest))
	closeErr := file.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}
