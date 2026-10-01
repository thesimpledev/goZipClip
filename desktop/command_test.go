package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewCommandKeepsPathAndArgs(t *testing.T) {
	cmd := newCommand(context.Background(), "sh", "-c", "true")
	if len(cmd.Args) != 3 || cmd.Args[0] != "sh" || cmd.Args[2] != "true" {
		t.Fatalf("args got %v", cmd.Args)
	}
	if !strings.HasSuffix(strings.TrimSuffix(cmd.Path, ".exe"), "sh") {
		t.Fatalf("path got %q", cmd.Path)
	}
}

func TestRunStreamingMergesBothStreams(t *testing.T) {
	cmd := newCommand(context.Background(), "sh", "-c", "echo out; echo err 1>&2")
	var lines []string
	if runErr := runStreaming(cmd, func(line string) { lines = append(lines, line) }); runErr != nil {
		t.Fatalf("run: %v", runErr)
	}
	joined := strings.Join(lines, ",")
	if !strings.Contains(joined, "out") || !strings.Contains(joined, "err") {
		t.Fatalf("lines got %v", lines)
	}
}

func TestCancelStopsToolAndItsChildren(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := newCommand(ctx, "sh", "-c", "sleep 60; true")
	done := make(chan error, 1)
	go func() { done <- runStreaming(cmd, func(string) {}) }()
	time.Sleep(500 * time.Millisecond)
	cancel()
	select {
	case runErr := <-done:
		if runErr == nil {
			t.Fatal("a cancelled tool must report an error")
		}
	case <-time.After(cancelWaitDelay + 10*time.Second):
		t.Fatal("the run was still waiting on the cancelled tool")
	}
}

func TestRunStreamingReportsExitError(t *testing.T) {
	cmd := newCommand(context.Background(), "sh", "-c", "exit 3")
	if runErr := runStreaming(cmd, func(string) {}); runErr == nil {
		t.Fatal("expected the exit error")
	}
	if runStreaming(nil, func(string) {}) == nil {
		t.Fatal("nil command must be rejected")
	}
}

func TestRunCapturingStderr(t *testing.T) {
	cmd := newCommand(context.Background(), "sh", "-c", "echo progress; echo log 1>&2")
	var lines []string
	stderr, runErr := runCapturingStderr(cmd, func(line string) { lines = append(lines, line) })
	if runErr != nil {
		t.Fatalf("run: %v", runErr)
	}
	if len(lines) != 1 || lines[0] != "progress" {
		t.Fatalf("stdout lines got %v", lines)
	}
	if strings.TrimSpace(string(stderr)) != "log" {
		t.Fatalf("stderr got %q", stderr)
	}
}

func TestRunCapturingStderrWithoutHandler(t *testing.T) {
	cmd := newCommand(context.Background(), "sh", "-c", "echo ignored; echo kept 1>&2")
	stderr, runErr := runCapturingStderr(cmd, nil)
	if runErr != nil {
		t.Fatalf("run: %v", runErr)
	}
	if strings.TrimSpace(string(stderr)) != "kept" {
		t.Fatalf("stderr got %q", stderr)
	}
	if _, guardErr := runCapturingStderr(nil, nil); guardErr == nil {
		t.Fatal("nil command must be rejected")
	}
}

func TestStreamLines(t *testing.T) {
	var lines []string
	if scanErr := streamLines(strings.NewReader("a\nb\n"), func(line string) { lines = append(lines, line) }); scanErr != nil {
		t.Fatalf("scan: %v", scanErr)
	}
	if len(lines) != 2 || lines[0] != "a" || lines[1] != "b" {
		t.Fatalf("got %v", lines)
	}
	if streamLines(nil, nil) == nil {
		t.Fatal("missing reader must be rejected")
	}
}
