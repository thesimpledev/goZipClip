package main

import (
	"fmt"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func (u *UI) buildAboutPane() fyne.CanvasObject {
	icon := canvas.NewImageFromResource(appIcon)
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(128, 128))
	title := widget.NewLabel("ZipClip")
	title.TextStyle = fyne.TextStyle{Bold: true}
	blurb := widget.NewLabel("Downloads new Twitch VODs, trims off the prestream waiting screen,\nsplices in your intro, and hands the result to your upload folder.")
	blurb.Wrapping = fyne.TextWrapWord
	site := widget.NewHyperlink("gozipclip.com", mustURL("https://gozipclip.com"))
	logo := canvas.NewImageFromResource(devLogo)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(128, 128))
	created := widget.NewLabel("Created by TheSimpleDev")
	devSite := widget.NewHyperlink("thesimpledev.com", mustURL("https://thesimpledev.com"))
	content := container.NewVBox(
		container.NewCenter(icon),
		container.NewCenter(title),
		container.NewCenter(blurb),
		container.NewCenter(site),
		widget.NewSeparator(),
		container.NewCenter(logo),
		container.NewCenter(created),
		container.NewCenter(devSite),
	)
	return container.NewVScroll(content)
}

// mustURL parses a compile-time constant URL; a failure is a broken
// program invariant.
func mustURL(raw string) *url.URL {
	parsed, parseErr := url.Parse(raw)
	if parseErr != nil {
		panic(fmt.Sprintf("bad built-in url %q: %v", raw, parseErr))
	}
	return parsed
}
