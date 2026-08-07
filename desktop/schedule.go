package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// NextRun returns the next occurrence of the daily run time hhmm
// strictly after now, in now's location.
func NextRun(now time.Time, hhmm string) (time.Time, error) {
	if hhmm == "" {
		return time.Time{}, errors.New("run time is empty")
	}
	at, parseErr := time.Parse("15:04", hhmm)
	if parseErr != nil {
		return time.Time{}, fmt.Errorf("bad run time %q: %w", hhmm, parseErr)
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), at.Hour(), at.Minute(), 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next, nil
}

// Scheduler fires the pipeline at the configured daily time, on
// demand through RunNow, and an hour later when a run was postponed
// because the channel was still live.
type Scheduler struct {
	store  *ConfigStore
	logger *Logger
	pipe   *Pipeline
	runNow chan struct{}

	mu       sync.Mutex
	paused   bool
	next     time.Time
	retryAt  time.Time
	onChange func()
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
	if !s.retryAt.IsZero() && s.retryAt.Before(daily) {
		daily = s.retryAt
	}
	s.next = daily
	s.mu.Unlock()
	s.notify()
	return time.Until(daily)
}

func (s *Scheduler) fire(ctx context.Context, manual bool) {
	s.mu.Lock()
	s.retryAt = time.Time{}
	paused := s.paused
	s.mu.Unlock()
	if paused && !manual {
		s.logger.Logf("scheduled run skipped: paused")
		return
	}
	runErr := s.pipe.Run(ctx)
	if errors.Is(runErr, ErrChannelLive) {
		s.mu.Lock()
		s.retryAt = time.Now().Add(time.Hour)
		s.mu.Unlock()
		s.logger.Logf("channel is live, trying again in an hour")
		return
	}
	if runErr != nil {
		s.logger.Logf("run failed: %v", runErr)
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
