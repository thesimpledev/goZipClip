package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// progressKey is the context key for the optional status callback a
// run carries. Tool helpers report through it and stay silent when a
// caller (a test, the Prepare intro button) attaches nothing.
type progressKey struct{}

// withProgress attaches a status callback to ctx.
func withProgress(ctx context.Context, report func(string)) context.Context {
	if ctx == nil || report == nil {
		return ctx
	}
	return context.WithValue(ctx, progressKey{}, report)
}

// reportProgress sends text to the callback attached by withProgress,
// if any.
func reportProgress(ctx context.Context, text string) {
	if ctx == nil {
		return
	}
	if report, ok := ctx.Value(progressKey{}).(func(string)); ok && report != nil {
		report(text)
	}
}

// hasProgress reports whether ctx carries a status callback.
func hasProgress(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	_, ok := ctx.Value(progressKey{}).(func(string))
	return ok
}

// ytdlpProgressPrefix marks the lines yt-dlp prints from ZipClip's
// progress template, so they can be told apart from its normal output.
const ytdlpProgressPrefix = "ZIPCLIP-PROGRESS "

// ytdlpProgressArgs makes yt-dlp print one machine-readable progress
// line per second instead of a redrawing progress bar.
func ytdlpProgressArgs() []string {
	template := "download:" + ytdlpProgressPrefix +
		"%(info.upload_date)s|%(info.id)s|%(progress._percent_str)s|" +
		"%(progress._total_bytes_str,progress._total_bytes_estimate_str)s|" +
		"%(progress._speed_str)s|%(progress._eta_str)s"
	return []string{"--newline", "--progress-delta", "1", "--progress-template", template}
}

// parseYtdlpProgress turns a ZipClip progress line into status text
// and reports whether the line was one.
func parseYtdlpProgress(line string) (string, bool) {
	rest, found := strings.CutPrefix(strings.TrimSpace(line), ytdlpProgressPrefix)
	if !found {
		return "", false
	}
	parts := strings.Split(rest, "|")
	if len(parts) != 6 {
		return "", false
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return fmt.Sprintf("downloading VOD %s (id %s): %s of %s at %s, %s left",
		formatUploadDate(parts[0]), parts[1], parts[2], parts[3], parts[4], parts[5]), true
}

// formatUploadDate renders yt-dlp's YYYYMMDD as YYYY-MM-DD and leaves
// anything else (such as NA) untouched.
func formatUploadDate(raw string) string {
	if len(raw) != 8 {
		return raw
	}
	for _, r := range raw {
		if r < '0' || r > '9' {
			return raw
		}
	}
	return raw[:4] + "-" + raw[4:6] + "-" + raw[6:]
}

var playlistItemPattern = regexp.MustCompile(`Downloading item (\d+) of (\d+)`)

// parsePlaylistItem reads yt-dlp's "[download] Downloading item N of M"
// line and reports whether the line was one.
func parsePlaylistItem(line string) (int, int, bool) {
	match := playlistItemPattern.FindStringSubmatch(line)
	if match == nil {
		return 0, 0, false
	}
	n, nErr := strconv.Atoi(match[1])
	m, mErr := strconv.Atoi(match[2])
	if nErr != nil || mErr != nil {
		return 0, 0, false
	}
	return n, m, true
}

// handleYtdlpLine routes one line of yt-dlp output: progress lines go
// to the status callback, everything else to the log.
func handleYtdlpLine(ctx context.Context, logf func(string, ...any), line string) {
	if text, isProgress := parseYtdlpProgress(line); isProgress {
		reportProgress(ctx, text)
		return
	}
	if strings.TrimSpace(line) != "" && logf != nil {
		logf("yt-dlp: %s", line)
	}
}

// handleCatalogLine routes one line of output from the channel
// catalog. Per-VOD chatter only moves the status line; warnings and
// errors still reach the log.
func handleCatalogLine(ctx context.Context, logf func(string, ...any), channel, line string) {
	if n, m, isItem := parsePlaylistItem(line); isItem {
		reportProgress(ctx, fmt.Sprintf("cataloging %s: %d of %d", channel, n, m))
		return
	}
	if logf != nil && (strings.Contains(line, "ERROR") || strings.Contains(line, "WARNING")) {
		logf("yt-dlp: %s", line)
	}
}

// ffmpegStatus accumulates one block of ffmpeg -progress output.
type ffmpegStatus struct {
	outTime float64
	speed   string
}

// consume reads one -progress line and reports whether it closed a
// block (a "progress=" line), meaning the status is ready to show.
func (s *ffmpegStatus) consume(line string) bool {
	key, value, found := strings.Cut(strings.TrimSpace(line), "=")
	if !found {
		return false
	}
	switch key {
	case "out_time_us":
		if us, parseErr := strconv.ParseFloat(value, 64); parseErr == nil && us >= 0 {
			s.outTime = us / 1e6
		}
	case "out_time":
		if secs, parseErr := parseTimestamp(value); parseErr == nil {
			s.outTime = secs
		}
	case "speed":
		s.speed = strings.TrimSpace(value)
	case "progress":
		return true
	}
	return false
}

// text renders the status as "label: done of total (pct%) at speed",
// dropping the parts that are unknown.
func (s *ffmpegStatus) text(label string, total float64) string {
	out := label + ": " + formatClock(s.outTime)
	if total > 0 {
		pct := min(int(s.outTime/total*100), 100)
		out += fmt.Sprintf(" of %s (%d%%)", formatClock(total), pct)
	}
	if s.speed != "" && s.speed != "N/A" {
		out += " at " + s.speed
	}
	return out
}

// formatClock renders seconds as H:MM:SS.
func formatClock(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	whole := int(seconds)
	return fmt.Sprintf("%d:%02d:%02d", whole/3600, (whole%3600)/60, whole%60)
}

// ffmpegProgressHandler returns a line handler for ffmpeg's -progress
// output that reports "label: done of total" through ctx. It returns
// nil, meaning no progress is wanted, when ctx carries no callback;
// total is only computed when it is.
func ffmpegProgressHandler(ctx context.Context, label string, total func() float64) func(string) {
	if !hasProgress(ctx) || total == nil {
		return nil
	}
	known := total()
	status := &ffmpegStatus{}
	return func(line string) {
		if status.consume(line) {
			reportProgress(ctx, status.text(label, known))
		}
	}
}

// ffmpegProgressArgs are the flags that make ffmpeg print progress
// blocks on stdout while its log stays on stderr.
func ffmpegProgressArgs() []string {
	return []string{"-progress", "pipe:1"}
}
