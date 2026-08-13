package main

import "testing"

func TestFindSum(t *testing.T) {
	data := []byte("1111aaaa  some-other-file.zip\n2222BBBB  yt-dlp.exe\n")
	got, findErr := findSum(data, "yt-dlp.exe")
	if findErr != nil {
		t.Fatalf("findSum: %v", findErr)
	}
	if got != "2222bbbb" {
		t.Fatalf("got %q, want lowercased digest", got)
	}
}

func TestFindSumBinaryModeMarker(t *testing.T) {
	data := []byte("3333cccc *yt-dlp.exe\n")
	got, findErr := findSum(data, "yt-dlp.exe")
	if findErr != nil {
		t.Fatalf("findSum: %v", findErr)
	}
	if got != "3333cccc" {
		t.Fatalf("got %q", got)
	}
}

func TestFindSumMissingName(t *testing.T) {
	data := []byte("4444dddd  some-other-file.zip\n")
	if _, findErr := findSum(data, "yt-dlp.exe"); findErr == nil {
		t.Fatal("expected an error for a missing entry")
	}
}

func TestFindSumEmptyName(t *testing.T) {
	if _, findErr := findSum([]byte("5555eeee  x\n"), ""); findErr == nil {
		t.Fatal("expected an error for an empty name")
	}
}
