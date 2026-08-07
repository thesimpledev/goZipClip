package main

import (
	"slices"
	"strings"
	"testing"
)

// synthesized ffprobe -of json output for a typical Twitch VOD
const probeFixture = `{
  "streams": [
    {
      "codec_type": "video",
      "codec_name": "h264",
      "width": 1920,
      "height": 1080,
      "avg_frame_rate": "60/1"
    },
    {
      "codec_type": "audio",
      "codec_name": "aac",
      "sample_rate": "44100",
      "channels": 2
    }
  ]
}`

func TestParseProbeParams(t *testing.T) {
	params, parseErr := parseProbeParams([]byte(probeFixture))
	if parseErr != nil {
		t.Fatalf("unexpected error: %v", parseErr)
	}
	if params.Codec != "h264" || params.Width != 1920 || params.Height != 1080 {
		t.Fatalf("video params wrong: %+v", params)
	}
	if params.FrameRate != "60/1" || params.SampleRate != "44100" || params.Channels != 2 {
		t.Fatalf("stream params wrong: %+v", params)
	}
}

func TestParseProbeParamsNoVideo(t *testing.T) {
	fixture := `{"streams":[{"codec_type":"audio","codec_name":"aac","sample_rate":"44100","channels":2}]}`
	if _, parseErr := parseProbeParams([]byte(fixture)); parseErr == nil {
		t.Fatal("expected an error without a video stream")
	}
}

func TestParseProbeParamsBadJSON(t *testing.T) {
	if _, parseErr := parseProbeParams([]byte("not json")); parseErr == nil {
		t.Fatal("expected an error for unparseable output")
	}
}

func TestIntroForConcatFallsBack(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	cfg.IntroFile = "/videos/intro.mp4"
	if got := introForConcat(cfg); got != "/videos/intro.mp4" {
		t.Fatalf("got %q, expected the configured intro", got)
	}
}

func TestIntroForConcatPrefersPrepared(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	cfg.IntroFile = "/videos/intro.mp4"
	ready := introReadyPath(cfg)
	writeTestFile(t, ready)
	if got := introForConcat(cfg); got != ready {
		t.Fatalf("got %q, expected the prepared intro %q", got, ready)
	}
}

func TestPrepareArgsIncludesAudioLayout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IntroFile = "/videos/intro.mp4"
	params := mediaParams{
		Codec: "h264", Width: 1920, Height: 1080,
		FrameRate: "60/1", SampleRate: "44100", Channels: 2,
	}
	args := prepareArgs(cfg, params, "/work/intro_ready.mp4")
	joined := strings.Join(args, " ")
	for _, want := range []string{"scale=1920:1080", "60/1", "44100", "libx264"} {
		if !slices.Contains(args, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
	if args[len(args)-1] != "/work/intro_ready.mp4" {
		t.Fatalf("output path must be last: %s", joined)
	}
}
