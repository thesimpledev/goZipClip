package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

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
