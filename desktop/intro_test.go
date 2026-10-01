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

func TestParseProbeParamsReadsTimeScale(t *testing.T) {
	fixture := `{"streams":[{"codec_type":"video","codec_name":"h264","width":1280,"height":720,` +
		`"avg_frame_rate":"30/1","time_base":"1/90000"}]}`
	params, parseErr := parseProbeParams([]byte(fixture))
	if parseErr != nil {
		t.Fatalf("unexpected error: %v", parseErr)
	}
	if params.TimeScale != "90000" {
		t.Fatalf("got time scale %q, want 90000", params.TimeScale)
	}
}

func TestTimeScale(t *testing.T) {
	cases := map[string]string{
		"1/90000": "90000",
		"1/15360": "15360",
		"":        "",
		"90000":   "",
		"2/90000": "",
		"1/abc":   "",
	}
	for input, want := range cases {
		if got := timeScale(input); got != want {
			t.Fatalf("%q: got %q want %q", input, got, want)
		}
	}
}

func TestJoinsWith(t *testing.T) {
	vod := mediaParams{
		Codec: "h264", Width: 1280, Height: 720,
		FrameRate: "30/1", TimeScale: "90000", SampleRate: "48000", Channels: 2,
	}
	same := vod
	same.FrameRate = "387162000/12905401"
	if !same.joinsWith(vod) {
		t.Fatal("a different frame rate spelling must not stop the join")
	}
	otherScale := vod
	otherScale.TimeScale = "15360"
	if otherScale.joinsWith(vod) {
		t.Fatal("a different video timescale must stop the join")
	}
	otherSize := vod
	otherSize.Height = 1080
	if otherSize.joinsWith(vod) {
		t.Fatal("a different resolution must stop the join")
	}
}

func TestIntroNeedsPrepareWithoutPreparedCopy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	stale, checkErr := introNeedsPrepare(t.Context(), cfg, "vod.mp4")
	if checkErr != nil {
		t.Fatalf("unexpected error: %v", checkErr)
	}
	if !stale {
		t.Fatal("a missing prepared intro must be reported as needing preparation")
	}
}

func TestPrepareArgsIncludesAudioLayout(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IntroFile = "/videos/intro.mp4"
	params := mediaParams{
		Codec: "h264", Width: 1920, Height: 1080,
		FrameRate: "60/1", TimeScale: "90000", SampleRate: "44100", Channels: 2,
	}
	args := prepareArgs(cfg, params, "/work/intro_ready.mp4")
	joined := strings.Join(args, " ")
	for _, want := range []string{"scale=1920:1080", "60/1", "44100", "libx264", "-video_track_timescale", "90000"} {
		if !slices.Contains(args, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
	if args[len(args)-1] != "/work/intro_ready.mp4" {
		t.Fatalf("output path must be last: %s", joined)
	}
}
