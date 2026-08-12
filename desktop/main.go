// ZipClip watches a Twitch channel for new VODs, trims off the
// prestream waiting screen, splices an intro on the front, and drops
// the result into a folder for upload. See README.md for setup.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	appDir := configDir()
	logger, logErr := NewLogger(filepath.Join(appDir, "zipclip.log"))
	if logErr != nil {
		// A stderr write failure at startup is not actionable.
		_, _ = fmt.Fprintln(os.Stderr, "log file unavailable:", logErr)
	}
	cfgPath := filepath.Join(appDir, "config.json")
	cfg := DefaultConfig()
	if loaded, loadErr := LoadConfig(cfgPath); loadErr == nil {
		cfg = loaded
	} else {
		logger.Logf("no usable config at %s: %v", cfgPath, loadErr)
	}
	store := &ConfigStore{}
	store.Set(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	pipe := NewPipeline(store, logger)
	sched := NewScheduler(store, logger, pipe)
	ui := NewUI(cfgPath, store, logger, pipe, sched)
	go sched.Loop(ctx)
	ui.ShowAndRun()
	cancel()
	logger.Close()
}

func executableDir() string {
	exe, exeErr := os.Executable()
	if exeErr != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// configDir returns the per-user folder where ZipClip keeps its
// settings and log, creating it if needed. It falls back to the
// executable's folder when no user config directory is available.
func configDir() string {
	base, baseErr := os.UserConfigDir()
	if baseErr != nil {
		return executableDir()
	}
	dir := filepath.Join(base, "zipclip")
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		return executableDir()
	}
	return dir
}
