package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// maxDownload caps every fetch so a misbehaving server cannot fill
// the disk. The largest legitimate download (the ffmpeg source) is
// well under it.
const maxDownload = 200 << 20

// checkout is the repository tree the tool works in. Every file read
// and write goes through fs, an os.Root that confines access to the
// checkout, so no constructed path can escape it. dir is the absolute
// location, used for running build commands and in messages.
type checkout struct {
	dir string
	fs  *os.Root
}

func openCheckout(dir string) (*checkout, error) {
	if dir == "" {
		return nil, errors.New("checkout directory is empty")
	}
	fs, openErr := os.OpenRoot(dir)
	if openErr != nil {
		return nil, openErr
	}
	return &checkout{dir: dir, fs: fs}, nil
}

func (c *checkout) close() {
	// Nothing actionable in a failed close of a directory handle.
	_ = c.fs.Close()
}

// rel converts an absolute path inside the checkout to the
// root-relative form the os.Root API takes.
func (c *checkout) rel(path string) (string, error) {
	relPath, relErr := filepath.Rel(c.dir, path)
	if relErr != nil {
		return "", relErr
	}
	return relPath, nil
}

// exists reports whether path is an existing regular file inside the
// checkout.
func (c *checkout) exists(path string) bool {
	relPath, relErr := c.rel(path)
	if relErr != nil {
		return false
	}
	info, statErr := c.fs.Stat(relPath)
	return statErr == nil && !info.IsDir()
}

// mkdirAll creates a directory tree inside the checkout.
func (c *checkout) mkdirAll(path string) error {
	relPath, relErr := c.rel(path)
	if relErr != nil {
		return relErr
	}
	return c.fs.MkdirAll(relPath, 0o750)
}

// removeAll deletes a tree inside the checkout.
func (c *checkout) removeAll(path string) error {
	relPath, relErr := c.rel(path)
	if relErr != nil {
		return relErr
	}
	return c.fs.RemoveAll(relPath)
}

// writeFile writes data to a file inside the checkout.
func (c *checkout) writeFile(path string, data []byte) error {
	relPath, relErr := c.rel(path)
	if relErr != nil {
		return relErr
	}
	return c.fs.WriteFile(relPath, data, 0o600)
}

// copyFile copies src to dst, both inside the checkout, replacing dst.
func (c *checkout) copyFile(src, dst string) error {
	relSrc, srcErr := c.rel(src)
	if srcErr != nil {
		return srcErr
	}
	relDst, dstErr := c.rel(dst)
	if dstErr != nil {
		return dstErr
	}
	data, readErr := c.fs.ReadFile(relSrc)
	if readErr != nil {
		return readErr
	}
	return c.fs.WriteFile(relDst, data, 0o600)
}

// fileSHA256 returns the hex SHA-256 of a file inside the checkout.
func (c *checkout) fileSHA256(path string) (string, error) {
	relPath, relErr := c.rel(path)
	if relErr != nil {
		return "", relErr
	}
	file, openErr := c.fs.Open(relPath)
	if openErr != nil {
		return "", openErr
	}
	defer func() {
		// Nothing actionable in a failed close of a read-only file.
		_ = file.Close()
	}()
	digest := sha256.New()
	if _, copyErr := io.Copy(digest, file); copyErr != nil {
		return "", copyErr
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// download fetches url into a file inside the checkout and returns
// the hex SHA-256 of what was written.
func (c *checkout) download(url, path string) (string, error) {
	relPath, relErr := c.rel(path)
	if relErr != nil {
		return "", relErr
	}
	resp, getErr := httpGet(url)
	if getErr != nil {
		return "", getErr
	}
	defer func() {
		// Nothing actionable in a failed close of a read-only body.
		_ = resp.Body.Close()
	}()
	file, createErr := c.fs.OpenFile(relPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if createErr != nil {
		return "", createErr
	}
	digest := sha256.New()
	_, copyErr := io.Copy(file, io.TeeReader(io.LimitReader(resp.Body, maxDownload), digest))
	closeErr := file.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// run executes one build step in dir with extra environment entries,
// streaming its output to the terminal.
func run(dir string, extraEnv []string, name string, args ...string) error {
	if name == "" || dir == "" {
		return errors.New("command or directory is empty")
	}
	// #nosec G204 -- every command is a fixed build step over pinned sources, not external input
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if runErr := cmd.Run(); runErr != nil {
		return fmt.Errorf("%s: %w", name, runErr)
	}
	return nil
}

// requireTools fails when any of the named executables is missing.
func requireTools(names ...string) error {
	for _, name := range names {
		if _, lookErr := exec.LookPath(name); lookErr != nil {
			return fmt.Errorf("missing required tool: %s", name)
		}
	}
	return nil
}

// httpGet issues a GET and fails on any non-OK status.
func httpGet(url string) (*http.Response, error) {
	// #nosec G107 -- every URL is one of the pinned constants in this package
	resp, getErr := http.Get(url)
	if getErr != nil {
		return nil, getErr
	}
	if resp.StatusCode != http.StatusOK {
		// The body of a rejected response carries nothing actionable.
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp, nil
}

// fetchText downloads a small text file (like a checksum list) and
// returns its contents.
func fetchText(url string) (string, error) {
	resp, getErr := httpGet(url)
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
	return string(data), nil
}

// findSum extracts the hex digest for name from a standard checksum
// list of "digest filename" lines.
func findSum(list, name string) (string, error) {
	if name == "" {
		return "", errors.New("checksum name is empty")
	}
	for line := range strings.SplitSeq(list, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == name {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum for %s in checksum list", name)
}
