package main

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
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

	tabs         *container.AppTabs
	dashboardTab *container.TabItem
	logTab       *container.TabItem
	approveTab   *container.TabItem
	cleanupTab   *container.TabItem
	settingsTab  *container.TabItem
	aboutTab     *container.TabItem
	stateLabel   *widget.Label
	nextRunLabel *widget.Label
	channelLabel *widget.Label
	logLabel     *widget.Label
	pauseButton  *widget.Button
	cancelButton *widget.Button

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
	u.fyneApp.Settings().SetTheme(newZipTheme())
	u.window = u.fyneApp.NewWindow("ZipClip")
	u.window.Resize(fyne.NewSize(windowWidth, windowHeight))
	u.buildContent()
	u.setupTray()
	u.window.SetCloseIntercept(u.onClose)
	u.wireCallbacks()
	return u
}

// Window dimensions: the default size, and the floor below which the
// layout would start clipping controls.
const (
	windowWidth     = 900
	windowHeight    = 720
	windowMinWidth  = 860
	windowMinHeight = 640
)

// ShowAndRun displays the window, starts the first-run questions
// when the app has not been set up yet, and blocks until the app
// quits.
func (u *UI) ShowAndRun() {
	u.refreshLog()
	u.refreshStatus()
	u.window.Show()
	u.firstRun()
	u.fyneApp.Run()
}

func (u *UI) buildContent() {
	u.dashboardTab = container.NewTabItem("Dashboard", u.buildDashboardPane())
	u.logTab = container.NewTabItem("Log", u.buildLogPane())
	u.approveTab = container.NewTabItem("Approve", u.buildApprovePane())
	u.cleanupTab = container.NewTabItem("Cleanup", u.buildCleanupPane())
	u.settingsTab = container.NewTabItem("Settings", u.buildSettingsPane())
	u.aboutTab = container.NewTabItem("About", u.buildAboutPane())
	u.tabs = container.NewAppTabs(u.tabItems(u.store.Get().DevMode)...)
	// Fyne windows have no minimum size of their own; a transparent
	// rectangle behind the tabs sets the floor.
	floor := canvas.NewRectangle(color.Transparent)
	floor.SetMinSize(fyne.NewSize(windowMinWidth, windowMinHeight))
	u.window.SetContent(container.NewStack(floor, u.tabs))
}

// tabItems is the tab order. Approve and Cleanup only matter in dev
// mode, so normal use shows four tabs.
func (u *UI) tabItems(devMode bool) []*container.TabItem {
	items := []*container.TabItem{u.dashboardTab, u.logTab}
	if devMode {
		items = append(items, u.approveTab, u.cleanupTab)
	}
	return append(items, u.settingsTab, u.aboutTab)
}

// applyDevMode shows or hides the dev-mode tabs, keeping whatever
// tab was selected.
func (u *UI) applyDevMode(devMode bool) {
	keep := u.tabs.Selected()
	u.tabs.SetItems(u.tabItems(devMode))
	if keep != nil {
		u.tabs.Select(keep)
	}
}

// padded gives a pane a margin so its controls do not touch the
// window edges.
func padded(content fyne.CanvasObject) fyne.CanvasObject {
	return container.New(layout.NewCustomPaddedLayout(12, 12, 16, 16), content)
}

func (u *UI) wireCallbacks() {
	u.pipe.SetOnChange(func() { fyne.Do(u.refreshStatus) })
	u.sched.SetOnChange(func() { fyne.Do(u.refreshStatus) })
	u.logger.SetOnLine(func() { fyne.Do(u.refreshLog) })
	// A scheduled run that finds settings missing brings the window
	// forward and walks the user through them; the run starts once
	// everything is in place.
	u.sched.SetOnProblems(func(_ []string) {
		fyne.Do(func() {
			u.window.Show()
			u.ensureReady(u.sched.RunNow)
		})
	})
}

// onRunNow starts a run once the settings allow one.
func (u *UI) onRunNow() {
	u.ensureReady(u.sched.RunNow)
}

func (u *UI) setupTray() {
	desk, ok := u.fyneApp.(desktop.App)
	if !ok {
		return
	}
	menu := fyne.NewMenu("ZipClip",
		fyne.NewMenuItem("Show window", u.window.Show),
		fyne.NewMenuItem("Run now", u.onRunNow),
		fyne.NewMenuItem("Pause or resume", func() { _ = u.sched.TogglePause() }),
		fyne.NewMenuItem("Quit", u.fyneApp.Quit),
	)
	desk.SetSystemTrayMenu(menu)
	desk.SetSystemTrayIcon(appIcon)
}

// buildDashboardPane is the home tab: what ZipClip is doing, when it
// runs next, what is switched on, and every action with a sentence
// saying what it does.
func (u *UI) buildDashboardPane() fyne.CanvasObject {
	u.stateLabel = widget.NewLabelWithStyle("idle", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	u.stateLabel.SizeName = theme.SizeNameSubHeadingText
	u.stateLabel.Wrapping = fyne.TextWrapWord
	u.nextRunLabel = widget.NewLabel("not scheduled yet")
	u.channelLabel = widget.NewLabel("")
	u.channelLabel.Wrapping = fyne.TextWrapWord
	runButton := widget.NewButtonWithIcon("Run now", theme.MediaPlayIcon(), u.onRunNow)
	runButton.Importance = widget.HighImportance
	runLatestButton := widget.NewButtonWithIcon("Run latest VOD", theme.DownloadIcon(), u.onRunLatest)
	u.pauseButton = widget.NewButton("Pause", func() { _ = u.sched.TogglePause() })
	u.cancelButton = widget.NewButton("Cancel", u.pipe.Cancel)
	u.cancelButton.Importance = widget.DangerImportance
	u.cancelButton.Disable()
	processedButton := widget.NewButton("Processed videos", u.onProcessedVideos)
	outputButton := widget.NewButtonWithIcon("Open output folder", theme.FolderOpenIcon(), u.onOpenOutput)
	actions := container.New(layout.NewFormLayout(),
		runButton, describe("Check the channel for VODs it has not handled yet and process them."),
		runLatestButton, describe("Process the newest finished VOD on the channel, even if it was already handled."),
		u.pauseButton, describe("Hold the daily scheduled run, or resume it. Runs you start by hand still work."),
		u.cancelButton, describe("Stop the run that is in progress. The next scheduled run still happens."),
		processedButton, describe("See every VOD ZipClip has handled, and forget one to process it again."),
		outputButton, describe("Open the folder where finished videos land."),
	)
	content := container.NewVBox(
		u.stateLabel,
		u.nextRunLabel,
		u.channelLabel,
		widget.NewSeparator(),
		actions,
	)
	return padded(container.NewVScroll(content))
}

// describe is the sentence shown next to a dashboard button.
func describe(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

// buildLogPane shows the running log on its own tab.
func (u *UI) buildLogPane() fyne.CanvasObject {
	u.logLabel = widget.NewLabel("")
	u.logLabel.Wrapping = fyne.TextWrapWord
	intro := widget.NewLabel("Everything ZipClip does is written here and to the log file.")
	intro.Wrapping = fyne.TextWrapWord
	openButton := widget.NewButtonWithIcon("Open log file", theme.DocumentIcon(), u.onOpenLog)
	top := container.NewBorder(nil, nil, nil, openButton, intro)
	return padded(container.NewBorder(top, nil, nil, nil, container.NewVScroll(u.logLabel)))
}

// onOpenOutput opens the output folder in the system file browser.
func (u *UI) onOpenOutput() {
	u.openPath(u.store.Get().OutputDir)
}

// onOpenLog opens the log file in whatever the system uses for text.
func (u *UI) onOpenLog() {
	u.openPath(filepath.Join(filepath.Dir(u.cfgPath), "zipclip.log"))
}

// openPath hands a local file or folder to the system to open.
func (u *UI) openPath(path string) {
	if path == "" {
		dialog.ShowInformation("Open", "Nothing to open: the path is not set.", u.window)
		return
	}
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(slashed, "/") {
		// Windows drive paths need the leading slash of file:///C:/...
		slashed = "/" + slashed
	}
	link := &url.URL{Scheme: "file", Path: slashed}
	if openErr := u.fyneApp.OpenURL(link); openErr != nil {
		dialog.ShowError(openErr, u.window)
	}
}

func (u *UI) refreshStatus() {
	state, detail := u.pipe.Status()
	text := state.String()
	if detail != "" {
		text += ": " + detail
	}
	u.stateLabel.SetText(text)
	u.channelLabel.SetText(summarize(u.store.Get()))
	next := "next run: not scheduled yet"
	if !u.sched.Next().IsZero() {
		next = "next run: " + u.sched.Next().Format("Mon 3:04 PM")
	}
	if u.sched.Paused() {
		next = "paused"
		u.pauseButton.SetText("Resume")
	} else {
		u.pauseButton.SetText("Pause")
	}
	u.nextRunLabel.SetText(next)
	if u.pipe.Running() {
		u.cancelButton.Enable()
	} else {
		u.cancelButton.Disable()
	}
	u.refreshApprove()
}

// summarize is the one-line description of the current setup shown
// on the dashboard.
func summarize(cfg Config) string {
	if cfg.Channel == "" {
		return "No Twitch channel set yet."
	}
	onOff := func(on bool) string {
		if on {
			return "on"
		}
		return "off"
	}
	return fmt.Sprintf("Channel %s. Cut %s, intro %s, YouTube uploads %s.",
		cfg.Channel, onOff(cfg.CutEnabled), onOff(cfg.IntroEnabled), onOff(cfg.AutoUpload))
}

func (u *UI) refreshLog() {
	u.logLabel.SetText(strings.Join(u.logger.Recent(), "\n"))
}

// onRunLatest downloads and processes the newest VOD on the channel,
// warning first when the archive already lists it as handled. Missing
// settings are walked through first.
func (u *UI) onRunLatest() {
	u.ensureReady(u.runLatestChecked)
}

func (u *UI) runLatestChecked() {
	cfg := u.store.Get()
	go func() {
		id, idErr := LatestVODID(context.Background(), cfg, u.logger.Logf)
		if idErr != nil {
			fyne.Do(func() { dialog.ShowError(idErr, u.window) })
			return
		}
		if !IsArchived(id) {
			u.runLatest()
			return
		}
		fyne.Do(func() {
			message := "The latest VOD is already marked as handled. Run it again anyway?"
			dialog.NewConfirm("Run latest VOD", message, func(confirmed bool) {
				if confirmed {
					u.runLatest()
				}
			}, u.window).Show()
		})
	}()
}

// runLatest runs the pipeline on the newest VOD and surfaces errors.
func (u *UI) runLatest() {
	go func() {
		runErr := u.pipe.RunLatest(context.Background())
		if runErr != nil && !errors.Is(runErr, context.Canceled) {
			fyne.Do(func() { dialog.ShowError(runErr, u.window) })
		}
	}()
}

// startCatalog records the channel's existing VODs in the background
// and shows the Status tab so the progress is visible.
func (u *UI) startCatalog() {
	u.tabs.Select(u.dashboardTab)
	go func() {
		catErr := u.pipe.Catalog(context.Background())
		if catErr != nil && !errors.Is(catErr, context.Canceled) {
			fyne.Do(func() { dialog.ShowError(catErr, u.window) })
		}
	}()
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

// onClose handles the window's close button. By default the window
// hides to the tray so the daily run keeps going; when the user has
// asked for it, closing quits ZipClip instead.
func (u *UI) onClose() {
	if u.store.Get().QuitOnClose {
		u.fyneApp.Quit()
		return
	}
	u.window.Hide()
}
