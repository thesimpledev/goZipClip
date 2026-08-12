package main

import "testing"

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("got %q", got)
	}
	if got := truncate("0123456789abc", 10); got != "0123456789..." {
		t.Fatalf("got %q", got)
	}
}
