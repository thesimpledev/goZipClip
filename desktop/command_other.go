//go:build !windows

package main

import "os/exec"

// hideConsole is a no-op outside Windows: console children of a GUI
// process do not open windows of their own there.
func hideConsole(_ *exec.Cmd) {}

// killTreeOnCancel is a no-op outside Windows: yt-dlp is not a launcher
// there, so the default kill of the tool itself stops the work.
func killTreeOnCancel(_ *exec.Cmd) {}
