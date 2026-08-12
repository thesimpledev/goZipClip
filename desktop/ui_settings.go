package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// settingsForm holds the widgets that edit the configuration.
type settingsForm struct {
	channel    *widget.Entry
	runTime    *widget.Entry
	scanWindow *widget.Entry
	threshold  *widget.Entry
	backoff    *widget.Entry
	intro      *widget.Entry
	outputDir  *widget.Entry
	workDir    *widget.Entry
	ytdlp      *widget.Entry
	ffmpeg     *widget.Entry
	ffprobe    *widget.Entry
	devMode    *widget.Check
	keepDays   *widget.Entry
}

func newSettingsForm() *settingsForm {
	return &settingsForm{
		channel:    widget.NewEntry(),
		runTime:    widget.NewEntry(),
		scanWindow: widget.NewEntry(),
		threshold:  widget.NewEntry(),
		backoff:    widget.NewEntry(),
		intro:      widget.NewEntry(),
		outputDir:  widget.NewEntry(),
		workDir:    widget.NewEntry(),
		ytdlp:      widget.NewEntry(),
		ffmpeg:     widget.NewEntry(),
		ffprobe:    widget.NewEntry(),
		devMode:    widget.NewCheck("Run step by step: approve each cut, keep intermediate files", nil),
		keepDays:   widget.NewEntry(),
	}
}

func (f *settingsForm) fill(cfg Config) {
	f.channel.SetText(cfg.Channel)
	f.runTime.SetText(cfg.DailyRunTime)
	f.scanWindow.SetText(strconv.Itoa(cfg.ScanWindowMinutes))
	f.threshold.SetText(strconv.FormatFloat(cfg.SceneThreshold, 'f', -1, 64))
	f.backoff.SetText(strconv.Itoa(cfg.CutBackoffSeconds))
	f.intro.SetText(cfg.IntroFile)
	f.outputDir.SetText(cfg.OutputDir)
	f.workDir.SetText(cfg.WorkDir)
	f.ytdlp.SetText(cfg.YtdlpPath)
	f.ffmpeg.SetText(cfg.FfmpegPath)
	f.ffprobe.SetText(cfg.FfprobePath)
	f.devMode.SetChecked(cfg.DevMode)
	f.keepDays.SetText(strconv.Itoa(cfg.KeepFinalDays))
}

// collect builds a Config from the widgets, reporting the first
// unparseable numeric field.
func (f *settingsForm) collect() (Config, error) {
	cfg := DefaultConfig()
	cfg.Channel = strings.TrimSpace(f.channel.Text)
	cfg.DailyRunTime = strings.TrimSpace(f.runTime.Text)
	cfg.IntroFile = strings.TrimSpace(f.intro.Text)
	cfg.OutputDir = strings.TrimSpace(f.outputDir.Text)
	cfg.WorkDir = strings.TrimSpace(f.workDir.Text)
	cfg.YtdlpPath = strings.TrimSpace(f.ytdlp.Text)
	cfg.FfmpegPath = strings.TrimSpace(f.ffmpeg.Text)
	cfg.FfprobePath = strings.TrimSpace(f.ffprobe.Text)
	cfg.DevMode = f.devMode.Checked
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
	u.form = newSettingsForm()
	u.form.fill(u.store.Get())
	form := widget.NewForm(
		widget.NewFormItem("Channel", u.form.channel),
		widget.NewFormItem("Daily run time", u.form.runTime),
		widget.NewFormItem("Scan window (minutes)", u.form.scanWindow),
		widget.NewFormItem("Scene threshold (0-1)", u.form.threshold),
		widget.NewFormItem("Cut backoff (seconds)", u.form.backoff),
		widget.NewFormItem("Intro file", u.withFilePicker(u.form.intro)),
		widget.NewFormItem("Output folder", u.withDirPicker(u.form.outputDir)),
		widget.NewFormItem("Work folder", u.withDirPicker(u.form.workDir)),
		widget.NewFormItem("yt-dlp path", u.form.ytdlp),
		widget.NewFormItem("ffmpeg path", u.form.ffmpeg),
		widget.NewFormItem("ffprobe path", u.form.ffprobe),
		widget.NewFormItem("Dev mode", u.form.devMode),
		widget.NewFormItem("Keep finished (days)", u.form.keepDays),
	)
	buttons := container.NewHBox(
		widget.NewButton("Save settings", u.onSaveSettings),
		widget.NewButton("Prepare intro", u.onPrepareIntro),
		widget.NewButton("Mark existing VODs as downloaded", u.onSeedArchive),
		widget.NewButton("Reset settings", u.onResetSettings),
		widget.NewButton("Reset archive", u.onResetArchive),
	)
	return container.NewBorder(nil, buttons, nil, nil, container.NewVScroll(form))
}

func (u *UI) withFilePicker(entry *widget.Entry) fyne.CanvasObject {
	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, openErr error) {
			if openErr != nil || reader == nil {
				return
			}
			entry.SetText(reader.URI().Path())
			// The file is not read here; only its path matters.
			_ = reader.Close()
		}, u.window)
	})
	return container.NewBorder(nil, nil, nil, browse, entry)
}

func (u *UI) withDirPicker(entry *widget.Entry) fyne.CanvasObject {
	browse := widget.NewButtonWithIcon("", theme.FolderOpenIcon(), func() {
		dialog.ShowFolderOpen(func(list fyne.ListableURI, openErr error) {
			if openErr != nil || list == nil {
				return
			}
			entry.SetText(list.Path())
		}, u.window)
	})
	return container.NewBorder(nil, nil, nil, browse, entry)
}

func (u *UI) onSaveSettings() {
	cfg, collectErr := u.form.collect()
	if collectErr != nil {
		dialog.ShowError(collectErr, u.window)
		return
	}
	if saveErr := cfg.Save(u.cfgPath); saveErr != nil {
		dialog.ShowError(saveErr, u.window)
		return
	}
	u.store.Set(cfg)
	problems := cfg.Validate()
	if len(problems) > 0 {
		dialog.ShowInformation("Saved with warnings",
			"Saved, but the pipeline cannot run yet:\n- "+strings.Join(problems, "\n- "), u.window)
		return
	}
	dialog.ShowInformation("Saved", "Settings saved.", u.window)
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
	}, u.window).Show()
}

// onResetArchive deletes the download archive after a confirmation,
// so every VOD still on the channel counts as new again.
func (u *UI) onResetArchive() {
	message := "Delete the download archive? Every VOD still on the channel " +
		"will be treated as new and downloaded on the next run."
	dialog.NewConfirm("Reset archive", message, func(confirmed bool) {
		if !confirmed {
			return
		}
		if rmErr := os.Remove(archivePath()); rmErr != nil && !os.IsNotExist(rmErr) {
			dialog.ShowError(rmErr, u.window)
			return
		}
		dialog.ShowInformation("Archive reset", "The download archive was deleted.", u.window)
	}, u.window).Show()
}

// onSeedArchive marks every VOD already on the channel as downloaded
// so a fresh install never pulls down the whole back catalog.
func (u *UI) onSeedArchive() {
	cfg := u.store.Get()
	message := "Mark every VOD currently on the channel as already downloaded? " +
		"Only VODs published after this point will be processed."
	dialog.NewConfirm("Skip old VODs", message, func(confirmed bool) {
		if !confirmed {
			return
		}
		go func() {
			seedErr := SeedArchive(context.Background(), cfg, u.logger.Logf)
			fyne.Do(func() {
				if seedErr != nil {
					dialog.ShowError(seedErr, u.window)
					return
				}
				dialog.ShowInformation("Done",
					"Existing VODs were marked as downloaded and will be skipped.", u.window)
			})
		}()
	}, u.window).Show()
}
