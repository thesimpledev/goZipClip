package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Candidate is one file the cleanup pass may delete.
type Candidate struct {
	Path string
	Size int64
}

// CleanupCandidates lists files that are safe to remove: everything
// in the work folder except the download archive and the prepared
// intro, plus finished outputs older than KeepFinalDays. The archive
// file is the record of which VODs were already handled and must
// survive every cleanup.
func CleanupCandidates(cfg Config, now time.Time) ([]Candidate, error) {
	if cfg.WorkDir == "" {
		return nil, errors.New("work folder is not set")
	}
	candidates, workErr := workFileCandidates(cfg)
	if workErr != nil {
		return nil, workErr
	}
	old, outErr := oldOutputCandidates(cfg, now)
	if outErr != nil {
		return nil, outErr
	}
	candidates = append(candidates, old...)
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Path < candidates[j].Path })
	return candidates, nil
}

func workFileCandidates(cfg Config) ([]Candidate, error) {
	var candidates []Candidate
	walkErr := filepath.WalkDir(cfg.WorkDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "archive.txt" || d.Name() == "intro_ready.mp4" {
			return nil
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		candidates = append(candidates, Candidate{Path: path, Size: info.Size()})
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	return candidates, nil
}

func oldOutputCandidates(cfg Config, now time.Time) ([]Candidate, error) {
	if cfg.OutputDir == "" {
		return nil, nil
	}
	entries, readErr := os.ReadDir(cfg.OutputDir)
	if readErr != nil {
		return nil, readErr
	}
	cutoff := now.AddDate(0, 0, -cfg.KeepFinalDays)
	var candidates []Candidate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".mp4") {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			candidates = append(candidates, Candidate{
				Path: filepath.Join(cfg.OutputDir, entry.Name()),
				Size: info.Size(),
			})
		}
	}
	return candidates, nil
}

// TotalSize sums candidate sizes in bytes.
func TotalSize(candidates []Candidate) int64 {
	var total int64
	for _, c := range candidates {
		total += c.Size
	}
	return total
}

// DeleteCandidates removes the listed files, reporting the first
// failure after attempting every file.
func DeleteCandidates(candidates []Candidate) error {
	var firstErr error
	for _, c := range candidates {
		if rmErr := os.Remove(c.Path); rmErr != nil && firstErr == nil {
			firstErr = rmErr
		}
	}
	return firstErr
}

// formatSize renders a byte count for the GUI.
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
