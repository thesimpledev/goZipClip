//go:build !windows

package main

import "os/exec"

// hideConsole is a no-op outside Windows: console children of a GUI
// process do not open windows of their own there.
func hideConsole(_ *exec.Cmd) {}
