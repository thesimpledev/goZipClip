//go:build windows

package main

import (
	"os/exec"
	"strconv"
	"syscall"
)

// killTreeOnCancel makes a cancelled context stop the tool together
// with every process it started. yt-dlp.exe is a launcher that runs the
// real program as a child process: killing only the launcher leaves the
// download running, and the run waiting on its output.
func killTreeOnCancel(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.Cancel = func() error {
		// #nosec G204 -- taskkill is a fixed system tool; the only variable argument is the child's numeric process id
		kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if killErr := kill.Run(); killErr != nil {
			return cmd.Process.Kill()
		}
		return nil
	}
}

// hideConsole keeps a console tool (yt-dlp, ffmpeg, ffprobe) from
// opening its own console window. zipclip.exe is a GUI-subsystem
// program, so without this Windows gives every child a fresh console,
// which shows up as an empty black box because the output is piped
// back to ZipClip.
func hideConsole(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
