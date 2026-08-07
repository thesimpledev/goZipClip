package main

import (
	"testing"
	"time"
)

func TestNextRunLaterToday(t *testing.T) {
	now := time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC)
	got, err := NextRun(now, "08:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 8, 6, 8, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNextRunTomorrow(t *testing.T) {
	now := time.Date(2026, 8, 6, 9, 30, 0, 0, time.UTC)
	got, err := NextRun(now, "08:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 8, 7, 8, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNextRunExactTimeGoesToTomorrow(t *testing.T) {
	now := time.Date(2026, 8, 6, 8, 0, 0, 0, time.UTC)
	got, err := NextRun(now, "08:00")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 8, 7, 8, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNextRunBadInput(t *testing.T) {
	now := time.Date(2026, 8, 6, 8, 0, 0, 0, time.UTC)
	for _, bad := range []string{"", "25:00", "8am", "08:60"} {
		if _, err := NextRun(now, bad); err == nil {
			t.Fatalf("run time %q should be rejected", bad)
		}
	}
}
