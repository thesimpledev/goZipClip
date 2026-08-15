# ZipClip

ZipClip turns Twitch VODs into upload-ready videos, automatically.

Website: https://gozipclip.com

## Why this exists

Many streamers repost their Twitch VODs to YouTube. Doing that by hand is
the same chore every time: wait for the VOD, download a multi-hour file,
scrub through it to find where the "starting soon" screen ends, cut off the
prestream dead air, stick an intro on the front, export, and upload. It is
twenty minutes of clicking per stream, and it never changes.

ZipClip does that chore on a schedule. It runs quietly on the streamer's own
PC, and the only part left for a human is pressing upload, or nothing at
all, if an upload tool watches the output folder.

## What it does

Once a day, ZipClip:

1. Checks the channel for new VODs at the time you picked.
2. Downloads any new VOD with yt-dlp, skipping everything it has already
   handled.
3. Finds where the real stream starts using ffmpeg scene detection: a
   "starting soon" screen barely changes between frames, so the jump to real
   content stands out.
4. Cuts off the prestream section and splices your intro onto the front.
   Both are stream copies, so hours of video process in minutes with no
   re-encode and no quality loss. Each step is optional and on by
   default; turn either off in Settings, and with both off the VOD lands
   in the output folder exactly as downloaded.
5. Drops the finished file into your output folder and verifies the result
   before anything is deleted.
6. Uploads the finished video to your YouTube channel as a private video,
   when automatic uploads are turned on. Otherwise the Status tab lists
   the files waiting for manual upload.

A small GUI shows status, runs the newest VOD on demand, and cleans up
the large intermediate files automatically. An optional dev mode runs
step by step instead, stopping at each detected cut for approval.

## What you need

On Windows, nothing: the download ships a purpose-built minimal ffmpeg and
the official yt-dlp next to zipclip.exe, and ZipClip updates yt-dlp at
launch so it keeps up with Twitch. See `installer/buildwin/README.md` for
how the package is built and how the GPL ffmpeg build relates to ZipClip's
MIT license.

On Linux, install ffmpeg and yt-dlp yourself, then point ZipClip at them
(or just have them on your PATH):

- Arch Linux: `sudo pacman -S yt-dlp` and `sudo pacman -S ffmpeg`
- Debian/Ubuntu: `sudo apt install yt-dlp` and `sudo apt install ffmpeg`

The full step-by-step setup guide, from installing the tools to the first
run, is in [desktop/README.md](desktop/README.md) under "Setup guide".

## Repository layout

- `desktop/` - the ZipClip application (Go + Fyne). Build and setup
  instructions are in its README.
- `website/` - the gozipclip.com site serving the downloads.

## Releasing

Desktop releases are built and published from the local machine, one
vetted command per platform:

- `just release-linux` and `just release-windows` each run the gates,
  build the archive, and upload it to the website's download location
  (see `website/DOWNLOADS.md`). The Windows package needs Linux with
  mingw-w64 for the bundled ffmpeg, see `installer/buildwin/README.md`.
- Tag the released commit `linux-v*` / `windows-v*` so the shipped code
  stays identifiable. The tags trigger nothing.
- The website deploys through its GitHub Actions workflow on a `web-v*`
  tag, independent of both.
- The Microsoft Store package is built on a Windows machine with
  `installer/build-msix.ps1` and uploaded to Partner Center by hand.

Releasing one platform never touches the other.

## License

MIT, see [desktop/LICENSE](desktop/LICENSE).
