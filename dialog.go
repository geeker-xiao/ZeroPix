// The windows the app opens beside the tray have one look: no frame of
// Windows, an ink title bar carrying the brand, a two-pixel accent rule,
// and a page of cards. The settings dialog set it; this file holds the parts
// they share so that the others keep it.

package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	// dialogRadius is how round the corners of such a window are.
	dialogRadius = 14
	// dialogFill is the window's own colour, seen in the corners where the
	// page's background does not reach.
	dialogFill = appPaper
)

// dialogWindow opens a window in the app's own style. stateKey remembers
// where it was placed and how big it was, and may be empty; it carries a
// number where the default size has changed since, so that a remembered
// window does not come back too small for the new title bar.
func dialogWindow(title string, width, height, minWidth, minHeight int, stateKey string, view func(*ui.Context)) *mygo.Window {
	return mygo.NewWindow(mygo.WindowOptions{
		Title:           title,
		Width:           width,
		Height:          height,
		MinWidth:        minWidth,
		MinHeight:       minHeight,
		Frameless:       true,
		CornerRadius:    dialogRadius,
		DisableMaximize: true,
		// Not DisableMove: mygo forces SWP_NOMOVE on such windows, which
		// undoes the placement (see pin.go).
		BackgroundColor: dialogFill,
		StateKey:        stateKey,
		Content:         ui.View(view),
	})
}

// dialogHeader is the band that stands in for the Windows title bar. It is
// ink in both appearances: the mark, a title over a muted line, a close
// button, and under them the accent rule that signs every ZeroPix window.
// The band itself moves the window; "app" draws the program's icon.
func dialogHeader(c *ui.Context, t *ui.Theme, icon, title, subtitle, closeLabel string, onClose func()) {
	ui.Column(c).FillWidth().Shrink(0).Children(func() {
		ui.Row(c).FillWidth().Padding(15, 14).Gap(12).AlignItems(ui.Center).
			Background(inkColor()).DragWindow().Children(func() {
			if icon == "app" {
				if bmp := appIconBitmap(); bmp != nil {
					ui.Image(c, bmp).Width(34).Height(34)
				}
			} else {
				ui.Box(c).Width(34).Height(34).Radius(9).
					Background(ui.RGBA(0, 0xC9, 0xA0, 0.14)).Children(func() {
					ui.Box(c).Fill().Padding(7).Draw(func(p *ui.Painter, r ui.Rect) {
						drawIcon(p, icon, r, accentColor())
					})
				})
			}
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, title).FontSize(15.5).Bold().TextColor(onInkColor())
				ui.Text(c, subtitle).FontSize(11).TextColor(onInkMutedColor())
			})
			b := ui.ButtonBase(c).Width(26).Height(26).Radius(6).Label(closeLabel).
				Tooltip("关闭 (Esc)")
			if b.Hovered() {
				b.Background(onInkHoverColor())
			}
			if b.Clicked() {
				onClose()
			}
			b.Children(func() {
				ui.Box(c).Fill().Padding(6).Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, "close", r, onInkMutedColor())
				})
			})
		})
		ui.Box(c).FillWidth().Height(2).Background(accentColor())
	})
}

// dialogBody is the page under the title bar: the soft gray the cards of the
// settings dialog sit on, inset by the same margin.
func dialogBody(c *ui.Context, t *ui.Theme, body func()) {
	ui.Column(c).FillWidth().Grow(1).Padding(16).Gap(12).Children(body)
}

// brandPrimaryButton is the one filled button of a page. The theme's own
// primary is a green with nothing to do with ZeroPix, so dialogs draw their
// own: ink, a short accent rule, and a lighter face when disabled.
func brandPrimaryButton(c *ui.Context, label string, width float32, disabled bool, onClick func()) {
	fill := inkColor()
	if disabled {
		fill = ui.RGB(0x9A, 0xA4, 0xAE)
	}
	b := ui.ButtonBase(c).Width(width).Height(32).Radius(8).Label(label).
		Background(fill).Disabled(disabled)
	if !disabled && b.Hovered() {
		b.Background(inkHoverColor())
	}
	if !disabled && b.Clicked() {
		onClick()
	}
	b.Children(func() {
		ui.Row(c).Fill().Gap(8).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Width(3).Height(14).Radius(2).Background(accentColor())
			ui.Text(c, label).FontSize(12.5).Bold().TextColor(onInkColor())
		})
	})
}
