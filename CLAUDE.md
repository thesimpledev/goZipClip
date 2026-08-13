# ZipClip

Desktop app (Go + Fyne) that watches a Twitch channel, downloads new
VODs, cuts off the starting-soon screen, splices an intro on without
re-encoding, and optionally uploads the result to YouTube. Alpha, no
users yet; breaking changes need no compatibility handling.

## Hard constraints

- No Twitch API integration, ever. No OAuth, no API keys, no login.
  Liveness comes from yt-dlp's anonymous metadata (`is_live` /
  `live_status`); the download paths pass `--match-filter "!is_live"`.
- Only completed VODs are ever downloaded. An in-progress stream is
  never touched, even by "Run latest VOD".

## Layout

- `desktop/` - the application. Go module, vendored deps. Its README
  is also the user-facing setup guide.
- `installer/buildwin/` - stdlib-only Go tool that builds the Windows
  package: zipclip.exe (mingw-w64 cross-compile), a purpose-built
  minimal ffmpeg, checksum-verified yt-dlp, licenses. See its README
  for the ffmpeg enable-list contract and version pins.
- `installer/build-msix.ps1` - Microsoft Store package, run on a
  Windows machine after `just ffmpeg` output is pulled over.
- `website/` - static site for gozipclip.com. `website/README.md` has
  the AWS resources, `website/DOWNLOADS.md` the download object rules.
- `justfile` - build and release entry points.

## Gates

`just check` runs everything that must pass before a release: gofmt,
go vet, staticcheck, errcheck, revive, then
`go test ./... -race -vet=all -shuffle=on -count=1`. goaudit is run
separately on `desktop/` and `installer/buildwin/` per the global
rules.

## Releasing

Three vectors, fully independent:

- **Linux and Windows desktop**: built and published from this (Linux)
  machine. `just release-linux` and `just release-windows` run the
  gates, build the archive, and upload it with `aws-sec s3api
  put-object` to `gozipclip.com-825a66c84550b1da` under `downloads/`
  (public bucket, fixed filenames, only ever write under
  `downloads/`). Tag the released commit `linux-vX.Y.Z` /
  `windows-vX.Y.Z`; the tags are records only and trigger nothing.
  There are deliberately no CI release workflows: the Windows package
  needs the Linux mingw-w64 ffmpeg build and the gitignored `bin/`,
  so CI cannot produce it. Do not reintroduce them.
- **Website**: push a `web-vX.Y.Z` tag; the Deploy Website workflow
  (the only workflow left) syncs `website/` to the bucket and
  invalidates CloudFront.
- **Microsoft Store**: pull on the Windows machine, run
  `installer/build-msix.ps1`, upload to Partner Center by hand.

When a build ships, bump the versions and add an entry in
`website/updates.html`, and update the three version strings in
`website/index.html`, then deploy the website.

## Windows tool bundling

Windows users install nothing: the zip carries ffmpeg/ffprobe/yt-dlp
next to zipclip.exe, and the app updates yt-dlp at launch (Windows
only, skipped when the user set an explicit path). Path resolution
lives in `desktop/binaries.go`: explicit setting, then bundled copy,
then managed copy, then PATH. Linux users install both tools from
their package manager.
