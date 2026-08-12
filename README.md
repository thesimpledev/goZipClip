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
   re-encode and no quality loss.
5. Drops the finished file into your output folder and verifies the result
   before anything is deleted.

A small GUI shows status, runs the newest VOD on demand, and cleans up
the large intermediate files automatically. An optional dev mode runs
step by step instead, stopping at each detected cut for approval.

## What you need

ZipClip deliberately does not bundle ffmpeg or yt-dlp. yt-dlp needs regular
updates to keep up with Twitch, and a bundled copy would go stale; ffmpeg
carries redistribution obligations that do not belong in an MIT project.
Install both yourself, then point ZipClip at them (or just have them on
your PATH):

- Windows: `winget install yt-dlp.yt-dlp` and `winget install Gyan.FFmpeg`
- Arch Linux: `sudo pacman -S yt-dlp` and `sudo pacman -S ffmpeg`
- Debian/Ubuntu: `sudo apt install yt-dlp` and `sudo apt install ffmpeg`

The full step-by-step setup guide, from installing the tools to the first
run, is in [desktop/README.md](desktop/README.md) under "Setup guide".

## Repository layout

- `desktop/` - the ZipClip application (Go + Fyne). Build and setup
  instructions are in its README.
- `website/` - the gozipclip.com site serving the downloads.

## Releasing

Each deployment is separate and has its own tag namespace:

- `linux-v*` triggers `Release Linux` only.
- `windows-v*` triggers `Release Windows` only.
- The website deploys through its own workflow, independent of both.

Each release workflow builds on a native runner of its platform, runs the
test suite, and uploads its archive to the website's download location.
Releasing one platform never touches the other. The one-time AWS setup is
documented at the top of `.github/workflows/release-linux.yml`.

## License

MIT, see [desktop/LICENSE](desktop/LICENSE).
