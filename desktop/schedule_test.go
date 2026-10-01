package main

import (
	"context"
	"testing"
	"time"
)

func TestNextRunLaterToday(t *testing.T) {
	now := time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC)
	got, err := NextRun(now, "8:00 AM")
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
	got, err := NextRun(now, "8:00 AM")
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
	got, err := NextRun(now, "8:00 AM")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 8, 7, 8, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNextRunEvening(t *testing.T) {
	now := time.Date(2026, 8, 6, 7, 0, 0, 0, time.UTC)
	got, err := NextRun(now, "8:30 pm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Date(2026, 8, 6, 20, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestWaitUntilWaitsOutAnEarlyTimer(t *testing.T) {
	target := time.Now().Add(150 * time.Millisecond)
	if !waitUntil(t.Context(), target) {
		t.Fatal("a live context must reach the target")
	}
	if time.Now().Before(target) {
		t.Fatal("waitUntil returned before the target time")
	}
	if !waitUntil(t.Context(), time.Now().Add(-time.Minute)) {
		t.Fatal("a target in the past must return at once")
	}
}

func TestWaitUntilStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitUntil(ctx, time.Now().Add(time.Hour)) {
		t.Fatal("a cancelled context must stop the wait")
	}
}

func TestNextRunBadInput(t *testing.T) {
	now := time.Date(2026, 8, 6, 8, 0, 0, 0, time.UTC)
	for _, bad := range []string{"", "25:00", "8am", "08:00", "8:60 PM"} {
		if _, err := NextRun(now, bad); err == nil {
			t.Fatalf("run time %q should be rejected", bad)
		}
	}
}
