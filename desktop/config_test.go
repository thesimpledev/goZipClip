package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigMissingFile(t *testing.T) {
	_, loadErr := LoadConfig(filepath.Join(t.TempDir(), "missing.json"))
	if loadErr == nil {
		t.Fatal("expected an error for a missing config file")
	}
}

func TestLoadConfigEmptyPath(t *testing.T) {
	_, loadErr := LoadConfig("")
	if loadErr == nil {
		t.Fatal("expected an error for an empty path")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	want := DefaultConfig()
	want.Channel = "example-channel"
	want.SceneThreshold = 0.55
	if saveErr := want.Save(path); saveErr != nil {
		t.Fatalf("save: %v", saveErr)
	}
	got, loadErr := LoadConfig(path)
	if loadErr != nil {
		t.Fatalf("load: %v", loadErr)
	}
	if got != want {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestLoadConfigKeepsDefaultsForMissingFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if writeErr := os.WriteFile(path, []byte(`{"channel":"example-channel"}`), 0o600); writeErr != nil {
		t.Fatalf("write: %v", writeErr)
	}
	got, loadErr := LoadConfig(path)
	if loadErr != nil {
		t.Fatalf("load: %v", loadErr)
	}
	if got.Channel != "example-channel" {
		t.Fatalf("channel not read: %+v", got)
	}
	if got.DailyRunTime != "08:00" || got.ScanWindowMinutes != 30 {
		t.Fatalf("defaults not kept: %+v", got)
	}
}

func TestValidateReportsProblems(t *testing.T) {
	problems := DefaultConfig().Validate()
	if len(problems) == 0 {
		t.Fatal("expected problems for an unconfigured install")
	}
	found := false
	for _, p := range problems {
		if p == "channel is not set" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing channel problem in %v", problems)
	}
}

func TestValidateThresholdRange(t *testing.T) {
	cfg := validTestConfig(t)
	for _, bad := range []float64{0, 1, -0.5, 1.5} {
		cfg.SceneThreshold = bad
		if len(cfg.Validate()) == 0 {
			t.Fatalf("threshold %v should be rejected", bad)
		}
	}
}

func TestValidateCleanConfig(t *testing.T) {
	cfg := validTestConfig(t)
	if problems := cfg.Validate(); len(problems) != 0 {
		t.Fatalf("expected no problems, got %v", problems)
	}
}

// validTestConfig builds a configuration that passes validation,
// backed by real temp directories and a placeholder intro file.
func validTestConfig(t *testing.T) Config {
	t.Helper()
	base := t.TempDir()
	intro := filepath.Join(base, "intro.mp4")
	if writeErr := os.WriteFile(intro, []byte("placeholder"), 0o600); writeErr != nil {
		t.Fatalf("write intro: %v", writeErr)
	}
	cfg := DefaultConfig()
	cfg.Channel = "example-channel"
	cfg.TwitchClientID = "example-client-id"
	cfg.TwitchClientSecret = "example-secret"
	cfg.IntroFile = intro
	cfg.OutputDir = base
	cfg.WorkDir = base
	return cfg
}
