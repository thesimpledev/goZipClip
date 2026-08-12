package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// State names the pipeline's current activity.
type State int

// Pipeline states, in rough order of a run.
const (
	StateIdle State = iota
	StateDownloading
	StateDetecting
	StateAwaitingApproval
	StateSplicing
	StateVerifying
	StateError
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateDownloading:
		return "downloading"
	case StateDetecting:
		return "detecting stream start"
	case StateAwaitingApproval:
		return "awaiting approval"
	case StateSplicing:
		return "splicing"
	case StateVerifying:
		return "verifying"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// PendingCut is a detected cut waiting for human approval.
type PendingCut struct {
	VodPath string
	Cut     float64
	Preview string
}

// Pipeline drives one VOD through download, detection, splice,
// verification, and cleanup.
type Pipeline struct {
	store  *ConfigStore
	logger *Logger

	mu       sync.Mutex
	state    State
	detail   string
	running  bool
	pending  *PendingCut
	approve  chan float64
	onChange func()
}

// NewPipeline wires a pipeline to the shared config and logger.
func NewPipeline(store *ConfigStore, logger *Logger) *Pipeline {
	return &Pipeline{store: store, logger: logger, approve: make(chan float64)}
}

// SetOnChange registers a callback fired on every state change. The
// GUI sets it once before any run starts.
func (p *Pipeline) SetOnChange(fn func()) {
	p.mu.Lock()
	p.onChange = fn
	p.mu.Unlock()
}

// Status returns the current state and detail text.
func (p *Pipeline) Status() (State, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.state, p.detail
}

// Pending returns a copy of the cut awaiting approval, or nil.
func (p *Pipeline) Pending() *PendingCut {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending == nil {
		return nil
	}
	copyOf := *p.pending
	return &copyOf
}

func (p *Pipeline) setState(s State, detail string) {
	p.mu.Lock()
	p.state = s
	p.detail = detail
	fn := p.onChange
	p.mu.Unlock()
	if detail != "" {
		p.logger.Logf("%s: %s", s, detail)
	} else {
		p.logger.Logf("%s", s)
	}
	if fn != nil {
		fn()
	}
}

func (p *Pipeline) begin() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return false
	}
	p.running = true
	return true
}

func (p *Pipeline) end() {
	p.mu.Lock()
	p.running = false
	p.pending = nil
	p.mu.Unlock()
}

// Run executes one full pass: download, then processing of every new
// VOD.
func (p *Pipeline) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	if !p.begin() {
		return errors.New("a run is already in progress")
	}
	defer p.end()
	cfg := p.store.Get()
	if problems := cfg.Validate(); len(problems) > 0 {
		p.setState(StateError, "configuration incomplete: "+strings.Join(problems, "; "))
		return errors.New("configuration incomplete")
	}
	files, downloadErr := p.download(ctx, cfg)
	if downloadErr != nil {
		return downloadErr
	}
	if len(files) == 0 {
		p.setState(StateIdle, "no new VODs")
		return nil
	}
	for _, vod := range files {
		if procErr := p.processOne(ctx, cfg, vod); procErr != nil {
			return procErr
		}
	}
	p.setState(StateIdle, fmt.Sprintf("finished %d VOD(s)", len(files)))
	return nil
}

// RunLatest downloads and processes the newest VOD on the channel,
// even when the archive already lists it as handled.
func (p *Pipeline) RunLatest(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	if !p.begin() {
		return errors.New("a run is already in progress")
	}
	defer p.end()
	cfg := p.store.Get()
	if problems := cfg.Validate(); len(problems) > 0 {
		p.setState(StateError, "configuration incomplete: "+strings.Join(problems, "; "))
		return errors.New("configuration incomplete")
	}
	p.setState(StateDownloading, "latest VOD on "+cfg.Channel)
	vod, dlErr := DownloadLatest(ctx, cfg, p.logger.Logf)
	if dlErr != nil {
		p.setState(StateError, dlErr.Error())
		return dlErr
	}
	if procErr := p.processOne(ctx, cfg, vod); procErr != nil {
		return procErr
	}
	p.setState(StateIdle, "finished latest VOD")
	return nil
}

func (p *Pipeline) download(ctx context.Context, cfg Config) ([]string, error) {
	p.setState(StateDownloading, cfg.Channel)
	files, dlErr := DownloadNew(ctx, cfg, p.logger.Logf)
	if dlErr != nil {
		p.setState(StateError, dlErr.Error())
		return nil, dlErr
	}
	return files, nil
}

func (p *Pipeline) processOne(ctx context.Context, cfg Config, vodPath string) error {
	cut, cutErr := p.resolveCut(ctx, cfg, vodPath)
	if cutErr != nil {
		p.setState(StateError, cutErr.Error())
		return cutErr
	}
	outPath, spliceErr := p.splice(ctx, cfg, vodPath, cut)
	if spliceErr != nil {
		p.setState(StateError, spliceErr.Error())
		return spliceErr
	}
	p.logger.Logf("finished %s", outPath)
	if !cfg.DevMode {
		p.removeIntermediates(cfg, vodPath)
	}
	return nil
}

// resolveCut detects the stream start and, in dev mode, waits for the
// human to approve or adjust it.
func (p *Pipeline) resolveCut(ctx context.Context, cfg Config, vodPath string) (float64, error) {
	p.setState(StateDetecting, filepath.Base(vodPath))
	cut, detectErr := DetectCut(ctx, cfg, vodPath)
	if detectErr != nil {
		return 0, detectErr
	}
	preview := filepath.Join(cfg.WorkDir, "preview.jpg")
	if prevErr := ExtractPreview(ctx, cfg, vodPath, cut, preview); prevErr != nil {
		p.logger.Logf("preview failed (continuing): %v", prevErr)
		preview = ""
	}
	if !cfg.DevMode {
		return cut, nil
	}
	return p.waitForApproval(ctx, PendingCut{VodPath: vodPath, Cut: cut, Preview: preview})
}

func (p *Pipeline) waitForApproval(ctx context.Context, pending PendingCut) (float64, error) {
	p.mu.Lock()
	p.pending = &pending
	p.mu.Unlock()
	p.setState(StateAwaitingApproval,
		fmt.Sprintf("%s at %s", filepath.Base(pending.VodPath), formatTimestamp(pending.Cut)))
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case approved := <-p.approve:
		p.mu.Lock()
		p.pending = nil
		p.mu.Unlock()
		return approved, nil
	}
}

// Approve delivers the final cut time to a pipeline waiting in dev
// mode. It reports whether anything was waiting.
func (p *Pipeline) Approve(cut float64) bool {
	select {
	case p.approve <- cut:
		return true
	default:
		return false
	}
}

// RegeneratePreview re-extracts the preview frame at a new timestamp
// for the pending VOD, so the human can check before approving.
func (p *Pipeline) RegeneratePreview(ctx context.Context, at float64) error {
	pending := p.Pending()
	if pending == nil {
		return errors.New("nothing is awaiting approval")
	}
	cfg := p.store.Get()
	preview := filepath.Join(cfg.WorkDir, "preview.jpg")
	return ExtractPreview(ctx, cfg, pending.VodPath, at, preview)
}

func (p *Pipeline) splice(ctx context.Context, cfg Config, vodPath string, cut float64) (string, error) {
	p.setState(StateSplicing, filepath.Base(vodPath))
	trimmed := filepath.Join(cfg.WorkDir, "trimmed.mp4")
	if trimErr := TrimFrom(ctx, cfg, vodPath, cut, trimmed); trimErr != nil {
		return "", trimErr
	}
	intro := introForConcat(cfg)
	base := strings.TrimSuffix(filepath.Base(vodPath), filepath.Ext(vodPath))
	outPath := filepath.Join(cfg.OutputDir, base+".mp4")
	if concatErr := ConcatIntro(ctx, cfg, intro, trimmed, outPath); concatErr != nil {
		return "", concatErr
	}
	p.setState(StateVerifying, filepath.Base(outPath))
	if verifyErr := p.verify(ctx, cfg, intro, trimmed, outPath); verifyErr != nil {
		return "", verifyErr
	}
	return outPath, nil
}

func (p *Pipeline) verify(ctx context.Context, cfg Config, introPath, trimmedPath, outPath string) error {
	introDur, introErr := MediaDuration(ctx, cfg, introPath)
	if introErr != nil {
		return introErr
	}
	trimmedDur, trimmedErr := MediaDuration(ctx, cfg, trimmedPath)
	if trimmedErr != nil {
		return trimmedErr
	}
	outDur, outErr := MediaDuration(ctx, cfg, outPath)
	if outErr != nil {
		return outErr
	}
	return VerifyDuration(introDur, trimmedDur, outDur, durationTolerance)
}

// removeIntermediates deletes the raw VOD and per-job scratch files
// once the output has been verified. Dev mode skips this and leaves
// deletion to the Cleanup pane.
func (p *Pipeline) removeIntermediates(cfg Config, vodPath string) {
	targets := []string{
		vodPath,
		filepath.Join(cfg.WorkDir, "trimmed.mp4"),
		filepath.Join(cfg.WorkDir, "preview.jpg"),
		filepath.Join(cfg.WorkDir, "concat.txt"),
	}
	for _, target := range targets {
		if rmErr := os.Remove(target); rmErr != nil && !os.IsNotExist(rmErr) {
			p.logger.Logf("cleanup: %v", rmErr)
		}
	}
}
