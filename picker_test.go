package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The window names the colour in the three formats, each with its own copy
// button, and shows the channels it is made of.
func TestColorWindowShowsEveryFormat(t *testing.T) {
	w := &colorWindow{col: color.RGBA{R: 0xE5, G: 0x39, B: 0x35, A: 255}}
	tt := ui.NewTester(w.view, colorWinW, colorWinH)
	tt.Frame()

	for _, want := range []string{
		"#E53935", "rgb(229, 57, 53)", "15022389",
		"RGB", "Dec", "Hex",
		"Red", "Green", "Blue", "229", "57", "53",
		"重新拾取", "关闭",
		"复制RGB", "复制Dec", "复制Hex",
	} {
		if !tt.HasText(want) {
			t.Errorf("the colour window is missing %q, have %q", want, tt.Texts())
		}
	}
}

// Clicking a copy button puts that format on the clipboard and says so.
func TestColorWindowCopies(t *testing.T) {
	var copied string
	old := writeClipboard
	writeClipboard = func(s string) { copied = s }
	t.Cleanup(func() { writeClipboard = old })

	w := &colorWindow{col: color.RGBA{R: 0x1B, G: 0x9E, B: 0x4B, A: 255}}
	tt := ui.NewTester(w.view, colorWinW, colorWinH)
	tt.Frame()

	if err := tt.Click("复制Hex"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if copied != "#1B9E4B" {
		t.Fatalf("copied %q, want the hex", copied)
	}
	if !tt.HasText("已复制 Hex：#1B9E4B") {
		t.Fatalf("the window did not confirm the copy: %q", tt.Texts())
	}

	if err := tt.Click("复制Dec"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if copied != "1809995" {
		t.Fatalf("copied %q, want the decimal value", copied)
	}

	if err := tt.Click("复制RGB"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if copied != "rgb(27, 158, 75)" {
		t.Fatalf("copied %q, want the rgb() form", copied)
	}
}

// The picker reads the frozen screen through its scale factor.
func TestPickSessionReadsThePixel(t *testing.T) {
	shot := image.NewRGBA(image.Rect(0, 0, 4, 2))
	shot.Set(0, 0, color.RGBA{R: 255, A: 255})
	shot.Set(1, 0, color.RGBA{B: 255, A: 255})
	shot.Set(2, 0, color.RGBA{G: 255, A: 255})
	p := &pickSession{shot: shot, frame: ui.Rect{W: 2, H: 1}, scale: 2}

	if c := p.colorAt(0.25, 0.25); c.R != 255 || c.B != 0 {
		t.Fatalf("0.25,0.25 = %v, want the red pixel", c)
	}
	if c := p.colorAt(0.75, 0.25); c.B != 255 || c.R != 0 {
		t.Fatalf("0.75,0.25 = %v, want the blue pixel", c)
	}
	if c := p.colorAt(1.25, 0.25); c.G != 255 {
		t.Fatalf("1.25,0.25 = %v, want the green pixel", c)
	}
	if c := p.colorAt(-1, -1); c != (color.RGBA{A: 255}) {
		t.Fatalf("outside the screen = %v, want black", c)
	}
}

// The magnifier stays on screen: beside the pointer, flipping to the other
// side near an edge.
func TestLoupeStaysInsideTheScreen(t *testing.T) {
	frame := ui.Rect{W: 800, H: 600}
	p := &pickSession{frame: frame}

	p.cursor = ui.Point{X: 400, Y: 300}
	mid := p.loupeRect(frame)
	if mid.X < p.cursor.X || mid.Y < p.cursor.Y {
		t.Fatalf("the loupe sits on the pointer: %v", mid)
	}

	p.cursor = ui.Point{X: 795, Y: 595}
	corner := p.loupeRect(frame)
	if corner.X+corner.W > frame.W || corner.Y+corner.H > frame.H {
		t.Fatalf("the loupe leaves the screen: %v", corner)
	}
	if corner.X >= p.cursor.X || corner.Y >= p.cursor.Y {
		t.Fatalf("the loupe covers the pointer at the corner: %v", corner)
	}

	p.cursor = ui.Point{X: 2, Y: 2}
	near := p.loupeRect(frame)
	if near.X < 0 || near.Y < 0 {
		t.Fatalf("the loupe leaves the screen at the origin: %v", near)
	}
}

// The panel says the colour: saturation across, value up.
func TestPanelPicksSaturationAndValue(t *testing.T) {
	w := &colorWindow{}
	w.setColor(color.RGBA{R: 0x80, G: 0x80, B: 0x80, A: 255}, true)
	tt := ui.NewTester(w.view, colorWinW, colorWinH)
	tt.Frame()

	r := w.panel
	if r.W < 100 || r.H < 100 {
		t.Fatalf("the panel has no box: %v", r)
	}
	drag := func(x, y float32) {
		tt.Press(x, y)
		tt.Move(x, y)
		tt.Release(x, y)
		tt.Frame()
	}
	// The bottom right is fully saturated and black.
	drag(r.X+r.W*0.99, r.Y+r.H*0.99)
	if w.s < 0.95 || w.v > 0.05 {
		t.Fatalf("bottom right gives s=%.2f v=%.2f, want saturated and black", w.s, w.v)
	}
	// The top left is white: no saturation, full value.
	drag(r.X+r.W*0.01, r.Y+r.H*0.01)
	if w.s > 0.05 || w.v < 0.95 {
		t.Fatalf("top left gives s=%.2f v=%.2f, want white", w.s, w.v)
	}
	if w.col.R < 240 || w.col.G < 240 || w.col.B < 240 {
		t.Fatalf("white reads %v", w.col)
	}
}

// The hue bar turns the colour, keeping its saturation and value.
func TestHueBarChoosesTheHue(t *testing.T) {
	w := &colorWindow{}
	w.setColor(color.RGBA{R: 255, A: 255}, true)
	tt := ui.NewTester(w.view, colorWinW, colorWinH)
	tt.Frame()

	r := w.hue
	if r.H < 100 {
		t.Fatalf("the hue bar has no box: %v", r)
	}
	// A third of the way down is green, 120 degrees.
	y := r.Y + r.H/3
	tt.Press(r.X+r.W/2, y)
	tt.Move(r.X+r.W/2, y)
	tt.Release(r.X+r.W/2, y)
	tt.Frame()
	if w.col.G < 240 || w.col.R > 12 || w.col.B > 12 {
		t.Fatalf("a third down gives %v, want green", w.col)
	}
}

// The arrows step one channel, leaving the colour as it was picked.
func TestChannelArrowsStepTheColour(t *testing.T) {
	w := &colorWindow{}
	w.setColor(color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 255}, true)
	tt := ui.NewTester(w.view, colorWinW, colorWinH)
	tt.Frame()

	if err := tt.Click("增加Red"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if w.col.R != 0x11 {
		t.Fatalf("red stepped to %d, want 17", w.col.R)
	}
	if w.orig.R != 0x10 {
		t.Fatalf("the original changed to %d", w.orig.R)
	}
	if err := tt.Click("减少Blue"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if w.col.B != 0x2F {
		t.Fatalf("blue stepped to %d, want 47", w.col.B)
	}
}

// Every control stays inside the window it is laid out in.
func TestColorWindowFitsTheWindow(t *testing.T) {
	w := &colorWindow{}
	w.setColor(color.RGBA{R: 0x11, G: 0x22, B: 0x33, A: 255}, true)
	const cw, ch = colorWinW, colorWinH
	tt := ui.NewTester(w.view, cw, ch)
	tt.Frame()

	if r := w.panel; r.W < 200 || r.H < 150 {
		t.Fatalf("the panel is too small: %v", r)
	}
	if r := w.hue; r.H < 100 {
		t.Fatalf("the hue bar is too small: %v", r)
	}
	for _, name := range []string{
		"Original", "New", "Red:", "Green:", "Blue:",
		"RGB:", "Dec:", "Hex:", "复制RGB", "复制Dec", "复制Hex",
		"重新拾取", "关闭",
	} {
		r, ok := tt.Find(name)
		if !ok {
			t.Errorf("no %q in the window", name)
			continue
		}
		if r.X < 0 || r.Y < 0 || r.X+r.W > cw || r.Y+r.H > ch {
			t.Errorf("%q leaves the window: %v", name, r)
		}
	}
}

// HSV keeps a colour to within a step of itself, and grays keep their hue.
func TestHSVRoundTrip(t *testing.T) {
	for _, c := range []color.RGBA{
		{R: 255, A: 255}, {G: 255, A: 255}, {B: 255, A: 255},
		{R: 0x1B, G: 0x9E, B: 0x4B, A: 255}, {R: 0x80, G: 0x80, B: 0x80, A: 255},
	} {
		got := hsvOf(c).rgba()
		if diff := absDiffByte(got.R, c.R); diff > 1 {
			t.Errorf("%v round trips to %v", c, got)
		}
		if diff := absDiffByte(got.G, c.G); diff > 1 {
			t.Errorf("%v round trips to %v", c, got)
		}
		if diff := absDiffByte(got.B, c.B); diff > 1 {
			t.Errorf("%v round trips to %v", c, got)
		}
	}
	w := &colorWindow{}
	w.setColor(color.RGBA{R: 0, G: 200, B: 255, A: 255}, true)
	hue := w.h
	w.setColor(color.RGBA{R: 0x40, G: 0x40, B: 0x40, A: 255}, false)
	if w.h != hue {
		t.Fatalf("a gray took the hue to %.1f, want %.1f", w.h, hue)
	}
}

func absDiffByte(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// Each format sits on the row of its channel: RGB with Red, Dec with
// Green and Hex with Blue, and the three rows are stacked.
func TestFormatsSitBesideTheirChannels(t *testing.T) {
	w := &colorWindow{}
	w.setColor(color.RGBA{R: 0x1B, G: 0x9E, B: 0x4B, A: 255}, true)
	const cw = colorWinW
	tt := ui.NewTester(w.view, cw, colorWinH)
	tt.Frame()

	find := func(name string) ui.Rect {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("no %q, have %q", name, tt.Texts())
		}
		return r
	}
	pairs := [][2]string{{"Red:", "RGB:"}, {"Green:", "Dec:"}, {"Blue:", "Hex:"}}
	for _, pair := range pairs {
		channel, format := find(pair[0]), find(pair[1])
		if d := format.Y - channel.Y; d > 2 || d < -2 {
			t.Errorf("%q is %.1f DIPs from %q, want the same row", pair[1], d, pair[0])
		}
		if format.X <= channel.X {
			t.Errorf("%q is not to the right of %q", pair[1], pair[0])
		}
	}
	// The rows themselves are still stacked, not collapsed into one.
	if find("Hex:").Y <= find("Red:").Y {
		t.Error("the three channel rows are not stacked")
	}
	// The long rgb() value still fits inside the window.
	if r, ok := tt.Find("rgb(27, 158, 75)"); ok && r.X+r.W > cw {
		t.Errorf("the rgb value runs past the window: %v", r)
	}
}

// The window wears a title bar of its own, as the settings dialog does,
// because Windows draws no frame around it: a white band with the title, a
// close button, and a hairline under them.
func TestColorWindowHasItsOwnTitleBar(t *testing.T) {
	w := &colorWindow{}
	w.setColor(color.RGBA{R: 0x20, G: 0x40, B: 0x60, A: 255}, true)
	tt := ui.NewTester(w.view, colorWinW, colorWinH)
	tt.Frame()

	if !tt.HasText(colorWinTitle) {
		t.Fatalf("no title in the window, have %q", tt.Texts())
	}
	close, ok := tt.Find("关闭颜色拾取")
	if !ok {
		t.Fatalf("no close button in the header, have %q", tt.Texts())
	}
	title, _ := tt.Find(colorWinTitle)
	if close.Y >= w.panel.Y || title.Y >= w.panel.Y {
		t.Errorf("the header is not above the panel: title %v, close %v, panel %v",
			title, close, w.panel)
	}
	if close.X+close.W > colorWinW {
		t.Errorf("the close button leaves the window: %v", close)
	}
	// The whole top band is the drag handle, so it must not take a click as
	// a colour change of the panel below it.
	tt.ClickAt(colorWinW/2, title.Y)
}

// The spinners draw the chevron icons, not a hand-painted triangle.
func TestSpinnerIconsExist(t *testing.T) {
	for _, name := range []string{"chevron-up", "chevron-down"} {
		if iconSet[name] == nil {
			t.Errorf("no %q icon", name)
		}
	}
}
