package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)

// Config holds every user-adjustable setting. It is persisted as
// config.json next to the executable and is normally edited through
// the Settings pane, but it stays hand-editable JSON.
type Config struct {
	Channel             string  `json:"channel"`
	DailyRunTime        string  `json:"dailyRunTime"`
	CutEnabled          bool    `json:"cutEnabled"`
	ScanWindowMinutes   int     `json:"scanWindowMinutes"`
	SceneThreshold      float64 `json:"sceneThreshold"`
	CutBackoffSeconds   int     `json:"cutBackoffSeconds"`
	IntroEnabled        bool    `json:"introEnabled"`
	IntroFile           string  `json:"introFile"`
	OutputDir           string  `json:"outputDir"`
	WorkDir             string  `json:"workDir"`
	YtdlpPath           string  `json:"ytdlpPath"`
	FfmpegPath          string  `json:"ffmpegPath"`
	FfprobePath         string  `json:"ffprobePath"`
	DevMode             bool    `json:"devMode"`
	KeepFinalDays       int     `json:"keepFinalDays"`
	AutoUpload          bool    `json:"autoUpload"`
	YouTubeClientID     string  `json:"youtubeClientId"`
	YouTubeClientSecret string  `json:"youtubeClientSecret"`
}

// DefaultConfig returns the settings a fresh install starts from.
func DefaultConfig() Config {
	return Config{
		DailyRunTime:      "8:00 AM",
		CutEnabled:        true,
		ScanWindowMinutes: 30,
		SceneThreshold:    0.4,
		CutBackoffSeconds: 5,
		IntroEnabled:      true,
		YtdlpPath:         "yt-dlp",
		FfmpegPath:        "ffmpeg",
		FfprobePath:       "ffprobe",
		KeepFinalDays:     3,
	}
}

// LoadConfig reads and parses the config file at path. Fields missing
// from the file keep their default values.
func LoadConfig(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("config path is empty")
	}
	// #nosec G304 -- the config path is built from the executable's own directory, not external input
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return Config{}, readErr
	}
	cfg := DefaultConfig()
	if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, jsonErr)
	}
	return cfg, nil
}

// Save writes the configuration as indented JSON to path.
func (c Config) Save(path string) error {
	if path == "" {
		return errors.New("config path is empty")
	}
	data, marshalErr := json.MarshalIndent(c, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

// Validate returns a list of problems that keep the pipeline from
// running. An empty list means the configuration is usable.
func (c Config) Validate() []string {
	problems := c.validateAccount()
	problems = append(problems, c.validateNumbers()...)
	return append(problems, c.validatePaths()...)
}

func (c Config) validateAccount() []string {
	var problems []string
	if c.Channel == "" {
		problems = append(problems, "channel is not set")
	}
	if _, timeErr := parseRunTime(c.DailyRunTime); timeErr != nil {
		problems = append(problems, "daily run time must look like 8:00 PM")
	}
	if c.AutoUpload {
		if c.YouTubeClientID == "" {
			problems = append(problems, "automatic uploads need a YouTube client ID")
		}
		if c.YouTubeClientSecret == "" {
			problems = append(problems, "automatic uploads need a YouTube client secret")
		}
	}
	return problems
}

func (c Config) validateNumbers() []string {
	var problems []string
	if c.CutEnabled {
		if c.ScanWindowMinutes <= 0 {
			problems = append(problems, "scan window must be a positive number of minutes")
		}
		if c.SceneThreshold <= 0 || c.SceneThreshold >= 1 {
			problems = append(problems, "scene threshold must be between 0 and 1")
		}
		if c.CutBackoffSeconds < 0 {
			problems = append(problems, "cut backoff cannot be negative")
		}
	}
	if c.KeepFinalDays < 0 {
		problems = append(problems, "keep finished days cannot be negative")
	}
	return problems
}

func (c Config) validatePaths() []string {
	var problems []string
	if c.IntroEnabled {
		if c.IntroFile == "" {
			problems = append(problems, "intro file is not set")
		} else if !fileExists(c.IntroFile) {
			problems = append(problems, "intro file does not exist: "+c.IntroFile)
		}
	}
	problems = append(problems, requireDir("output folder", c.OutputDir)...)
	problems = append(problems, requireDir("work folder", c.WorkDir)...)
	if c.YtdlpPath == "" {
		problems = append(problems, "yt-dlp path is not set")
	}
	if c.FfmpegPath == "" {
		problems = append(problems, "ffmpeg path is not set")
	}
	if c.FfprobePath == "" {
		problems = append(problems, "ffprobe path is not set")
	}
	return problems
}

func requireDir(label, path string) []string {
	if path == "" {
		return []string{label + " is not set"}
	}
	info, statErr := os.Stat(path)
	if statErr != nil || !info.IsDir() {
		return []string{label + " is not a folder: " + path}
	}
	return nil
}

func fileExists(path string) bool {
	info, statErr := os.Stat(path)
	return statErr == nil && !info.IsDir()
}

// ConfigStore guards shared access to the live configuration between
// the GUI and the pipeline goroutines.
type ConfigStore struct {
	mu  sync.Mutex
	cfg Config
}

// Get returns a copy of the current configuration.
func (s *ConfigStore) Get() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// Set replaces the current configuration.
func (s *ConfigStore) Set(c Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = c
}
