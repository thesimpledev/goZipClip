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

func TestSceneScanArgs(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SceneThreshold = 0.4
	cfg.ScanWindowMinutes = 30
	args := sceneScanArgs(cfg, "vod.mp4")
	joined := strings.Join(args, " ")
	for _, want := range []string{
		"-i vod.mp4", "-t 1800", "-an", "-c:v mjpeg", "-f null -",
		"fps=1,scale=320:-1,select='eq(n,0)+gt(scene,0.4)',showinfo",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
}

// synthesized ffmpeg stderr output in the shape showinfo produces
const showinfoFixture = `Input #0, mov,mp4,m4a,3gp,3g2,mj2, from 'vod.mp4':
  Duration: 03:52:10.00, start: 0.000000, bitrate: 6000 kb/s
Stream mapping:
  Stream #0:0 -> #0:0 (h264 (native) -> mjpeg (native))
[Parsed_showinfo_3 @ 0x5591a] n:   0 pts:      0 pts_time:0       duration_time:1 fmt:yuv420p
[Parsed_showinfo_3 @ 0x5591a] n:   1 pts:    850 pts_time:850     duration_time:1 fmt:yuv420p
[Parsed_showinfo_3 @ 0x5591a] n:   2 pts:    866 pts_time:866     duration_time:1 fmt:yuv420p
[out#0/null @ 0x5591b] video:41KiB audio:0KiB subtitle:0KiB
`

func TestSceneChangeTime(t *testing.T) {
	got, found := sceneChangeTime(strings.NewReader(showinfoFixture))
	if !found {
		t.Fatal("expected a timestamp")
	}
	if math.Abs(got-850) > 0.001 {
		t.Fatalf("got %v want 850", got)
	}
}

func TestSceneChangeTimeOnlyOpeningFrame(t *testing.T) {
	fixture := "[Parsed_showinfo_3 @ 0x5591a] n:   0 pts:      0 pts_time:0       duration_time:1 fmt:yuv420p\n"
	if _, found := sceneChangeTime(strings.NewReader(fixture)); found {
		t.Fatal("the opening frame alone is not a scene change")
	}
}

func TestSceneChangeTimeNoMatch(t *testing.T) {
	fixture := "Input #0, mov, from 'vod.mp4':\n  Duration: 03:52:10.00\n"
	if _, found := sceneChangeTime(strings.NewReader(fixture)); found {
		t.Fatal("expected no timestamp")
	}
}

func TestSceneChangeTimeNilReader(t *testing.T) {
	if _, found := sceneChangeTime(nil); found {
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
