package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// onProcessedVideos lists the VODs in the download archive, newest
// first, with a Forget button on each so the next run downloads and
// processes that VOD again.
func (u *UI) onProcessedVideos() {
	entries, listErr := ProcessedVODs()
	if listErr != nil {
		dialog.ShowError(listErr, u.window)
		return
	}
	if len(entries) == 0 {
		dialog.ShowInformation("Processed videos",
			"Nothing has been processed yet. Once the channel is cataloged, its VODs show up here.", u.window)
		return
	}
	var box *dialog.CustomDialog
	rows := make([]fyne.CanvasObject, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, u.processedRow(entry, func() { box.Hide() }))
	}
	closeIcon := widget.NewButtonWithIcon("", theme.CancelIcon(), func() { box.Hide() })
	header := container.NewBorder(nil, nil,
		widget.NewLabel("VODs ZipClip has already handled. Forget one to download and process it again."),
		closeIcon)
	content := container.NewBorder(header, nil, nil, nil, container.NewVScroll(container.NewVBox(rows...)))
	box = dialog.NewCustomWithoutButtons("Processed videos", content, u.window)
	box.SetButtons([]fyne.CanvasObject{widget.NewButton("Close", func() { box.Hide() })})
	size := u.window.Canvas().Size()
	box.Resize(fyne.NewSize(size.Width*0.9, size.Height*0.85))
	box.Show()
}

// processedRow renders one archive entry with its Forget button. The
// closer hides the dialog before the list is rebuilt.
func (u *UI) processedRow(entry ProcessedVOD, closer func()) fyne.CanvasObject {
	label := widget.NewLabel(entry.Label())
	label.Truncation = fyne.TextTruncateEllipsis
	forget := widget.NewButton("Forget", func() {
		message := "Forget this VOD? It will be downloaded and processed again on the next run.\n\n" + entry.Label()
		dialog.NewConfirm("Forget VOD", message, func(confirmed bool) {
			if !confirmed {
				return
			}
			if forgetErr := ForgetVOD(entry.ID); forgetErr != nil {
				dialog.ShowError(forgetErr, u.window)
				return
			}
			closer()
			u.onProcessedVideos()
		}, u.window).Show()
	})
	return container.NewBorder(nil, nil, nil, forget, label)
}
