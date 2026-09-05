package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// firstRun asks the questions a fresh install needs answered, one
// modal at a time: the Twitch channel, then whether to cut the
// starting-soon screen, then whether to add an intro. Everything
// else has a working default. The questions stop once SetupDone is
// recorded; "Not now" on the channel leaves them for the next launch
// or the next run.
func (u *UI) firstRun() {
	cfg := u.store.Get()
	if cfg.Channel == "" {
		u.askChannel()
		return
	}
	if !cfg.SetupDone {
		u.askCut()
	}
}

// askChannel is the first-run question for the channel. Saving goes
// through the usual channel check and catalog confirmation, and the
// cut question follows once the channel is accepted.
func (u *UI) askChannel() {
	message := widget.NewLabel("Which Twitch channel should ZipClip watch? Enter the name as it appears in the channel's URL. " +
		"Everything else has a working default.")
	message.Wrapping = fyne.TextWrapWord
	entry := widget.NewEntry()
	entry.SetPlaceHolder("channel name")
	box := dialog.NewCustomWithoutButtons("Welcome to ZipClip", container.NewVBox(message, entry), u.window)
	save := widget.NewButton("Save and continue", func() {
		box.Hide()
		u.form.setValue(FieldChannel, entry.Text)
		u.saveSettings(u.askCut)
	})
	save.Importance = widget.HighImportance
	box.SetButtons([]fyne.CanvasObject{save, widget.NewButton("Not now", box.Hide)})
	box.Resize(fyne.NewSize(560, 0))
	box.Show()
}

// askCut is the first-run question about the cut.
func (u *UI) askCut() {
	if u.store.Get().SetupDone {
		return
	}
	message := "Should ZipClip cut the starting-soon screen off the front of each VOD?\n\n" +
		"It looks for the moment the picture changes from the waiting screen to the stream and drops everything before it. " +
		"You can change this any time in Settings."
	confirm := dialog.NewConfirm("Cut the starting-soon screen?", message, func(yes bool) {
		// SetChecked saves the setting through the form's own callback.
		u.form.cutEnabled.SetChecked(yes)
		u.askIntro()
	}, u.window)
	confirm.SetConfirmText("Yes, cut it")
	confirm.SetDismissText("No, keep the whole VOD")
	confirm.Show()
}

// askIntro is the first-run question about the intro. Yes opens the
// file picker; cancelling the picker leaves the intro on with no
// file, and the run walkthrough asks for it later.
func (u *UI) askIntro() {
	message := "Should ZipClip put an intro video of your own on the front of each VOD?\n\n" +
		"You will be asked to pick the video file next. It must have an audio track. " +
		"You can change this any time in Settings."
	confirm := dialog.NewConfirm("Add an intro video?", message, func(yes bool) {
		u.form.introEnabled.SetChecked(yes)
		if !yes {
			u.finishSetup()
			return
		}
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, openErr error) {
			if openErr == nil && reader != nil {
				u.form.intro.SetText(reader.URI().Path())
				// The file is not read here; only its path matters.
				_ = reader.Close()
			}
			u.finishSetup()
		}, u.window)
	}, u.window)
	confirm.SetConfirmText("Yes, add an intro")
	confirm.SetDismissText("No intro")
	confirm.Show()
}

// finishSetup records that the first-run questions were answered
// and saves everything the questions set.
func (u *UI) finishSetup() {
	cfg := u.store.Get()
	cfg.SetupDone = true
	u.store.Set(cfg)
	u.saveSettings(nil)
}

// ensureReady runs then once the settings allow a run. While something
// is missing it walks the user through the items one modal at a time,
// each with the choice of filling the setting in or turning the
// feature off, so a run never fails on an unset value.
func (u *UI) ensureReady(then func()) {
	problems := u.store.Get().Problems()
	if len(problems) == 0 {
		if then != nil {
			then()
		}
		return
	}
	u.showSetupStep(problems[0], then)
}

// showSetupStep shows the modal for one missing setting.
func (u *UI) showSetupStep(problem Problem, then func()) {
	message := widget.NewLabel(problem.Text)
	message.Wrapping = fyne.TextWrapWord
	entry := widget.NewEntry()
	entry.SetText(u.form.valueOf(problem.Field))
	content := container.NewVBox(message, u.setupControl(problem.Field, entry))
	box := dialog.NewCustomWithoutButtons("Before ZipClip can run", content, u.window)
	save := widget.NewButton("Save and continue", func() {
		box.Hide()
		u.form.setValue(problem.Field, entry.Text)
		u.saveSettings(func() { u.ensureReady(then) })
	})
	save.Importance = widget.HighImportance
	buttons := []fyne.CanvasObject{save}
	if problem.Feature != "" {
		buttons = append(buttons, widget.NewButton("Turn off "+problem.Feature.Label(), func() {
			box.Hide()
			u.form.disable(problem.Feature)
			u.saveSettings(func() { u.ensureReady(then) })
		}))
	}
	buttons = append(buttons, widget.NewButton("Not now", box.Hide))
	box.SetButtons(buttons)
	box.Resize(fyne.NewSize(560, 0))
	box.Show()
}

// setupControl pairs the entry with a browse button for the settings
// that are files or folders.
func (u *UI) setupControl(field Field, entry *widget.Entry) fyne.CanvasObject {
	switch field {
	case FieldIntroFile, FieldYtdlp, FieldFfmpeg, FieldFfprobe:
		return u.withFilePicker(entry, nil)
	case FieldOutputDir, FieldWorkDir:
		return u.withDirPicker(entry, nil)
	default:
		return entry
	}
}
