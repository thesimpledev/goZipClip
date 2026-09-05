package main

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

// relativeLuminance follows the WCAG 2 definition for sRGB colors.
func relativeLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	channel := func(v uint32) float64 {
		s := float64(v) / 0xffff
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*channel(r) + 0.7152*channel(g) + 0.0722*channel(b)
}

// contrastRatio is the WCAG 2 contrast between two colors, always
// at least 1.
func contrastRatio(a, b color.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TestThemeContrast pins the overridden colors to WCAG AA: 3:1 for
// control outlines, 4.5:1 for text, against the backgrounds each is
// drawn on in both variants.
func TestThemeContrast(t *testing.T) {
	// The stock theme reads the primary color from the current app.
	test.NewApp()
	th := newZipTheme()
	for _, variant := range []fyne.ThemeVariant{theme.VariantDark, theme.VariantLight} {
		window := th.Color(theme.ColorNameBackground, variant)
		input := th.Color(theme.ColorNameInputBackground, variant)
		checks := []struct {
			name    fyne.ThemeColorName
			against color.Color
			min     float64
		}{
			{theme.ColorNameInputBorder, input, 3},
			{theme.ColorNameInputBorder, window, 3},
			{theme.ColorNameDisabled, window, 4.5},
			{theme.ColorNamePlaceHolder, input, 4.5},
			{theme.ColorNameForeground, window, 4.5},
		}
		for _, check := range checks {
			ratio := contrastRatio(th.Color(check.name, variant), check.against)
			if ratio < check.min {
				t.Errorf("variant %d: %s has contrast %.2f, want at least %.1f", variant, check.name, ratio, check.min)
			}
		}
	}
}

// TestThemeDelegates makes sure the wrapper does not change anything
// it was not meant to.
func TestThemeDelegates(t *testing.T) {
	test.NewApp()
	th := newZipTheme()
	base := theme.DefaultTheme()
	if th.Color(theme.ColorNamePrimary, theme.VariantDark) != base.Color(theme.ColorNamePrimary, theme.VariantDark) {
		t.Fatal("primary color should be the stock value")
	}
	if th.Size(theme.SizeNameText) != base.Size(theme.SizeNameText) {
		t.Fatal("text size should be the stock value")
	}
}
