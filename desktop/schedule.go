package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// NextRun returns the next occurrence of the daily run time strictly
// after now, in now's location.
func NextRun(now time.Time, runTime string) (time.Time, error) {
	if runTime == "" {
		return time.Time{}, errors.New("run time is empty")
	}
	at, parseErr := parseRunTime(runTime)
	if parseErr != nil {
		return time.Time{}, parseErr
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), at.Hour(), at.Minute(), 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next, nil
}

// parseRunTime parses a 12-hour clock time like "8:00 PM".
func parseRunTime(value string) (time.Time, error) {
	at, parseErr := time.Parse("3:04 PM", strings.ToUpper(strings.TrimSpace(value)))
	if parseErr != nil {
		return time.Time{}, fmt.Errorf("bad run time %q, use a 12-hour time like 8:00 PM", value)
	}
	return at, nil
}

// Scheduler fires the pipeline at the configured daily time and on
// demand through RunNow.
type Scheduler struct {
	store  *ConfigStore
	logger *Logger
	pipe   *Pipeline
	runNow chan struct{}

	mu         sync.Mutex
	paused     bool
	next       time.Time
	onChange   func()
	onProblems func([]string)
}

// NewScheduler wires a scheduler to the shared config, logger, and
// pipeline.
func NewScheduler(store *ConfigStore, logger *Logger, pipe *Pipeline) *Scheduler {
	return &Scheduler{store: store, logger: logger, pipe: pipe, runNow: make(chan struct{}, 1)}
}

// SetOnChange registers a callback fired whenever the schedule state
// changes. The GUI sets it once before Loop starts.
func (s *Scheduler) SetOnChange(fn func()) {
	s.mu.Lock()
	s.onChange = fn
	s.mu.Unlock()
}

// SetOnProblems registers a callback that receives the settings
// problems keeping a run from starting, so the GUI can walk the user
// through fixing them instead of the run failing. The GUI sets it once
// before Loop starts.
func (s *Scheduler) SetOnProblems(fn func([]string)) {
	s.mu.Lock()
	s.onProblems = fn
	s.mu.Unlock()
}

func (s *Scheduler) notify() {
	s.mu.Lock()
	fn := s.onChange
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Loop runs until ctx is canceled. It is the program's only unbounded
// loop and sits at the timer boundary; all work happens in fire.
func (s *Scheduler) Loop(ctx context.Context) {
	if ctx == nil {
		return
	}
	for {
		timer := time.NewTimer(s.untilNext(time.Now()))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.fire(ctx, false)
		case <-s.runNow:
			timer.Stop()
			s.fire(ctx, true)
		}
	}
}

func (s *Scheduler) untilNext(now time.Time) time.Duration {
	cfg := s.store.Get()
	daily, nextErr := NextRun(now, cfg.DailyRunTime)
	if nextErr != nil {
		s.logger.Logf("scheduler: %v, checking again in an hour", nextErr)
		daily = now.Add(time.Hour)
	}
	s.mu.Lock()
	s.next = daily
	s.mu.Unlock()
	s.notify()
	return time.Until(daily)
}

func (s *Scheduler) fire(ctx context.Context, manual bool) {
	s.mu.Lock()
	paused := s.paused
	s.mu.Unlock()
	if paused && !manual {
		s.logger.Logf("scheduled run skipped: paused")
		return
	}
	if problems := s.store.Get().Validate(); len(problems) > 0 {
		s.logger.Logf("run is waiting for settings: %s", strings.Join(problems, "; "))
		s.askForSettings(problems)
		return
	}
	runErr := s.pipe.Run(ctx)
	switch {
	case runErr == nil:
	case errors.Is(runErr, context.Canceled):
		s.logger.Logf("run cancelled")
	default:
		s.logger.Logf("run failed: %v", runErr)
	}
}

// askForSettings hands the problems to the GUI's walkthrough, when one
// is registered.
func (s *Scheduler) askForSettings(problems []string) {
	s.mu.Lock()
	fn := s.onProblems
	s.mu.Unlock()
	if fn != nil {
		fn(problems)
	}
}

// RunNow triggers an immediate run if one is not already queued.
func (s *Scheduler) RunNow() {
	select {
	case s.runNow <- struct{}{}:
	default:
	}
}

// TogglePause flips the paused flag and returns the new value.
func (s *Scheduler) TogglePause() bool {
	s.mu.Lock()
	s.paused = !s.paused
	value := s.paused
	s.mu.Unlock()
	s.notify()
	return value
}

// Paused reports whether scheduled runs are currently suppressed.
func (s *Scheduler) Paused() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

// Next returns the next scheduled fire time.
func (s *Scheduler) Next() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.next
}
