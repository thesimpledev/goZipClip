# buildwin: the Windows package build

One Go tool (stdlib only) produces everything a Windows user
downloads. From the repository root:

    just windows    # or: go -C installer/buildwin run . zip
    just ffmpeg     # or: go -C installer/buildwin run . ffmpeg

`windows` builds `bin/zipclip-windows-amd64.zip` (the fixed filename
the website serves, see `website/DOWNLOADS.md`), the single download:
it unzips to a `ZipClip` folder holding `zipclip.exe` (cross-compiled
with mingw-w64 on Linux, natively on Windows), the bundled minimal
ffmpeg build, the current official `yt-dlp.exe` (checksum-verified
against the release's published SHA-256), and the license texts.
Download, unzip, run; nothing to install. ZipClip runs `yt-dlp -U` at
launch to keep yt-dlp current between releases.

`ffmpeg` rebuilds the bundled ffmpeg into `bin/ffmpeg-windows/`. The
`windows` task runs it automatically when that folder is empty. This
step needs Linux with the cross toolchain; on Arch/CachyOS:

    sudo pacman -S mingw-w64-gcc nasm make

## Why a custom ffmpeg build

A stock ffmpeg build is over 100MB and carries network protocols,
capture devices, and hundreds of codecs ZipClip never touches. The
pipeline needs a small, known surface: h264/aac in and out, mp4, the
scene-detection filters, and a jpg preview frame. Building with
`--disable-everything` plus an explicit enable list keeps the binary
around 14MB and removes the unused attack surface. The enable list
(documented on `ffmpegConfigureArgs` in `ffmpeg.go`) is a contract
with the pipeline: the yt-dlp download step pins its output to
h264/aac in mp4 (`-S codec:h264:aac --remux-video mp4`), so the
decoder list is guaranteed to cover what arrives.

Third-party sources are downloaded into `installer/vendor/`, which is
gitignored: none of it is our code and none of it is committed. The
build tree under `installer/vendor/build/` is wiped on every run, and
`bin/` outputs are rebuilt per release like zipclip.exe itself.

## Version pins

`ffmpeg.go` pins an exact ffmpeg release tarball, verified against a
SHA-256 recorded next to it, and an exact x264 commit on the upstream
`stable` branch, verified by git itself. To bump:

1. Pick the new ffmpeg release, download it, run `sha256sum`, and
   update `ffmpegVersion` and `ffmpegSHA256`.
2. Update `x264Commit` to the current tip of the `stable` branch:
   `git ls-remote https://code.videolan.org/videolan/x264.git stable`
3. `just ffmpeg`, then `just windows` for the next release.

ffmpeg is stable; expect to bump for point releases and CVEs, not on
a schedule. yt-dlp is not pinned: the zip carries whatever release is
current at package time, and the app updates it at launch anyway.

## License

libx264 makes the ffmpeg build GPL (`--enable-gpl`). ZipClip only
invokes the exes as separate processes, so this does not change
ZipClip's own license, but the package must carry the GPL text (it
does, via FFMPEG_LICENSE.txt) and the source must be obtainable. The
pins identify the exact upstream sources; this tool is the build
recipe. yt-dlp is distributed under the Unlicense, which permits
redistribution without conditions.
