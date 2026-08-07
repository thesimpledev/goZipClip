package main

import (
	"math"
	"testing"
)

func TestVerifyDurationWithinTolerance(t *testing.T) {
	if err := VerifyDuration(15, 13000, 13014, 3); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerifyDurationOutsideTolerance(t *testing.T) {
	if err := VerifyDuration(15, 13000, 12900, 3); err == nil {
		t.Fatal("expected an error for a short output")
	}
}

func TestVerifyDurationRejectsNonPositive(t *testing.T) {
	if err := VerifyDuration(0, 13000, 13015, 3); err == nil {
		t.Fatal("expected an error for a zero intro duration")
	}
	if err := VerifyDuration(15, -1, 14, 3); err == nil {
		t.Fatal("expected an error for a negative trimmed duration")
	}
}

func TestConcatEntryEscapesQuotes(t *testing.T) {
	got := concatEntry("/videos/it's here.mp4")
	want := "file '/videos/it'\\''s here.mp4'\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestConcatEntryPlainPath(t *testing.T) {
	got := concatEntry("/videos/intro.mp4")
	want := "file '/videos/intro.mp4'\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestParseDurationOutput(t *testing.T) {
	got, err := parseDurationOutput("13950.123456\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if math.Abs(got-13950.123456) > 0.001 {
		t.Fatalf("got %v", got)
	}
}

func TestParseDurationOutputRejectsBadInput(t *testing.T) {
	for _, bad := range []string{"", "N/A", "-5", "0"} {
		if _, err := parseDurationOutput(bad); err == nil {
			t.Fatalf("%q should be rejected", bad)
		}
	}
}
