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
	exeDir := executableDir()
	logger, logErr := NewLogger(filepath.Join(exeDir, "zipclip.log"))
	if logErr != nil {
		// A stderr write failure at startup is not actionable.
		_, _ = fmt.Fprintln(os.Stderr, "log file unavailable:", logErr)
	}
	cfgPath := filepath.Join(exeDir, "config.json")
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
