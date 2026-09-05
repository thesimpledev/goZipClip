package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// zipTheme is the default Fyne theme with a few colors raised to
// WCAG AA contrast. Fyne's stock input border is nearly invisible
// (1.4:1 in the dark variant), which is what draws an unchecked
// checkbox and every entry outline; disabled and placeholder text
// fail too. Everything else is delegated unchanged.
type zipTheme struct {
	base fyne.Theme
}

func newZipTheme() fyne.Theme {
	return zipTheme{base: theme.DefaultTheme()}
}

// Overridden colors per variant. Each pair is checked by the test
// in ui_theme_test.go against the variant's backgrounds.
var (
	darkInputBorder  = color.NRGBA{R: 0x8a, G: 0x8a, B: 0x8a, A: 0xff}
	darkDisabled     = color.NRGBA{R: 0x9a, G: 0x9a, B: 0x9a, A: 0xff}
	lightInputBorder = color.NRGBA{R: 0x76, G: 0x76, B: 0x76, A: 0xff}
	lightDisabled    = color.NRGBA{R: 0x6b, G: 0x6b, B: 0x6b, A: 0xff}
	lightPlaceHolder = color.NRGBA{R: 0x6a, G: 0x6a, B: 0x6a, A: 0xff}
)

// Color returns the raised-contrast value for the few names that
// need it and the stock value for everything else.
func (t zipTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	dark := variant == theme.VariantDark
	switch name {
	case theme.ColorNameInputBorder:
		if dark {
			return darkInputBorder
		}
		return lightInputBorder
	case theme.ColorNameDisabled:
		if dark {
			return darkDisabled
		}
		return lightDisabled
	case theme.ColorNamePlaceHolder:
		if !dark {
			return lightPlaceHolder
		}
	}
	return t.base.Color(name, variant)
}

// Font delegates to the default theme.
func (t zipTheme) Font(style fyne.TextStyle) fyne.Resource {
	return t.base.Font(style)
}

// Icon delegates to the default theme.
func (t zipTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return t.base.Icon(name)
}

// Size delegates to the default theme.
func (t zipTheme) Size(name fyne.ThemeSizeName) float32 {
	return t.base.Size(name)
}
