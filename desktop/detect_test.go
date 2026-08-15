package main

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestNoSceneChangeIsSentinel(t *testing.T) {
	detectErr := noSceneChange(DefaultConfig())
	if !errors.Is(detectErr, errNoSceneChange) {
		t.Fatalf("expected the sentinel, got %v", detectErr)
	}
	if !strings.Contains(detectErr.Error(), "no scene change") {
		t.Fatalf("unexpected message: %v", detectErr)
	}
}

// synthesized ffmpeg stderr output in the shape showinfo produces
const showinfoFixture = `Input #0, mov,mp4,m4a,3gp,3g2,mj2, from 'vod.mp4':
  Duration: 03:52:10.00, start: 0.000000, bitrate: 6000 kb/s
Stream mapping:
  Stream #0:0 -> #0:0 (h264 (native) -> wrapped_avframe (native))
[Parsed_showinfo_2 @ 0x5591a] n:   0 pts: 776448 pts_time:849.5   duration_time:0.016667 fmt:yuv420p
[Parsed_showinfo_2 @ 0x5591a] n:   1 pts: 792115 pts_time:866.375 duration_time:0.016667 fmt:yuv420p
[out#0/null @ 0x5591b] video:41KiB audio:0KiB subtitle:0KiB
`

func TestFirstPtsTime(t *testing.T) {
	got, found := firstPtsTime(strings.NewReader(showinfoFixture))
	if !found {
		t.Fatal("expected a timestamp")
	}
	if math.Abs(got-849.5) > 0.001 {
		t.Fatalf("got %v want 849.5", got)
	}
}

func TestFirstPtsTimeNoMatch(t *testing.T) {
	fixture := "Input #0, mov, from 'vod.mp4':\n  Duration: 03:52:10.00\n"
	if _, found := firstPtsTime(strings.NewReader(fixture)); found {
		t.Fatal("expected no timestamp")
	}
}

func TestFirstPtsTimeNilReader(t *testing.T) {
	if _, found := firstPtsTime(nil); found {
		t.Fatal("expected no timestamp from a nil reader")
	}
}

func TestParseTimestamp(t *testing.T) {
	cases := map[string]float64{
		"90":         90,
		"01:30":      90,
		"1:02:03":    3723,
		"00:14:32.5": 872.5,
		" 45 ":       45,
	}
	for input, want := range cases {
		got, err := parseTimestamp(input)
		if err != nil {
			t.Fatalf("%q: unexpected error %v", input, err)
		}
		if math.Abs(got-want) > 0.001 {
			t.Fatalf("%q: got %v want %v", input, got, want)
		}
	}
}

func TestParseTimestampRejectsBadInput(t *testing.T) {
	for _, bad := range []string{"", "a", "1:2:3:4", "-5", "1:-2"} {
		if _, err := parseTimestamp(bad); err == nil {
			t.Fatalf("%q should be rejected", bad)
		}
	}
}

func TestFormatTimestamp(t *testing.T) {
	cases := map[float64]string{
		0:      "00:00:00.0",
		872.5:  "00:14:32.5",
		3723:   "01:02:03.0",
		-10:    "00:00:00.0",
		7325.4: "02:02:05.4",
	}
	for input, want := range cases {
		if got := formatTimestamp(input); got != want {
			t.Fatalf("%v: got %q want %q", input, got, want)
		}
	}
}

func TestFormatParseRoundTrip(t *testing.T) {
	for _, seconds := range []float64{0, 61.5, 872.5, 3600, 7325.4} {
		parsed, err := parseTimestamp(formatTimestamp(seconds))
		if err != nil {
			t.Fatalf("%v: %v", seconds, err)
		}
		if math.Abs(parsed-seconds) > 0.05 {
			t.Fatalf("%v round-tripped to %v", seconds, parsed)
		}
	}
}
