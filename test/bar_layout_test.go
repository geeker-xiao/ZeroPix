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

// The swatches keep the last line of the bar whichever tool is in use, so
// that picking a tool never changes the shape of the bar under the hand.
func TestColourRowIsAlwaysTheLastOne(t *testing.T) {
	useTestToolbar(t)

	s := &snipSession{}
	for _, tl := range []tool{
		toolSelect, toolRect, toolEllipse, toolCircle, toolArrow, toolLine,
		toolBrush, toolHighlighter, toolMosaic, toolBlur, toolSpotlight,
		toolText, toolNumber,
	} {
		s.active = tl
		rows := s.layoutBar(1200)
		if len(rows) < 2 {
			t.Fatalf("with tool %d the bar has %d rows, want at least 2", tl, len(rows))
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
// opens the picker, plus one dot per pen width.
func TestSwatchGroupKeepsTheCustomDot(t *testing.T) {
	s := &snipSession{}
	if got, want := len(s.swatchGroup()), len(annotationColors)+1; got != want {
		t.Fatalf("the colour line has %d dots, want %d", got, want)
	}
	if len(annotationWidths) != len(annotationWidthNames) {
		t.Fatalf("%d widths but %d names", len(annotationWidths), len(annotationWidthNames))
	}
	if len(annotationWidths) < 3 {
		t.Fatalf("only %d widths to choose from", len(annotationWidths))
	}
	for i := 1; i < len(annotationWidths); i++ {
		if annotationWidths[i] <= annotationWidths[i-1] {
			t.Fatalf("the widths are not in order: %v", annotationWidths)
		}
	}
}

// Choosing a width puts it on the snip and keeps it for the next one, the
// way the colour is kept.
func TestPickingAWidthKeepsItForTheNextSnip(t *testing.T) {
	s := &snipSession{width: annotationWidths[1]}
	s.pickWidth(annotationWidths[3])
	if s.width != annotationWidths[3] {
		t.Fatalf("the snip draws at %v, want %v", s.width, annotationWidths[3])
	}
	if lastWidth != annotationWidths[3] {
		t.Fatalf("the next snip starts at %v, want %v", lastWidth, annotationWidths[3])
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
