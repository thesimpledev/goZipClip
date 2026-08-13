package main

import (
	"path/filepath"
	"testing"
)

func TestPickToolPrefersExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	bundled := filepath.Join(dir, "ffmpeg")
	writeTestFile(t, bundled)
	got := pickTool(filepath.Join(dir, "custom-ffmpeg"), "ffmpeg", bundled)
	if got != filepath.Join(dir, "custom-ffmpeg") {
		t.Fatalf("override lost, got %q", got)
	}
}

func TestPickToolUsesFirstExistingCandidate(t *testing.T) {
	dir := t.TempDir()
	bundled := filepath.Join(dir, "ffmpeg")
	writeTestFile(t, bundled)
	got := pickTool("ffmpeg", "ffmpeg", filepath.Join(dir, "missing"), bundled)
	if got != bundled {
		t.Fatalf("expected %q, got %q", bundled, got)
	}
}

func TestPickToolFallsBackToPathLookup(t *testing.T) {
	if got := pickTool("ffmpeg", "ffmpeg", "/does/not/exist"); got != "ffmpeg" {
		t.Fatalf("expected PATH fallback, got %q", got)
	}
}

func TestPickToolKeepsEmptyConfigured(t *testing.T) {
	if got := pickTool("", "ffmpeg"); got != "" {
		t.Fatalf("expected empty result for empty config, got %q", got)
	}
}
