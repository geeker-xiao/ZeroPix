// Tests for the drawing tools that came with the expanded toolbar: the
// freehand strokes, the blur and the spotlight, the measuring tools, the
// eraser, the eyedropper and the toolbar the settings dialog shapes.

package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

// fillShot paints the whole snapshot one colour.
func fillShot(s *snipSession, c color.RGBA) {
	img := s.shot
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			i := img.PixOffset(x, y)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, 255
		}
	}
}

// at reads a rendered pixel.
func at(img *image.RGBA, x, y int) color.RGBA {
	i := img.PixOffset(x, y)
	return color.RGBA{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: 255}
}

// The brush keeps every point of the drag and draws them into the export.
func TestBrushKeepsItsPath(t *testing.T) {
	s := newTestSession()
	fillShot(s, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	s.sel = ui.Rect{X: 0, Y: 0, W: 300, H: 300}
	s.hasSel = true
	s.setTool(toolBrush)
	s.color = annotationColors[0]

	s.beginShape(40, 40)
	for i := 1; i <= 20; i++ {
		s.drawing.b = ui.Point{X: 40 + float32(i)*10, Y: 40 + float32(i)*5}
		s.drawing.pts = append(s.drawing.pts, s.drawing.b)
	}
	s.finishShape()

	if len(s.shapes) != 1 {
		t.Fatalf("shapes = %d", len(s.shapes))
	}
	if n := len(s.shapes[0].pts); n < 20 {
		t.Fatalf("the stroke kept %d points", n)
	}
	out := s.render()
	if out == nil {
		t.Fatal("nothing rendered")
	}
	// A point on the stroke is red, a point well away stays white.
	if c := at(out, 120, 80); c.R < 200 || c.G > 120 {
		t.Errorf("the stroke at 120,80 is %v", c)
	}
	if c := at(out, 280, 20); c.R < 250 || c.G < 250 {
		t.Errorf("the canvas at 280,20 is %v, want white", c)
	}
}

// The highlighter writes wide and see-through, so the text under it stays
// readable.
func TestHighlighterIsTranslucent(t *testing.T) {
	s := newTestSession()
	fillShot(s, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	s.sel = ui.Rect{X: 0, Y: 0, W: 200, H: 100}
	s.hasSel = true
	s.color = ui.RGB(0xE5, 0x39, 0x35)

	out := s.render()
	sh := &shape{kind: toolHighlighter, a: ui.Point{X: 20, Y: 50}, b: ui.Point{X: 180, Y: 50},
		pts: []ui.Point{{X: 20, Y: 50}, {X: 180, Y: 50}}, color: s.color, width: 2.5}
	rasterShapes(out, []*shape{sh}, 0, 0, 1)

	c := at(out, 100, 50)
	if c.R < 200 {
		t.Fatalf("the highlight is %v, want a red that keeps the white behind it", c)
	}
	if c.G > 220 || c.B > 220 {
		t.Fatalf("the highlight is %v, want it tinted", c)
	}
	// Wide enough to cover a line of text.
	if c := at(out, 100, 56); c.G > 250 {
		t.Fatalf("the highlight is only a hairline: %v", c)
	}
}

// Blur softens the region: a hard edge becomes a gradient.
func TestBlurSoftensTheRegion(t *testing.T) {
	s := newTestSession()
	fillShot(s, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	// A hard edge down the middle of the region.
	for y := 0; y < 200; y++ {
		for x := 0; x < 100; x++ {
			i := s.shot.PixOffset(x, y)
			s.shot.Pix[i], s.shot.Pix[i+1], s.shot.Pix[i+2] = 0, 0, 0
		}
	}
	s.sel = ui.Rect{X: 0, Y: 0, W: 200, H: 200}
	s.hasSel = true

	out := s.render()
	if c := at(out, 99, 100); c.R != 0 {
		t.Fatalf("before the blur the edge is %v", c)
	}
	rasterShapes(out, []*shape{{
		kind: toolBlur, a: ui.Point{X: 60, Y: 60}, b: ui.Point{X: 140, Y: 140},
	}}, 0, 0, 1)
	mid := at(out, 100, 100)
	if mid.R < 40 || mid.R > 215 {
		t.Fatalf("the blurred edge is %v, want a middling gray", mid)
	}
	// The edge is no longer a step: two pixels either side differ less.
	left, right := at(out, 97, 100), at(out, 103, 100)
	if d := absDiffByte(left.R, mid.R); d > 90 {
		t.Fatalf("the blur left a step of %d", d)
	}
	if d := absDiffByte(right.R, mid.R); d > 90 {
		t.Fatalf("the blur left a step of %d", d)
	}
}

// Spotlight darkens everything but its region.
func TestSpotlightDarkensOutside(t *testing.T) {
	s := newTestSession()
	fillShot(s, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	s.sel = ui.Rect{X: 0, Y: 0, W: 200, H: 200}
	s.hasSel = true

	out := s.render()
	rasterShapes(out, []*shape{{
		kind: toolSpotlight, a: ui.Point{X: 50, Y: 50}, b: ui.Point{X: 150, Y: 150},
	}}, 0, 0, 1)

	if c := at(out, 100, 100); c.R < 250 {
		t.Fatalf("the lit region is %v, want it untouched", c)
	}
	if c := at(out, 10, 10); c.R > 140 {
		t.Fatalf("the area outside is %v, want it dimmed", c)
	}
}

// The measuring tool draws a line with a tick at each end and the length in
// pixels over it.
func TestDimensionDrawsLineAndLabel(t *testing.T) {
	s := newTestSession()
	fillShot(s, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	s.sel = ui.Rect{X: 0, Y: 0, W: 300, H: 200}
	s.hasSel = true

	out := s.render()
	rasterShapes(out, []*shape{{
		kind: toolDimension, a: ui.Point{X: 40, Y: 100}, b: ui.Point{X: 240, Y: 100},
		color: annotationColors[0], width: 2,
	}}, 0, 0, 1)

	if c := at(out, 100, 100); c.R < 180 || c.G > 120 {
		t.Fatalf("the measuring line is %v", c)
	}
	// The tick stands across the line at the far end.
	if c := at(out, 240, 94); c.R < 180 {
		t.Fatalf("no tick at the end: %v", c)
	}
	// The label sits above the middle.
	dark := false
	for x := 115; x <= 165; x++ {
		if at(out, x, 87).R < 60 {
			dark = true
			break
		}
	}
	if !dark {
		t.Fatal("no label chip over the line")
	}
}

// The ruler draws ticks over the drag.
func TestRulerDrawsTicks(t *testing.T) {
	s := newTestSession()
	fillShot(s, color.RGBA{R: 30, G: 30, B: 30, A: 255})
	s.sel = ui.Rect{X: 0, Y: 0, W: 300, H: 120}
	s.hasSel = true

	out := s.render()
	rasterShapes(out, []*shape{{
		kind: toolRuler, a: ui.Point{X: 20, Y: 40}, b: ui.Point{X: 220, Y: 60},
	}}, 0, 0, 1)

	if c := at(out, 125, 50); c.R < 200 {
		t.Fatalf("the ruler bar is %v, want it light", c)
	}
	// A tick reaches into the bar from its top edge.
	if c := at(out, 20, 42); c.R > 120 {
		t.Fatalf("no tick at the start: %v", c)
	}
}

// The circle tool inscribes a square, so a drag of any shape gives a circle.
func TestCircleIsRound(t *testing.T) {
	sh := &shape{kind: toolCircle, a: ui.Point{X: 10, Y: 10}, b: ui.Point{X: 110, Y: 60}}
	r := sh.circleRect()
	if r.W != r.H {
		t.Fatalf("circle box = %v", r)
	}
	if r.W != 50 {
		t.Fatalf("circle takes %v, want the shorter side", r.W)
	}
	if r.X != 35 {
		t.Fatalf("circle centred at %v, want 35", r.X)
	}
}

// The eraser takes away the shapes it touches, and undo puts them back.
func TestEraserRemovesAndUndoRestores(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 0, Y: 0, W: 400, H: 400}
	s.hasSel = true

	s.push(&shape{kind: toolRect, a: ui.Point{X: 50, Y: 50}, b: ui.Point{X: 150, Y: 150},
		color: annotationColors[0], width: 2})
	s.push(&shape{kind: toolRect, a: ui.Point{X: 250, Y: 250}, b: ui.Point{X: 350, Y: 350},
		color: annotationColors[1], width: 2})

	s.eraseAt(100, 100)
	if len(s.shapes) != 1 {
		t.Fatalf("after erasing there are %d shapes", len(s.shapes))
	}
	s.undo()
	if len(s.shapes) != 2 {
		t.Fatalf("undo left %d shapes", len(s.shapes))
	}
	s.redo()
	if len(s.shapes) != 1 {
		t.Fatalf("redo left %d shapes", len(s.shapes))
	}
}

// The eyedropper takes the colour under the pointer and keeps it as a
// swatch of its own.
func TestEyedropperPicksTheColor(t *testing.T) {
	s := newTestSession()
	fillShot(s, color.RGBA{R: 10, G: 20, B: 30, A: 255})

	s.pickColorAt(50, 60)

	if s.color != ui.RGB(10, 20, 30) {
		t.Fatalf("picked %v", s.color)
	}
	if !s.hasCustom || s.customColor != s.color {
		t.Fatal("the picked colour did not become a swatch")
	}
	if s.pickedNote != "#0A141E" {
		t.Fatalf("the note says %q", s.pickedNote)
	}
	if lastColor != s.color {
		t.Fatal("the picked colour is not the one a pin would use")
	}
}

// The toolbar shows what the settings store says, and the presets switch it.
func TestToolbarFollowsTheSettings(t *testing.T) {
	old := settings.Toolbar
	t.Cleanup(func() { settings.Toolbar = old })

	s := newTestSession()
	s.sel = ui.Rect{X: 100, Y: 100, W: 700, H: 400}
	s.hasSel, s.phase = true, phaseEdit

	settings.Toolbar = "rect,text,close"
	tt := ui.NewTester(s.view, 2560, 1440)
	tt.Frame()
	narrow := s.barRect.W
	if !tt.HasText("矩形描边 (1)") || !tt.HasText("取消截图 (Esc)") {
		t.Fatalf("the chosen buttons are missing: %q", tt.Texts())
	}
	if tt.HasText("箭头 (5)") || tt.HasText("马赛克 (0)") {
		t.Fatalf("a button the settings left out is on the bar: %q", tt.Texts())
	}

	applyBarPreset("火力全开")
	tt.Frame()
	if s.barRect.W <= narrow {
		t.Fatalf("the full bar (%v) is no wider than the minimal one (%v)", s.barRect.W, narrow)
	}
	for _, label := range []string{"矩形描边 (1)", "圆形 (3)", "标尺", "取色（从截图取颜色）", "取消截图 (Esc)"} {
		if !tt.HasText(label) {
			t.Errorf("the full bar misses %q", label)
		}
	}

	applyBarPreset("个性极简")
	tt.Frame()
	if !tt.HasText("矩形描边 (1)") || tt.HasText("圆形 (3)") {
		t.Fatalf("the minimal preset did not take: %q", tt.Texts())
	}
}

// With many buttons on a narrow screen the bar wraps instead of running off
// the edge.
func TestToolbarWrapsIntoRows(t *testing.T) {
	old := settings.Toolbar
	t.Cleanup(func() { settings.Toolbar = old })
	applyBarPreset("火力全开")

	s := newTestSession()
	s.frame = ui.Rect{W: 640, H: 480}
	s.sel = ui.Rect{X: 20, Y: 20, W: 400, H: 200}
	s.hasSel, s.phase = true, phaseEdit

	tt := ui.NewTester(s.view, 640, 480)
	tt.Frame()
	bar := s.barRect
	if bar.H <= tbRowH {
		t.Fatalf("the bar is %v high, want more than one row", bar.H)
	}
	if bar.X+bar.W > s.frame.W || bar.Y+bar.H > s.frame.H {
		t.Fatalf("the bar runs off the screen: %v", bar)
	}
}

// The new icons parse, so the buttons draw something.
func TestNewToolIconsExist(t *testing.T) {
	for _, name := range []string{
		"circle", "line", "brush", "highlighter", "blur", "spotlight",
		"dimension", "ruler", "eraser", "mask", "settings",
	} {
		if iconSet[name] == nil {
			t.Errorf("no %q icon", name)
		}
	}
}
