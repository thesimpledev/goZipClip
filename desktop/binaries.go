package main

import (
	"os"
	"path/filepath"
	"runtime"
)

// exeSuffix is the executable file extension on this platform.
func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// pickTool decides which binary to run. An explicit configured path
// (anything other than the tool's bare default name) always wins.
// Otherwise the first candidate that exists on disk is used, and when
// none does the default name is returned for a PATH lookup.
func pickTool(configured, name string, candidates ...string) string {
	if configured != "" && configured != name {
		return configured
	}
	for _, candidate := range candidates {
		if candidate != "" && fileExists(candidate) {
			return candidate
		}
	}
	return configured
}

// bundledToolPath is where a tool shipped alongside the ZipClip
// executable would live (the MSIX package folder on Windows).
func bundledToolPath(name string) string {
	return filepath.Join(executableDir(), name+exeSuffix())
}

// managedBinDir is the user-writable folder where ZipClip keeps
// binaries it downloads and updates itself. It is empty when the
// platform offers no per-user cache directory.
func managedBinDir() string {
	base, baseErr := os.UserCacheDir()
	if baseErr != nil {
		return ""
	}
	return filepath.Join(base, "zipclip", "bin")
}

// managedYtdlpPath is where the self-updating yt-dlp copy lives.
func managedYtdlpPath() string {
	dir := managedBinDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "yt-dlp"+exeSuffix())
}

// resolveFfmpeg returns the ffmpeg binary to run: the configured
// override, else the bundled copy, else a PATH lookup.
func resolveFfmpeg(cfg Config) string {
	return pickTool(cfg.FfmpegPath, "ffmpeg", bundledToolPath("ffmpeg"))
}

// resolveFfprobe returns the ffprobe binary to run.
func resolveFfprobe(cfg Config) string {
	return pickTool(cfg.FfprobePath, "ffprobe", bundledToolPath("ffprobe"))
}

// resolveYtdlp returns the yt-dlp binary to run: the configured
// override, else the copy shipped next to the executable (zip
// installs), else the managed self-updating copy (MSIX installs),
// else a PATH lookup.
func resolveYtdlp(cfg Config) string {
	return pickTool(cfg.YtdlpPath, "yt-dlp", bundledToolPath("yt-dlp"), managedYtdlpPath())
}
