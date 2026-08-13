# Build the single-download Windows package (bin/zipclip-windows-amd64.zip)
windows:
    go -C installer/buildwin run . zip

# Rebuild the bundled minimal ffmpeg (needs Linux with mingw-w64)
ffmpeg:
    go -C installer/buildwin run . ffmpeg
