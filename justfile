# Build the single-download Windows package (bin/zipclip-windows-amd64.zip)
windows:
    go -C installer/buildwin run . zip

# Rebuild the bundled minimal ffmpeg (needs Linux with mingw-w64)
ffmpeg:
    go -C installer/buildwin run . ffmpeg

# Build the Linux download (bin/zipclip-linux-amd64.tar.gz)
linux:
    go -C desktop build -trimpath -o zipclip .
    mkdir -p bin
    tar -czf bin/zipclip-linux-amd64.tar.gz -C desktop zipclip LICENSE README.md

# Gates that must pass before any release ships
check:
    test -z "$(gofmt -l desktop/*.go installer/buildwin/*.go)"
    go -C desktop vet ./...
    cd desktop && staticcheck ./...
    cd desktop && errcheck ./...
    cd desktop && revive ./...
    go -C desktop test ./... -race -vet=all -shuffle=on -count=1

# Build and publish the Linux download to the website bucket
release-linux: check linux
    aws-sec s3api put-object --bucket gozipclip.com-825a66c84550b1da --key downloads/zipclip-linux-amd64.tar.gz --body bin/zipclip-linux-amd64.tar.gz --content-type application/gzip --cache-control "max-age=3600"

# Build and publish the Windows download to the website bucket
release-windows: check windows
    aws-sec s3api put-object --bucket gozipclip.com-825a66c84550b1da --key downloads/zipclip-windows-amd64.zip --body bin/zipclip-windows-amd64.zip --content-type application/zip --cache-control "max-age=3600"
