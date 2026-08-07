package main

import (
	"os"
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

func TestChannelVideosURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Channel = "example-channel"
	want := "https://www.twitch.tv/example-channel/videos?filter=archives"
	if got := channelVideosURL(cfg); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
