package main

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

// TestSettingsFormRoundTripsToggles checks that what the checkboxes
// show is what gets saved, in particular an unticked cut, and that
// settings without a widget are carried over from the live config.
func TestSettingsFormRoundTripsToggles(t *testing.T) {
	test.NewApp()
	saves := 0
	form := newSettingsForm(func() { saves++ })
	base := DefaultConfig()
	base.Channel = "example-channel"
	base.SetupDone = true
	form.fill(base)
	if saves != 0 {
		t.Fatalf("filling the form must not save, got %d saves", saves)
	}
	form.cutEnabled.SetChecked(false)
	form.introEnabled.SetChecked(false)
	if saves != 2 {
		t.Fatalf("each checkbox change should save once, got %d saves", saves)
	}
	got, collectErr := form.collect(base)
	if collectErr != nil {
		t.Fatalf("collect: %v", collectErr)
	}
	if got.CutEnabled || got.IntroEnabled {
		t.Fatalf("unticked toggles were not collected: %+v", got)
	}
	if got.Channel != "example-channel" || !got.SetupDone {
		t.Fatalf("channel or setup flag lost: %+v", got)
	}
	if got.OutputDir != base.OutputDir || got.KeepFinalDays != 0 {
		t.Fatalf("defaults changed on the way through the form: %+v", got)
	}
}

// TestSettingsFormReportsBadNumbers keeps the first parse error.
func TestSettingsFormReportsBadNumbers(t *testing.T) {
	test.NewApp()
	form := newSettingsForm(func() {})
	form.fill(DefaultConfig())
	form.keepDays.SetText("many")
	if _, collectErr := form.collect(DefaultConfig()); collectErr == nil {
		t.Fatal("expected an error for a non-numeric keep days value")
	}
}
