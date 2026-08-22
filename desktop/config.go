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

// Field names one user-adjustable setting, so the GUI can offer the
// right control for fixing a problem.
type Field string

// The settings a problem can point at.
const (
	FieldChannel       Field = "channel"
	FieldRunTime       Field = "runTime"
	FieldScanWindow    Field = "scanWindow"
	FieldThreshold     Field = "threshold"
	FieldBackoff       Field = "backoff"
	FieldKeepDays      Field = "keepDays"
	FieldIntroFile     Field = "introFile"
	FieldOutputDir     Field = "outputDir"
	FieldWorkDir       Field = "workDir"
	FieldYtdlp         Field = "ytdlp"
	FieldFfmpeg        Field = "ffmpeg"
	FieldFfprobe       Field = "ffprobe"
	FieldYouTubeID     Field = "youtubeClientId"
	FieldYouTubeSecret Field = "youtubeClientSecret"
)

// Feature names an optional part of the pipeline that can be switched
// off instead of configured.
type Feature string

// The optional features.
const (
	FeatureCut     Feature = "cut"
	FeatureIntro   Feature = "intro"
	FeatureUploads Feature = "uploads"
)

// Label is how the feature is named in a sentence.
func (f Feature) Label() string {
	switch f {
	case FeatureCut:
		return "the cut"
	case FeatureIntro:
		return "the intro"
	case FeatureUploads:
		return "automatic uploads"
	default:
		return string(f)
	}
}

// Problem is one reason the pipeline cannot run yet: the message, the
// setting it is about, and the optional feature it belongs to (empty
// for settings that are always required).
type Problem struct {
	Text    string
	Field   Field
	Feature Feature
}

// Validate returns a list of problems that keep the pipeline from
// running. An empty list means the configuration is usable.
func (c Config) Validate() []string {
	problems := c.Problems()
	texts := make([]string, 0, len(problems))
	for _, problem := range problems {
		texts = append(texts, problem.Text)
	}
	return texts
}

// Problems is Validate with the setting and feature behind each
// problem, for the GUI's walkthrough.
func (c Config) Problems() []Problem {
	problems := c.validateAccount()
	problems = append(problems, c.validateNumbers()...)
	return append(problems, c.validatePaths()...)
}

func (c Config) validateAccount() []Problem {
	var problems []Problem
	if c.Channel == "" {
		problems = append(problems, Problem{Text: "channel is not set", Field: FieldChannel})
	}
	if _, timeErr := parseRunTime(c.DailyRunTime); timeErr != nil {
		problems = append(problems, Problem{Text: "daily run time must look like 8:00 PM", Field: FieldRunTime})
	}
	if c.AutoUpload {
		if c.YouTubeClientID == "" {
			problems = append(problems, Problem{Text: "automatic uploads need a YouTube client ID",
				Field: FieldYouTubeID, Feature: FeatureUploads})
		}
		if c.YouTubeClientSecret == "" {
			problems = append(problems, Problem{Text: "automatic uploads need a YouTube client secret",
				Field: FieldYouTubeSecret, Feature: FeatureUploads})
		}
	}
	return problems
}

func (c Config) validateNumbers() []Problem {
	var problems []Problem
	if c.CutEnabled {
		if c.ScanWindowMinutes <= 0 {
			problems = append(problems, Problem{Text: "scan window must be a positive number of minutes",
				Field: FieldScanWindow, Feature: FeatureCut})
		}
		if c.SceneThreshold <= 0 || c.SceneThreshold >= 1 {
			problems = append(problems, Problem{Text: "scene threshold must be between 0 and 1",
				Field: FieldThreshold, Feature: FeatureCut})
		}
		if c.CutBackoffSeconds < 0 {
			problems = append(problems, Problem{Text: "cut backoff cannot be negative",
				Field: FieldBackoff, Feature: FeatureCut})
		}
	}
	if c.KeepFinalDays < 0 {
		problems = append(problems, Problem{Text: "keep finished days cannot be negative", Field: FieldKeepDays})
	}
	return problems
}

func (c Config) validatePaths() []Problem {
	var problems []Problem
	if c.IntroEnabled {
		if c.IntroFile == "" {
			problems = append(problems, Problem{Text: "intro file is not set", Field: FieldIntroFile, Feature: FeatureIntro})
		} else if !fileExists(c.IntroFile) {
			problems = append(problems, Problem{Text: "intro file does not exist: " + c.IntroFile,
				Field: FieldIntroFile, Feature: FeatureIntro})
		}
	}
	problems = append(problems, requireDir("output folder", c.OutputDir, FieldOutputDir)...)
	problems = append(problems, requireDir("work folder", c.WorkDir, FieldWorkDir)...)
	if c.YtdlpPath == "" {
		problems = append(problems, Problem{Text: "yt-dlp path is not set", Field: FieldYtdlp})
	}
	if c.FfmpegPath == "" {
		problems = append(problems, Problem{Text: "ffmpeg path is not set", Field: FieldFfmpeg})
	}
	if c.FfprobePath == "" {
		problems = append(problems, Problem{Text: "ffprobe path is not set", Field: FieldFfprobe})
	}
	return problems
}

func requireDir(label, path string, field Field) []Problem {
	if path == "" {
		return []Problem{{Text: label + " is not set", Field: field}}
	}
	// #nosec G703 -- the folder path is the user's own setting from config.json, checked here so a bad one is reported, not used
	info, statErr := os.Stat(path)
	if statErr != nil || !info.IsDir() {
		return []Problem{{Text: label + " is not a folder: " + path, Field: field}}
	}
	return nil
}

func fileExists(path string) bool {
	// #nosec G703 -- paths checked here are the user's own settings or files in the app's own folders; nothing is opened
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
