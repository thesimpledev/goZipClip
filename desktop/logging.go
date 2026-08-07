package main

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

const logRingSize = 200

// Logger writes timestamped lines to a file and keeps a bounded ring
// of recent lines for the GUI.
type Logger struct {
	mu     sync.Mutex
	file   *os.File
	ring   []string
	onLine func()
}

// NewLogger opens (or creates) the log file at path in append mode.
// A non-nil error still comes with a usable Logger that only keeps
// the in-memory ring.
func NewLogger(path string) (*Logger, error) {
	l := &Logger{}
	if path == "" {
		return l, errors.New("log path is empty")
	}
	// #nosec G304 -- the log path is built from the executable's own directory, not external input
	file, openErr := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if openErr != nil {
		return l, openErr
	}
	l.file = file
	return l, nil
}

// SetOnLine registers a callback fired after every logged line.
func (l *Logger) SetOnLine(fn func()) {
	l.mu.Lock()
	l.onLine = fn
	l.mu.Unlock()
}

// Logf formats, timestamps, and records one log line.
func (l *Logger) Logf(format string, args ...any) {
	line := time.Now().Format("2006-01-02 15:04:05") + " " + fmt.Sprintf(format, args...)
	l.mu.Lock()
	if len(l.ring) >= logRingSize {
		l.ring = l.ring[1:]
	}
	l.ring = append(l.ring, line)
	if l.file != nil {
		// A failed write to the log file has nowhere better to be
		// reported, so it is dropped deliberately.
		_, _ = l.file.WriteString(line + "\n")
	}
	fn := l.onLine
	l.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Recent returns a copy of the buffered log lines, oldest first.
func (l *Logger) Recent() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.ring))
	copy(out, l.ring)
	return out
}

// Close releases the log file.
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		// Nothing actionable can be done with a close error at exit.
		_ = l.file.Close()
		l.file = nil
	}
}
