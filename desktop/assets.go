package main

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed icon.png
var iconPNG []byte

//go:embed thesimpledev-logo.png
var devLogoPNG []byte

// appIcon is the ZipClip application icon, shown in the window,
// taskbar, and system tray.
var appIcon = fyne.NewStaticResource("icon.png", iconPNG)

// devLogo is the TheSimpleDev logo shown in the About pane.
var devLogo = fyne.NewStaticResource("thesimpledev-logo.png", devLogoPNG)
