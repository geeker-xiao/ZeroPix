// ZeroPix's one palette. The brand band is ink in either appearance so the
// windows are recognisable at a glance; only the paper underneath follows
// the theme. Every coloured surface in the dialogs comes from here, so a
// change of accent is one edit rather than a hunt through the views.

package main

import "github.com/egoist/mygo/ui"

func inkColor() ui.Color { return ui.RGB(0x11, 0x18, 0x20) }

func inkHoverColor() ui.Color { return ui.RGB(0x1C, 0x28, 0x35) }

func accentColor() ui.Color { return ui.RGB(0x00, 0xC9, 0xA0) }

// accentInkColor is the accent darkened enough to carry text on paper.
func accentInkColor() ui.Color { return ui.RGB(0x00, 0x7C, 0x66) }

func onInkColor() ui.Color { return ui.RGB(0xF7, 0xFA, 0xFC) }

func onInkMutedColor() ui.Color { return ui.RGB(0x8F, 0xA0, 0xB0) }

func onInkHoverColor() ui.Color { return ui.RGBA(255, 255, 255, 0.10) }

func onInkSoftColor() ui.Color { return ui.RGBA(255, 255, 255, 0.07) }

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
