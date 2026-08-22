package main

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestScannedChannels(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if IsChannelScanned("example-channel") {
		t.Fatal("nothing is scanned on a fresh install")
	}
	if markErr := MarkChannelScanned("example-channel"); markErr != nil {
		t.Fatalf("mark: %v", markErr)
	}
	if !IsChannelScanned("example-channel") || !IsChannelScanned("Example-Channel") {
		t.Fatal("channel should be recorded, case-insensitively")
	}
	if markErr := MarkChannelScanned("example-channel"); markErr != nil {
		t.Fatalf("second mark: %v", markErr)
	}
	data, readErr := os.ReadFile(scannedChannelsPath())
	if readErr != nil {
		t.Fatalf("read: %v", readErr)
	}
	if strings.Count(string(data), "example-channel") != 1 {
		t.Fatalf("channel recorded more than once: %q", data)
	}
	if MarkChannelScanned("") == nil {
		t.Fatal("empty channel must be rejected")
	}
}

func TestProcessedVODsAndForget(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeTestContent(t, archivePath(), "twitchvod 111\ntwitchvod 222\ntwitchvod 333\n")
	writeTestContent(t, vodNamesPath(), "222\t20260820\tSecond stream\n111\t20260819\tFirst\tstream\n")
	got, listErr := ProcessedVODs()
	if listErr != nil {
		t.Fatalf("list: %v", listErr)
	}
	if len(got) != 3 || got[0].ID != "222" || got[1].ID != "111" || got[2].ID != "333" {
		t.Fatalf("wrong order: %+v", got)
	}
	if got[0].Label() != "2026-08-20  Second stream" {
		t.Fatalf("label got %q", got[0].Label())
	}
	if got[1].Label() != "2026-08-19  First\tstream" {
		t.Fatalf("title with a tab got %q", got[1].Label())
	}
	if got[2].Label() != "id 333" {
		t.Fatalf("unnamed entry got %q", got[2].Label())
	}
	if forgetErr := ForgetVOD("222"); forgetErr != nil {
		t.Fatalf("forget: %v", forgetErr)
	}
	archive, readErr := os.ReadFile(archivePath())
	if readErr != nil {
		t.Fatalf("read archive: %v", readErr)
	}
	if string(archive) != "twitchvod 111\ntwitchvod 333\n" {
		t.Fatalf("archive after forget: %q", archive)
	}
	if _, known := readVODNames()["222"]; known {
		t.Fatal("name of the forgotten VOD must be dropped")
	}
	if IsArchived("222") || !IsArchived("111") {
		t.Fatal("archive membership wrong after forget")
	}
}

func TestForgetVODWithoutFiles(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if forgetErr := ForgetVOD("1"); forgetErr != nil {
		t.Fatalf("missing files must not be an error: %v", forgetErr)
	}
	if ForgetVOD("") == nil {
		t.Fatal("empty id must be rejected")
	}
}

func TestResetArchive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, path := range []string{archivePath(), scannedChannelsPath(), vodNamesPath()} {
		writeTestContent(t, path, "x\n")
	}
	if resetErr := ResetArchive(); resetErr != nil {
		t.Fatalf("reset: %v", resetErr)
	}
	for _, path := range []string{archivePath(), scannedChannelsPath(), vodNamesPath()} {
		if fileExists(path) {
			t.Fatalf("%s should be gone", path)
		}
	}
	if resetErr := ResetArchive(); resetErr != nil {
		t.Fatalf("second reset must be quiet: %v", resetErr)
	}
}

func TestSeedArgsLeaveNewestThree(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg := DefaultConfig()
	cfg.Channel = "example-channel"
	args := seedArgs(cfg)
	if !hasArgPair(args, "--playlist-items", "4:") {
		t.Fatalf("newest three must stay unarchived: %v", args)
	}
	if !hasArgPair(args, "--print-to-file", "%(id)s\t%(upload_date)s\t%(title)s") {
		t.Fatalf("names file flag missing: %v", args)
	}
	if args[len(args)-1] != channelVideosURL(cfg) {
		t.Fatalf("channel URL must come last: %v", args)
	}
}

func TestYtdlpNamesArgsStage(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	args := ytdlpNamesArgs("after_move:")
	if len(args) != 3 || args[0] != "--print-to-file" || !strings.HasPrefix(args[1], "after_move:%(id)s\t") {
		t.Fatalf("got %v", args)
	}
	if args[2] != vodNamesPath() {
		t.Fatalf("names file path got %q", args[2])
	}
}

func TestLookupChannelGuards(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Channel = "example-channel"
	var nilCtx context.Context
	if LookupChannel(nilCtx, cfg, func(string, ...any) {}) == nil {
		t.Fatal("nil context must be rejected")
	}
	if LookupChannel(context.Background(), DefaultConfig(), func(string, ...any) {}) == nil {
		t.Fatal("empty channel must be rejected")
	}
}

func TestLastLine(t *testing.T) {
	if got := lastLine([]byte("first\nERROR: gone\n\n")); got != "ERROR: gone" {
		t.Fatalf("got %q", got)
	}
	if got := lastLine(nil); got != "no output" {
		t.Fatalf("empty got %q", got)
	}
}

func writeTestContent(t *testing.T, path, content string) {
	t.Helper()
	if writeErr := os.WriteFile(path, []byte(content), 0o600); writeErr != nil {
		t.Fatalf("write %s: %v", path, writeErr)
	}
}
