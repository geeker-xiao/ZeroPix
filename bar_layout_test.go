package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// useTestToolbar pins the preset the bar carries, so that the rows these
// tests count do not depend on the settings of the machine.
func useTestToolbar(t *testing.T) {
	t.Helper()
	old := settings.Toolbar
	settings.Toolbar = "rect,text,number,mosaic,pin,copy,settings,close"
	t.Cleanup(func() { settings.Toolbar = old })
}

// While a drawing tool is in use the swatches take a line of their own,
// under the tools they colour; with no such tool they sit in the first row.
func TestColourRowGoesUnderTheTools(t *testing.T) {
	useTestToolbar(t)

	s := &snipSession{active: toolSelect}
	if rows := s.layoutBar(1200); len(rows) != 1 {
		t.Fatalf("with no tool the bar has %d rows, want 1", len(rows))
	}

	for _, tl := range []tool{
		toolRect, toolEllipse, toolCircle, toolLine, toolArrow,
		toolBrush, toolHighlighter, toolText, toolNumber,
	} {
		s.active = tl
		rows := s.layoutBar(1200)
		if len(rows) != 2 {
			t.Fatalf("with tool %d the bar has %d rows, want 2", tl, len(rows))
		}
		if !swatchRow(rows[len(rows)-1]) {
			t.Fatalf("with tool %d the colour line is not the last row", tl)
		}
		if !rows[0].groups[0][0].isTool() {
			t.Fatalf("the first row starts with %q, want a tool", rows[0].groups[0][0].id)
		}
	}
}

// The line carries the four presets and, after them, the mixed colour that
// opens the picker.
func TestSwatchGroupKeepsTheCustomDot(t *testing.T) {
	s := &snipSession{}
	if got, want := len(s.swatchGroup()), len(annotationColors)+1; got != want {
		t.Fatalf("the colour line has %d dots, want %d", got, want)
	}
}

// Picking on the square and on the hue bar moves the colour and keeps the
// mixed swatch in step with it.
func TestPanelPicksAColour(t *testing.T) {
	s := &snipSession{panelSq: ui.Rect{X: 0, Y: 0, W: 100, H: 100}, panelHueRect: ui.Rect{X: 0, Y: 0, W: 360, H: 12}}

	s.panelH, s.panelS, s.panelV = 0, 1, 1
	s.panelPick(panelDragHue, 120, 6) // a third of the way round: green
	if s.panelH < 115 || s.panelH > 125 {
		t.Fatalf("the hue is %.1f, want about 120", s.panelH)
	}

	s.panelPick(panelDragSquare, 0, 0) // top left: white
	if !s.hasCustom || s.customColor.R != 255 || s.customColor.G != 255 || s.customColor.B != 255 {
		t.Fatalf("the pick made %v, want white", s.customColor)
	}
	if s.color != s.customColor {
		t.Fatal("the snip did not take the picked colour")
	}
}
