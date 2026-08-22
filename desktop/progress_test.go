package main

import (
	"context"
	"testing"
)

func TestParseYtdlpProgress(t *testing.T) {
	line := "ZIPCLIP-PROGRESS 20260821|2345678901|  43.2%|3.20GiB|12.1MiB/s|04:10"
	text, ok := parseYtdlpProgress(line)
	if !ok {
		t.Fatal("expected a progress line")
	}
	want := "downloading VOD 2026-08-21 (id 2345678901): 43.2% of 3.20GiB at 12.1MiB/s, 04:10 left"
	if text != want {
		t.Fatalf("got %q want %q", text, want)
	}
}

func TestParseYtdlpProgressIgnoresOtherLines(t *testing.T) {
	for _, line := range []string{"[download] Destination: raw/x.mp4", "", "ZIPCLIP-PROGRESS only|two"} {
		if _, ok := parseYtdlpProgress(line); ok {
			t.Fatalf("%q must not parse as progress", line)
		}
	}
}

func TestParsePlaylistItem(t *testing.T) {
	n, m, ok := parsePlaylistItem("[download] Downloading item 3 of 57")
	if !ok || n != 3 || m != 57 {
		t.Fatalf("got %d of %d (%v)", n, m, ok)
	}
	if _, _, ok := parsePlaylistItem("[download] Destination: x"); ok {
		t.Fatal("destination line must not parse as an item count")
	}
}

func TestFormatUploadDate(t *testing.T) {
	if got := formatUploadDate("20260821"); got != "2026-08-21" {
		t.Fatalf("got %q", got)
	}
	if got := formatUploadDate("NA"); got != "NA" {
		t.Fatalf("NA must pass through, got %q", got)
	}
}

func TestFfmpegStatus(t *testing.T) {
	status := &ffmpegStatus{}
	for _, line := range []string{"frame=100", "out_time_us=754000000", "speed=32.1x"} {
		if status.consume(line) {
			t.Fatalf("%q must not close a block", line)
		}
	}
	if !status.consume("progress=continue") {
		t.Fatal("progress= must close the block")
	}
	want := "cutting a.mp4: 0:12:34 of 0:50:00 (25%) at 32.1x"
	if got := status.text("cutting a.mp4", 3000); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got := status.text("cutting a.mp4", 0); got != "cutting a.mp4: 0:12:34 at 32.1x" {
		t.Fatalf("without a total got %q", got)
	}
}

func TestFfmpegStatusOutTimeFallback(t *testing.T) {
	status := &ffmpegStatus{}
	_ = status.consume("out_time=00:01:05.500000")
	if status.outTime != 65.5 {
		t.Fatalf("got %v", status.outTime)
	}
}

func TestProgressHook(t *testing.T) {
	reportProgress(context.Background(), "nothing listens")
	if hasProgress(context.Background()) {
		t.Fatal("bare context must not report a hook")
	}
	var got string
	ctx := withProgress(context.Background(), func(text string) { got = text })
	if !hasProgress(ctx) {
		t.Fatal("hook not attached")
	}
	reportProgress(ctx, "hello")
	if got != "hello" {
		t.Fatalf("hook got %q", got)
	}
	if ffmpegProgressHandler(context.Background(), "x", func() float64 { return 1 }) != nil {
		t.Fatal("handler must be nil without a hook")
	}
}

func TestHandleYtdlpLineRoutes(t *testing.T) {
	var reported, logged string
	ctx := withProgress(context.Background(), func(text string) { reported = text })
	logf := func(format string, _ ...any) { logged = format }
	handleYtdlpLine(ctx, logf, "ZIPCLIP-PROGRESS 20260821|1|1%|2|3|4")
	if reported == "" || logged != "" {
		t.Fatalf("progress line misrouted: reported %q logged %q", reported, logged)
	}
	handleYtdlpLine(ctx, logf, "[download] Destination: x")
	if logged != "yt-dlp: %s" {
		t.Fatalf("plain line not logged: %q", logged)
	}
	handleCatalogLine(ctx, logf, "example-channel", "[download] Downloading item 2 of 9")
	if reported != "cataloging example-channel: 2 of 9" {
		t.Fatalf("catalog progress got %q", reported)
	}
}

func TestFormatClock(t *testing.T) {
	if got := formatClock(3723); got != "1:02:03" {
		t.Fatalf("got %q", got)
	}
	if got := formatClock(-5); got != "0:00:00" {
		t.Fatalf("negative got %q", got)
	}
}
