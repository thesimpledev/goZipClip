package main

import (
	"path/filepath"
	"testing"
)

func TestOutputPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OutputDir = filepath.Join("videos", "upload")
	vod := filepath.Join("videos", "work", "raw", "20260101-v123.mp4")
	want := filepath.Join("videos", "upload", "20260101-v123.mp4")
	if got := outputPath(cfg, vod); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestOutputPathReplacesExtension(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OutputDir = "out"
	if got := outputPath(cfg, "raw/20260101-v123.mkv"); got != filepath.Join("out", "20260101-v123.mp4") {
		t.Fatalf("extension not replaced: %q", got)
	}
}
