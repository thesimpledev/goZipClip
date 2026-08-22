//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

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
