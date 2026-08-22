package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
)

// newCommand builds a child process command the way every tool launch
// in ZipClip must: tied to ctx so a cancelled run kills it, and with
// no console window of its own on Windows (see hideConsole).
func newCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	// #nosec G204 -- the executable and arguments come from the user's own configuration
	cmd := exec.CommandContext(ctx, name, args...)
	hideConsole(cmd)
	return cmd
}

// streamLines feeds r to onLine one line at a time until EOF. Lines
// are capped at 1 MiB, which is far beyond anything yt-dlp or ffmpeg
// print.
func streamLines(r io.Reader, onLine func(string)) error {
	if r == nil || onLine == nil {
		return errors.New("missing reader or line handler")
	}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		onLine(scanner.Text())
	}
	return scanner.Err()
}

// runStreaming runs cmd with stdout and stderr merged and hands every
// output line to onLine as it arrives, so the GUI can show progress
// while the tool is still working. It returns the process's exit
// error.
func runStreaming(cmd *exec.Cmd, onLine func(string)) error {
	if cmd == nil || onLine == nil {
		return errors.New("missing command or line handler")
	}
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	cmd.Stderr = writer
	scanErr := make(chan error, 1)
	go func() { scanErr <- streamLines(reader, onLine) }()
	if startErr := cmd.Start(); startErr != nil {
		// Closing the pipe only unblocks the scanner goroutine; a
		// pipe writer's Close never reports an error.
		_ = writer.Close()
		<-scanErr
		return startErr
	}
	waitErr := cmd.Wait()
	_ = writer.Close()
	readErr := <-scanErr
	if waitErr != nil {
		return waitErr
	}
	return readErr
}

// runCapturingStderr runs cmd, streaming stdout lines to onLine (when
// it is not nil) and returning the full stderr, which is where ffmpeg
// writes its log. Progress goes to stdout through -progress pipe:1.
func runCapturingStderr(cmd *exec.Cmd, onLine func(string)) ([]byte, error) {
	if cmd == nil {
		return nil, errors.New("missing command")
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if onLine == nil {
		cmd.Stdout = io.Discard
		runErr := cmd.Run()
		return stderr.Bytes(), runErr
	}
	reader, writer := io.Pipe()
	cmd.Stdout = writer
	scanErr := make(chan error, 1)
	go func() { scanErr <- streamLines(reader, onLine) }()
	if startErr := cmd.Start(); startErr != nil {
		_ = writer.Close()
		<-scanErr
		return nil, startErr
	}
	waitErr := cmd.Wait()
	_ = writer.Close()
	readErr := <-scanErr
	if waitErr != nil {
		return stderr.Bytes(), waitErr
	}
	return stderr.Bytes(), readErr
}
