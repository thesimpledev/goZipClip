// ZipClip watches a Twitch channel for new VODs, trims off the
// prestream waiting screen, splices an intro on the front, and drops
// the result into a folder for upload. See README.md for setup.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
		// Whatever was there (an older version's file, or nothing) is
		// replaced with this build's defaults, so the next launch
		// reads a current file.
		if saveErr := cfg.Save(cfgPath); saveErr != nil {
			logger.Logf("could not write default config: %v", saveErr)
		}
	}
	if dirErr := EnsureFolders(cfg); dirErr != nil {
		logger.Logf("could not create folders: %v", dirErr)
	}
	store := &ConfigStore{}
	store.Set(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	ytdlpReady := make(chan struct{})
	go func() {
		EnsureYtdlp(ctx, cfg, logger.Logf)
		close(ytdlpReady)
	}()
	pipe := NewPipeline(store, logger)
	pipe.SetToolsReady(ytdlpReady)
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

// dataDir returns the per-user folder for the large files ZipClip
// produces: the default output and work folders live in it. On
// Windows it is %LOCALAPPDATA%\zipclip, the folder the managed
// yt-dlp copy already uses; on other systems it follows the XDG data
// directory (~/.local/share/zipclip). It falls back to the
// executable's folder when no per-user location is available.
func dataDir() string {
	var base string
	switch runtime.GOOS {
	case "windows":
		local, localErr := os.UserCacheDir()
		if localErr != nil {
			return executableDir()
		}
		base = local
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return executableDir()
			}
			base = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(base, "zipclip")
}
