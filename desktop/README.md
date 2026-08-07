# ZipClip

ZipClip watches a Twitch channel for new VODs and turns each one into an
upload-ready file, automatically:

1. Once a day it checks the channel. If the channel is live it waits an hour
   and tries again.
2. New VODs are downloaded with yt-dlp. VODs it has already handled are
   skipped, tracked in `archive.txt` in the work folder.
3. It finds where the real stream starts. Most streams open with a
   "starting soon" screen that barely changes from frame to frame; the switch
   to real content changes most of the picture at once. ZipClip uses ffmpeg
   scene detection to find that jump.
4. The prestream section is cut off and your intro is spliced onto the front.
   Both steps are stream copies, so a multi-hour VOD is processed in minutes
   without re-encoding.
5. The finished file lands in the output folder, ready for whatever upload
   tool watches that folder.
6. Intermediate files can be deleted automatically, or listed and deleted by
   hand from the Cleanup tab.

Website: https://gozipclip.com
Created by TheSimpleDev: https://thesimpledev.com

## Requirements

- [yt-dlp](https://github.com/yt-dlp/yt-dlp)
- [ffmpeg](https://ffmpeg.org) (includes ffprobe)
- A Twitch application (free): https://dev.twitch.tv/console/apps
  Register an app, set the OAuth redirect to `http://localhost`, and copy the
  client ID and client secret into ZipClip's Settings tab. These credentials
  can only read public data (is the channel live), nothing more.

Point the Settings tab at the yt-dlp and ffmpeg executables if they are not
on your PATH.

## First run

1. Start ZipClip. It opens on the Settings tab.
2. Fill in the channel name, Twitch credentials, folders, and intro file,
   then press Save.
3. Press "Mark existing VODs as downloaded". This records every VOD already
   on the channel so ZipClip never downloads your back catalog. Only VODs
   published after this point are processed.
4. Leave Automatic mode off for the first few runs. After each detection,
   the Approve tab shows a preview frame at the detected cut point; adjust
   the time if needed and press Approve. Nothing is deleted in manual mode.
5. After the first VOD has downloaded, press "Prepare intro" in Settings.
   This re-encodes your intro to exactly match the VOD's codec, resolution,
   and framerate, which is what makes the no-re-encode splice possible. The
   intro must have an audio track.
6. Once detection has proven itself, turn on Automatic mode. From then on
   ZipClip splices without asking and deletes the raw VOD after the output
   file passes verification.

## Settings notes

- **Daily run time**: 24-hour clock, for example `08:00`.
- **Scene threshold**: fraction of the picture that must change in one frame
  step to count as the stream starting. `0.4` works for a mostly static
  waiting screen. If detection fires too early (an alert popup, a busy
  animation), raise it; if it never fires, lower it.
- **Cut backoff**: seconds subtracted from the detected point so the cut
  lands just before the stream starts. Also absorbs the keyframe snap from
  stream copying.
- **Keep finished (days)**: finished files older than this become cleanup
  candidates.
- The Twitch client secret is stored as plain text in `config.json` next to
  the executable. It grants only public reads, but treat the file
  accordingly.
- `config.json` is plain JSON and can also be edited by hand while ZipClip
  is not running.

## What is never deleted

The download archive (`archive.txt` in the work folder) is the permanent
record of which VODs were already handled. Cleanup never touches it, and it
should never be deleted by hand; without it every VOD still on the channel
would be downloaded again. The prepared intro (`intro_ready.mp4`) is also
kept.

## Verification before deletion

After splicing, ZipClip compares the output's duration against intro plus
trimmed VOD. If they disagree by more than a few seconds, nothing is
deleted and the run stops with an error, because Twitch keeps a VOD for
only about 14 days and the raw download is the only re-download window.

## Autostart

- **Windows**: press Win+R, run `shell:startup`, and put a shortcut to
  `zipclip.exe` there.
- **Linux**: create `~/.config/autostart/zipclip.desktop` with
  `Exec=/path/to/zipclip`.

## Building from source

Go 1.26 or later with cgo. Dependencies are vendored; no network access is
needed to build.

Linux (needs GL, X11, and Wayland development headers):

    go build -o zipclip .

Windows (hides the console window):

    go build -ldflags -H=windowsgui -o zipclip.exe .

Note on vendoring: `go mod vendor` drops the generated Wayland protocol
headers from `vendor/github.com/go-gl/glfw/v3.4/glfw/glfw/include/` because
that folder holds no buildable Go files. After re-running `go mod vendor`,
restore them from the module cache:

    cp --no-preserve=mode "$(go env GOMODCACHE)"/github.com/go-gl/glfw/v3.4/glfw@*/glfw/include/*.h \
        vendor/github.com/go-gl/glfw/v3.4/glfw/glfw/include/

## License

MIT, see LICENSE.
