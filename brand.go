// ZeroPix's one palette. The brand band follows the appearance: ink in the
// dark theme, a cool light grey in the light one, so a light window does not
// carry a black slab across its top. The page under it is always the colder
// paper. Every coloured surface in the dialogs comes from here, so a change
// of accent is one edit rather than a hunt through the views.

package main

import "github.com/egoist/mygo/ui"

func inkColor() ui.Color { return ui.RGB(0x14, 0x1B, 0x23) }

func inkHoverColor() ui.Color { return ui.RGB(0x1F, 0x28, 0x33) }

// cardColor is the sheet a section sits on. In the dark theme the page and
// the brand column share one field of ink, so the sheets are a shade lifted
// off it rather than the theme's neutral grey, which reads brownish beside
// the teal.
func cardColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGB(0x1B, 0x24, 0x30)
	}
	return t.Surface
}

// cardHoverColor is a sheet, a secondary button or a field under the pointer.
func cardHoverColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGB(0x22, 0x2C, 0x39)
	}
	return t.SurfaceHover
}

// fieldBorder is the hairline around an input in the dark theme; the theme's
// own border is a warm grey that fights the cool field.
func fieldBorder(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.14)
	}
	return t.Border
}

func accentColor() ui.Color { return ui.RGB(0x00, 0xC9, 0xA0) }

// accentInkColor is the accent darkened enough to carry text on paper.
func accentInkColor() ui.Color { return ui.RGB(0x00, 0x7C, 0x66) }

func onInkColor() ui.Color { return ui.RGB(0xF7, 0xFA, 0xFC) }

func onInkMutedColor() ui.Color { return ui.RGB(0x8F, 0xA0, 0xB0) }

func onInkHoverColor() ui.Color { return ui.RGBA(255, 255, 255, 0.10) }

func onInkSoftColor() ui.Color { return ui.RGBA(255, 255, 255, 0.07) }

// chromeColor is the band that carries the brand: the title bar of every
// dialog and the settings column. It is ink only in the dark appearance; in
// the light one it is a cool grey a step below the paper, which keeps the
// three levels of the window readable without a black field.
func chromeColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return inkColor()
	}
	return ui.RGB(0xE9, 0xEF, 0xF3)
}

// onChromeColor is the title and the selected label on that band.
func onChromeColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return onInkColor()
	}
	return inkColor()
}

// onChromeMuted is the subtitle, the unselected labels and the close icon.
func onChromeMuted(t *ui.Theme) ui.Color {
	if t.Dark {
		return onInkMutedColor()
	}
	return ui.RGB(0x5A, 0x68, 0x75)
}

// chromeHover is the face of a button under the pointer.
func chromeHover(t *ui.Theme) ui.Color {
	if t.Dark {
		return onInkHoverColor()
	}
	return ui.RGBA(0x14, 0x1B, 0x23, 0.06)
}

// chromeSelected is the face of the open section: a lifted card in the light
// band, a lighter patch of ink in the dark one.
func chromeSelected(t *ui.Theme) ui.Color {
	if t.Dark {
		return onInkSoftColor()
	}
	return ui.RGB(0xFF, 0xFF, 0xFF)
}

// chromeBorder draws the hairline between the band and the page, and around
// the selected row.
func chromeBorder(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.10)
	}
	return ui.RGBA(0x14, 0x1B, 0x23, 0.10)
}

// chromeAccent is the accent as an icon on the band: the pure teal reads on
// ink, while the light band needs the darker one to hold its edge.
func chromeAccent(t *ui.Theme) ui.Color {
	if t.Dark {
		return accentColor()
	}
	return accentInkColor()
}

// chromeNumber is the small index beside a section name: a teal-grey on ink,
// a plain cool grey on the light band.
func chromeNumber(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGB(0x7E, 0xAB, 0xA1)
	}
	return ui.RGB(0x6B, 0x7C, 0x8A)
}

func accentSoftColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(0, 0xC9, 0xA0, 0.16)
	}
	return ui.RGB(0xE4, 0xF8, 0xF2)
}

// mutedOnPaper keeps secondary copy legible on the cold white sheet: the
// theme's own muted grey is too faint there, while it is fine on ink.
func mutedOnPaper(t *ui.Theme) ui.Color {
	if t.Dark {
		return t.TextMuted
	}
	return ui.RGB(0x58, 0x66, 0x75)
}
