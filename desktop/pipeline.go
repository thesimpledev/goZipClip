package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// State names the pipeline's current activity.
type State int

// Pipeline states, in rough order of a run.
const (
	StateIdle State = iota
	StateCataloging
	StateDownloading
	StateDetecting
	StateAwaitingApproval
	StateSplicing
	StateVerifying
	StateUploading
	StateError
)

func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateCataloging:
		return "cataloging"
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
	case StateUploading:
		return "uploading to YouTube"
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

	mu         sync.Mutex
	state      State
	detail     string
	running    bool
	cancel     context.CancelFunc
	pending    *PendingCut
	approve    chan float64
	onChange   func()
	toolsReady <-chan struct{}
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

// SetToolsReady registers a channel that is closed once the
// launch-time yt-dlp install or update has finished. Runs wait on it
// so a download never races the binary being replaced.
func (p *Pipeline) SetToolsReady(ready <-chan struct{}) {
	p.mu.Lock()
	p.toolsReady = ready
	p.mu.Unlock()
}

// waitTools blocks until the launch-time binary setup is over, or the
// run is cancelled.
func (p *Pipeline) waitTools(ctx context.Context) error {
	p.mu.Lock()
	ready := p.toolsReady
	p.mu.Unlock()
	if ready == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ready:
		return nil
	}
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

// progress updates the detail text of the current state without
// writing a log line, for per-second progress from the tools.
func (p *Pipeline) progress(detail string) {
	p.mu.Lock()
	p.detail = detail
	fn := p.onChange
	p.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// begin claims the pipeline for one run and returns the run's own
// context, which Cancel ends and which carries the progress callback.
// It reports false when a run is already in progress.
func (p *Pipeline) begin(ctx context.Context) (context.Context, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running {
		return nil, false
	}
	runCtx, cancel := context.WithCancel(ctx)
	p.running = true
	p.cancel = cancel
	return withProgress(runCtx, p.progress), true
}

func (p *Pipeline) end() {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.running = false
	p.pending = nil
	p.mu.Unlock()
}

// Cancel stops the run in progress, if any: the tool it is waiting on
// is killed and the run ends with the status "cancelled". Scheduled
// runs are not affected; Pause is for those.
func (p *Pipeline) Cancel() {
	p.mu.Lock()
	cancel := p.cancel
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Running reports whether a run is in progress.
func (p *Pipeline) Running() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// settle turns the error of a cancelled run into the "cancelled"
// status and leaves every other outcome alone.
func (p *Pipeline) settle(ctx context.Context, runErr error) error {
	if runErr != nil && errors.Is(ctx.Err(), context.Canceled) {
		p.setState(StateIdle, "cancelled")
		return context.Canceled
	}
	return runErr
}

// setError records a failed step, unless the run was cancelled, in
// which case the kill is not an error worth showing.
func (p *Pipeline) setError(ctx context.Context, stepErr error) {
	if ctx.Err() != nil || stepErr == nil {
		return
	}
	p.setState(StateError, stepErr.Error())
}

// Run executes one full pass: download, then processing of every new
// VOD.
func (p *Pipeline) Run(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	runCtx, ok := p.begin(ctx)
	if !ok {
		return errors.New("a run is already in progress")
	}
	defer p.end()
	return p.settle(runCtx, p.run(runCtx))
}

func (p *Pipeline) run(ctx context.Context) error {
	cfg := p.store.Get()
	if problems := cfg.Validate(); len(problems) > 0 {
		p.setState(StateError, "configuration incomplete: "+strings.Join(problems, "; "))
		return errors.New("configuration incomplete")
	}
	if waitErr := p.waitTools(ctx); waitErr != nil {
		return waitErr
	}
	if !IsChannelScanned(cfg.Channel) {
		if seedErr := p.catalog(ctx, cfg); seedErr != nil {
			return seedErr
		}
	}
	files, downloadErr := p.download(ctx, cfg)
	if downloadErr != nil {
		return downloadErr
	}
	if len(files) == 0 {
		p.setState(StateIdle, "no new VODs")
		return nil
	}
	return p.processAll(ctx, cfg, files)
}

// processAll processes every VOD in turn. A VOD that fails does not
// stop the ones after it: its raw file stays in the work folder and the
// next run tries it again. A cancelled run stops at once.
func (p *Pipeline) processAll(ctx context.Context, cfg Config, files []string) error {
	var firstErr error
	failed := 0
	for _, vod := range files {
		procErr := p.processOne(ctx, cfg, vod)
		if procErr == nil {
			continue
		}
		if ctx.Err() != nil {
			return procErr
		}
		failed++
		if firstErr == nil {
			firstErr = procErr
		}
	}
	if firstErr != nil {
		p.setState(StateError, fmt.Sprintf("%d of %d VOD(s) failed, first: %v", failed, len(files), firstErr))
		return firstErr
	}
	p.setState(StateIdle, fmt.Sprintf("finished %d VOD(s)", len(files)))
	return nil
}

// WaitIdle blocks until no run is in progress or the timeout passes,
// and reports whether the pipeline went idle. The app calls it on quit,
// after cancelling, so the tools of the cancelled run are stopped
// before the process exits.
func (p *Pipeline) WaitIdle(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for p.Running() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
	return true
}

// RunLatest downloads and processes the newest VOD on the channel,
// even when the archive already lists it as handled.
func (p *Pipeline) RunLatest(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	runCtx, ok := p.begin(ctx)
	if !ok {
		return errors.New("a run is already in progress")
	}
	defer p.end()
	return p.settle(runCtx, p.runLatest(runCtx))
}

func (p *Pipeline) runLatest(ctx context.Context) error {
	cfg := p.store.Get()
	if problems := cfg.Validate(); len(problems) > 0 {
		p.setState(StateError, "configuration incomplete: "+strings.Join(problems, "; "))
		return errors.New("configuration incomplete")
	}
	if waitErr := p.waitTools(ctx); waitErr != nil {
		return waitErr
	}
	p.setState(StateDownloading, "latest VOD on "+cfg.Channel)
	vod, dlErr := DownloadLatest(ctx, cfg, p.logger.Logf)
	if dlErr != nil {
		p.setError(ctx, dlErr)
		return dlErr
	}
	if procErr := p.processOne(ctx, cfg, vod); procErr != nil {
		return procErr
	}
	p.setState(StateIdle, "finished latest VOD")
	return nil
}

// Catalog records the channel's existing VODs in the archive as a job
// of its own, so it shows progress on the Status tab and can be
// cancelled like a run. It happens once per channel, when the channel
// is set.
func (p *Pipeline) Catalog(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	runCtx, ok := p.begin(ctx)
	if !ok {
		return errors.New("a run is already in progress")
	}
	defer p.end()
	cfg := p.store.Get()
	if cfg.Channel == "" {
		return errors.New("channel is not set")
	}
	if waitErr := p.waitTools(runCtx); waitErr != nil {
		return p.settle(runCtx, waitErr)
	}
	catErr := p.catalog(runCtx, cfg)
	if catErr == nil {
		p.setState(StateIdle, "catalog of "+cfg.Channel+" finished")
	}
	return p.settle(runCtx, catErr)
}

func (p *Pipeline) catalog(ctx context.Context, cfg Config) error {
	p.setState(StateCataloging, cfg.Channel)
	if seedErr := SeedArchive(ctx, cfg, p.logger.Logf); seedErr != nil {
		p.setError(ctx, seedErr)
		return seedErr
	}
	p.logger.Logf("cataloged the existing VODs on %s; runs now download anything newer", cfg.Channel)
	return nil
}

func (p *Pipeline) download(ctx context.Context, cfg Config) ([]string, error) {
	p.setState(StateDownloading, cfg.Channel)
	files, dlErr := DownloadNew(ctx, cfg, p.logger.Logf)
	if dlErr != nil {
		p.setError(ctx, dlErr)
		return nil, dlErr
	}
	return files, nil
}

func (p *Pipeline) processOne(ctx context.Context, cfg Config, vodPath string) error {
	cut := 0.0
	if cfg.CutEnabled {
		var cutErr error
		cut, cutErr = p.resolveCut(ctx, cfg, vodPath)
		if cutErr != nil {
			p.setError(ctx, cutErr)
			return cutErr
		}
	}
	outPath, spliceErr := p.splice(ctx, cfg, vodPath, cut)
	if spliceErr != nil {
		p.setError(ctx, spliceErr)
		return spliceErr
	}
	p.logger.Logf("finished %s", outPath)
	if !cfg.DevMode {
		p.removeIntermediates(cfg, vodPath)
	}
	if cfg.AutoUpload {
		return p.upload(ctx, cfg, outPath)
	}
	return nil
}

// upload sends a finished video to YouTube. The output file stays in
// the output folder either way; a failed upload can be retried with
// Run latest VOD or uploaded by hand.
func (p *Pipeline) upload(ctx context.Context, cfg Config, outPath string) error {
	p.setState(StateUploading, filepath.Base(outPath))
	if upErr := NewYouTubeClient(cfg).Upload(ctx, outPath); upErr != nil {
		p.setError(ctx, upErr)
		return upErr
	}
	p.logger.Logf("uploaded %s as a private YouTube video", filepath.Base(outPath))
	return nil
}

// resolveCut detects the stream start and, in dev mode, waits for the
// human to approve or adjust it.
func (p *Pipeline) resolveCut(ctx context.Context, cfg Config, vodPath string) (float64, error) {
	p.setState(StateDetecting, filepath.Base(vodPath))
	cut, detectErr := DetectCut(ctx, cfg, vodPath)
	if errors.Is(detectErr, errNoSceneChange) {
		p.logger.Logf("%v; keeping the full VOD", detectErr)
		cut, detectErr = 0, nil
	}
	if detectErr != nil {
		return 0, detectErr
	}
	if cut > 0 {
		p.logger.Logf("stream start found in %s; cutting at %s", filepath.Base(vodPath), formatTimestamp(cut))
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

// splice turns the raw VOD into the finished output, running only the
// steps the configuration enables.
func (p *Pipeline) splice(ctx context.Context, cfg Config, vodPath string, cut float64) (string, error) {
	p.setState(StateSplicing, filepath.Base(vodPath))
	outPath := outputPath(cfg, vodPath)
	var stepErr error
	switch {
	case cfg.CutEnabled && cfg.IntroEnabled:
		stepErr = p.trimAndConcat(ctx, cfg, vodPath, cut, outPath)
	case cfg.CutEnabled:
		stepErr = p.trimOnly(ctx, cfg, vodPath, cut, outPath)
	case cfg.IntroEnabled:
		stepErr = p.concatOnly(ctx, cfg, vodPath, outPath)
	default:
		stepErr = p.deliverRaw(vodPath, outPath)
	}
	if stepErr != nil {
		return "", stepErr
	}
	return outPath, nil
}

// outputPath is where the finished video for vodPath lands.
func outputPath(cfg Config, vodPath string) string {
	base := strings.TrimSuffix(filepath.Base(vodPath), filepath.Ext(vodPath))
	return filepath.Join(cfg.OutputDir, base+".mp4")
}

// trimAndConcat cuts the VOD, splices the intro on, and verifies the
// combined duration.
func (p *Pipeline) trimAndConcat(ctx context.Context, cfg Config, vodPath string, cut float64, outPath string) error {
	trimmed := filepath.Join(cfg.WorkDir, "trimmed.mp4")
	if trimErr := TrimFrom(ctx, cfg, vodPath, cut, trimmed); trimErr != nil {
		return trimErr
	}
	intro, introErr := p.readyIntro(ctx, cfg, trimmed)
	if introErr != nil {
		return introErr
	}
	if concatErr := ConcatIntro(ctx, cfg, intro, trimmed, outPath); concatErr != nil {
		return concatErr
	}
	p.setState(StateVerifying, filepath.Base(outPath))
	return p.verify(ctx, cfg, intro, trimmed, outPath)
}

// readyIntro returns the intro to join onto vodPath, re-encoding the
// configured intro first when no prepared copy matches the VOD. Joining
// an intro that does not match produces a broken file.
func (p *Pipeline) readyIntro(ctx context.Context, cfg Config, vodPath string) (string, error) {
	stale, checkErr := introNeedsPrepare(ctx, cfg, vodPath)
	if checkErr != nil {
		return "", checkErr
	}
	if stale {
		if prepErr := prepareIntroFor(ctx, cfg, vodPath, p.logger.Logf); prepErr != nil {
			return "", prepErr
		}
	}
	return introReadyPath(cfg), nil
}

// trimOnly cuts the VOD straight into the output folder. With no intro
// there is no combined duration to check, so verification is a
// readback of the output's duration.
func (p *Pipeline) trimOnly(ctx context.Context, cfg Config, vodPath string, cut float64, outPath string) error {
	if trimErr := TrimFrom(ctx, cfg, vodPath, cut, outPath); trimErr != nil {
		return trimErr
	}
	p.setState(StateVerifying, filepath.Base(outPath))
	_, durErr := MediaDuration(ctx, cfg, outPath)
	return durErr
}

// concatOnly splices the intro onto the full VOD and verifies the
// combined duration. The VOD goes through the cut step with a cut of
// zero first: a downloaded VOD carries its audio track before its video
// track, and that stream copy puts them in the order the join expects.
func (p *Pipeline) concatOnly(ctx context.Context, cfg Config, vodPath, outPath string) error {
	return p.trimAndConcat(ctx, cfg, vodPath, 0, outPath)
}

// deliverRaw moves the untouched VOD into the output folder. The move
// preserves the bytes as they are, so there is nothing to verify.
func (p *Pipeline) deliverRaw(vodPath, outPath string) error {
	p.logger.Logf("cut and intro are both off; moving %s to the output folder", filepath.Base(vodPath))
	return moveFile(vodPath, outPath)
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
