// Command buildwin produces the ZipClip Windows package.
//
//	go -C installer/buildwin run . zip      the single-download zip
//	go -C installer/buildwin run . ffmpeg   rebuild the bundled ffmpeg
//
// The zip task cross-compiles zipclip.exe (with mingw-w64 on Linux,
// natively on Windows), bundles the minimal ffmpeg build from
// bin/ffmpeg-windows (building it first when missing, which needs
// Linux), and writes bin/ZipClip-windows-x64.zip. The result unzips
// to a ZipClip folder the user just runs.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	task := "zip"
	if len(os.Args) > 1 {
		task = os.Args[1]
	}
	root, rootErr := repoRoot()
	if rootErr != nil {
		fail(rootErr)
	}
	repo, openErr := openCheckout(root)
	if openErr != nil {
		fail(openErr)
	}
	defer repo.close()
	if taskErr := repo.runTask(task); taskErr != nil {
		fail(taskErr)
	}
}

func fail(failure error) {
	// The build environment is unusable; exiting is the only option.
	fmt.Fprintln(os.Stderr, "buildwin:", failure)
	os.Exit(1)
}

func (c *checkout) runTask(task string) error {
	switch task {
	case "zip":
		return c.buildZip()
	case "ffmpeg":
		return c.buildFfmpeg()
	default:
		return fmt.Errorf("unknown task %q (want zip or ffmpeg)", task)
	}
}

// repoRoot walks up from the working directory to the checkout root,
// identified by the desktop module living in it. This is the only
// file access outside the os.Root handle, because the root is what
// it discovers.
func repoRoot() (string, error) {
	dir, wdErr := os.Getwd()
	if wdErr != nil {
		return "", wdErr
	}
	for {
		if markerErr := markerStat(dir); markerErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("not inside the zipclip checkout")
		}
		dir = parent
	}
}

// markerStat reports whether dir holds the desktop module marking the
// checkout root.
func markerStat(dir string) error {
	_, statErr := os.Stat(filepath.Join(dir, "desktop", "go.mod"))
	return statErr
}
