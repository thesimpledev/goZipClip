package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestOutputPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OutputDir = filepath.Join("videos", "upload")
	vod := filepath.Join("videos", "work", "raw", "20260101-v123.mp4")
	want := filepath.Join("videos", "upload", "20260101-v123.mp4")
	if got := outputPath(cfg, vod); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestOutputPathReplacesExtension(t *testing.T) {
	cfg := DefaultConfig()
	cfg.OutputDir = "out"
	if got := outputPath(cfg, "raw/20260101-v123.mkv"); got != filepath.Join("out", "20260101-v123.mp4") {
		t.Fatalf("extension not replaced: %q", got)
	}
}

func TestCancelWithoutRunIsQuiet(t *testing.T) {
	p := NewPipeline(&ConfigStore{}, &Logger{})
	p.Cancel()
	if p.Running() {
		t.Fatal("nothing should be running")
	}
}

func TestBeginCancelEnd(t *testing.T) {
	p := NewPipeline(&ConfigStore{}, &Logger{})
	runCtx, ok := p.begin(context.Background())
	if !ok || !p.Running() {
		t.Fatal("begin should claim the pipeline")
	}
	if !hasProgress(runCtx) {
		t.Fatal("run context must carry the progress hook")
	}
	if _, again := p.begin(context.Background()); again {
		t.Fatal("a second begin must be refused while running")
	}
	p.Cancel()
	select {
	case <-runCtx.Done():
	default:
		t.Fatal("Cancel must end the run context")
	}
	p.end()
	if p.Running() {
		t.Fatal("end should release the pipeline")
	}
}

func TestSettleMarksCancelledRuns(t *testing.T) {
	p := NewPipeline(&ConfigStore{}, &Logger{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := p.settle(ctx, errors.New("signal: killed")); !errors.Is(got, context.Canceled) {
		t.Fatalf("cancelled run got %v", got)
	}
	if state, detail := p.Status(); state != StateIdle || detail != "cancelled" {
		t.Fatalf("status got %v %q", state, detail)
	}
	plain := errors.New("boom")
	if got := p.settle(context.Background(), plain); got != plain {
		t.Fatalf("live run error changed to %v", got)
	}
	if got := p.settle(ctx, nil); got != nil {
		t.Fatalf("a clean finish must stay clean, got %v", got)
	}
}

func TestProgressUpdatesDetailOnly(t *testing.T) {
	logger := &Logger{}
	p := NewPipeline(&ConfigStore{}, logger)
	p.progress("downloading VOD: 10%")
	if _, detail := p.Status(); detail != "downloading VOD: 10%" {
		t.Fatalf("detail got %q", detail)
	}
	if len(logger.Recent()) != 0 {
		t.Fatal("progress must not write log lines")
	}
	if StateCataloging.String() != "cataloging" {
		t.Fatal("state name changed")
	}
}
