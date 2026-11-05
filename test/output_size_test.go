package main

import (
	"image"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Choosing an aspect reshapes the region around its middle and updates the
// numbers the bar shows.
func TestAspectReshapesTheRegion(t *testing.T) {
	s := &snipSession{scale: 1, frame: ui.Rect{W: 1920, H: 1080}}
	s.sel = ui.Rect{X: 400, Y: 300, W: 800, H: 600}
	s.hasSel = true

	cx, cy := s.sel.X+s.sel.W/2, s.sel.Y+s.sel.H/2
	s.applyAspect(16, 9)

	if got, want := float64(s.sel.W)/float64(s.sel.H), 16.0/9.0; math.Abs(got-want) > 0.01 {
		t.Fatalf("the region is %v, want 16:9", got)
	}
	if got := s.sel.X + s.sel.W/2; math.Abs(float64(got-cx)) > 1 {
		t.Fatalf("the middle moved from %v to %v", cx, got)
	}
	if got := s.sel.Y + s.sel.H/2; math.Abs(float64(got-cy)) > 1 {
		t.Fatalf("the middle moved from %v to %v", cy, got)
	}
	if s.outW != math.Round(float64(s.sel.W)) || s.outH != math.Round(float64(s.sel.H)) {
		t.Fatalf("the bar shows %v × %v for a %v × %v region", s.outW, s.outH, s.sel.W, s.sel.H)
	}

	// The same aspect, the other way round.
	s.applyAspect(1, 16)
	if got := float64(s.sel.W) / float64(s.sel.H); math.Abs(got-1.0/16.0) > 0.001 {
		t.Fatalf("1:16 gave %v", got)
	}
	if s.sel.H > s.frame.H || s.sel.W > s.frame.W {
		t.Fatalf("the region %v leaves the screen %v", s.sel, s.frame)
	}
}

// A typed width sets the height from the held aspect, and the other way
// round.
func TestTypedSizeKeepsTheAspect(t *testing.T) {
	s := &snipSession{scale: 1, frame: ui.Rect{W: 1920, H: 1080}}
	s.sel = ui.Rect{X: 100, Y: 100, W: 800, H: 600}
	s.hasSel = true

	s.applyAspect(1, 1)
	s.outW = 400
	s.applyWidth()
	if s.outH != 400 {
		t.Fatalf("a 400 wide square came out %v high", s.outH)
	}

	s.outH = 300
	s.applyHeight()
	if s.outW != 300 {
		t.Fatalf("a 300 high square came out %v wide", s.outW)
	}

	// Without an aspect the other side stays as it was.
	s.applyAspect(0, 0)
	s.outW = 500
	s.applyWidth()
	if s.outH != 300 {
		t.Fatalf("free resizing changed the height to %v", s.outH)
	}

	// The region never leaves the screen.
	s.outW = 9000
	s.applyWidth()
	if s.sel.X+s.sel.W > s.frame.W+0.5 || s.sel.W > s.frame.W {
		t.Fatalf("the region %v is off screen", s.sel)
	}
}

// Dragging a corner of a held aspect keeps the shape.
func TestCornerDragKeepsTheAspect(t *testing.T) {
	orig := ui.Rect{X: 100, Y: 100, W: 400, H: 400}
	got := resizeRectKeepsAspect(orig, handleSE, 900, 500, 16.0/9.0)
	if ratio := float64(got.W) / float64(got.H); math.Abs(ratio-16.0/9.0) > 0.01 {
		t.Fatalf("the dragged region is %v, want 16:9", ratio)
	}
	// The opposite corner is the anchor.
	if got.X != orig.X || got.Y != orig.Y {
		t.Fatalf("the corner opposite the drag moved: %v", got)
	}
}

// The export clears what is outside the rounded rectangle and leaves the
// middle alone.
func TestRoundCornersClearsTheCorners(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	roundCorners(img, 24)

	if a := img.RGBAAt(0, 0).A; a != 0 {
		t.Fatalf("the corner keeps alpha %d", a)
	}
	if a := img.RGBAAt(99, 99).A; a != 0 {
		t.Fatalf("the far corner keeps alpha %d", a)
	}
	if a := img.RGBAAt(50, 50).A; a != 255 {
		t.Fatalf("the middle lost alpha, now %d", a)
	}
	if a := img.RGBAAt(50, 1).A; a != 255 {
		t.Fatalf("the straight edge lost alpha, now %d", a)
	}

	// A radius of zero leaves the image square.
	square := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for i := range square.Pix {
		square.Pix[i] = 255
	}
	roundCorners(square, 0)
	if a := square.RGBAAt(0, 0).A; a != 255 {
		t.Fatalf("a square export lost alpha %d", a)
	}
}

// The radius shown in the bar is in output pixels, so the preview has to
// divide by the display scale.
func TestCornerRadiusFollowsTheScale(t *testing.T) {
	s := &snipSession{scale: 2, radiusPx: 24}
	s.sel = ui.Rect{W: 400, H: 300}
	if got := s.cornerRadiusDip(); got != 12 {
		t.Fatalf("24px at 2x is %v DIP, want 12", got)
	}
	// More than half the short side would eat the image.
	s.radiusPx = 400
	if got := s.cornerRadiusDip(); got != 150 {
		t.Fatalf("a huge radius is %v DIP, want half of 300", got)
	}
}

// The corner field takes a typed radius, which the export then uses.
func TestTypedCornerRadiusReachesTheExport(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 100, Y: 100, W: 400, H: 300}
	s.hasSel, s.phase = true, phaseEdit
	tt := ui.NewTester(s.view, 1280, 800)
	tt.Frame()

	if err := tt.Click("圆角半径"); err != nil {
		t.Fatalf("the bar has no corner field: %v", err)
	}
	tt.Type("40")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if s.radiusPx != 40 {
		t.Fatalf("the corner field holds %v, want 40", s.radiusPx)
	}

	img := s.render()
	if img == nil {
		t.Fatal("nothing to export")
	}
	if a := img.RGBAAt(0, 0).A; a != 0 {
		t.Fatalf("the export keeps the corner, alpha %d", a)
	}
}

// The aspect buttons on the bar reshape the region, and the size fields
// follow them.
func TestAspectButtonReshapesTheRegion(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 100, Y: 100, W: 500, H: 500}
	s.hasSel, s.phase = true, phaseEdit
	tt := ui.NewTester(s.view, 1280, 800)
	tt.Frame()

	if err := tt.Click("16:9"); err != nil {
		t.Fatalf("the bar has no 16:9 button: %v", err)
	}
	tt.Frame()
	if ratio := float64(s.sel.W) / float64(s.sel.H); math.Abs(ratio-16.0/9.0) > 0.02 {
		t.Fatalf("the region is %v after the button, want 16:9", ratio)
	}
	if s.outW != math.Round(float64(s.sel.W)) || s.outH != math.Round(float64(s.sel.H)) {
		t.Fatalf("the fields show %v × %v for a %v × %v region", s.outW, s.outH, s.sel.W, s.sel.H)
	}

	// Free again leaves the region where it is.
	w, h := s.sel.W, s.sel.H
	if err := tt.Click("自由"); err != nil {
		t.Fatalf("the bar has no free button: %v", err)
	}
	tt.Frame()
	if s.sel.W != w || s.sel.H != h {
		t.Fatalf("free resized the region from %v × %v to %v × %v", w, h, s.sel.W, s.sel.H)
	}
}

// The corner field starts empty, its placeholder reading 0: typing a radius
// then lands on a blank field instead of behind a zero.
func TestCornerFieldStartsEmpty(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 100, Y: 100, W: 400, H: 300}
	s.hasSel, s.phase = true, phaseEdit
	tt := ui.NewTester(s.view, 1280, 800)
	tt.Frame()
	if s.radText != "" {
		t.Fatalf("the corner field starts holding %q, want it empty", s.radText)
	}
	// An empty field commits to a square region.
	s.radText = ""
	s.takeRadius()
	if s.radiusPx != 0 {
		t.Fatalf("an empty corner field left %v", s.radiusPx)
	}
}

// One Enter commits a typed size, and typing over a field that already holds
// a number replaces it instead of appending to it.
func TestOneEnterTakesTheTypedWidth(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 100, Y: 100, W: 400, H: 300}
	s.hasSel, s.phase = true, phaseEdit
	tt := ui.NewTester(s.view, 1280, 800)
	tt.Frame()

	if err := tt.Click("输出宽度"); err != nil {
		t.Fatalf("the bar has no width field: %v", err)
	}
	tt.Type("900")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if s.outW != 900 {
		t.Fatalf("one Enter left the width at %v, want 900", s.outW)
	}

	// The field holds 900 now; a second edit replaces it.
	if err := tt.Click("输出宽度"); err != nil {
		t.Fatalf("the width field went away: %v", err)
	}
	tt.Frame()
	tt.Type("300")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if s.outW != 300 {
		t.Fatalf("typing over 900 gave %v, want 300", s.outW)
	}
}
