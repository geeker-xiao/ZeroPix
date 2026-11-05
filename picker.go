// A screen colour picker: the display freezes under a magnifier, a click
// takes the pixel under the pointer, and a window shows that colour in the
// formats worth copying.

package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	loupeSide = 11                    // source pixels on a side
	loupeCell = 12                    // DIPs each of them takes
	loupeSize = loupeSide * loupeCell // the magnifier's side
	loupeGap  = 20                    // between the pointer and it
	loupeChip = 22                    // the label under it
)

// pickSession is the full-screen overlay that takes the colour.
type pickSession struct {
	app    *zooApp
	win    *mygo.Window
	shot   *image.RGBA
	bmp    *ui.Bitmap
	frame  ui.Rect // the display in DIPs
	scale  float64 // source pixels per DIP
	cursor ui.Point
}

// startPick freezes the display under the cursor and opens the picker on
// it. A window already showing a colour steps aside and comes back if the
// pick is abandoned.
func startPick() {
	if app.pick != nil || app.snip != nil || app.rec != nil {
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
			app.toast("取色失败: " + errText(err))
			return
		}
		p := &pickSession{app: app, shot: shot}
		p.frame = ui.Rect{W: float32(disp.Bounds.Width), H: float32(disp.Bounds.Height)}
		p.scale = float64(shot.Bounds().Dx()) / float64(p.frame.W)
		p.cursor = ui.Point{
			X: float32(pt.X - disp.Bounds.X),
			Y: float32(pt.Y - disp.Bounds.Y),
		}
		p.win = mygo.NewWindow(mygo.WindowOptions{
			Title:           appName + " 取色",
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
			Content:         ui.View(p.view),
		})
		app.pick = p
		p.win.Focus()
	}()
}

func (p *pickSession) view(c *ui.Context) {
	if p.bmp == nil {
		p.bmp = ui.NewBitmap(p.shot)
	}
	c.OnShortcut(0, ui.KeyEscape, p.cancel)
	c.OnShortcut(0, ui.KeyEnter, func() { p.pick(p.cursor.X, p.cursor.Y) })
	ui.Box(c).Fill().Children(func() {
		ui.Box(c).Fill().Cursor(ui.CursorCrosshair).HandleInput(p.input).
			Draw(func(pn *ui.Painter, r ui.Rect) { p.paint(pn, r) })
	})
}

func (p *pickSession) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerMove:
		p.cursor = ui.Point{X: ev.X, Y: ev.Y}
		return true
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		p.pick(ev.X, ev.Y)
		return true
	}
	return false
}

func (p *pickSession) paint(pn *ui.Painter, r ui.Rect) {
	pn.Image(p.bmp, r, ui.FillBox)
	// A dark line and a light one beside it: the pair reads on a white
	// page and on a black one alike.
	dark, light := ui.RGBA(0, 0, 0, 0.32), ui.RGBA(255, 255, 255, 0.62)
	pn.Stroke(ui.Rect{X: p.cursor.X, Y: 0, W: 1, H: r.H}, dark, 0, 1)
	pn.Stroke(ui.Rect{X: p.cursor.X + 1, Y: 0, W: 1, H: r.H}, light, 0, 1)
	pn.Stroke(ui.Rect{X: 0, Y: p.cursor.Y, W: r.W, H: 1}, dark, 0, 1)
	pn.Stroke(ui.Rect{X: 0, Y: p.cursor.Y + 1, W: r.W, H: 1}, light, 0, 1)
	p.paintLoupe(pn, r)
}

// loupeRect places the magnifier beside the pointer, on the side that has
// room for it and for the label under it.
func (p *pickSession) loupeRect(r ui.Rect) ui.Rect {
	x := p.cursor.X + loupeGap
	y := p.cursor.Y + loupeGap
	if x+loupeSize > r.W-4 {
		x = p.cursor.X - loupeGap - loupeSize
	}
	if y+loupeSize+loupeChip+6 > r.H-4 {
		y = p.cursor.Y - loupeGap - loupeSize
	}
	return ui.Rect{
		X: min(max(x, 4), max(r.W-loupeSize-4, 4)),
		Y: min(max(y, 4), max(r.H-loupeSize-loupeChip-6, 4)),
		W: loupeSize,
		H: loupeSize,
	}
}

func (p *pickSession) paintLoupe(pn *ui.Painter, r ui.Rect) {
	bx := p.loupeRect(r)
	pn.Fill(ui.Rect{X: bx.X - 5, Y: bx.Y - 5, W: bx.W + 10, H: bx.H + 10},
		ui.RGBA(0, 0, 0, 0.30), 8)
	center := loupeSide / 2
	sx := int(math.Floor(float64(p.cursor.X) * p.scale))
	sy := int(math.Floor(float64(p.cursor.Y) * p.scale))
	for gy := 0; gy < loupeSide; gy++ {
		for gx := 0; gx < loupeSide; gx++ {
			c := p.pixel(sx+gx-center, sy+gy-center)
			pn.Fill(ui.Rect{
				X: bx.X + float32(gx)*loupeCell,
				Y: bx.Y + float32(gy)*loupeCell,
				W: loupeCell,
				H: loupeCell,
			}, ui.RGBA(c.R, c.G, c.B, 1), 0)
		}
	}
	// Hairlines between the cells, so that a run of one colour still reads
	// as pixels rather than as a blank square.
	grid := ui.RGBA(0, 0, 0, 0.10)
	for i := 1; i < loupeSide; i++ {
		x := bx.X + float32(i)*loupeCell
		pn.Stroke(ui.Rect{X: x, Y: bx.Y, W: 1, H: bx.H}, grid, 0, 1)
		y := bx.Y + float32(i)*loupeCell
		pn.Stroke(ui.Rect{X: bx.X, Y: y, W: bx.W, H: 1}, grid, 0, 1)
	}

	// The pixel the pointer is on, ringed so it reads as one pixel.
	mx := bx.X + float32(center)*loupeCell
	my := bx.Y + float32(center)*loupeCell
	pn.Stroke(ui.Rect{X: mx - 1, Y: my - 1, W: loupeCell + 2, H: loupeCell + 2},
		ui.RGBA(255, 255, 255, 0.95), 2, 2)
	pn.Stroke(ui.Rect{X: mx - 3, Y: my - 3, W: loupeCell + 6, H: loupeCell + 6},
		ui.RGBA(0, 0, 0, 0.55), 5, 1)
	pn.Stroke(ui.Rect{X: bx.X, Y: bx.Y, W: bx.W, H: bx.H},
		ui.RGBA(255, 255, 255, 0.35), 0, 1)

	// Hex and RGB under it, on the dark chip the size label uses.
	col := p.pixel(sx, sy)
	label := fmt.Sprintf("%s  %s", hexOf(col), rgbOf(col))
	lw := float32(approxTextWidth(label, 12.5)) + 18
	lx := bx.X + (bx.W-lw)/2
	ly := bx.Y + bx.H + 8
	if ly+loupeChip > r.H-4 {
		ly = bx.Y - loupeChip - 8
	}
	lx = min(max(lx, 4), max(r.W-lw-4, 4))
	pn.Fill(ui.Rect{X: lx, Y: ly, W: lw, H: loupeChip},
		ui.RGBA(17, 17, 17, 0.82), 5)
	pn.Fill(ui.Rect{X: lx + 6, Y: ly + 5, W: 12, H: 12},
		ui.RGBA(col.R, col.G, col.B, 1), 3)
	pn.Stroke(ui.Rect{X: lx + 6, Y: ly + 5, W: 12, H: 12},
		ui.RGBA(255, 255, 255, 0.55), 3, 1)
	pn.Text(lx+24, ly+5, label, 12, ui.RGBA(255, 255, 255, 1))
}

// pick takes the colour under a point of the overlay and shows it.
func (p *pickSession) pick(x, y float32) {
	col := p.colorAt(x, y)
	p.close()
	showColorWindow(col)
}

func (p *pickSession) cancel() { p.close() }

func (p *pickSession) close() {
	if p.win != nil && !p.win.IsDestroyed() {
		p.win.Destroy()
	}
	if app.pick == p {
		app.pick = nil
	}
	app.restoreAfterShot()
}

// colorAt reads the frozen screen at a point of the overlay, in DIPs.
func (p *pickSession) colorAt(x, y float32) color.RGBA {
	return p.pixel(
		int(math.Floor(float64(x)*p.scale)),
		int(math.Floor(float64(y)*p.scale)),
	)
}

// pixel reads the frozen screen, answering black outside it.
func (p *pickSession) pixel(x, y int) color.RGBA {
	b := p.shot.Bounds()
	if x < b.Min.X || y < b.Min.Y || x >= b.Max.X || y >= b.Max.Y {
		return color.RGBA{A: 255}
	}
	i := p.shot.PixOffset(x, y)
	return color.RGBA{R: p.shot.Pix[i], G: p.shot.Pix[i+1], B: p.shot.Pix[i+2], A: 255}
}

// ---------------------------------------------------------------------------
// The window that shows the colour

// What the pointer holds in the window, if anything.
const (
	holdNone = iota
	holdPanel
	holdHue
)

// swatchSide is the side of the Original and the New patch.
const swatchSide = 80

// The colour window's size in DIPs, its header included.
const (
	colorWinW = 580
	colorWinH = 484
)

// colorWinTitle is what the header of the colour window says.
const colorWinTitle = "颜色拾取"

type colorWindow struct {
	win *mygo.Window

	orig color.RGBA // as it was picked
	col  color.RGBA // as it is now
	// The colour on the panel: hue in degrees, saturation and value from
	// 0 to 1. It keeps the hue of a grey, which its red, green and blue
	// have lost.
	h, s, v float64
	ready   bool

	drag  int
	panel ui.Rect // where the panel and the hue bar were laid out
	hue   ui.Rect

	note   string // what was copied last
	hidden bool
}

// hsvColor is a colour as its hue in degrees, and its saturation and
// value, from 0 to 1.
type hsvColor struct{ H, S, V float64 }

func hsvOf(c color.RGBA) hsvColor {
	r, g, b := float64(c.R)/255, float64(c.G)/255, float64(c.B)/255
	hi := math.Max(r, math.Max(g, b))
	lo := math.Min(r, math.Min(g, b))
	d := hi - lo
	var h float64
	switch {
	case d == 0:
	case hi == r:
		h = 60 * math.Mod((g-b)/d, 6)
	case hi == g:
		h = 60 * ((b-r)/d + 2)
	default:
		h = 60 * ((r-g)/d + 4)
	}
	if h < 0 {
		h += 360
	}
	sat := 0.0
	if hi > 0 {
		sat = d / hi
	}
	return hsvColor{H: h, S: sat, V: hi}
}

func (x hsvColor) rgba() color.RGBA {
	h := math.Mod(x.H, 360)
	if h < 0 {
		h += 360
	}
	sat, val := clamp01(x.S), clamp01(x.V)
	c := val * sat
	k := h / 60
	y := c * (1 - math.Abs(math.Mod(k, 2)-1))
	var r, g, b float64
	switch {
	case k < 1:
		r, g = c, y
	case k < 2:
		r, g = y, c
	case k < 3:
		g, b = c, y
	case k < 4:
		g, b = y, c
	case k < 5:
		r, b = y, c
	default:
		r, b = c, y
	}
	m := val - c
	return color.RGBA{R: byteOf(r + m), G: byteOf(g + m), B: byteOf(b + m), A: 255}
}

func clamp01(x float64) float64 { return math.Max(0, math.Min(1, x)) }

func byteOf(x float64) uint8 { return uint8(math.Round(clamp01(x) * 255)) }

func clampByte(n int) uint8 { return uint8(math.Max(0, math.Min(255, float64(n)))) }

// setColor shows c, keeping the hue the panel had when c is a grey.
func (w *colorWindow) setColor(c color.RGBA, asOriginal bool) {
	c.A = 255
	w.col = c
	x := hsvOf(c)
	if x.S == 0 && w.ready {
		x.H = w.h
	}
	w.h, w.s, w.v = x.H, x.S, x.V
	if asOriginal {
		w.orig = c
	}
	w.ready = true
}

// applyHSV moves the panel's mark, and the colour with it.
func (w *colorWindow) applyHSV(x hsvColor) {
	w.h, w.s, w.v = x.H, clamp01(x.S), clamp01(x.V)
	w.col = x.rgba()
}

// showColorWindow shows col, reusing the window already open.
func showColorWindow(col color.RGBA) {
	if w := app.colorWin; w != nil && w.win != nil && !w.win.IsDestroyed() {
		w.setColor(col, true)
		w.note, w.hidden = "", false
		w.win.Show()
		w.win.Update(func() {})
		w.win.Focus()
		return
	}
	w := &colorWindow{}
	w.setColor(col, true)
	// Frameless like the settings dialog, with the same header of its own:
	// the title bar of Windows would cut the window off the sheet.
	w.win = dialogWindow(colorWinTitle, colorWinW, colorWinH, 540, 450, "", w.view)
	app.colorWin = w
}

func (w *colorWindow) view(c *ui.Context) {
	t := c.Theme()
	if !w.ready {
		w.setColor(w.col, true)
	}
	c.OnShortcut(0, ui.KeyEscape, w.close)
	c.OnShortcut(ui.Ctrl, ui.KeyC, func() { w.copyValue(w.hex(), "Hex") })
	ui.Column(c).Fill().Background(pageBackground(t)).Children(func() {
		w.header(c, t)
		dialogBody(c, t, func() {
			// The panel of saturation and value, with the hue bar beside it.
			ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
				w.panelView(c)
				w.hueView(c)
			})
			// What was picked, what it is now, and a row for each channel.
			// Every channel keeps its format on its right: RGB beside Red,
			// Dec beside Green and Hex beside Blue.
			ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
				w.swatch(c, t, "Original", w.orig)
				w.swatch(c, t, "New", w.col)
				ui.Column(c).Gap(6).Grow(1).Height(swatchSide + 22).Justify(ui.Center).Children(func() {
					w.channelRow(c, t, "Red", w.col.R, 0, "RGB", w.rgb())
					w.channelRow(c, t, "Green", w.col.G, 1, "Dec", w.dec())
					w.channelRow(c, t, "Blue", w.col.B, 2, "Hex", w.hex())
				})
			})
			// The line keeps its height whether or not it has anything to
			// say, so that the buttons stay put after a copy.
			note := w.note
			if note == "" {
				note = " "
			}
			ui.Text(c, note).FontSize(11.5).TextColor(t.TextMuted)
			ui.Row(c).Gap(10).Justify(ui.Center).Children(func() {
				ui.Button(c, "重新拾取").Width(150).OnClick(func() { startPick() })
				brandPrimaryButton(c, "关闭", 150, false, w.close)
			})
		})
	})
}

// header is the window's own title bar, the same band the other dialogs
// wear in place of the Windows frame.
func (w *colorWindow) header(c *ui.Context, t *ui.Theme) {
	dialogHeader(c, t, "pipette", colorWinTitle,
		"拖动标题栏移动窗口，Esc 关闭", "关闭颜色拾取", w.close)
}

// panelView is the square that chooses saturation across and value up.
func (w *colorWindow) panelView(c *ui.Context) {
	hue := hsvColor{H: w.h, S: 1, V: 1}.rgba()
	sat, val := w.s, w.v
	ui.Box(c).Grow(1).Height(196).Radius(8).Cursor(ui.CursorCrosshair).
		HandleInput(w.pointer(holdPanel)).
		Draw(func(p *ui.Painter, r ui.Rect) {
			w.panel = r
			p.FillGradient(r, ui.LinearGradient{
				From: ui.RGB(255, 255, 255), To: ui.RGBA(hue.R, hue.G, hue.B, 1), Angle: 90,
			}, 8)
			p.FillGradient(r, ui.LinearGradient{
				From: ui.RGBA(0, 0, 0, 0), To: ui.RGB(0, 0, 0), Angle: 180,
			}, 8)
			cx := r.X + r.W*float32(sat)
			cy := r.Y + r.H*float32(1-val)
			p.Stroke(ui.Rect{X: cx - 7, Y: cy - 7, W: 14, H: 14}, ui.RGBA(0, 0, 0, 0.35), 7, 1)
			p.Stroke(ui.Rect{X: cx - 5.5, Y: cy - 5.5, W: 11, H: 11}, ui.RGBA(255, 255, 255, 1), 5.5, 2)
		})
}

// hueView is the bar of hues, red at the top and red again at the bottom.
func (w *colorWindow) hueView(c *ui.Context) {
	hues := []ui.Color{
		ui.RGB(255, 0, 0), ui.RGB(255, 255, 0), ui.RGB(0, 255, 0),
		ui.RGB(0, 255, 255), ui.RGB(0, 0, 255), ui.RGB(255, 0, 255), ui.RGB(255, 0, 0),
	}
	at := w.h / 360
	ui.Box(c).Width(16).Height(196).Cursor(ui.CursorCrosshair).
		HandleInput(w.pointer(holdHue)).
		Draw(func(p *ui.Painter, r ui.Rect) {
			w.hue = r
			p.Clip(r, 8, func() {
				seg := r.H / 6
				for i := 0; i < 6; i++ {
					band := ui.Rect{X: r.X, Y: r.Y + seg*float32(i), W: r.W, H: seg + 0.5}
					p.FillGradient(band, ui.LinearGradient{From: hues[i], To: hues[i+1], Angle: 180}, 0)
				}
			})
			// The mark stays inside the bar at either end of it.
			y := min(max(r.Y+r.H*float32(at), r.Y+2), r.Y+r.H-2)
			p.Stroke(ui.Rect{X: r.X - 2, Y: y - 2, W: r.W + 4, H: 4}, ui.RGBA(0, 0, 0, 0.30), 2, 1)
			p.Stroke(ui.Rect{X: r.X - 1, Y: y - 1, W: r.W + 2, H: 2}, ui.RGBA(255, 255, 255, 0.95), 1, 1)
		})
}

// swatch is a patch of a colour under its name.
func (w *colorWindow) swatch(c *ui.Context, t *ui.Theme, name string, col color.RGBA) {
	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, name).FontSize(11.5).TextColor(t.TextMuted)
		ui.Box(c).Width(swatchSide).Height(swatchSide).Radius(8).
			Background(ui.RGBA(col.R, col.G, col.B, 1)).
			Border(1, ui.RGBA(0, 0, 0, 0.14)).
			Shadow(0, 2, 8, 0, ui.RGBA(0, 0, 0, 0.16))
	})
}

func (w *colorWindow) copyButton(c *ui.Context, t *ui.Theme, name, value string) {
	b := ui.ButtonBase(c.Key("copy-" + name)).Width(30).Height(28).Radius(7).
		Center().Label("复制" + name).Tooltip("复制 " + value)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	if b.Clicked() {
		w.copyValue(value, name)
	}
	b.Children(func() {
		ui.Box(c).Width(15).Height(15).Draw(func(p *ui.Painter, r ui.Rect) {
			drawIcon(p, "copy", r, t.TextMuted)
		})
	})
}

// writeClipboard puts text on the clipboard; tests swap it for a recorder.
var writeClipboard = func(s string) { mygo.Clipboard.WriteText(s) }

// channelRow is one of Red, Green and Blue: its value with its arrows,
// and the format that belongs to that row, with a button copying it.
func (w *colorWindow) channelRow(c *ui.Context, t *ui.Theme, name string, v uint8, idx int, fmtName, fmtValue string) {
	ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Text(c, name+":").Width(44).FontSize(12).TextColor(t.TextMuted)
		ui.Box(c).Width(46).Height(26).Radius(6).Background(t.Surface).
			Border(1, t.Border).Center().Children(func() {
			ui.Text(c, strconv.Itoa(int(v))).FontSize(12.5).Font("Consolas, 微软雅黑, monospace")
		})
		ui.Column(c).Children(func() {
			w.spinner(c, t, idx, true)
			w.spinner(c, t, idx, false)
		})
		ui.Box(c).Width(1).Height(20).Background(t.Border)
		ui.Text(c, fmtName+":").Width(34).FontSize(11.5).TextColor(t.TextMuted)
		ui.Box(c).Grow(1).Height(28).Radius(6).Background(t.Surface).
			Border(1, t.Border).Padding(8, 0).Clip().Center().Children(func() {
			ui.Text(c, fmtValue).FontSize(11.5).Font("Consolas, 微软雅黑, monospace")
		})
		w.copyButton(c, t, fmtName, fmtValue)
	})
}

// spinner is one arrow of a channel's + and -.
func (w *colorWindow) spinner(c *ui.Context, t *ui.Theme, idx int, up bool) {
	name, delta := "增加"+channelName(idx), 1
	if !up {
		name, delta = "减少"+channelName(idx), -1
	}
	icon := "chevron-down"
	if up {
		icon = "chevron-up"
	}
	b := ui.ButtonBase(c.Key(fmt.Sprintf("spin-%d-%t", idx, up))).Width(18).Height(13).Radius(3).Label(name)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	if b.Clicked() {
		w.bump(idx, delta)
	}
	b.Children(func() {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			drawIcon(p, icon, r, t.TextMuted)
		})
	})
}

// channelName is what one of the three channels is called.
func channelName(idx int) string {
	return [3]string{"Red", "Green", "Blue"}[idx]
}

// bump moves one channel of the colour, by -1 or +1.
func (w *colorWindow) bump(idx, delta int) {
	c := w.col
	switch idx {
	case 0:
		c.R = clampByte(int(c.R) + delta)
	case 1:
		c.G = clampByte(int(c.G) + delta)
	default:
		c.B = clampByte(int(c.B) + delta)
	}
	w.setColor(c, false)
	w.refresh()
}

// pointer hands the panel and the hue bar their drags.
func (w *colorWindow) pointer(target int) func(ui.InputEvent) bool {
	return func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			w.drag = target
			w.dragTo(target, ev.X, ev.Y)
			return true
		case ui.InputPointerMove:
			if w.drag != target {
				return false
			}
			w.dragTo(target, ev.X, ev.Y)
			return true
		case ui.InputPointerUp:
			if w.drag != target {
				return false
			}
			w.drag = holdNone
			return true
		}
		return false
	}
}

// dragTo takes the colour under a point of the panel or the hue bar, the
// coordinates being those of that bar.
func (w *colorWindow) dragTo(target int, x, y float32) {
	switch target {
	case holdPanel:
		r := w.panel
		if r.W <= 0 || r.H <= 0 {
			return
		}
		w.applyHSV(hsvColor{H: w.h, S: float64(x) / float64(r.W), V: 1 - float64(y)/float64(r.H)})
	case holdHue:
		r := w.hue
		if r.H <= 0 {
			return
		}
		h := float64(y) / float64(r.H) * 360
		// The bottom of the bar is the top of it, a full turn on.
		if h >= 360 {
			h = 359.9
		}
		w.applyHSV(hsvColor{H: h, S: w.s, V: w.v})
	}
	w.refresh()
}

// refresh asks for a frame after a change the window made itself.
func (w *colorWindow) refresh() {
	if w.win != nil && !w.win.IsDestroyed() {
		w.win.Update(func() {})
	}
}

func (w *colorWindow) copyValue(value, name string) {
	writeClipboard(value)
	w.note = "已复制 " + name + "：" + value
	w.refresh()
}

func (w *colorWindow) hex() string { return hexOf(w.col) }

func (w *colorWindow) rgb() string { return rgbOf(w.col) }

// dec is the colour as one number: 0xRRGGBB.
func (w *colorWindow) dec() string {
	return strconv.Itoa(int(w.col.R)<<16 | int(w.col.G)<<8 | int(w.col.B))
}

func hexOf(c color.RGBA) string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

func rgbOf(c color.RGBA) string {
	return fmt.Sprintf("rgb(%d, %d, %d)", c.R, c.G, c.B)
}

func (w *colorWindow) close() {
	if w.win != nil && !w.win.IsDestroyed() {
		w.win.Destroy()
	}
	if app.colorWin == w {
		app.colorWin = nil
	}
}

// hideForShot takes the window away so that it is not in the screen the
// picker freezes.
func (w *colorWindow) hideForShot() {
	if w.win == nil || w.win.IsDestroyed() || !w.win.IsVisible() {
		return
	}
	w.hidden = true
	w.win.Hide()
}

func (w *colorWindow) restoreAfterShot() {
	if !w.hidden {
		return
	}
	w.hidden = false
	if w.win != nil && !w.win.IsDestroyed() {
		w.win.Show()
	}
}
