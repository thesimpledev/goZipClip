package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestListVideoFilesSkipsPartials(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "20260801-v123.mp4"))
	writeTestFile(t, filepath.Join(dir, "20260802-v456.mp4.part"))
	writeTestFile(t, filepath.Join(dir, "20260802-v456.mp4.ytdl"))
	if mkErr := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	files, listErr := listVideoFiles(dir)
	if listErr != nil {
		t.Fatalf("list: %v", listErr)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %v", files)
	}
	if !files[filepath.Join(dir, "20260801-v123.mp4")] {
		t.Fatalf("finished file missing from %v", files)
	}
}

func TestNewFiles(t *testing.T) {
	before := map[string]bool{"/w/raw/a.mp4": true}
	after := map[string]bool{
		"/w/raw/a.mp4": true,
		"/w/raw/c.mp4": true,
		"/w/raw/b.mp4": true,
	}
	got := newFiles(before, after)
	if len(got) != 2 || got[0] != "/w/raw/b.mp4" || got[1] != "/w/raw/c.mp4" {
		t.Fatalf("got %v", got)
	}
}

func TestNewFilesNoneNew(t *testing.T) {
	same := map[string]bool{"/w/raw/a.mp4": true}
	if got := newFiles(same, same); len(got) != 0 {
		t.Fatalf("expected no new files, got %v", got)
	}
}

func TestYtdlpArgsPinFormat(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := DefaultConfig()
	cfg.Channel = "example-channel"
	args := ytdlpArgs(cfg, filepath.Join(t.TempDir(), "raw"))
	if !hasArgPair(args, "-S", "codec:h264:aac") {
		t.Fatalf("format sort missing from %v", args)
	}
	if !hasArgPair(args, "--remux-video", "mp4") {
		t.Fatalf("mp4 remux missing from %v", args)
	}
	if args[len(args)-1] != channelVideosURL(cfg) {
		t.Fatalf("channel URL must come last, got %v", args)
	}
}

func TestYtdlpArgsDownloadEverythingNewWithProgress(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := DefaultConfig()
	cfg.Channel = "example-channel"
	args := ytdlpArgs(cfg, filepath.Join(t.TempDir(), "raw"))
	for _, arg := range args {
		if arg == "--playlist-items" || arg == "--no-progress" {
			t.Fatalf("%s must be gone from a normal run: %v", arg, args)
		}
	}
	if !hasArgPair(args, "--progress-delta", "1") || !hasArgPair(args, "--download-archive", archivePath()) {
		t.Fatalf("progress or archive flags missing: %v", args)
	}
	found := false
	for _, arg := range args {
		if arg == "--newline" {
			found = true
		}
	}
	if !found {
		t.Fatalf("--newline missing: %v", args)
	}
	if !hasArgPair(args, "--print-to-file", "after_move:%(id)s\t%(upload_date)s\t%(title)s") {
		t.Fatalf("names must be recorded after the move: %v", args)
	}
}

func hasArgPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func TestVodIDFromFilename(t *testing.T) {
	id, idErr := vodIDFromFilename("/w/raw/20260812-2345678901.mp4")
	if idErr != nil {
		t.Fatalf("unexpected error: %v", idErr)
	}
	if id != "2345678901" {
		t.Fatalf("got %q want %q", id, "2345678901")
	}
}

func TestVodIDFromFilenameNoDash(t *testing.T) {
	if _, idErr := vodIDFromFilename("/w/raw/clip.mp4"); idErr == nil {
		t.Fatal("expected an error for a name without a VOD id")
	}
}

func TestVodIDFromFilenameEmptyID(t *testing.T) {
	if _, idErr := vodIDFromFilename("/w/raw/20260812-.mp4"); idErr == nil {
		t.Fatal("expected an error for an empty VOD id")
	}
}

func TestIsMaxDownloadsAbort(t *testing.T) {
	if isMaxDownloadsAbort(nil) {
		t.Fatal("nil error must not count as a max-downloads abort")
	}
	if isMaxDownloadsAbort(exitWithCode(t, 1)) {
		t.Fatal("exit code 1 must not count as a max-downloads abort")
	}
	if !isMaxDownloadsAbort(exitWithCode(t, 101)) {
		t.Fatal("exit code 101 must count as a max-downloads abort")
	}
}

// exitWithCode runs a shell that exits with the given code and returns
// the resulting error.
func exitWithCode(t *testing.T, code int) error {
	t.Helper()
	runErr := exec.Command("sh", "-c", fmt.Sprintf("exit %d", code)).Run()
	if runErr == nil {
		t.Fatalf("expected exit code %d to produce an error", code)
	}
	return runErr
}

func TestChannelVideosURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Channel = "example-channel"
	want := "https://www.twitch.tv/example-channel/videos?filter=archives"
	if got := channelVideosURL(cfg); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
