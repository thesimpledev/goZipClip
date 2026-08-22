package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// introReadyPath is where the re-encoded, VOD-matched intro lives.
func introReadyPath(cfg Config) string {
	return filepath.Join(cfg.WorkDir, "intro_ready.mp4")
}

// introForConcat prefers the prepared intro when it exists, falling
// back to the configured intro file.
func introForConcat(cfg Config) string {
	ready := introReadyPath(cfg)
	if fileExists(ready) {
		return ready
	}
	return cfg.IntroFile
}

// mediaParams are the stream properties both concat inputs must share
// for stream copying to produce a playable file.
type mediaParams struct {
	Codec      string
	Width      int
	Height     int
	FrameRate  string
	SampleRate string
	Channels   int
}

type probeStream struct {
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	AvgFrameRate string `json:"avg_frame_rate"`
	SampleRate   string `json:"sample_rate"`
	Channels     int    `json:"channels"`
}

// probeParams reads the stream parameters of path.
func probeParams(ctx context.Context, cfg Config, path string) (mediaParams, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "stream=codec_type,codec_name,width,height,avg_frame_rate,sample_rate,channels",
		"-of", "json",
		path,
	}
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := newCommand(ctx, resolveFfprobe(cfg), args...)
	out, runErr := cmd.Output()
	if runErr != nil {
		return mediaParams{}, fmt.Errorf("ffprobe %s: %w", filepath.Base(path), runErr)
	}
	return parseProbeParams(out)
}

// parseProbeParams extracts the first video and audio stream
// parameters from ffprobe JSON output.
func parseProbeParams(out []byte) (mediaParams, error) {
	var parsed struct {
		Streams []probeStream `json:"streams"`
	}
	if jsonErr := json.Unmarshal(out, &parsed); jsonErr != nil {
		return mediaParams{}, fmt.Errorf("ffprobe output: %w", jsonErr)
	}
	var params mediaParams
	for _, stream := range parsed.Streams {
		params.apply(stream)
	}
	if params.Codec == "" || params.Width <= 0 || params.Height <= 0 {
		return mediaParams{}, errors.New("no video stream found")
	}
	return params, nil
}

func (m *mediaParams) apply(s probeStream) {
	if s.CodecType == "video" && m.Codec == "" {
		m.Codec = s.CodecName
		m.Width = s.Width
		m.Height = s.Height
		m.FrameRate = s.AvgFrameRate
	}
	if s.CodecType == "audio" && m.SampleRate == "" {
		m.SampleRate = s.SampleRate
		m.Channels = s.Channels
	}
}

// newestRawVod returns the most recently modified downloaded VOD,
// used as the reference for intro preparation.
func newestRawVod(cfg Config) (string, error) {
	rawDir := filepath.Join(cfg.WorkDir, "raw")
	files, listErr := listVideoFiles(rawDir)
	if listErr != nil {
		return "", fmt.Errorf("no downloaded VODs yet: %w", listErr)
	}
	newest := ""
	var newestTime time.Time
	for path := range files {
		info, statErr := os.Stat(path)
		if statErr != nil {
			continue
		}
		if newest == "" || info.ModTime().After(newestTime) {
			newest = path
			newestTime = info.ModTime()
		}
	}
	if newest == "" {
		return "", errors.New("no downloaded VOD to match against; run the pipeline once first")
	}
	return newest, nil
}

// PrepareIntro re-encodes the configured intro to match the most
// recent VOD's codec, resolution, framerate, and audio layout so the
// final concat can stream-copy.
func PrepareIntro(ctx context.Context, cfg Config, logf func(string, ...any)) error {
	if ctx == nil || logf == nil {
		return errors.New("missing context or logger")
	}
	if cfg.IntroFile == "" || !fileExists(cfg.IntroFile) {
		return errors.New("set a valid intro file first")
	}
	ref, refErr := newestRawVod(cfg)
	if refErr != nil {
		return refErr
	}
	params, probeErr := probeParams(ctx, cfg, ref)
	if probeErr != nil {
		return probeErr
	}
	if params.Codec != "h264" {
		return fmt.Errorf("reference VOD codec is %s, not h264; prepare the intro manually", params.Codec)
	}
	out := introReadyPath(cfg)
	cmd := newCommand(ctx, resolveFfmpeg(cfg), prepareArgs(cfg, params, out)...)
	handler := ffmpegProgressHandler(ctx, "preparing the intro",
		func() float64 { return combinedDuration(ctx, cfg, cfg.IntroFile) })
	output, runErr := runCapturingStderr(cmd, handler)
	if runErr != nil {
		return fmt.Errorf("ffmpeg intro encode: %w: %s", runErr, truncate(string(output), 300))
	}
	logf("prepared intro at %s (%dx%d @ %s)", out, params.Width, params.Height, params.FrameRate)
	return nil
}

func prepareArgs(cfg Config, params mediaParams, outPath string) []string {
	args := []string{"-hide_banner", "-nostats", "-y"}
	args = append(args, ffmpegProgressArgs()...)
	args = append(args,
		"-i", cfg.IntroFile,
		"-vf", "scale="+strconv.Itoa(params.Width)+":"+strconv.Itoa(params.Height),
		"-r", params.FrameRate,
		"-c:v", "libx264", "-preset", "medium", "-profile:v", "high", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "160k",
	)
	if params.SampleRate != "" {
		args = append(args, "-ar", params.SampleRate)
	}
	if params.Channels > 0 {
		args = append(args, "-ac", strconv.Itoa(params.Channels))
	}
	return append(args, outPath)
}
