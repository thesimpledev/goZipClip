# ZipClip

ZipClip watches a Twitch channel for new VODs and turns each one into an
upload-ready file, automatically:

1. Once a day, at the time you pick, it checks the channel for new VODs.
2. New VODs are downloaded with yt-dlp. VODs it has already handled are
   skipped, tracked in a download archive kept in your user
   configuration folder.
3. It finds where the real stream starts. Most streams open with a
   "starting soon" screen that barely changes from frame to frame; the switch
   to real content changes most of the picture at once. ZipClip uses ffmpeg
   scene detection to find that jump.
4. The prestream section is cut off and your intro is spliced onto the front.
   Both steps are stream copies, so a multi-hour VOD is processed in minutes
   without re-encoding. Each step has its own checkbox in Settings and is
   on by default; with both off, the downloaded VOD is delivered to the
   output folder unchanged.
5. The finished file lands in the output folder. With automatic uploads
   turned on, ZipClip then uploads it to your YouTube channel as a
   private video; otherwise the Status tab lists the files waiting for
   manual upload.
6. Intermediate files are deleted automatically once the output is
   verified. In dev mode they are kept, listed on the Cleanup tab, and
   deleted only when you press the button.

Website: https://gozipclip.com
Created by TheSimpleDev: https://thesimpledev.com

## Setup guide

### 1. Install yt-dlp and ffmpeg

ZipClip drives two well-known free tools: yt-dlp downloads the VODs and
ffmpeg does the video work (ffprobe comes with it).

**On Windows there is nothing to install.** The ZipClip download ships
both tools in the same folder as zipclip.exe: a purpose-built ffmpeg
(see `installer/buildwin/README.md`) and the official yt-dlp, which
ZipClip updates automatically on every launch, because yt-dlp needs
regular updates to keep up with Twitch. Setting an explicit path in
Settings still overrides both.

On Linux, install them with your package manager.

Arch Linux:

    sudo pacman -S yt-dlp
    sudo pacman -S ffmpeg

Debian/Ubuntu:

    sudo apt install yt-dlp
    sudo apt install ffmpeg

Direct downloads, if you would rather not use a package manager:

- yt-dlp: https://github.com/yt-dlp/yt-dlp/releases
- ffmpeg: https://ffmpeg.org/download.html

If they end up on your PATH (the commands above do that), ZipClip finds
them automatically. Otherwise note where they are for step 3.

The paths in Settings decide what actually runs: an explicit path wins,
then the bundled ffmpeg (Windows) or the self-updating yt-dlp copy, and
PATH is the fallback.

### 2. Install ZipClip

Download the build for your system from https://gozipclip.com and unpack it
into a folder of your choice. ZipClip keeps its settings (`config.json`),
log, and download archive in your user configuration folder (Windows:
`%AppData%\zipclip`, Linux: `~/.config/zipclip`), so the program itself
can live anywhere. Building from source is described below.

### 3. First launch

Start ZipClip. It opens on the Settings tab. Fill in:

- **Channel**: the Twitch channel name as it appears in the URL.
- **Daily run time**: when the daily check runs, 12-hour clock, for
  example `8:00 AM`. Pick a time the stream is normally over.
- **Intro file**: the video spliced onto the front of every VOD. It must
  have an audio track. Only needed while "Add an intro video to the
  start of each VOD" is ticked; untick that box to skip the intro
  entirely.
- **Output folder**: where finished videos land. Point your upload tool's
  watch folder here.
- **Work folder**: scratch space for downloads. Needs room for a full VOD
  (often 5 to 15 GB).
- **yt-dlp / ffmpeg / ffprobe paths**: leave as-is if they are on PATH,
  otherwise browse to the executables from step 1.

There is no Save button. Checkboxes save as soon as they change, and
text fields save when you press Enter or move to another field; the
line at the bottom of the tab shows a green "Saved" or a red "Not
saved" so you always know where things stand.

When you enter the channel, ZipClip checks that it exists on Twitch
(a name it cannot find is put back) and asks you to confirm it.
Confirming starts the one-time catalog described in step 5.

### 4. Set up YouTube uploads (optional)

ZipClip can upload each finished video to your channel by itself. Skip
this step if you would rather upload by hand: finished videos wait in
the output folder, and the "Processed videos" button on the Status tab
lists every VOD ZipClip has handled.

Automatic uploads need a Google OAuth client, created once in your own
Google account:

1. Go to https://console.cloud.google.com and sign in with the Google
   account that owns your YouTube channel. Create a new project; the
   name does not matter.
2. Under "APIs & Services", open "Library", search for
   `YouTube Data API v3`, and press Enable.
3. Under "APIs & Services", open "OAuth consent screen". Choose
   External, fill in the app name and your email address, and save.
   Then press "Publish app" so the status is "In production". This
   matters: while the status is Testing, Google cuts the connection
   after 7 days and you would have to reconnect weekly.
4. Under "APIs & Services", open "Credentials", press
   "Create Credentials", pick "OAuth client ID", and choose the
   "Desktop app" type. Copy the Client ID and Client Secret.
5. In ZipClip's Settings: paste both values and tick "Upload finished
   videos to YouTube automatically" (they save as soon as you leave
   each field). Then press "Connect YouTube". Your browser opens a Google consent page;
   because this is your own unverified project, Google shows a
   warning screen first. Continue past it (Advanced, then "Go to
   ...") and allow the upload permission.

Uploads arrive on your channel as private videos; review and publish
them from YouTube Studio. YouTube locks API uploads from unaudited
projects to private, so this is also the only mode Google allows a
personal project.

### 5. Your back catalog

When you confirm the channel, ZipClip catalogs every VOD already on it
so your history is never downloaded: all but the newest three are
recorded as handled. The Status tab shows the count as it goes, and the
red Cancel button stops it. From then on every run downloads whatever
the channel has that ZipClip has not handled yet. If the catalog did not
finish (you cancelled it, say), the first run does it before
downloading.

The "Processed videos" button on the Status tab lists what has been
handled, newest first. Press Forget on an entry to have that VOD
downloaded and processed again on the next run. "Reset archive" in
Settings forgets everything and catalogs the channel again, like a
fresh install.

### 6. First run

Press "Run now" on the Status tab, or wait for the scheduled time. The
whole chain runs unattended: download, detect where the real stream
starts, cut, splice, verify, delete the large intermediate files, and,
with automatic uploads on, upload the result to your channel as a
private video. The cut and the intro splice can each be turned off in
Settings; a run with both off simply delivers the downloaded VOD to the
output folder unchanged.

The Status tab shows what is happening as it happens: the download's
percentage, speed, and time left, and each ffmpeg step with its
position in the video. The red Cancel button stops the run in progress
(the next scheduled run still happens; Pause is what holds those).
Anything a run needs but does not have yet, such as an intro file, is
asked for in a dialog with the choice to fill it in or turn that step
off, instead of failing the run.

To try ZipClip on a stream that is already over, press "Run latest VOD".
It downloads and processes the newest VOD on the channel even if it was
already marked as handled; ZipClip warns you first in that case.

### 7. Prepare the intro

Once the first VOD has downloaded, press "Prepare intro" in Settings. This
re-encodes your intro to exactly match the VOD's codec, resolution, and
framerate, which is what lets ZipClip splice without re-encoding hours of
video. Do this once, and again whenever you change the intro file or your
stream output settings.

ZipClip minimizes to the system tray; closing the window hides it, and the
tray menu has Run now, Pause, and Quit.

## Settings notes

- **Daily run time**: 12-hour clock, for example `8:00 AM` or `10:30 PM`.
- **Cut**: whether the starting-soon screen is detected and cut off.
  Untick it to keep every VOD from the very beginning. The scan window,
  scene threshold, and cut backoff only apply while this is on.
- **Intro**: whether the intro is spliced onto the front. Untick it and
  no intro file is needed.
- **Scene threshold**: fraction of the picture that must change in one frame
  step to count as the stream starting. `0.4` works for a mostly static
  waiting screen. If detection fires too early (an alert popup, a busy
  animation), raise it; if it never fires, lower it. A run where nothing
  is detected no longer stops with an error: the full VOD is kept, and
  in dev mode the Approve tab still lets you set the time by hand.
- **Cut backoff**: seconds subtracted from the detected point so the cut
  lands just before the stream starts. Also absorbs the keyframe snap from
  stream copying.
- **Keep finished (days)**: finished files older than this become cleanup
  candidates.
- **Dev mode**: runs step by step. Each run stops on the Approve tab with
  a preview frame at the detected cut point; check the frame, adjust the
  time and press "Preview at time" if it is off, then press "Approve and
  splice". Intermediate files are kept and the Cleanup tab deletes them
  only when you press the button. Leave this off for normal use.
- **Uploads**: unchecked means manual mode; use "Show files to upload"
  on the Status tab and upload however you like. Checked means every
  finished video is uploaded to your channel as a private video.
- **Reset settings** puts every setting back to its default. **Reset
  archive** deletes the download archive, so every VOD still on the
  channel counts as new again.
- The YouTube client secret and the sign-in token
  (`youtube_token.json`) are stored in the user configuration folder.
  They only grant permission to upload videos to your channel, and you
  can revoke them any time at https://myaccount.google.com/permissions.
- `config.json` is plain JSON and can also be edited by hand while ZipClip
  is not running. It lives in your user configuration folder (Windows:
  `%AppData%\zipclip`, Linux: `~/.config/zipclip`).

## What is never deleted

The download archive (`archive.txt` in the user configuration folder) is
the permanent record of which VODs were already handled. Cleanup never
touches it; deleting it by hand or with "Reset archive" means every VOD
still on the channel counts as new again. The prepared intro
(`intro_ready.mp4`) is also kept.

## Verification before deletion

Before anything is deleted, ZipClip checks the output it produced. With
both steps on, the output's duration must match intro plus trimmed VOD;
with only the intro on, intro plus the full VOD. With only the cut on,
the output's duration is read back as a sanity check. With both steps
off the file is moved as-is, so there is nothing to verify. If a check
disagrees by more than a few seconds, nothing is deleted and the run
stops with an error, because Twitch keeps a VOD for only about 14 days
and the raw download is the only re-download window.

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
