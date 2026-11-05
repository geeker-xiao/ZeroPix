package main

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

// newTestSession builds a snip session over a synthetic 1280x800 screen,
// without a window.
func newTestSession() *snipSession {
	s := &snipSession{
		app:      app,
		shot:     image.NewRGBA(image.Rect(0, 0, 1280, 800)),
		frame:    ui.Rect{W: 1280, H: 800},
		scale:    1,
		color:    annotationColors[0],
		width:    2.5,
		fontSize: 16,
		mosaic:   12,
		handle:   -1,
		want:     ui.CursorCrosshair,
		active:   toolSelect,
		phase:    phaseSelect,
	}
	return s
}

// Dragging out a region puts the overlay into its editing phase and builds
// the toolbar.
func TestSnipDragSelectsAndBuildsToolbar(t *testing.T) {
	s := newTestSession()
	tt := ui.NewTester(s.view, 1280, 800)
	tt.Frame()
	tt.Press(200, 150)
	tt.Move(260, 190)
	tt.Move(700, 500)
	tt.Release(700, 500)
	tt.Frame()

	if !s.hasSel || s.phase != phaseEdit {
		t.Fatalf("selection = %+v, phase = %v", s.sel, s.phase)
	}
	if s.sel.W < 400 || s.sel.H < 300 {
		t.Fatalf("selection too small: %+v", s.sel)
	}
	for _, label := range []string{"矩形描边 (1)", "椭圆描边 (2)", "箭头 (5)", "马赛克 (0)", "文字 (8)", "序号 (9)", "复制到剪贴板 (Enter)"} {
		if !tt.HasText(label) {
			t.Errorf("toolbar missing %q, have %q", label, tt.Texts())
		}
	}
}

// A click without a drag leaves no selection behind.
func TestSnipClickDiscardsSelection(t *testing.T) {
	s := newTestSession()
	tt := ui.NewTester(s.view, 1280, 800)
	tt.Frame()
	tt.Press(300, 300)
	tt.Release(302, 301)
	tt.Frame()
	if s.hasSel || s.phase != phaseSelect {
		t.Fatalf("hasSel = %v, phase = %v", s.hasSel, s.phase)
	}
}

// Drawing a shape commits it, and undo/redo move it between the stacks.
func TestSnipShapesUndoRedo(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 100, Y: 100, W: 600, H: 400}
	s.hasSel, s.phase = true, phaseEdit
	tt := ui.NewTester(s.view, 1280, 800)
	tt.Frame()

	s.setTool(toolRect)
	tt.Press(200, 200)
	tt.Move(300, 260)
	tt.Release(300, 260)
	tt.Frame()
	if len(s.shapes) != 1 {
		t.Fatalf("shapes = %d", len(s.shapes))
	}
	s.undo()
	if len(s.shapes) != 0 || len(s.redone) != 1 {
		t.Fatalf("undo: shapes %d, redone %d", len(s.shapes), len(s.redone))
	}
	s.redo()
	if len(s.shapes) != 1 || len(s.redone) != 0 {
		t.Fatalf("redo: shapes %d, redone %d", len(s.shapes), len(s.redone))
	}
}

// Numbered markers carry on from the highest one still on the image.
func TestSnipNumbering(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 100, Y: 100, W: 600, H: 400}
	s.hasSel, s.phase = true, phaseEdit
	tt := ui.NewTester(s.view, 1280, 800)
	tt.Frame()

	s.setTool(toolNumber)
	for i := 0; i < 3; i++ {
		tt.Press(float32(200+60*i), 200)
		tt.Release(float32(200+60*i), 200)
	}
	if len(s.shapes) != 3 {
		t.Fatalf("shapes = %d", len(s.shapes))
	}
	for i, sh := range s.shapes {
		if sh.number != i+1 {
			t.Fatalf("marker %d numbered %d", i, sh.number)
		}
	}
	s.undo()
	if got := s.nextNumber(); got != 3 {
		t.Fatalf("nextNumber = %d, want 3", got)
	}
}

// The export crops the selection and draws the annotations at pixel scale.
func TestSnipExport(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 100, Y: 100, W: 200, H: 100}
	s.hasSel, s.phase = true, phaseEdit
	s.shapes = []*shape{{
		kind: toolRect, a: ui.Point{X: 110, Y: 110}, b: ui.Point{X: 200, Y: 160},
		color: annotationColors[0], width: 2,
	}}
	img := s.render()
	if img == nil || img.Bounds().Dx() != 200 || img.Bounds().Dy() != 100 {
		t.Fatalf("render = %v", img.Bounds())
	}
	// The outline is drawn at (10, 10) in the crop: red where it passes.
	if r, g, b, _ := img.At(10, 30).RGBA(); r>>8 < 0xB0 || g>>8 > 0x60 || b>>8 > 0x60 {
		t.Fatalf("outline pixel = %x %x %x", r, g, b)
	}
}

func TestNormalizeRect(t *testing.T) {
	r := normalizeRect(100, 50, 20, 10)
	if r != (ui.Rect{X: 20, Y: 10, W: 80, H: 40}) {
		t.Fatalf("normalizeRect = %+v", r)
	}
}

func TestPointInRect(t *testing.T) {
	r := ui.Rect{X: 10, Y: 10, W: 20, H: 20}
	if !pointInRect(15, 15, r) || pointInRect(5, 15, r) || pointInRect(30, 30, r) {
		t.Fatal("pointInRect bounds are wrong")
	}
}

func TestCropRGBA(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for i := range img.Pix {
		img.Pix[i] = uint8(i)
	}
	c := cropRGBA(img, 2, 2, 5, 4)
	if c == nil || c.Bounds().Dx() != 3 || c.Bounds().Dy() != 2 {
		t.Fatalf("crop bounds = %v", c.Bounds())
	}
	if c.Pix[0] != img.Pix[img.PixOffset(2, 2)] {
		t.Fatal("crop copied the wrong pixels")
	}
	if cropRGBA(img, 12, 12, 15, 15) != nil {
		t.Fatal("fully clipped crop should be nil")
	}
	if c := cropRGBA(img, -5, -5, 2, 2); c == nil || c.Bounds().Dx() != 2 || c.Bounds().Dy() != 2 {
		t.Fatal("negative offsets should clamp to the image origin")
	}
}

// The toolbar stays on screen wherever the region is.
func TestToolbarStaysOnScreen(t *testing.T) {
	s := newTestSession()
	cases := []ui.Rect{
		{X: 0, Y: 0, W: 100, H: 80},
		{X: 1200, Y: 0, W: 80, H: 80},
		{X: 0, Y: 740, W: 200, H: 60},
		{X: 400, Y: 300, W: 200, H: 100},
	}
	for _, sel := range cases {
		s.sel = sel
		s.hasSel, s.phase = true, phaseEdit
		tt := ui.NewTester(s.view, 1280, 800)
		tt.Frame()
		bar := s.barRect
		if bar.W <= 0 || bar.H <= 0 {
			t.Fatalf("sel %+v built no toolbar", sel)
		}
		if bar.X < 0 || bar.Y < 0 || bar.X+bar.W > s.frame.W || bar.Y+bar.H > s.frame.H {
			t.Errorf("sel %+v puts the toolbar at %v,%v (size %vx%v)", sel, bar.X, bar.Y, bar.W, bar.H)
		}
	}
}

func TestHandleRectsAndResize(t *testing.T) {
	r := ui.Rect{X: 100, Y: 100, W: 200, H: 100}
	handles := handleRects(r)
	if handles[handleNW] != (ui.Rect{X: 95.5, Y: 95.5, W: 9, H: 9}) {
		t.Fatalf("NW handle = %+v", handles[handleNW])
	}
	if got := resizeRect(r, handleSE, 400, 300); got != (ui.Rect{X: 100, Y: 100, W: 300, H: 200}) {
		t.Fatalf("resize SE = %+v", got)
	}
	if got := resizeRect(r, handleW, 400, 0); got.W != 4 {
		t.Fatalf("resize W past the other edge = %+v", got)
	}
	// A handle dragged past the opposite edges keeps a minimal rectangle.
	got := resizeRect(r, handleNW, 400, 300)
	if got.X != 296 || got.Y != 196 || got.W != 4 || got.H != 4 {
		t.Fatalf("crossed resize = %+v", got)
	}
}

func TestIconsParse(t *testing.T) {
	for _, name := range []string{
		"rect", "arrow", "mosaic", "text", "number", "undo", "redo",
		"scan", "translate", "video", "close", "pin", "save", "copy",
		"stop", "fullscreen",
	} {
		if iconSet[name] == nil {
			t.Errorf("icon %q did not parse", name)
		}
	}
}

func TestQuantizeFrame(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			c := color.RGBA{R: 200, G: 30, B: 40, A: 255}
			if y >= 16 {
				c = color.RGBA{R: 20, G: 180, B: 60, A: 255}
			}
			src.Set(x, y, c)
		}
	}
	p := quantizeFrame(src)
	if p.Bounds().Dx() != 64 || p.Bounds().Dy() != 32 {
		t.Fatalf("quantized bounds = %v", p.Bounds())
	}
	if len(p.Palette) > 256 {
		t.Fatalf("palette has %d colors", len(p.Palette))
	}
	// The two halves keep their colors apart.
	top := p.Palette[p.Pix[p.PixOffset(10, 5)]]
	bottom := p.Palette[p.Pix[p.PixOffset(10, 25)]]
	tr, _, _, _ := top.RGBA()
	br, _, _, _ := bottom.RGBA()
	if tr>>8 < 0x80 || br>>8 > 0x60 {
		t.Fatalf("colors merged: top %v bottom %v", top, bottom)
	}
}

func TestRecPlan(t *testing.T) {
	fps, frames := recPlan(800, 600)
	if fps < 6 || frames < 10 || frames > recMaxFrames {
		t.Fatalf("recPlan small = %d fps, %d frames", fps, frames)
	}
	_, big := recPlan(3840, 2160)
	if big > recMaxFrames {
		t.Fatalf("recPlan big = %d frames", big)
	}
	if px := 1280 * 720 * big; px > 6*recMaxBudget {
		t.Fatalf("recPlan big exceeds the memory budget: %d bytes", px)
	}
}

func TestTrimMessage(t *testing.T) {
	if got := trimMessage("abc", 10); got != "abc" {
		t.Fatalf("trimMessage short = %q", got)
	}
	got := trimMessage("这是一条很长很长的消息内容", 6)
	if []rune(got) == nil || len([]rune(got)) != 6 {
		t.Fatalf("trimMessage long = %q", got)
	}
}

// The four swatches sit at the left of the bar and set the annotation colour.
func TestColorSwatches(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 400, Y: 200, W: 715, H: 435}
	s.hasSel, s.phase = true, phaseEdit
	tt := ui.NewTester(s.view, 2560, 1440)
	tt.Frame()

	if len(annotationColors) != 4 || len(annotationColorNames) != 4 {
		t.Fatalf("want four swatches, have %d colours, %d names",
			len(annotationColors), len(annotationColorNames))
	}
	for i, name := range annotationColorNames {
		if err := tt.Click(name); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		tt.Frame()
		if s.color != annotationColors[i] {
			t.Fatalf("%s: colour = %v", name, s.color)
		}
	}
}

// A text annotation keeps the family and the size chosen in the editor.
func TestTextAnnotationFontAndSize(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 400, Y: 200, W: 715, H: 435}
	s.hasSel, s.phase = true, phaseEdit
	s.active, s.font = toolText, annotationFonts[2].Family
	tt := ui.NewTester(s.view, 2560, 1440)
	tt.Frame()

	s.startText(ui.Point{X: 500, Y: 300})
	tt.Frame()

	// Every control of the editor stays inside its card, so none of them
	// hangs over the edge of the white panel.
	for _, name := range []string{"字体", "字号", "Increase", "Decrease"} {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("the editor has no %s", name)
		}
		if r.X < s.editorRect.X || r.X+r.W > s.editorRect.X+s.editorRect.W {
			t.Fatalf("%s at %v leaves the editor %v", name, r, s.editorRect)
		}
	}

	before := s.fontSize
	if err := tt.Click("Increase"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.fontSize <= before {
		t.Fatalf("size button did not grow the text: %v -> %v", before, s.fontSize)
	}

	s.editText = "宋体标注"
	s.commitText()
	if len(s.shapes) != 1 {
		t.Fatalf("committed %d shapes", len(s.shapes))
	}
	sh := s.shapes[0]
	if sh.font != annotationFonts[2].Family || sh.size != s.fontSize {
		t.Fatalf("shape font=%q size=%v", sh.font, sh.size)
	}
}

// The toolbar's buttons switch tools and drive undo/redo. A press on the
// bar's own padding must not start a new selection.
// An ellipse keeps its outline on the exported image, with its inside
// untouched, as a rectangle does.
func TestEllipseRasterizes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 120))
	for i := range img.Pix {
		img.Pix[i] = 255
	}
	sh := &shape{
		kind:  toolEllipse,
		a:     ui.Point{X: 20, Y: 20},
		b:     ui.Point{X: 180, Y: 100},
		color: annotationColors[0],
		width: 3,
	}
	rasterShapes(img, []*shape{sh}, 0, 0, 1)

	at := func(x, y int) color.RGBA {
		i := img.PixOffset(x, y)
		return color.RGBA{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: img.Pix[i+3]}
	}
	if c := at(100, 20); c.R < 180 || c.G > 120 {
		t.Fatalf("the top of the ellipse is %v, not the annotation colour", c)
	}
	if c := at(100, 60); c.R != 255 || c.G != 255 || c.B != 255 {
		t.Fatalf("the middle of the ellipse was painted: %v", c)
	}
	if c := at(24, 24); c.R != 255 || c.G != 255 || c.B != 255 {
		t.Fatalf("the corner was painted: %v", c)
	}
}

func TestToolbarButtons(t *testing.T) {
	s := newTestSession()
	s.sel = ui.Rect{X: 400, Y: 200, W: 715, H: 435}
	s.hasSel, s.phase = true, phaseEdit
	tt := ui.NewTester(s.view, 2560, 1440)
	tt.Frame()

	for _, tc := range []struct {
		label string
		want  tool
	}{
		{"矩形描边 (1)", toolRect},
		{"椭圆描边 (2)", toolEllipse},
		{"箭头 (5)", toolArrow},
		{"马赛克 (0)", toolMosaic},
		{"文字 (8)", toolText},
		{"序号 (9)", toolNumber},
	} {
		if err := tt.Click(tc.label); err != nil {
			t.Fatalf("%s: %v", tc.label, err)
		}
		tt.Frame()
		if s.active != tc.want {
			t.Fatalf("%s: active = %v", tc.label, s.active)
		}
	}
	// The same tool again goes back to selecting.
	tt.Click("序号 (9)")
	tt.Frame()
	if s.active != toolSelect {
		t.Fatalf("second click: active = %v", s.active)
	}

	s.push(&shape{
		kind: toolRect, a: ui.Point{X: 500, Y: 300}, b: ui.Point{X: 600, Y: 400},
		color: annotationColors[0], width: 2,
	})
	tt.Frame()
	if err := tt.Click("撤销 (Ctrl+Z)"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(s.shapes) != 0 {
		t.Fatalf("undo left %d shapes", len(s.shapes))
	}
	if err := tt.Click("重做 (Ctrl+Shift+Z)"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(s.shapes) != 1 {
		t.Fatalf("redo left %d shapes", len(s.shapes))
	}

	// A press on the bar's padding keeps the region as it is.
	bar := s.barRect
	tt.Press(bar.X+2, bar.Y+2)
	tt.Release(bar.X+2, bar.Y+2)
	tt.Frame()
	if s.sel != (ui.Rect{X: 400, Y: 200, W: 715, H: 435}) {
		t.Fatalf("bar press changed the region: %+v", s.sel)
	}
}

func TestDipRect(t *testing.T) {
	x, y, w, h := dipRect(1300, 300, 600, 320, 2)
	if x != 650 || y != 150 || w != 300 || h != 160 {
		t.Fatalf("at 200%% got (%d,%d %dx%d), want (650,150 300x160)", x, y, w, h)
	}
	// A display without a scale factor, or a zero one, is left alone.
	for _, scale := range []float64{0, 1} {
		x, y, w, h = dipRect(1300, 300, 600, 320, scale)
		if x != 1300 || y != 300 || w != 600 || h != 320 {
			t.Fatalf("at scale %g got (%d,%d %dx%d)", scale, x, y, w, h)
		}
	}
}
