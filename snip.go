// The fullscreen snip overlay: a frozen screenshot of the display under the
// cursor, a drag-to-select region, the annotation tools of the toolbar and
// the actions that finish a snip (pin, copy, save, recognize, translate,
// record).

package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// snipPhase is what the overlay is waiting for.
type snipPhase int

const (
	phaseSelect snipPhase = iota // dragging out a region
	phaseEdit                    // a region is chosen, tools are usable
)

// dragKind is the gesture the pointer is in.
type dragKind int

const (
	dragNone dragKind = iota
	dragNew
	dragMove
	dragResize
	dragShape
	dragErase
)

// Selection handles, clockwise from the top-left corner.
const (
	handleNW = iota
	handleN
	handleNE
	handleE
	handleSE
	handleS
	handleSW
	handleW
	handleCount
)

type snipSession struct {
	app     *zooApp
	display mygo.Display
	shot    *image.RGBA // the whole monitor, physical pixels
	bmp     *ui.Bitmap
	win     *mygo.Window

	phase  snipPhase
	frame  ui.Rect // the overlay in DIPs, origin 0,0
	sel    ui.Rect
	hasSel bool
	scale  float64 // snapshot pixels per DIP

	// Pointer state, touched only on the main thread.
	drag     dragKind
	dragFrom ui.Point
	dragSel  ui.Rect
	handle   int
	drawing  *shape
	cursor   ui.Point
	want     ui.Cursor

	// Annotations.
	shapes   []*shape
	history  []step   // what undo steps back through
	redone   []step   // what redo steps forward through
	path     *ui.Path // scratch geometry for strokes and arrow heads
	active   tool
	color    ui.Color
	width    float32
	fontSize float32
	font     string // the family text annotations are typed in
	mosaic   float32
	blurSize float32

	// The colour the eyedropper last took off the image, kept as the mixed
	// swatch so that the choice survives a few annotations.
	customColor ui.Color
	hasCustom   bool
	pickedNote  string // the hex just taken, shown for a moment

	// The picker the mixed swatch opens: where it was laid out, the colour
	// it shows as hue, saturation and value, and which piece is held.
	colorPanel   bool
	panelRect    ui.Rect
	panelSq      ui.Rect
	panelHueRect ui.Rect
	panelH       float64
	panelS       float64
	panelV       float64
	panelHeld    int

	// The family and size controls of the text editor keep their own state,
	// as the widgets write into them.
	fontPick  string
	sizeValue float64

	// In-place text entry.
	editing    bool
	editText   string
	editAt     ui.Point
	editKey    int     // identity of the editor, so focus lands once per entry
	editFocus  int     // bumped when a control needs the field refocused
	barRect    ui.Rect // the toolbar, so the canvas leaves its presses alone
	editorRect ui.Rect

	// actions running in the background
	busy string
}

// step is one thing undo can take back: a shape that was added, or the
// shapes an eraser stroke removed.
type step struct {
	shape  *shape
	erased []*shape
}

var (
	// The swatches the toolbar offers, left to right. The first one is what
	// a new snip starts with.
	annotationColors = []ui.Color{
		ui.RGB(0xE5, 0x39, 0x35), // 红
		ui.RGB(0x1B, 0x9E, 0x4B), // 绿
		ui.RGB(0x1F, 0x33, 0x55), // 蓝黑
		ui.RGB(0x8B, 0x5A, 0x2B), // 棕
	}
	annotationColorNames = []string{"红色", "绿色", "蓝黑色", "棕色"}
	// The pen widths the bar offers, thin to thick. A new snip starts on the
	// second one, which is also what a shape falls back to.
	annotationWidths     = []float32{1.5, 2.5, 4, 6.5}
	annotationWidthNames = []string{"细", "标准", "中粗", "粗"}
	snipDim              = ui.RGBA(0, 0, 0, 0.45)
	snipAccent           = ui.RGBA(0x00, 0xC9, 0xA0, 1)

	// lastColor is the swatch in use, so that a pin takes the frame of the
	// snip it came from, and one from the clipboard the last one picked.
	lastColor = annotationColors[0]
	// lastWidth is the pen width in use, kept from one snip to the next the
	// same way the colour is.
	lastWidth = annotationWidths[1]
)

// isPresetColor reports whether c is one of the four dots the bar ships
// with, rather than a colour the user mixed.
func isPresetColor(c ui.Color) bool {
	for _, p := range annotationColors {
		if p == c {
			return true
		}
	}
	return false
}

// startSnip hides the app's windows, freezes the display under the cursor
// and opens the overlay on it. full selects the whole screen at once.
func startSnip(full bool) {
	if app.snip != nil || app.rec != nil {
		return
	}
	pt := mygo.Screen.CursorScreenPoint()
	disp := mygo.Screen.DisplayNearestPoint(pt)
	go func() {
		app.hideForShot()
		x, y, w, h := displayPixelRect(disp)
		shot, err := captureScreenRect(x, y, w, h)
		if err != nil || shot == nil {
			app.restoreAfterShot()
			app.toast("截图失败: " + errText(err))
			return
		}
		s := &snipSession{
			app:      app,
			display:  disp,
			shot:     shot,
			active:   toolSelect,
			color:    lastColor,
			width:    lastWidth,
			fontSize: 16,
			font:     defaultFamily(),
			mosaic:   12,
			blurSize: 16,
			handle:   -1,
			want:     ui.CursorCrosshair,
			phase:    phaseSelect,
			// The mixed swatch starts on the last colour, so a colour
			// chosen once is still there on the next snip.
			customColor: lastColor,
			hasCustom:   !isPresetColor(lastColor),
		}
		s.frame = ui.Rect{W: float32(disp.Bounds.Width), H: float32(disp.Bounds.Height)}
		s.scale = float64(shot.Bounds().Dx()) / float64(s.frame.W)
		if full {
			s.sel = s.frame
			s.hasSel, s.phase = true, phaseEdit
			s.want = ui.CursorDefault
		}
		s.win = mygo.NewWindow(mygo.WindowOptions{
			Title:           appName + " 截图",
			X:               disp.Bounds.X,
			Y:               disp.Bounds.Y,
			Width:           disp.Bounds.Width,
			Height:          disp.Bounds.Height,
			Frameless:       true,
			AlwaysOnTop:     true,
			SkipTaskbar:     true,
			DisableResize:   true,
			DisableMove:     true,
			DisableShadow:   true,
			DisableMinimize: true,
			DisableMaximize: true,
			BackgroundColor: "#000000",
			Content:         ui.View(s.view),
		})
		app.snip = s
		s.win.Focus()
	}()
}

// displayPixelRect returns the physical pixel rectangle of a display.
func displayPixelRect(d mygo.Display) (x, y, w, h int) {
	s := d.ScaleFactor
	if s <= 0 {
		s = 1
	}
	x = int(math.Round(float64(d.Bounds.X) * s))
	y = int(math.Round(float64(d.Bounds.Y) * s))
	w = int(math.Round(float64(d.Bounds.Width) * s))
	h = int(math.Round(float64(d.Bounds.Height) * s))
	return
}

// displayAtPhysical returns the display that holds a physical pixel point.
// Screen queries take DIPs, so a physical point cannot ask them directly.
func displayAtPhysical(x, y int) mygo.Display {
	for _, d := range mygo.Screen.Displays() {
		dx, dy, dw, dh := displayPixelRect(d)
		if x >= dx && x < dx+dw && y >= dy && y < dy+dh {
			return d
		}
	}
	return mygo.Screen.PrimaryDisplay()
}

// dipRect converts a physical pixel rectangle to DIPs, the unit window
// options and screen queries use.
func dipRect(x, y, w, h int, scale float64) (int, int, int, int) {
	if scale <= 0 {
		scale = 1
	}
	return int(math.Round(float64(x) / scale)),
		int(math.Round(float64(y) / scale)),
		int(math.Round(float64(w) / scale)),
		int(math.Round(float64(h) / scale))
}

func (s *snipSession) view(c *ui.Context) {
	if s.bmp == nil {
		s.bmp = ui.NewBitmap(s.shot)
	}
	s.bindShortcuts(c)
	ui.Box(c).Fill().Children(func() {
		ui.Box(c).Fill().
			Cursor(s.want).
			HandleInput(s.input).
			Draw(func(p *ui.Painter, r ui.Rect) { s.paint(p, r) })
		s.barRect, s.editorRect = ui.Rect{}, ui.Rect{}
		if s.phase == phaseEdit && s.hasSel {
			s.buildToolbar(c)
		}
		if s.editing {
			s.buildTextEditor(c)
		}
	})
}

// bindShortcuts installs the overlay's keys. Escape and Enter mean
// different things while typing a text annotation.
func (s *snipSession) bindShortcuts(c *ui.Context) {
	c.OnShortcut(0, ui.KeyEscape, func() {
		if s.editing {
			s.cancelText()
			return
		}
		s.cancel()
	})
	c.OnShortcut(0, ui.KeyEnter, func() {
		if s.editing {
			s.commitText()
			return
		}
		if s.hasSel {
			s.finish(actCopy)
		}
	})
	c.OnShortcut(ui.Ctrl, ui.KeyZ, s.undo)
	c.OnShortcut(ui.Ctrl|ui.Shift, ui.KeyZ, s.redo)
	c.OnShortcut(ui.Ctrl, ui.KeyY, s.redo)
	c.OnShortcut(ui.Ctrl, ui.KeyC, func() {
		if s.hasSel {
			s.finish(actCopy)
		}
	})
	c.OnShortcut(ui.Ctrl, ui.KeyS, func() {
		if s.hasSel {
			s.finish(actSave)
		}
	})
	c.OnShortcut(ui.Ctrl, ui.KeyA, func() {
		s.sel = s.frame
		s.hasSel, s.phase = true, phaseEdit
	})
	// Tool keys: the first ten drawing tools, 1 through 9 and 0.
	toolKeys := []ui.Key{ui.Key1, ui.Key2, ui.Key3, ui.Key4, ui.Key5,
		ui.Key6, ui.Key7, ui.Key8, ui.Key9, ui.Key0}
	n := 0
	for _, it := range barCatalog {
		if !it.isTool() || n >= len(toolKeys) {
			continue
		}
		tt, k := it.tool, toolKeys[n]
		c.OnShortcut(0, k, func() {
			if !s.editing {
				s.setTool(tt)
			}
		})
		n++
	}
	c.OnShortcut(ui.Ctrl, ui.KeyD, func() {
		if !s.editing {
			s.redact()
		}
	})
}

// ---------------------------------------------------------------------------
// Pointer input.

func (s *snipSession) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerMove:
		s.cursor = ui.Point{X: ev.X, Y: ev.Y}
		switch s.drag {
		case dragNew:
			s.sel = normalizeRect(s.dragFrom.X, s.dragFrom.Y, ev.X, ev.Y)
			s.sel = intersectRect(s.sel, s.frame)
		case dragMove:
			s.sel.X = s.dragSel.X + (ev.X - s.dragFrom.X)
			s.sel.Y = s.dragSel.Y + (ev.Y - s.dragFrom.Y)
			s.clampSelection()
		case dragResize:
			s.sel = resizeRect(s.dragSel, s.handle, ev.X, ev.Y)
		case dragShape:
			if s.drawing != nil {
				s.drawing.b = ui.Point{X: ev.X, Y: ev.Y}
				if s.drawing.kind == toolBrush || s.drawing.kind == toolHighlighter {
					s.drawing.pts = append(s.drawing.pts, ui.Point{X: ev.X, Y: ev.Y})
				}
			}
		case dragErase:
			s.eraseAt(ev.X, ev.Y)
		}
		s.refreshCursor(ev.X, ev.Y)
		return true

	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		s.cursor = ui.Point{X: ev.X, Y: ev.Y}
		// The picker floats over the capture: a press inside it is its own,
		// and one outside closes it before it draws anything.
		if s.colorPanel {
			if pointInRect(ev.X, ev.Y, s.panelRect) {
				// The picker sits above the canvas: leave the press to it.
				return false
			}
			s.colorPanel = false
		}
		if s.inFloating(ev.X, ev.Y) {
			// The toolbar and the text field take their own presses; one
			// that reached here fell on their padding.
			return true
		}
		if s.editing {
			s.commitText()
			return true
		}
		if s.phase == phaseSelect {
			if ev.Clicks >= 2 {
				s.sel = s.frame
				s.hasSel, s.phase = true, phaseEdit
				return true
			}
			s.beginNewSelection(ev.X, ev.Y)
			return true
		}
		if h := s.handleAt(ev.X, ev.Y); h >= 0 {
			s.drag, s.handle, s.dragFrom, s.dragSel = dragResize, h, ui.Point{X: ev.X, Y: ev.Y}, s.sel
			return true
		}
		if pointInRect(ev.X, ev.Y, s.sel) {
			switch s.active {
			case toolSelect:
				s.drag, s.dragFrom, s.dragSel = dragMove, ui.Point{X: ev.X, Y: ev.Y}, s.sel
			case toolEyedropper:
				s.pickColorAt(ev.X, ev.Y)
			case toolEraser:
				s.drag = dragErase
				s.eraseAt(ev.X, ev.Y)
			default:
				s.beginShape(ev.X, ev.Y)
			}
			return true
		}
		// A press outside the region starts a new one.
		s.shapes, s.history, s.redone = nil, nil, nil
		s.beginNewSelection(ev.X, ev.Y)
		return true

	case ui.InputPointerUp:
		if ev.Button != 0 {
			return false
		}
		switch s.drag {
		case dragNew:
			s.drag = dragNone
			if s.sel.W < 4 || s.sel.H < 4 {
				s.sel, s.hasSel, s.phase = ui.Rect{}, false, phaseSelect
			} else {
				s.phase = phaseEdit
			}
		case dragShape:
			s.finishShape()
		case dragMove, dragResize, dragErase:
			s.drag = dragNone
		}
		s.refreshCursor(ev.X, ev.Y)
		return true
	}
	return false
}

func (s *snipSession) beginNewSelection(x, y float32) {
	s.drag = dragNew
	s.hasSel = true
	s.phase = phaseSelect
	s.dragFrom = ui.Point{X: x, Y: y}
	s.sel = ui.Rect{X: x, Y: y}
}

func (s *snipSession) beginShape(x, y float32) {
	sh := &shape{
		kind:  s.active,
		a:     ui.Point{X: x, Y: y},
		b:     ui.Point{X: x, Y: y},
		color: s.color,
		width: s.width,
		size:  s.fontSize,
	}
	switch s.active {
	case toolMosaic:
		sh.size = s.mosaic
	case toolBlur:
		sh.size = s.blurSize
	case toolBrush, toolHighlighter:
		sh.pts = []ui.Point{{X: x, Y: y}}
	}
	s.drawing = sh
	s.drag = dragShape
}

// finishShape commits the shape being dragged, or opens the text editor for
// a text annotation.
func (s *snipSession) finishShape() {
	s.drag = dragNone
	sh := s.drawing
	s.drawing = nil
	if sh == nil {
		return
	}
	if sh.kind == toolText {
		s.startText(sh.a)
		return
	}
	if sh.kind == toolNumber {
		sh.number = s.nextNumber()
		sh.a = ui.Point{X: sh.b.X, Y: sh.b.Y}
		s.push(sh)
		return
	}
	if sh.kind == toolBrush || sh.kind == toolHighlighter {
		if math.Hypot(float64(sh.b.X-sh.a.X), float64(sh.b.Y-sh.a.Y)) < 2 {
			return
		}
		if len(sh.pts) < 2 {
			sh.pts = append(sh.pts, sh.b)
		}
		s.push(sh)
		return
	}
	// Straight lines, arrows and the measuring tools are often drawn flat,
	// when one side of their box is zero: their length is what counts.
	switch sh.kind {
	case toolLine, toolArrow, toolDimension:
		if math.Hypot(float64(sh.b.X-sh.a.X), float64(sh.b.Y-sh.a.Y)) < 4 {
			return
		}
		s.push(sh)
		return
	case toolRuler:
		bar, _ := rulerBar(sh)
		if bar.W < 8 || bar.H < 8 {
			return
		}
		s.push(sh)
		return
	}
	r := sh.rect()
	if r.W < 3 || r.H < 3 {
		return
	}
	s.push(sh)
}

func (s *snipSession) push(sh *shape) {
	s.shapes = append(s.shapes, sh)
	s.history = append(s.history, step{shape: sh})
	s.redone = nil
}

// eraseAt takes away the shapes the eraser touches, as one undo step.
func (s *snipSession) eraseAt(x, y float32) {
	var gone []*shape
	kept := s.shapes[:0]
	for _, sh := range s.shapes {
		if shapeHit(sh, x, y) {
			gone = append(gone, sh)
			continue
		}
		kept = append(kept, sh)
	}
	if len(gone) == 0 {
		return
	}
	s.shapes = kept
	s.history = append(s.history, step{erased: gone})
	s.redone = nil
}

// nextNumber is one more than the highest marker still on the image.
func (s *snipSession) nextNumber() int {
	n := 1
	for _, sh := range s.shapes {
		if sh.kind == toolNumber && sh.number >= n {
			n = sh.number + 1
		}
	}
	return n
}

func (s *snipSession) undo() {
	if len(s.history) == 0 {
		return
	}
	st := s.history[len(s.history)-1]
	s.history = s.history[:len(s.history)-1]
	if st.shape != nil {
		for i := len(s.shapes) - 1; i >= 0; i-- {
			if s.shapes[i] == st.shape {
				s.shapes = append(s.shapes[:i], s.shapes[i+1:]...)
				break
			}
		}
	}
	if len(st.erased) > 0 {
		s.shapes = append(s.shapes, st.erased...)
	}
	s.redone = append(s.redone, st)
}

func (s *snipSession) redo() {
	if len(s.redone) == 0 {
		return
	}
	st := s.redone[len(s.redone)-1]
	s.redone = s.redone[:len(s.redone)-1]
	if st.shape != nil {
		s.shapes = append(s.shapes, st.shape)
	}
	if len(st.erased) > 0 {
		gone := make(map[*shape]bool, len(st.erased))
		for _, sh := range st.erased {
			gone[sh] = true
		}
		kept := s.shapes[:0]
		for _, sh := range s.shapes {
			if !gone[sh] {
				kept = append(kept, sh)
			}
		}
		s.shapes = kept
	}
	s.history = append(s.history, st)
}

func (s *snipSession) clampSelection() {
	if s.sel.X < 0 {
		s.sel.X = 0
	}
	if s.sel.Y < 0 {
		s.sel.Y = 0
	}
	if s.sel.X+s.sel.W > s.frame.W {
		s.sel.X = s.frame.W - s.sel.W
	}
	if s.sel.Y+s.sel.H > s.frame.H {
		s.sel.Y = s.frame.H - s.sel.H
	}
}

// handleAt returns the index of the resize handle under (x, y), or -1.
func (s *snipSession) handleAt(x, y float32) int {
	for i, r := range handleRects(s.sel) {
		if pointInRect(x, y, growRect(r, 3)) {
			return i
		}
	}
	return -1
}

func (s *snipSession) refreshCursor(x, y float32) {
	switch s.drag {
	case dragNew, dragShape:
		s.want = ui.CursorCrosshair
		return
	}
	if s.phase == phaseSelect {
		s.want = ui.CursorCrosshair
		return
	}
	if h := s.handleAt(x, y); h >= 0 && s.drag != dragMove {
		s.want = handleCursor(h)
		return
	}
	if pointInRect(x, y, s.sel) {
		if s.active == toolSelect {
			s.want = ui.CursorMove
		} else {
			s.want = ui.CursorCrosshair
		}
		return
	}
	s.want = ui.CursorCrosshair
}

func handleCursor(h int) ui.Cursor {
	switch h {
	case handleNW, handleSE:
		return ui.CursorResizeNWSE
	case handleNE, handleSW:
		return ui.CursorResizeNESW
	case handleN, handleS:
		return ui.CursorResizeNS
	default:
		return ui.CursorResizeEW
	}
}

// ---------------------------------------------------------------------------
// Painting.

func (s *snipSession) paint(p *ui.Painter, r ui.Rect) {
	p.Image(s.bmp, r, ui.FillBox)
	if !s.hasSel || s.sel.W < 1 || s.sel.H < 1 {
		p.Fill(r, snipDim, 0)
		s.paintCrosshair(p, r)
		return
	}
	// Dim everything but the chosen region.
	sel := s.sel
	p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: max(sel.Y-r.Y, 0)}, snipDim, 0)
	p.Fill(ui.Rect{X: r.X, Y: sel.Y + sel.H, W: r.W, H: max(r.Y+r.H-sel.Y-sel.H, 0)}, snipDim, 0)
	p.Fill(ui.Rect{X: r.X, Y: sel.Y, W: max(sel.X-r.X, 0), H: sel.H}, snipDim, 0)
	p.Fill(ui.Rect{X: sel.X + sel.W, Y: sel.Y, W: max(r.X+r.W-sel.X-sel.W, 0), H: sel.H}, snipDim, 0)

	// Annotations stay inside the region, as they will in the export.
	p.Clip(sel, 0, func() {
		if s.path == nil {
			s.path = &ui.Path{}
		}
		if s.drawing != nil {
			paintShape(p, s.drawing, s.path, s.shot, s.scale, s.scale, sel)
		}
		paintShapes(p, s.shapes, s.path, s.shot, s.scale, s.scale, sel)
	})

	if s.phase == phaseEdit {
		// A pale halo keeps the brand rule visible over a capture that is
		// itself green or dark.
		p.Stroke(growRect(sel, 1), ui.RGBA(255, 255, 255, 0.8), 0, 3)
		p.Stroke(sel, snipAccent, 0, 1.5)
		handleCol := ui.RGBA(255, 255, 255, 1)
		for _, hr := range handleRects(sel) {
			p.Fill(hr, handleCol, 2)
			p.Stroke(hr, snipAccent, 2, 1.2)
		}
	}
	s.paintSizeLabel(p, sel)
	s.paintPicked(p)
	s.paintBusy(p, sel)
	// Guides belong to choosing the region alone: while an annotation is
	// being drawn they only get in the way, and the number tool in
	// particular is a single click.
	if s.drag == dragNew || s.phase == phaseSelect {
		s.paintCrosshair(p, r)
	}
}

func (s *snipSession) paintSizeLabel(p *ui.Painter, sel ui.Rect) {
	w := int(math.Round(float64(sel.W) * s.scale))
	h := int(math.Round(float64(sel.H) * s.scale))
	label := fmt.Sprintf("%d × %d", w, h)
	lw := float32(len(label))*7 + 14
	lr := ui.Rect{X: sel.X, Y: sel.Y - 26, W: lw, H: 20}
	if lr.Y < 4 {
		lr.Y = sel.Y + 6
	}
	p.Fill(lr, ui.RGBA(17, 17, 17, 0.78), 4)
	p.Text(lr.X+7, lr.Y+3, label, 12, ui.RGBA(255, 255, 255, 1))
}

func (s *snipSession) paintCrosshair(p *ui.Painter, r ui.Rect) {
	x, y := s.cursor.X, s.cursor.Y
	if x <= 0 && y <= 0 {
		return
	}
	guide := ui.RGBA(255, 255, 255, 0.55)
	p.Stroke(ui.Rect{X: x, Y: 0, W: 1, H: r.H}, guide, 0, 1)
	p.Stroke(ui.Rect{X: 0, Y: y, W: r.W, H: 1}, guide, 0, 1)
}

// ---------------------------------------------------------------------------
// The eyedropper and the actions that work on the region.

// crop copies the chosen region out of the frozen screen, at full scale.
func (s *snipSession) crop() *image.RGBA {
	if !s.hasSel || s.shot == nil {
		return nil
	}
	k := s.scale
	x0 := int(math.Round(float64(s.sel.X) * k))
	y0 := int(math.Round(float64(s.sel.Y) * k))
	x1 := int(math.Round(float64(s.sel.X+s.sel.W) * k))
	y1 := int(math.Round(float64(s.sel.Y+s.sel.H) * k))
	return cropRGBA(s.shot, x0, y0, x1, y1)
}

// pixelAt reads one pixel of the snapshot, black when it is out of bounds.
func pixelAt(img *image.RGBA, x, y int) color.RGBA {
	if img == nil {
		return color.RGBA{A: 255}
	}
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return color.RGBA{A: 255}
	}
	i := img.PixOffset(x, y)
	return color.RGBA{R: img.Pix[i], G: img.Pix[i+1], B: img.Pix[i+2], A: 255}
}

// pickColorAt takes the colour under a point of the overlay, as the
// eyedropper does, and keeps it as an extra swatch.
func (s *snipSession) pickColorAt(x, y float32) {
	px := int(math.Floor(float64(x) * s.scale))
	py := int(math.Floor(float64(y) * s.scale))
	c := pixelAt(s.shot, px, py)
	s.color = ui.RGB(c.R, c.G, c.B)
	lastColor = s.color
	s.customColor, s.hasCustom = s.color, true
	s.pickedNote = hexOf(c)
	s.refresh()
	time.AfterFunc(1500*time.Millisecond, func() {
		if s.win != nil && !s.win.IsDestroyed() {
			s.win.Update(func() { s.pickedNote = "" })
		}
	})
}

// paintPicked shows the hex the eyedropper just took, beside the pointer.
func (s *snipSession) paintPicked(p *ui.Painter) {
	if s.pickedNote == "" {
		return
	}
	label := s.pickedNote
	w := float32(measureText(label, 12)) + 34
	x := min(s.cursor.X+16, s.frame.W-w-6)
	y := min(s.cursor.Y+16, s.frame.H-30)
	box := ui.Rect{X: x, Y: y, W: w, H: 24}
	p.Fill(box, ui.RGBA(17, 17, 17, 0.86), 5)
	p.Fill(ui.Rect{X: x + 6, Y: y + 6, W: 12, H: 12}, s.color, 3)
	p.Text(x+24, y+4, label, 12, ui.RGBA(255, 255, 255, 1))
}

// paintBusy says what a background action is doing, over the region.
func (s *snipSession) paintBusy(p *ui.Painter, sel ui.Rect) {
	if s.busy == "" {
		return
	}
	w := float32(measureText(s.busy, 12.5)) + 26
	x := sel.X + (sel.W-w)/2
	y := sel.Y + 14
	p.Fill(ui.Rect{X: x, Y: y, W: w, H: 30}, ui.RGBA(17, 17, 17, 0.84), 6)
	p.Text(x+13, y+7, s.busy, 12.5, ui.RGBA(255, 255, 255, 1))
}

// refresh asks for a frame after the session changed something itself.
func (s *snipSession) refresh() {
	if s.win != nil && !s.win.IsDestroyed() {
		s.win.Update(func() {})
	}
}

// redact reads the text in the region and mosaics every line it finds, so
// that a screenshot of a form or a chat can be shared safely.
func (s *snipSession) redact() {
	if s.busy != "" {
		return
	}
	img := s.crop()
	if img == nil {
		return
	}
	s.busy = "正在识别文字…"
	s.refresh()
	go func() {
		lines, err := ocrEng.Recognize(img)
		if s.win == nil || s.win.IsDestroyed() {
			return
		}
		s.win.Update(func() {
			s.busy = ""
			if err != nil {
				app.toast("脱敏失败: " + err.Error())
				return
			}
			n := s.addRedactions(lines)
			if n == 0 {
				app.toast("没有识别到文字")
				return
			}
			app.toast("已遮蔽 " + itoa(n) + " 处文字")
		})
	}()
}

// addRedactions turns recognized lines into mosaic over the overlay.
func (s *snipSession) addRedactions(lines []ocrLine) int {
	n := 0
	for _, ln := range lines {
		if strings.TrimSpace(ln.Text) == "" {
			continue
		}
		b := ln.Box
		x0 := s.sel.X + float32(float64(b.Min.X)/s.scale)
		y0 := s.sel.Y + float32(float64(b.Min.Y)/s.scale)
		w := float32(float64(b.Dx()) / s.scale)
		h := float32(float64(b.Dy()) / s.scale)
		if w < 4 || h < 4 {
			continue
		}
		s.push(&shape{
			kind:  toolMosaic,
			a:     ui.Point{X: x0, Y: y0},
			b:     ui.Point{X: x0 + w, Y: y0 + h},
			color: s.color,
			size:  max(8, h*0.35),
		})
		n++
	}
	return n
}

// openSettings leaves the snip and shows the settings dialog.
func (s *snipSession) openSettings() {
	s.destroy()
	openSettings()
}

// ---------------------------------------------------------------------------
// Text annotations.

func (s *snipSession) startText(at ui.Point) {
	s.editing = true
	s.editText = ""
	s.editAt = at
	s.editKey++
	s.fontPick = fontLabel(s.font)
	s.sizeValue = float64(s.fontSize)
}

func (s *snipSession) commitText() {
	if !s.editing {
		return
	}
	text := s.editText
	s.editing = false
	s.editText = ""
	if text == "" {
		return
	}
	s.push(&shape{
		kind:  toolText,
		a:     s.editAt,
		b:     s.editAt,
		text:  text,
		font:  s.font,
		color: s.color,
		size:  s.fontSize,
	})
}

func (s *snipSession) cancelText() {
	s.editing = false
	s.editText = ""
}

// ---------------------------------------------------------------------------
// Finishing a snip.

type snipAction int

const (
	actCopy snipAction = iota
	actSave
	actPin
	actOCR
	actTranslate
	actGIF
	actVideo
	actClose
)

// render crops the region at physical scale and draws the annotations on it.
func (s *snipSession) render() *image.RGBA {
	if !s.hasSel || s.sel.W < 1 || s.sel.H < 1 {
		return nil
	}
	crop := s.crop()
	if crop == nil {
		return nil
	}
	rasterShapes(crop, s.shapes, s.sel.X, s.sel.Y, s.scale)
	return crop
}

// screenRect is the region in physical screen pixels, for recording.
func (s *snipSession) screenRect() (x, y, w, h int) {
	ox, oy, _, _ := displayPixelRect(s.display)
	k := s.scale
	return ox + int(math.Round(float64(s.sel.X)*k)),
		oy + int(math.Round(float64(s.sel.Y)*k)),
		int(math.Round(float64(s.sel.W) * k)),
		int(math.Round(float64(s.sel.H) * k))
}

func (s *snipSession) finish(act snipAction) {
	if act == actClose || !s.hasSel {
		s.cancel()
		return
	}
	if s.editing {
		s.commitText()
	}
	img := s.render()
	if img == nil {
		s.cancel()
		return
	}
	rx, ry, rw, rh := s.screenRect()
	s.destroy()
	switch act {
	case actCopy:
		go func() {
			if err := copyImageToClipboard(img); err != nil {
				app.toast("复制失败: " + err.Error())
			}
		}()
	case actSave:
		go saveImageDialog(img)
	case actPin:
		app.pinImageAt(img, rx, ry, s.scale, s.color)
	case actOCR:
		showOCRResult(img)
	case actTranslate:
		showTranslation(img)
	case actGIF:
		startRecording(recOptions{x: rx, y: ry, w: rw, h: rh, video: false})
	case actVideo:
		startRecording(recOptions{x: rx, y: ry, w: rw, h: rh, video: true})
	}
}

func (s *snipSession) cancel() {
	s.destroy()
}

func (s *snipSession) destroy() {
	if s.win != nil && !s.win.IsDestroyed() {
		s.win.Destroy()
	}
	if s.app.snip == s {
		s.app.snip = nil
	}
	s.shot = nil
	s.bmp = nil
	s.app.restoreAfterShot()
}

// ---------------------------------------------------------------------------
// Geometry helpers.

func pointInRect(x, y float32, r ui.Rect) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func normalizeRect(x0, y0, x1, y1 float32) ui.Rect {
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	return ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

func growRect(r ui.Rect, d float32) ui.Rect {
	return ui.Rect{X: r.X - d, Y: r.Y - d, W: r.W + 2*d, H: r.H + 2*d}
}

// handleRects returns the eight handle boxes around r, DIPs.
func handleRects(r ui.Rect) [handleCount]ui.Rect {
	const side = 9
	half := float32(side) / 2
	cx := r.X + r.W/2
	cy := r.Y + r.H/2
	x0, x1 := r.X, r.X+r.W
	y0, y1 := r.Y, r.Y+r.H
	at := func(x, y float32) ui.Rect {
		return ui.Rect{X: x - half, Y: y - half, W: side, H: side}
	}
	return [handleCount]ui.Rect{
		handleNW: at(x0, y0),
		handleN:  at(cx, y0),
		handleNE: at(x1, y0),
		handleE:  at(x1, cy),
		handleSE: at(x1, y1),
		handleS:  at(cx, y1),
		handleSW: at(x0, y1),
		handleW:  at(x0, cy),
	}
}

// resizeRect moves the edges of orig that the handle controls to (x, y).
func resizeRect(orig ui.Rect, handle int, x, y float32) ui.Rect {
	l, t, rr, b := orig.X, orig.Y, orig.X+orig.W, orig.Y+orig.H
	switch handle {
	case handleNW:
		l, t = x, y
	case handleN:
		t = y
	case handleNE:
		rr, t = x, y
	case handleE:
		rr = x
	case handleSE:
		rr, b = x, y
	case handleS:
		b = y
	case handleSW:
		l, b = x, y
	case handleW:
		l = x
	}
	if rr-l < 4 {
		if handle == handleNW || handle == handleW || handle == handleSW {
			l = rr - 4
		} else {
			rr = l + 4
		}
	}
	if b-t < 4 {
		if handle == handleNW || handle == handleN || handle == handleNE {
			t = b - 4
		} else {
			b = t + 4
		}
	}
	return normalizeRect(l, t, rr, b)
}

// cropRGBA copies a sub-rectangle into a fresh RGBA image.
func cropRGBA(src *image.RGBA, x0, y0, x1, y1 int) *image.RGBA {
	b := src.Bounds()
	x0, y0 = max(x0, b.Min.X), max(y0, b.Min.Y)
	x1, y1 = min(x1, b.Max.X), min(y1, b.Max.Y)
	if x1 <= x0 || y1 <= y0 {
		return nil
	}
	dst := image.NewRGBA(image.Rect(0, 0, x1-x0, y1-y0))
	for y := y0; y < y1; y++ {
		s := src.PixOffset(x0, y)
		d := dst.PixOffset(0, y-y0)
		copy(dst.Pix[d:d+4*(x1-x0)], src.Pix[s:s+4*(x1-x0)])
	}
	return dst
}

func errText(err error) string {
	if err == nil {
		return "未知错误"
	}
	return err.Error()
}

func intersectRect(a, b ui.Rect) ui.Rect {
	x0 := max(a.X, b.X)
	y0 := max(a.Y, b.Y)
	x1 := min(a.X+a.W, b.X+b.W)
	y1 := min(a.Y+a.H, b.Y+b.H)
	if x1 <= x0 || y1 <= y0 {
		return ui.Rect{}
	}
	return ui.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}
