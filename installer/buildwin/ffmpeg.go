package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"time"
)

// Pinned upstream sources for the bundled ffmpeg. The tarball is
// verified against the recorded SHA-256; the x264 checkout is
// verified by git against the commit id, which hashes the tree.
const (
	ffmpegVersion = "8.1.2"
	ffmpegSHA256  = "464beb5e7bf0c311e68b45ae2f04e9cc2af88851abb4082231742a74d97b524c"
	ffmpegURL     = "https://ffmpeg.org/releases/ffmpeg-" + ffmpegVersion + ".tar.xz"
	x264Commit    = "b35605ace3ddf7c1a5d67a2eb553f034aef41d55"
	x264Repo      = "https://code.videolan.org/videolan/x264.git"
	triplet       = "x86_64-w64-mingw32"
)

// ffmpegPaths are the fixed locations the ffmpeg build works in:
// sources in the gitignored installer/vendor, a scratch tree wiped
// every run, and the dist the zip and MSIX consume.
type ffmpegPaths struct {
	vendor string
	build  string
	prefix string
	src    string
	dist   string
}

func newFfmpegPaths(root string) ffmpegPaths {
	vendor := filepath.Join(root, "installer", "vendor")
	build := filepath.Join(vendor, "build")
	return ffmpegPaths{
		vendor: vendor,
		build:  build,
		prefix: filepath.Join(build, "prefix"),
		src:    filepath.Join(build, "ffmpeg-"+ffmpegVersion),
		dist:   filepath.Join(root, "bin", "ffmpeg-windows"),
	}
}

// buildFfmpeg cross-compiles the minimal static ffmpeg.exe and
// ffprobe.exe into bin/ffmpeg-windows. See installer/buildwin/README.md
// for what the enable list covers and how to bump the pins.
func (c *checkout) buildFfmpeg() error {
	if runtime.GOOS != "linux" {
		return errors.New("the ffmpeg cross build needs Linux with mingw-w64; copy in bin/ffmpeg-windows built elsewhere")
	}
	toolErr := requireTools(triplet+"-gcc", triplet+"-strip", "nasm", "make", "git", "tar", "pkg-config")
	if toolErr != nil {
		return toolErr
	}
	paths := newFfmpegPaths(c.dir)
	if rmErr := c.removeAll(paths.build); rmErr != nil {
		return rmErr
	}
	if mkErr := c.mkdirAll(paths.prefix); mkErr != nil {
		return mkErr
	}
	if srcErr := c.fetchFfmpegSource(paths); srcErr != nil {
		return srcErr
	}
	if x264Err := c.fetchX264(paths); x264Err != nil {
		return x264Err
	}
	if buildErr := buildX264(paths); buildErr != nil {
		return buildErr
	}
	if buildErr := compileFfmpeg(paths); buildErr != nil {
		return buildErr
	}
	return c.stageFfmpegDist(paths)
}

// fetchFfmpegSource downloads the pinned release tarball if missing,
// verifies it, and unpacks it into the build tree.
func (c *checkout) fetchFfmpegSource(paths ffmpegPaths) error {
	tarball := filepath.Join(paths.vendor, "ffmpeg-"+ffmpegVersion+".tar.xz")
	if !c.exists(tarball) {
		fmt.Println("downloading ffmpeg", ffmpegVersion, "...")
		if _, dlErr := c.download(ffmpegURL, tarball); dlErr != nil {
			return dlErr
		}
	}
	sum, sumErr := c.fileSHA256(tarball)
	if sumErr != nil {
		return sumErr
	}
	if sum != ffmpegSHA256 {
		return fmt.Errorf("ffmpeg tarball checksum mismatch: got %s want %s", sum, ffmpegSHA256)
	}
	fmt.Println("unpacking ffmpeg...")
	return run(paths.build, nil, "tar", "-xf", tarball)
}

// fetchX264 clones x264 if missing and pins the checkout to the
// recorded commit.
func (c *checkout) fetchX264(paths ffmpegPaths) error {
	clone := filepath.Join(paths.vendor, "x264")
	if !c.exists(filepath.Join(clone, ".git", "HEAD")) {
		fmt.Println("cloning x264...")
		if cloneErr := run(paths.vendor, nil, "git", "clone", x264Repo, clone); cloneErr != nil {
			return cloneErr
		}
	}
	if fetchErr := run(clone, nil, "git", "fetch", "--quiet", "origin"); fetchErr != nil {
		return fetchErr
	}
	if coErr := run(clone, nil, "git", "checkout", "--quiet", x264Commit); coErr != nil {
		return coErr
	}
	return run(clone, nil, "git", "clean", "-xfdq")
}

// buildX264 compiles the static encoder library the ffmpeg build
// links against.
func buildX264(paths ffmpegPaths) error {
	fmt.Println("building x264", x264Commit, "...")
	clone := filepath.Join(paths.vendor, "x264")
	configureErr := run(clone, nil, "./configure",
		"--host="+triplet,
		"--cross-prefix="+triplet+"-",
		"--prefix="+paths.prefix,
		"--enable-static",
		"--disable-cli",
		"--disable-opencl",
		"--disable-avs",
		"--disable-swscale",
		"--disable-lavf",
		"--disable-ffms",
		"--disable-gpac",
		"--disable-lsmash")
	if configureErr != nil {
		return configureErr
	}
	if makeErr := run(clone, nil, "make", "-j", jobs()); makeErr != nil {
		return makeErr
	}
	return run(clone, nil, "make", "install")
}

// compileFfmpeg configures and builds ffmpeg with only what ZipClip
// and yt-dlp's mp4 remux actually use.
func compileFfmpeg(paths ffmpegPaths) error {
	fmt.Println("configuring ffmpeg", ffmpegVersion, "...")
	env := []string{"PKG_CONFIG_LIBDIR=" + filepath.Join(paths.prefix, "lib", "pkgconfig")}
	configureErr := run(paths.src, env, "./configure", ffmpegConfigureArgs(paths.prefix)...)
	if configureErr != nil {
		return configureErr
	}
	fmt.Println("building ffmpeg (this takes a few minutes)...")
	return run(paths.src, nil, "make", "-j", jobs())
}

// ffmpegConfigureArgs is the whole contract with the pipeline. What
// each enable group is for:
//
//	protocols file,pipe,fd     local files and stdout ("-f null -")
//	demuxer mov                mp4/mov input (VODs, intro files)
//	demuxer matroska,mpegts    mkv/webm intros, yt-dlp's HLS output
//	demuxer concat             the intro + trimmed VOD join
//	demuxer image2,mjpeg       still-image input
//	muxer mp4                  every video output ZipClip writes
//	muxer image2,mjpeg         the preview frame jpg
//	muxer null                 the scene-detection scan pass
//	decoders                   h264/aac for VODs, the rest so a user's
//	                           intro file in a common format decodes
//	encoders                   libx264 + aac (intro re-encode),
//	                           mjpeg (preview frame)
//	bsf aac_adtstoasc          yt-dlp's MPEG-TS to mp4 remux
//	bsf h264_mp4toannexb       stream-copy between container types
//	filters                    scene scan (select, showinfo), intro
//	                           scaling (scale), plus the conversion
//	                           filters ffmpeg inserts automatically
func ffmpegConfigureArgs(prefix string) []string {
	return []string{
		"--arch=x86_64",
		"--target-os=mingw32",
		"--cross-prefix=" + triplet + "-",
		"--pkg-config=pkg-config",
		"--pkg-config-flags=--static",
		"--enable-gpl",
		"--enable-libx264",
		"--enable-static",
		"--disable-shared",
		"--disable-debug",
		"--disable-doc",
		"--disable-avdevice",
		"--disable-network",
		"--disable-autodetect",
		"--disable-everything",
		"--disable-ffplay",
		"--enable-w32threads",
		"--enable-protocol=file,pipe,fd",
		"--enable-demuxer=mov,matroska,mpegts,concat,image2,mjpeg",
		"--enable-muxer=mp4,image2,mjpeg,null",
		"--enable-decoder=h264,hevc,vp8,vp9,mpeg4,mjpeg,aac,mp3,opus,vorbis,flac,pcm_s16le,pcm_f32le",
		"--enable-encoder=libx264,aac,mjpeg",
		"--enable-parser=h264,hevc,aac,vp9,mpeg4video,mpegaudio,opus,vorbis,flac,mjpeg",
		"--enable-bsf=aac_adtstoasc,h264_mp4toannexb,extract_extradata",
		"--enable-filter=scale,select,showinfo,format,aformat,aresample,fps,null,anull",
		"--extra-cflags=-I" + filepath.Join(prefix, "include"),
		"--extra-ldflags=-L" + filepath.Join(prefix, "lib") + " -static",
	}
}

// stageFfmpegDist copies the stripped binaries, the license texts,
// and a record of what was built into bin/ffmpeg-windows.
func (c *checkout) stageFfmpegDist(paths ffmpegPaths) error {
	if mkErr := c.mkdirAll(paths.dist); mkErr != nil {
		return mkErr
	}
	copies := [][2]string{
		{filepath.Join(paths.src, "ffmpeg.exe"), "ffmpeg.exe"},
		{filepath.Join(paths.src, "ffprobe.exe"), "ffprobe.exe"},
		{filepath.Join(paths.src, "COPYING.GPLv3"), "FFMPEG_LICENSE.txt"},
		{filepath.Join(paths.src, "LICENSE.md"), "FFMPEG_LICENSE_OVERVIEW.md"},
		{filepath.Join(paths.vendor, "x264", "COPYING"), "X264_LICENSE.txt"},
	}
	for _, pair := range copies {
		if copyErr := c.copyFile(pair[0], filepath.Join(paths.dist, pair[1])); copyErr != nil {
			return copyErr
		}
	}
	stripErr := run(paths.dist, nil, triplet+"-strip", "ffmpeg.exe", "ffprobe.exe")
	if stripErr != nil {
		return stripErr
	}
	versions := fmt.Sprintf("ffmpeg %s (%s)\nx264 commit %s (%s)\nbuilt %s, GPLv3 static build\n",
		ffmpegVersion, ffmpegURL, x264Commit, x264Repo,
		time.Now().UTC().Format(time.RFC3339))
	return c.writeFile(filepath.Join(paths.dist, "VERSIONS.txt"), []byte(versions))
}

// jobs is the parallelism for make, matching the machine.
func jobs() string {
	return fmt.Sprintf("%d", runtime.NumCPU())
}
