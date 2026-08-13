package main

import (
	"archive/zip"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
)

// The official standalone yt-dlp build and its published checksum
// list, both served from the project's latest GitHub release. The zip
// ships the current release; the app keeps it updated with -U.
const (
	ytdlpExeURL  = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp.exe"
	ytdlpSumsURL = "https://github.com/yt-dlp/yt-dlp/releases/latest/download/SHA2-256SUMS"
)

// zipName is the folder every entry lives under, so the archive
// unzips to a single ZipClip folder.
const zipName = "ZipClip"

// buildZip produces bin/zipclip-windows-amd64.zip (the fixed name the
// website serves, see website/DOWNLOADS.md): zipclip.exe, the bundled
// ffmpeg build (built first when missing), the current yt-dlp, and
// the license texts. Download, unzip, run.
func (c *checkout) buildZip() error {
	paths := newFfmpegPaths(c.dir)
	if !c.exists(filepath.Join(paths.dist, "ffmpeg.exe")) {
		if ffErr := c.buildFfmpeg(); ffErr != nil {
			return ffErr
		}
	}
	stage := filepath.Join(c.dir, "bin", zipName)
	if rmErr := c.removeAll(stage); rmErr != nil {
		return rmErr
	}
	if mkErr := c.mkdirAll(stage); mkErr != nil {
		return mkErr
	}
	if exeErr := compileZipclip(c.dir, stage); exeErr != nil {
		return exeErr
	}
	if ytErr := c.fetchYtdlp(stage); ytErr != nil {
		return ytErr
	}
	if copyErr := c.stageSupportFiles(paths.dist, stage); copyErr != nil {
		return copyErr
	}
	out := filepath.Join(c.dir, "bin", "zipclip-windows-amd64.zip")
	if zipErr := c.writeZip(stage, out); zipErr != nil {
		return zipErr
	}
	fmt.Println("built:", out)
	return nil
}

// compileZipclip builds the Windows GUI executable: cross-compiled
// through mingw-w64 on Linux, natively on Windows.
func compileZipclip(root, stage string) error {
	fmt.Println("compiling zipclip.exe...")
	env := []string{"CGO_ENABLED=1", "GOOS=windows", "GOARCH=amd64"}
	if runtime.GOOS != "windows" {
		if toolErr := requireTools(triplet + "-gcc"); toolErr != nil {
			return toolErr
		}
		env = append(env, "CC="+triplet+"-gcc", "CXX="+triplet+"-g++")
	}
	return run(filepath.Join(root, "desktop"), env,
		"go", "build", "-trimpath", "-ldflags", "-H=windowsgui",
		"-o", filepath.Join(stage, "zipclip.exe"), ".")
}

// fetchYtdlp downloads the official yt-dlp.exe into the stage,
// verified against the release's published SHA-256 sum.
func (c *checkout) fetchYtdlp(stage string) error {
	fmt.Println("downloading yt-dlp...")
	sums, sumsErr := fetchText(ytdlpSumsURL)
	if sumsErr != nil {
		return sumsErr
	}
	want, findErr := findSum(sums, "yt-dlp.exe")
	if findErr != nil {
		return findErr
	}
	dest := filepath.Join(stage, "yt-dlp.exe")
	got, dlErr := c.download(ytdlpExeURL, dest)
	if dlErr != nil {
		return dlErr
	}
	if got != want {
		return fmt.Errorf("yt-dlp checksum mismatch: got %s want %s", got, want)
	}
	return nil
}

// stageSupportFiles copies the ffmpeg build and the license texts in
// next to the executables.
func (c *checkout) stageSupportFiles(ffmpegDist, stage string) error {
	copies := []string{
		filepath.Join(ffmpegDist, "ffmpeg.exe"),
		filepath.Join(ffmpegDist, "ffprobe.exe"),
		filepath.Join(ffmpegDist, "FFMPEG_LICENSE.txt"),
		filepath.Join(ffmpegDist, "FFMPEG_LICENSE_OVERVIEW.md"),
		filepath.Join(ffmpegDist, "X264_LICENSE.txt"),
		filepath.Join(ffmpegDist, "VERSIONS.txt"),
		filepath.Join(c.dir, "desktop", "LICENSE"),
		filepath.Join(c.dir, "desktop", "README.md"),
	}
	for _, src := range copies {
		if copyErr := c.copyFile(src, filepath.Join(stage, filepath.Base(src))); copyErr != nil {
			return copyErr
		}
	}
	return nil
}

// writeZip archives the staged folder so it unzips to ZipClip/...
func (c *checkout) writeZip(stage, out string) error {
	if stage == "" || out == "" {
		return errors.New("zip paths missing")
	}
	relOut, relErr := c.rel(out)
	if relErr != nil {
		return relErr
	}
	file, createErr := c.fs.Create(relOut)
	if createErr != nil {
		return createErr
	}
	writer := zip.NewWriter(file)
	addErr := c.addZipEntries(writer, stage)
	closeErr := writer.Close()
	fileErr := file.Close()
	if addErr != nil {
		return addErr
	}
	if closeErr != nil {
		return closeErr
	}
	return fileErr
}

// addZipEntries writes every staged file under the ZipClip/ folder
// inside the archive.
func (c *checkout) addZipEntries(writer *zip.Writer, stage string) error {
	relStage, relErr := c.rel(stage)
	if relErr != nil {
		return relErr
	}
	dir, openErr := c.fs.Open(relStage)
	if openErr != nil {
		return openErr
	}
	entries, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if addErr := c.addZipEntry(writer, relStage, entry.Name()); addErr != nil {
			return addErr
		}
	}
	return nil
}

// addZipEntry copies one staged file into the archive.
func (c *checkout) addZipEntry(writer *zip.Writer, relStage, name string) error {
	data, fileErr := c.fs.ReadFile(relStage + "/" + name)
	if fileErr != nil {
		return fileErr
	}
	dest, createErr := writer.Create(zipName + "/" + name)
	if createErr != nil {
		return createErr
	}
	_, writeErr := dest.Write(data)
	return writeErr
}
