package main

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// savingEntry is an Entry that saves the settings when the user presses
// Enter or moves focus away, so no Save button is needed. Edits in
// between are flagged through onEdit.
type savingEntry struct {
	widget.Entry
	onSave func()
}

func newSavingEntry(onEdit, onSave func()) *savingEntry {
	entry := &savingEntry{onSave: onSave}
	entry.ExtendBaseWidget(entry)
	entry.OnChanged = func(string) { onEdit() }
	entry.OnSubmitted = func(string) { onSave() }
	return entry
}

// FocusLost saves after the user moves on to another widget.
func (e *savingEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onSave != nil {
		e.onSave()
	}
}

// settingsForm holds the widgets that edit the configuration.
type settingsForm struct {
	channel      *savingEntry
	runTime      *savingEntry
	cutEnabled   *widget.Check
	scanWindow   *savingEntry
	threshold    *savingEntry
	backoff      *savingEntry
	introEnabled *widget.Check
	intro        *savingEntry
	outputDir    *savingEntry
	workDir      *savingEntry
	ytdlp        *savingEntry
	ffmpeg       *savingEntry
	ffprobe      *savingEntry
	devMode      *widget.Check
	keepDays     *savingEntry
	autoUpload   *widget.Check
	ytClientID   *savingEntry
	ytSecret     *savingEntry

	// status shows whether what is on screen has been saved.
	status *widget.Label
	// filling is set while the widgets are being loaded from a
	// Config, so the loads do not count as edits or trigger saves.
	filling bool
	// restoring is set while a rejected channel is being put back, so
	// the save does not check that channel again.
	restoring bool
}

// newSettingsForm builds the widgets. save is called whenever a
// setting should be written: a checkbox changed, or a text field was
// submitted or left.
func newSettingsForm(save func()) *settingsForm {
	f := &settingsForm{status: widget.NewLabel("")}
	onSave := func() {
		if !f.filling {
			save()
		}
	}
	onEdit := func() {
		if !f.filling {
			f.setStatus("Not saved: press Enter or click elsewhere to save", widget.DangerImportance)
		}
	}
	f.build(onEdit, onSave)
	return f
}

// build creates every widget, wired to the edit and save callbacks.
func (f *settingsForm) build(onEdit, onSave func()) {
	check := func(label string) *widget.Check {
		return widget.NewCheck(label, func(bool) { onSave() })
	}
	entry := func() *savingEntry { return newSavingEntry(onEdit, onSave) }
	f.channel = entry()
	f.runTime = entry()
	f.cutEnabled = check("Cut the starting-soon screen")
	f.scanWindow = entry()
	f.threshold = entry()
	f.backoff = entry()
	f.introEnabled = check("Add an intro video to the start of each VOD")
	f.intro = entry()
	f.outputDir = entry()
	f.workDir = entry()
	f.ytdlp = entry()
	f.ffmpeg = entry()
	f.ffprobe = entry()
	f.devMode = check("Run step by step: approve each cut, keep intermediate files")
	f.keepDays = entry()
	f.autoUpload = check("Upload finished videos to YouTube automatically")
	f.ytClientID = entry()
	f.ytSecret = entry()
	f.ytSecret.Password = true
}

// setStatus shows the save state: green for saved, red for not.
func (f *settingsForm) setStatus(text string, importance widget.Importance) {
	f.status.Importance = importance
	f.status.SetText(text)
}

func (f *settingsForm) fill(cfg Config) {
	f.filling = true
	defer func() { f.filling = false }()
	f.channel.SetText(cfg.Channel)
	f.runTime.SetText(cfg.DailyRunTime)
	f.cutEnabled.SetChecked(cfg.CutEnabled)
	f.scanWindow.SetText(strconv.Itoa(cfg.ScanWindowMinutes))
	f.threshold.SetText(strconv.FormatFloat(cfg.SceneThreshold, 'f', -1, 64))
	f.backoff.SetText(strconv.Itoa(cfg.CutBackoffSeconds))
	f.introEnabled.SetChecked(cfg.IntroEnabled)
	f.intro.SetText(cfg.IntroFile)
	f.outputDir.SetText(cfg.OutputDir)
	f.workDir.SetText(cfg.WorkDir)
	f.ytdlp.SetText(cfg.YtdlpPath)
	f.ffmpeg.SetText(cfg.FfmpegPath)
	f.ffprobe.SetText(cfg.FfprobePath)
	f.devMode.SetChecked(cfg.DevMode)
	f.keepDays.SetText(strconv.Itoa(cfg.KeepFinalDays))
	f.autoUpload.SetChecked(cfg.AutoUpload)
	f.ytClientID.SetText(cfg.YouTubeClientID)
	f.ytSecret.SetText(cfg.YouTubeClientSecret)
}

// collect builds a Config from the widgets, reporting the first
// unparseable numeric field.
func (f *settingsForm) collect() (Config, error) {
	cfg := DefaultConfig()
	cfg.Channel = strings.TrimSpace(f.channel.Text)
	cfg.DailyRunTime = strings.TrimSpace(f.runTime.Text)
	cfg.CutEnabled = f.cutEnabled.Checked
	cfg.IntroEnabled = f.introEnabled.Checked
	cfg.IntroFile = strings.TrimSpace(f.intro.Text)
	cfg.OutputDir = strings.TrimSpace(f.outputDir.Text)
	cfg.WorkDir = strings.TrimSpace(f.workDir.Text)
	cfg.YtdlpPath = strings.TrimSpace(f.ytdlp.Text)
	cfg.FfmpegPath = strings.TrimSpace(f.ffmpeg.Text)
	cfg.FfprobePath = strings.TrimSpace(f.ffprobe.Text)
	cfg.DevMode = f.devMode.Checked
	cfg.AutoUpload = f.autoUpload.Checked
	cfg.YouTubeClientID = strings.TrimSpace(f.ytClientID.Text)
	cfg.YouTubeClientSecret = strings.TrimSpace(f.ytSecret.Text)
	var numErr error
	cfg.ScanWindowMinutes, numErr = parseIntField(numErr, "scan window", f.scanWindow.Text)
	cfg.SceneThreshold, numErr = parseFloatField(numErr, "scene threshold", f.threshold.Text)
	cfg.CutBackoffSeconds, numErr = parseIntField(numErr, "cut backoff", f.backoff.Text)
	cfg.KeepFinalDays, numErr = parseIntField(numErr, "keep finished days", f.keepDays.Text)
	return cfg, numErr
}

// parseIntField keeps the first error while parsing later fields.
func parseIntField(prev error, label, text string) (int, error) {
	value, parseErr := strconv.Atoi(strings.TrimSpace(text))
	if parseErr != nil {
		if prev != nil {
			return 0, prev
		}
		return 0, fmt.Errorf("%s must be a whole number", label)
	}
	return value, prev
}

// parseFloatField keeps the first error while parsing later fields.
func parseFloatField(prev error, label, text string) (float64, error) {
	value, parseErr := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if parseErr != nil {
		if prev != nil {
			return 0, prev
		}
		return 0, fmt.Errorf("%s must be a number", label)
	}
	return value, prev
}

func (u *UI) buildSettingsPane() fyne.CanvasObject {
	u.form = newSettingsForm(func() { u.saveSettings(nil) })
	u.form.fill(u.store.Get())
	u.form.setStatus("Saved", widget.SuccessImportance)
	picked := func() { u.saveSettings(nil) }
	form := widget.NewForm(
		widget.NewFormItem("Channel", u.form.channel),
		widget.NewFormItem("Daily run time", u.form.runTime),
		widget.NewFormItem("Cut", u.form.cutEnabled),
		widget.NewFormItem("Scan window (minutes)", u.form.scanWindow),
		widget.NewFormItem("Scene threshold (0-1)", u.form.threshold),
		widget.NewFormItem("Cut backoff (seconds)", u.form.backoff),
		widget.NewFormItem("Intro", u.form.introEnabled),
		widget.NewFormItem("Intro file", u.withFilePicker(u.form.intro, picked)),
		widget.NewFormItem("Output folder", u.withDirPicker(u.form.outputDir, picked)),
		widget.NewFormItem("Work folder", u.withDirPicker(u.form.workDir, picked)),
		widget.NewFormItem("yt-dlp path", u.form.ytdlp),
		widget.NewFormItem("ffmpeg path", u.form.ffmpeg),
		widget.NewFormItem("ffprobe path", u.form.ffprobe),
		widget.NewFormItem("Dev mode", u.form.devMode),
		widget.NewFormItem("Keep finished (days)", u.form.keepDays),
		widget.NewFormItem("Uploads", u.form.autoUpload),
		widget.NewFormItem("YouTube client ID", u.form.ytClientID),
		widget.NewFormItem("YouTube client secret", u.form.ytSecret),
	)
	buttons := container.NewHBox(
		widget.NewButton("Prepare intro", u.onPrepareIntro),
		widget.NewButton("Connect YouTube", u.onConnectYouTube),
		widget.NewButton("Reset settings", u.onResetSettings),
		widget.NewButton("Reset archive", u.onResetArchive),
	)
	bar := container.NewBorder(nil, nil, u.form.status, buttons)
	return container.NewBorder(nil, bar, nil, nil, container.NewVScroll(form))
}

// pickTarget is an entry a browse button fills in. It is the widget
// itself, so the extended settings entries keep their focus handling.
type pickTarget interface {
	fyne.CanvasObject
	SetText(string)
}

// withFilePicker adds a browse button to entry. onPick, when not nil,
// runs after a file was chosen (the settings form saves then).
func (u *UI) withFilePicker(entry pickTarget, onPick func()) fyne.CanvasObject {
	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, openErr error) {
			if openErr != nil || reader == nil {
				return
			}
			entry.SetText(reader.URI().Path())
			// The file is not read here; only its path matters.
			_ = reader.Close()
			if onPick != nil {
				onPick()
			}
		}, u.window)
	})
	return container.NewBorder(nil, nil, nil, browse, entry)
}

// withDirPicker adds a browse button to entry. onPick, when not nil,
// runs after a folder was chosen.
func (u *UI) withDirPicker(entry pickTarget, onPick func()) fyne.CanvasObject {
	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(list fyne.ListableURI, openErr error) {
			if openErr != nil || list == nil {
				return
			}
			entry.SetText(list.Path())
			if onPick != nil {
				onPick()
			}
		}, u.window)
	})
	return container.NewBorder(nil, nil, nil, browse, entry)
}

// saveSettings writes what is on screen to disk and the live config,
// reporting the result in the status line rather than a dialog. A new
// channel name is checked and confirmed before it is kept. done, when
// not nil, runs once the save (and any channel check) has finished.
func (u *UI) saveSettings(done func()) {
	cfg, collectErr := u.form.collect()
	if collectErr != nil {
		u.form.setStatus("Not saved: "+collectErr.Error(), widget.DangerImportance)
		return
	}
	if saveErr := cfg.Save(u.cfgPath); saveErr != nil {
		u.form.setStatus("Not saved: "+saveErr.Error(), widget.DangerImportance)
		return
	}
	previous := u.store.Get()
	u.store.Set(cfg)
	u.form.setStatus("Saved", widget.SuccessImportance)
	if cfg.Channel != previous.Channel && cfg.Channel != "" && !u.form.restoring {
		u.confirmChannel(previous.Channel, cfg, done)
		return
	}
	if done != nil {
		done()
	}
}

// confirmChannel checks a newly entered channel against Twitch and, if
// it exists, asks before cataloging it. A channel that cannot be found
// or is declined is put back to the previous value.
func (u *UI) confirmChannel(previous string, cfg Config, done func()) {
	u.form.setStatus("Checking channel "+cfg.Channel+"...", widget.MediumImportance)
	go func() {
		lookErr := LookupChannel(context.Background(), cfg, u.logger.Logf)
		fyne.Do(func() {
			if lookErr != nil {
				dialog.ShowError(lookErr, u.window)
				u.restoreChannel(previous)
				return
			}
			u.form.setStatus("Saved", widget.SuccessImportance)
			if IsChannelScanned(cfg.Channel) {
				if done != nil {
					done()
				}
				return
			}
			u.askToCatalog(previous, cfg.Channel, done)
		})
	}()
}

// askToCatalog confirms the channel and starts the one-time catalog
// of its existing VODs.
func (u *UI) askToCatalog(previous, channel string, done func()) {
	message := fmt.Sprintf("Use Twitch channel %q?\n\nZipClip will catalog its existing VODs now "+
		"(all but the newest three are marked as done) and download anything newer on every run.", channel)
	dialog.NewConfirm("Confirm channel", message, func(confirmed bool) {
		if !confirmed {
			u.restoreChannel(previous)
			return
		}
		u.startCatalog()
		if done != nil {
			done()
		}
	}, u.window).Show()
}

// restoreChannel puts the previous channel back without checking it
// again.
func (u *UI) restoreChannel(previous string) {
	u.form.restoring = true
	defer func() { u.form.restoring = false }()
	u.form.channel.SetText(previous)
	u.saveSettings(nil)
}

func (u *UI) onPrepareIntro() {
	cfg := u.store.Get()
	go func() {
		prepErr := PrepareIntro(context.Background(), cfg, u.logger.Logf)
		fyne.Do(func() {
			if prepErr != nil {
				dialog.ShowError(prepErr, u.window)
				return
			}
			dialog.ShowInformation("Intro ready",
				"The intro was re-encoded to match the latest VOD.", u.window)
		})
	}()
}

// onConnectYouTube runs the one-time Google authorization with the
// saved YouTube client credentials, opening the consent page in the
// user's browser.
func (u *UI) onConnectYouTube() {
	cfg := u.store.Get()
	openURL := func(link string) error {
		parsed, parseErr := url.Parse(link)
		if parseErr != nil {
			return parseErr
		}
		return u.fyneApp.OpenURL(parsed)
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		authErr := NewYouTubeClient(cfg).Authorize(ctx, openURL)
		fyne.Do(func() {
			if authErr != nil {
				dialog.ShowError(authErr, u.window)
				return
			}
			dialog.ShowInformation("YouTube connected",
				"ZipClip can now upload finished videos to your channel.", u.window)
		})
	}()
}

// onResetSettings puts every setting back to its default after a
// confirmation, and saves the result.
func (u *UI) onResetSettings() {
	message := "Put every setting back to its default? Your current settings are overwritten."
	dialog.NewConfirm("Reset settings", message, func(confirmed bool) {
		if !confirmed {
			return
		}
		cfg := DefaultConfig()
		if saveErr := cfg.Save(u.cfgPath); saveErr != nil {
			dialog.ShowError(saveErr, u.window)
			return
		}
		u.store.Set(cfg)
		u.form.fill(cfg)
		u.form.setStatus("Saved", widget.SuccessImportance)
	}, u.window).Show()
}

// onResetArchive deletes the download archive after a confirmation,
// along with the record of which channels were cataloged, so the app
// starts over like a fresh install: the channel is cataloged again and
// only its newest VODs are processed.
func (u *UI) onResetArchive() {
	message := "Delete the download archive? ZipClip will catalog the channel again " +
		"on the next run and process only its newest VODs, as on a fresh install."
	dialog.NewConfirm("Reset archive", message, func(confirmed bool) {
		if !confirmed {
			return
		}
		if rmErr := ResetArchive(); rmErr != nil {
			dialog.ShowError(rmErr, u.window)
			return
		}
		dialog.ShowInformation("Archive reset", "The download archive was deleted.", u.window)
	}, u.window).Show()
}

// valueOf returns what the form currently shows for a setting, for
// the walkthrough to prefill.
func (f *settingsForm) valueOf(field Field) string {
	if entry := f.entryFor(field); entry != nil {
		return entry.Text
	}
	return ""
}

// setValue puts text into the form widget for a setting; the caller
// saves afterwards.
func (f *settingsForm) setValue(field Field, text string) {
	if entry := f.entryFor(field); entry != nil {
		entry.SetText(strings.TrimSpace(text))
	}
}

// disable unticks the checkbox of an optional feature.
func (f *settingsForm) disable(feature Feature) {
	switch feature {
	case FeatureCut:
		f.cutEnabled.SetChecked(false)
	case FeatureIntro:
		f.introEnabled.SetChecked(false)
	case FeatureUploads:
		f.autoUpload.SetChecked(false)
	}
}

// entryFor maps a setting to its text widget.
func (f *settingsForm) entryFor(field Field) *savingEntry {
	switch field {
	case FieldChannel:
		return f.channel
	case FieldRunTime:
		return f.runTime
	case FieldScanWindow:
		return f.scanWindow
	case FieldThreshold:
		return f.threshold
	case FieldBackoff:
		return f.backoff
	case FieldKeepDays:
		return f.keepDays
	case FieldIntroFile:
		return f.intro
	case FieldOutputDir:
		return f.outputDir
	case FieldWorkDir:
		return f.workDir
	case FieldYtdlp:
		return f.ytdlp
	case FieldFfmpeg:
		return f.ffmpeg
	case FieldFfprobe:
		return f.ffprobe
	case FieldYouTubeID:
		return f.ytClientID
	case FieldYouTubeSecret:
		return f.ytSecret
	default:
		return nil
	}
}
