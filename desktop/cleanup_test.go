package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// buildCleanupFixture lays out a work folder and output folder with
// one deletable raw VOD, one scratch file, protected files, and one
// old plus one fresh output.
func buildCleanupFixture(t *testing.T, now time.Time) Config {
	t.Helper()
	workDir := t.TempDir()
	outDir := t.TempDir()
	rawDir := filepath.Join(workDir, "raw")
	if mkErr := os.MkdirAll(rawDir, 0o755); mkErr != nil {
		t.Fatalf("mkdir: %v", mkErr)
	}
	writeTestFile(t, filepath.Join(rawDir, "20260801-v123.mp4"))
	writeTestFile(t, filepath.Join(workDir, "trimmed.mp4"))
	writeTestFile(t, filepath.Join(workDir, "archive.txt"))
	writeTestFile(t, filepath.Join(workDir, "intro_ready.mp4"))
	oldOut := filepath.Join(outDir, "old.mp4")
	writeTestFile(t, oldOut)
	if chErr := os.Chtimes(oldOut, now.AddDate(0, 0, -10), now.AddDate(0, 0, -10)); chErr != nil {
		t.Fatalf("chtimes: %v", chErr)
	}
	writeTestFile(t, filepath.Join(outDir, "new.mp4"))
	cfg := DefaultConfig()
	cfg.WorkDir = workDir
	cfg.OutputDir = outDir
	cfg.KeepFinalDays = 3
	return cfg
}

func TestCleanupCandidates(t *testing.T) {
	now := time.Now()
	cfg := buildCleanupFixture(t, now)
	candidates, listErr := CleanupCandidates(cfg, now)
	if listErr != nil {
		t.Fatalf("candidates: %v", listErr)
	}
	got := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		got[filepath.Base(c.Path)] = true
	}
	for _, wantName := range []string{"20260801-v123.mp4", "trimmed.mp4", "old.mp4"} {
		if !got[wantName] {
			t.Fatalf("expected %s in candidates, got %v", wantName, got)
		}
	}
	for _, keepName := range []string{"archive.txt", "intro_ready.mp4", "new.mp4"} {
		if got[keepName] {
			t.Fatalf("%s must never be a cleanup candidate", keepName)
		}
	}
}

func TestCleanupCandidatesRequiresWorkDir(t *testing.T) {
	cfg := DefaultConfig()
	if _, listErr := CleanupCandidates(cfg, time.Now()); listErr == nil {
		t.Fatal("expected an error without a work folder")
	}
}

func TestTotalSize(t *testing.T) {
	candidates := []Candidate{{Path: "a", Size: 100}, {Path: "b", Size: 250}}
	if got := TotalSize(candidates); got != 350 {
		t.Fatalf("got %d want 350", got)
	}
	if got := TotalSize(nil); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}

func TestDeleteCandidates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gone.mp4")
	writeTestFile(t, path)
	if delErr := DeleteCandidates([]Candidate{{Path: path, Size: 4}}); delErr != nil {
		t.Fatalf("delete: %v", delErr)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatal("file should be gone")
	}
}

func TestFormatSize(t *testing.T) {
	cases := map[int64]string{
		512:             "512 B",
		2048:            "2.0 KB",
		5 * 1024 * 1024: "5.0 MB",
		3221225472:      "3.0 GB",
	}
	for input, want := range cases {
		if got := formatSize(input); got != want {
			t.Fatalf("%d: got %q want %q", input, got, want)
		}
	}
}

func writeTestFile(t *testing.T, path string) {
	t.Helper()
	if writeErr := os.WriteFile(path, []byte("data"), 0o600); writeErr != nil {
		t.Fatalf("write %s: %v", path, writeErr)
	}
}
