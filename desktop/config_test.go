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
	want.IntroEnabled = false
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
	if got.DailyRunTime != "8:00 AM" || got.ScanWindowMinutes != 30 {
		t.Fatalf("defaults not kept: %+v", got)
	}
	if !got.CutEnabled || !got.IntroEnabled {
		t.Fatalf("toggles should default to on: %+v", got)
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

func TestValidateSkipsCutNumbersWhenCutOff(t *testing.T) {
	cfg := validTestConfig(t)
	cfg.CutEnabled = false
	cfg.SceneThreshold = 1.5
	cfg.ScanWindowMinutes = 0
	cfg.CutBackoffSeconds = -1
	if problems := cfg.Validate(); len(problems) != 0 {
		t.Fatalf("expected no problems with the cut off, got %v", problems)
	}
}

func TestValidateSkipsIntroWhenIntroOff(t *testing.T) {
	cfg := validTestConfig(t)
	cfg.IntroEnabled = false
	cfg.IntroFile = ""
	if problems := cfg.Validate(); len(problems) != 0 {
		t.Fatalf("expected no problems with an empty intro, got %v", problems)
	}
	cfg.IntroFile = filepath.Join(t.TempDir(), "missing.mp4")
	if problems := cfg.Validate(); len(problems) != 0 {
		t.Fatalf("expected no problems with a missing intro, got %v", problems)
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
	cfg.IntroFile = intro
	cfg.OutputDir = base
	cfg.WorkDir = base
	return cfg
}

func TestProblemsCarryFieldAndFeature(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AutoUpload = true
	byText := map[string]Problem{}
	for _, problem := range cfg.Problems() {
		byText[problem.Text] = problem
	}
	checks := []struct {
		text    string
		field   Field
		feature Feature
	}{
		{"channel is not set", FieldChannel, ""},
		{"intro file is not set", FieldIntroFile, FeatureIntro},
		{"automatic uploads need a YouTube client ID", FieldYouTubeID, FeatureUploads},
		{"output folder is not set", FieldOutputDir, ""},
	}
	for _, check := range checks {
		problem, found := byText[check.text]
		if !found {
			t.Fatalf("missing problem %q in %v", check.text, cfg.Validate())
		}
		if problem.Field != check.field || problem.Feature != check.feature {
			t.Fatalf("%q: got field %q feature %q", check.text, problem.Field, problem.Feature)
		}
	}
}

func TestValidateMatchesProblems(t *testing.T) {
	cfg := DefaultConfig()
	texts := cfg.Validate()
	problems := cfg.Problems()
	if len(texts) != len(problems) {
		t.Fatalf("%d texts for %d problems", len(texts), len(problems))
	}
	for i := range texts {
		if texts[i] != problems[i].Text {
			t.Fatalf("text %d: %q vs %q", i, texts[i], problems[i].Text)
		}
	}
}

func TestFeatureLabel(t *testing.T) {
	if FeatureIntro.Label() != "the intro" || FeatureCut.Label() != "the cut" || FeatureUploads.Label() != "automatic uploads" {
		t.Fatal("feature labels changed")
	}
	if Feature("other").Label() != "other" {
		t.Fatal("unknown feature must fall back to its name")
	}
}
