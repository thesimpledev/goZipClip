package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// UI owns the Fyne application: window, tabs, and tray icon.
type UI struct {
	fyneApp fyne.App
	window  fyne.Window
	cfgPath string
	store   *ConfigStore
	logger  *Logger
	pipe    *Pipeline
	sched   *Scheduler

	stateLabel   *widget.Label
	nextRunLabel *widget.Label
	logLabel     *widget.Label
	pauseButton  *widget.Button

	approveInfo  *widget.Label
	approveTime  *widget.Entry
	previewImage *canvas.Image

	cleanupLabel *widget.Label
	candidates   []Candidate

	form *settingsForm
}

// NewUI builds the window, tabs, and tray icon. It does not show
// anything until ShowAndRun.
func NewUI(cfgPath string, store *ConfigStore, logger *Logger, pipe *Pipeline, sched *Scheduler) *UI {
	u := &UI{
		fyneApp: app.New(),
		cfgPath: cfgPath,
		store:   store,
		logger:  logger,
		pipe:    pipe,
		sched:   sched,
	}
	u.fyneApp.SetIcon(appIcon)
	u.window = u.fyneApp.NewWindow("ZipClip")
	u.window.Resize(fyne.NewSize(820, 600))
	u.buildContent()
	u.setupTray()
	u.window.SetCloseIntercept(u.window.Hide)
	u.wireCallbacks()
	return u
}

// ShowAndRun displays the window and blocks until the app quits.
func (u *UI) ShowAndRun() {
	u.refreshLog()
	u.refreshStatus()
	u.window.ShowAndRun()
}

func (u *UI) buildContent() {
	tabs := container.NewAppTabs(
		container.NewTabItem("Status", u.buildStatusPane()),
		container.NewTabItem("Approve", u.buildApprovePane()),
		container.NewTabItem("Cleanup", u.buildCleanupPane()),
		container.NewTabItem("Settings", u.buildSettingsPane()),
		container.NewTabItem("About", u.buildAboutPane()),
	)
	if len(u.store.Get().Validate()) > 0 {
		tabs.SelectIndex(3)
	}
	u.window.SetContent(tabs)
}

func (u *UI) wireCallbacks() {
	u.pipe.SetOnChange(func() { fyne.Do(u.refreshStatus) })
	u.sched.SetOnChange(func() { fyne.Do(u.refreshStatus) })
	u.logger.SetOnLine(func() { fyne.Do(u.refreshLog) })
}

func (u *UI) setupTray() {
	desk, ok := u.fyneApp.(desktop.App)
	if !ok {
		return
	}
	menu := fyne.NewMenu("ZipClip",
		fyne.NewMenuItem("Show window", u.window.Show),
		fyne.NewMenuItem("Run now", u.sched.RunNow),
		fyne.NewMenuItem("Pause or resume", func() { _ = u.sched.TogglePause() }),
	)
	desk.SetSystemTrayMenu(menu)
	desk.SetSystemTrayIcon(appIcon)
}

func (u *UI) buildStatusPane() fyne.CanvasObject {
	u.stateLabel = widget.NewLabel("idle")
	u.stateLabel.Wrapping = fyne.TextWrapWord
	u.nextRunLabel = widget.NewLabel("not scheduled yet")
	u.logLabel = widget.NewLabel("")
	u.logLabel.Wrapping = fyne.TextWrapWord
	runButton := widget.NewButton("Run now", u.sched.RunNow)
	u.pauseButton = widget.NewButton("Pause", func() { _ = u.sched.TogglePause() })
	top := container.NewVBox(
		u.stateLabel,
		u.nextRunLabel,
		container.NewHBox(runButton, u.pauseButton),
		widget.NewSeparator(),
	)
	return container.NewBorder(top, nil, nil, nil, container.NewVScroll(u.logLabel))
}

func (u *UI) refreshStatus() {
	state, detail := u.pipe.Status()
	text := state.String()
	if detail != "" {
		text += ": " + detail
	}
	u.stateLabel.SetText(text)
	next := "next run: not scheduled yet"
	if !u.sched.Next().IsZero() {
		next = "next run: " + u.sched.Next().Format("Mon 15:04")
	}
	if u.sched.Paused() {
		next = "paused"
		u.pauseButton.SetText("Resume")
	} else {
		u.pauseButton.SetText("Pause")
	}
	u.nextRunLabel.SetText(next)
	u.refreshApprove()
}

func (u *UI) refreshLog() {
	lines := u.logger.Recent()
	start := 0
	if len(lines) > 50 {
		start = len(lines) - 50
	}
	u.logLabel.SetText(strings.Join(lines[start:], "\n"))
}

func (u *UI) buildApprovePane() fyne.CanvasObject {
	u.approveInfo = widget.NewLabel("Nothing is awaiting approval.")
	u.approveInfo.Wrapping = fyne.TextWrapWord
	u.approveTime = widget.NewEntry()
	u.approveTime.SetPlaceHolder("HH:MM:SS")
	u.previewImage = canvas.NewImageFromResource(nil)
	u.previewImage.FillMode = canvas.ImageFillContain
	u.previewImage.SetMinSize(fyne.NewSize(640, 360))
	previewButton := widget.NewButton("Preview at time", u.onPreviewAt)
	approveButton := widget.NewButton("Approve and splice", u.onApprove)
	timeRow := container.NewBorder(nil, nil,
		widget.NewLabel("Stream starts at"),
		container.NewHBox(previewButton, approveButton),
		u.approveTime)
	controls := container.NewVBox(u.approveInfo, timeRow)
	return container.NewBorder(controls, nil, nil, nil, u.previewImage)
}

func (u *UI) refreshApprove() {
	pending := u.pipe.Pending()
	if pending == nil {
		u.approveInfo.SetText("Nothing is awaiting approval.")
		u.previewImage.File = ""
		u.previewImage.Refresh()
		return
	}
	u.approveInfo.SetText(fmt.Sprintf(
		"Detected stream start for %s. Check the frame below, adjust the time if needed, then approve.",
		filepath.Base(pending.VodPath)))
	if u.approveTime.Text == "" {
		u.approveTime.SetText(formatTimestamp(pending.Cut))
	}
	u.previewImage.File = pending.Preview
	u.previewImage.Refresh()
}

func (u *UI) onPreviewAt() {
	at, parseErr := parseTimestamp(u.approveTime.Text)
	if parseErr != nil {
		dialog.ShowError(parseErr, u.window)
		return
	}
	go func() {
		if prevErr := u.pipe.RegeneratePreview(context.Background(), at); prevErr != nil {
			fyne.Do(func() { dialog.ShowError(prevErr, u.window) })
			return
		}
		fyne.Do(u.previewImage.Refresh)
	}()
}

func (u *UI) onApprove() {
	at, parseErr := parseTimestamp(u.approveTime.Text)
	if parseErr != nil {
		dialog.ShowError(parseErr, u.window)
		return
	}
	if !u.pipe.Approve(at) {
		dialog.ShowInformation("Approve", "Nothing is awaiting approval.", u.window)
		return
	}
	u.approveTime.SetText("")
}

func (u *UI) buildCleanupPane() fyne.CanvasObject {
	u.cleanupLabel = widget.NewLabel("Press Refresh to list deletable files.")
	u.cleanupLabel.Wrapping = fyne.TextWrapWord
	refreshButton := widget.NewButton("Refresh", u.refreshCleanup)
	deleteButton := widget.NewButton("Delete listed files", u.onDeleteCandidates)
	buttons := container.NewHBox(refreshButton, deleteButton)
	return container.NewBorder(buttons, nil, nil, nil, container.NewVScroll(u.cleanupLabel))
}

func (u *UI) refreshCleanup() {
	candidates, listErr := CleanupCandidates(u.store.Get(), time.Now())
	if listErr != nil {
		dialog.ShowError(listErr, u.window)
		return
	}
	u.candidates = candidates
	if len(candidates) == 0 {
		u.cleanupLabel.SetText("Nothing to clean up.")
		return
	}
	var b strings.Builder
	// strings.Builder writes cannot fail; the returned values carry
	// no information here.
	for _, c := range candidates {
		_, _ = fmt.Fprintf(&b, "%s (%s)\n", c.Path, formatSize(c.Size))
	}
	_, _ = fmt.Fprintf(&b, "\nTotal: %s in %d file(s)", formatSize(TotalSize(candidates)), len(candidates))
	u.cleanupLabel.SetText(b.String())
}

func (u *UI) onDeleteCandidates() {
	if len(u.candidates) == 0 {
		dialog.ShowInformation("Cleanup", "Nothing listed. Press Refresh first.", u.window)
		return
	}
	message := fmt.Sprintf("Delete %d file(s), %s total? This cannot be undone.",
		len(u.candidates), formatSize(TotalSize(u.candidates)))
	dialog.NewConfirm("Confirm cleanup", message, func(confirmed bool) {
		if !confirmed {
			return
		}
		if delErr := DeleteCandidates(u.candidates); delErr != nil {
			dialog.ShowError(delErr, u.window)
		}
		u.candidates = nil
		u.refreshCleanup()
	}, u.window).Show()
}
