package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFileRenames(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "a.mp4")
	dst := filepath.Join(base, "b.mp4")
	if writeErr := os.WriteFile(src, []byte("video bytes"), 0o600); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	if moveErr := moveFile(src, dst); moveErr != nil {
		t.Fatalf("move: %v", moveErr)
	}
	if _, statErr := os.Stat(src); !os.IsNotExist(statErr) {
		t.Fatal("source should be gone after the move")
	}
	got, readErr := os.ReadFile(dst)
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}
	if string(got) != "video bytes" {
		t.Fatalf("content mismatch: %q", got)
	}
}

func TestMoveFileMissingSource(t *testing.T) {
	base := t.TempDir()
	if moveErr := moveFile(filepath.Join(base, "missing.mp4"), filepath.Join(base, "out.mp4")); moveErr == nil {
		t.Fatal("expected an error for a missing source")
	}
}

func TestMoveFileEmptyPaths(t *testing.T) {
	if moveErr := moveFile("", ""); moveErr == nil {
		t.Fatal("expected an error for empty paths")
	}
}

func TestCopyFilePreservesContent(t *testing.T) {
	base := t.TempDir()
	src := filepath.Join(base, "a.mp4")
	dst := filepath.Join(base, "b.mp4")
	if writeErr := os.WriteFile(src, []byte("video bytes"), 0o600); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	if copyErr := copyFile(src, dst); copyErr != nil {
		t.Fatalf("copy: %v", copyErr)
	}
	if _, statErr := os.Stat(src); statErr != nil {
		t.Fatalf("source should still exist: %v", statErr)
	}
	got, readErr := os.ReadFile(dst)
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}
	if string(got) != "video bytes" {
		t.Fatalf("content mismatch: %q", got)
	}
}
