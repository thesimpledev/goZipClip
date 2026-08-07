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

## Setup guide

### 1. Install yt-dlp and ffmpeg

ZipClip drives two well-known free tools: yt-dlp downloads the VODs and
ffmpeg does the video work (ffprobe comes with it). ZipClip does not bundle
them, because yt-dlp needs regular updates to keep up with Twitch.

- Windows: `winget install yt-dlp.yt-dlp Gyan.FFmpeg`
- Arch Linux: `sudo pacman -S yt-dlp ffmpeg`
- Debian/Ubuntu: `sudo apt install yt-dlp ffmpeg`

If they end up on your PATH (the commands above do that), ZipClip finds
them automatically. Otherwise note where they are for step 4.

### 2. Create a Twitch application (free)

ZipClip needs Twitch API credentials for exactly one question: is the
channel live right now. The credentials can only read public data.

1. Go to https://dev.twitch.tv/console/apps and log in with your Twitch
   account. Twitch may ask you to enable two-factor authentication first.
2. Press "Register Your Application".
3. Name: anything, for example `ZipClip`.
4. OAuth Redirect URL: `http://localhost` (ZipClip never uses it, but the
   field is required).
5. Category: Application Integration. Press Create.
6. Press "Manage" on the new application. Copy the Client ID, then press
   "New Secret" and copy the Client Secret. The secret is shown only once;
   if you lose it, generate a new one.

### 3. Install ZipClip

Download the build for your system from https://gozipclip.com and unpack it
into a folder of your choice. ZipClip keeps its settings (`config.json`)
and log next to the executable, so pick a folder you can write to, not
`Program Files`. Building from source is described below.

### 4. First launch

Start ZipClip. It opens on the Settings tab. Fill in:

- **Channel**: the Twitch channel name as it appears in the URL.
- **Twitch client ID / client secret**: from step 2.
- **Daily run time**: when the daily check runs, 24-hour clock, for
  example `08:00`. Pick a time the stream is normally over.
- **Intro file**: the video spliced onto the front of every VOD. It must
  have an audio track.
- **Output folder**: where finished videos land. Point your upload tool's
  watch folder here.
- **Work folder**: scratch space for downloads. Needs room for a full VOD
  (often 5 to 15 GB).
- **yt-dlp / ffmpeg / ffprobe paths**: leave as-is if they are on PATH,
  otherwise browse to the executables from step 1.
- Leave **Automatic mode** off for now.

Press Save.

### 5. Skip your back catalog

Press "Mark existing VODs as downloaded". This records every VOD currently
on the channel as already handled, so ZipClip never downloads your history.
Only VODs published after this point are processed. Skip this step only if
you really want the newest existing VODs pulled down on the first run.

### 6. First runs, with approval

Press "Run now" on the Status tab (or wait for the scheduled time). After
the download, ZipClip detects where the real stream starts and stops on
the Approve tab: a preview frame at the detected cut point, and the
timestamp next to it. Check the frame, adjust the time and press "Preview
at time" if it is off, then press "Approve and splice". In this mode
nothing is ever deleted; the Cleanup tab lists what can go, and deletes
only when you press the button.

### 7. Prepare the intro

Once the first VOD has downloaded, press "Prepare intro" in Settings. This
re-encodes your intro to exactly match the VOD's codec, resolution, and
framerate, which is what lets ZipClip splice without re-encoding hours of
video. Do this once, and again whenever you change the intro file or your
stream output settings.

### 8. Turn on automatic mode

After a few streams of the detection landing where you expect, turn on
Automatic mode in Settings. From then on the whole chain runs unattended:
download, detect, splice, verify, and delete the large intermediate files.
The finished video still only leaves your machine when your upload tool
picks it up.

### 9. Start ZipClip with your computer

- **Windows**: press Win+R, run `shell:startup`, and put a shortcut to
  `zipclip.exe` there.
- **Linux**: create `~/.config/autostart/zipclip.desktop` with
  `Exec=/path/to/zipclip`.

ZipClip minimizes to the system tray; closing the window hides it, and the
tray menu has Run now, Pause, and Quit.

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
