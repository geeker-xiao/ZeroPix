// Pin windows show a captured image on top of everything: drag to move,
// wheel to zoom, right-click for copy, recognition, translation and save,
// Esc or a double click to close.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type pinWindow struct {
	img    *image.RGBA
	bmp    *ui.Bitmap
	win    *mygo.Window
	baseW  float32 // the image at 100%, in DIPs
	baseH  float32
	zoom   float32
	scale  float64  // the display's scale factor
	border ui.Color // the frame around the image, in the annotation colour
	radius float32  // the corners of the image at 100%, in DIPs
}

// pinBorder is how wide the frame around a pinned image is, in DIPs.
const pinBorder = 3

// pinImageAt shows img in a new pin window whose top-left goes at (x, y) in
// physical screen pixels. scale is the display's scale factor, border the
// colour of the frame around it, and radius the corners the image was cut
// with, in DIPs at 100%.
func (a *zooApp) pinImageAt(img *image.RGBA, x, y int, scale float64, border ui.Color, radius float32) {
	if scale <= 0 {
		scale = 1
	}
	p := &pinWindow{img: img, zoom: 1, scale: scale, border: border, radius: max(radius, 0)}
	p.baseW = float32(float64(img.Bounds().Dx()) / scale)
	p.baseH = float32(float64(img.Bounds().Dy()) / scale)
	dipX := int(math.Round(float64(x) / scale))
	dipY := int(math.Round(float64(y) / scale))
	// Movable stays on: mygo turns WM_WINDOWPOSCHANGING into SWP_NOMOVE for
	// windows that are not, which drops the placement below back to (0, 0).
	// The frame has no caption, so only DragWindow moves it.
	// The window is transparent so that the corners the image was cut with
	// show the desktop, instead of the square of an opaque background.
	p.win = mygo.NewWindow(mygo.WindowOptions{
		Title:           "贴图",
		X:               dipX,
		Y:               dipY,
		Width:           int(p.winW() + 0.5),
		Height:          int(p.winH() + 0.5),
		Frameless:       true,
		Transparent:     true,
		AlwaysOnTop:     true,
		SkipTaskbar:     true,
		DisableResize:   true,
		DisableMaximize: true,
		DisableMinimize: true,
		DisableShadow:   true,
		Content:         ui.View(p.view),
	})
	if p.win == nil || p.win.IsDestroyed() {
		a.toast("无法创建贴图窗口")
		return
	}
	p.applyRegion()
	a.pins = append(a.pins, p)
}

// applyRegion cuts the window to the corners the image was cut with, at the
// zoom it shows: the window then has no square of its own around them.
func (p *pinWindow) applyRegion() {
	if p.win == nil || p.win.IsDestroyed() {
		return
	}
	if p.radius <= 0 {
		return
	}
	scale := float32(p.scale)
	radius := (p.radius*p.zoom + pinBorder) * scale
	w := int(p.winW()*scale + 0.5)
	h := int(p.winH()*scale + 0.5)
	roundWindow(p.win.NativeHandle(), w, h, radius)
}

// winW and winH are the window's size: the image at the current zoom plus
// the frame around it, which stays the same width at every zoom.
func (p *pinWindow) winW() float32 { return p.baseW*p.zoom + 2*pinBorder }
func (p *pinWindow) winH() float32 { return p.baseH*p.zoom + 2*pinBorder }

func (p *pinWindow) view(c *ui.Context) {
	if p.bmp == nil {
		p.bmp = ui.NewBitmap(p.img)
	}
	c.OnShortcut(0, ui.KeyEscape, p.close)
	// The window draws nothing of its own, so that the corners the image
	// was cut with show the desktop.
	c.Root().Background(ui.Transparent)
	w := p.baseW * p.zoom
	h := p.baseH * p.zoom
	ui.Box(c).Fill().
		Draw(func(paint *ui.Painter, r ui.Rect) { p.paintFrame(paint, r) }).
		Children(func() {
			// The image sits inside the frame, over a window that draws
			// nothing behind it: the corners it was cut with stay clear.
			ui.Box(c).Fill().Padding(pinBorder).HandleInput(p.input).Children(func() {
				ui.Image(c, p.bmp).Width(w).Height(h).Fit(ui.FillBox).DragWindow()
			})
			// A close button stays in the corner, as Snipaste's pin shows on hover.
			ui.ButtonBase(c).Absolute().Right(6).Top(6).Size(22, 22).Radius(4).
				Label("关闭贴图").Tooltip("关闭 (Esc)").
				Background(ui.RGBA(20, 20, 22, 0.68)).
				OnClick(p.close).Children(func() {
				ui.Box(c).Fill().Padding(5).Draw(func(paint *ui.Painter, r ui.Rect) {
					drawIcon(paint, "close", r, ui.RGBA(255, 255, 255, 1))
				})
			})
		})
}

// paintFrame draws the border around the image, round as the corners the
// image was cut with: the line straddles the edge, so it fills the frame
// without spilling outside the window.
func (p *pinWindow) paintFrame(paint *ui.Painter, r ui.Rect) {
	if p.border.A == 0 || r.W <= pinBorder || r.H <= pinBorder {
		return
	}
	half := float32(pinBorder) / 2
	edge := ui.Rect{
		X: r.X + half,
		Y: r.Y + half,
		W: r.W - pinBorder,
		H: r.H - pinBorder,
	}
	radius := p.radius*p.zoom + half
	paint.Stroke(edge, p.border, radius, pinBorder)
}

func (p *pinWindow) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputScroll:
		f := float32(1.12)
		if ev.DY < 0 {
			f = 1 / f
		}
		p.setZoom(p.zoom * f)
		return true
	case ui.InputPointerDown:
		switch ev.Button {
		case 2:
			p.openMenu()
			return true
		case 0:
			if ev.Clicks >= 2 {
				p.close()
				return true
			}
		}
	}
	return false
}

func (p *pinWindow) setZoom(z float32) {
	z = min(max(z, 0.1), 8)
	if z == p.zoom {
		return
	}
	p.zoom = z
	p.win.SetContentSize(int(p.winW()+0.5), int(p.winH()+0.5))
	p.applyRegion()
}

func (p *pinWindow) openMenu() {
	m := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "复制图片", Click: func(*mygo.MenuItem, *mygo.Window) { p.copy() }},
		{Label: "识别文字", Click: func(*mygo.MenuItem, *mygo.Window) { showOCRResult(p.img) }},
		{Label: "翻译文字", Click: func(*mygo.MenuItem, *mygo.Window) { showTranslation(p.img) }},
		{Label: "另存为 PNG…", Click: func(*mygo.MenuItem, *mygo.Window) { go saveImageDialog(p.img) }},
		mygo.Separator(),
		{Label: "放大", Accelerator: "CmdOrCtrl+Up", Click: func(*mygo.MenuItem, *mygo.Window) { p.setZoom(p.zoom * 1.25) }},
		{Label: "缩小", Accelerator: "CmdOrCtrl+Down", Click: func(*mygo.MenuItem, *mygo.Window) { p.setZoom(p.zoom / 1.25) }},
		{Label: "恢复 100%", Click: func(*mygo.MenuItem, *mygo.Window) { p.setZoom(1) }},
		mygo.Separator(),
		{Label: "关闭贴图", Click: func(*mygo.MenuItem, *mygo.Window) { p.close() }},
	})
	m.Popup(p.win)
}

func (p *pinWindow) copy() {
	if err := copyImageToClipboard(p.img); err != nil {
		app.toast("复制失败: " + err.Error())
	}
}

func (p *pinWindow) close() {
	if p.win != nil && !p.win.IsDestroyed() {
		p.win.Destroy()
	}
	app.removePin(p)
}

func (a *zooApp) removePin(p *pinWindow) {
	for i, q := range a.pins {
		if q == p {
			a.pins = append(a.pins[:i], a.pins[i+1:]...)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// Clipboard and file helpers for images.

func encodePNG(img *image.RGBA) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil
	}
	return b.Bytes()
}

// copyImageToClipboard writes img to the clipboard as PNG.
func copyImageToClipboard(img *image.RGBA) error {
	data := encodePNG(img)
	if len(data) == 0 {
		return errors.New("PNG 编码失败")
	}
	return mygo.Clipboard.WriteImage(data)
}

// saveImageDialog asks for a file and writes img there. It blocks on the
// system dialog, so callers run it on a goroutine.
func saveImageDialog(img *image.RGBA) {
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Title:       "保存截图",
		DefaultPath: defaultSavePath(),
		Filters:     []mygo.FileFilter{{Name: "PNG 图片", Extensions: []string{"png"}}},
	})
	if err != nil || path == "" {
		return
	}
	if err := writePNGFile(path, img); err != nil {
		app.toast("保存失败: " + err.Error())
		return
	}
	app.toast("已保存到 " + filepath.Base(path))
}

// defaultSavePath suggests a timestamped name in the user's Pictures folder.
func defaultSavePath() string {
	name := fmt.Sprintf("%s_%s.png", appSlug, time.Now().Format("20060102_150405"))
	home, err := os.UserHomeDir()
	if err != nil {
		return name
	}
	pics := filepath.Join(home, "Pictures")
	if st, err := os.Stat(pics); err == nil && st.IsDir() {
		return filepath.Join(pics, name)
	}
	return filepath.Join(home, name)
}

func writePNGFile(path string, img *image.RGBA) error {
	data := encodePNG(img)
	if len(data) == 0 {
		return errors.New("PNG 编码失败")
	}
	if filepath.Ext(path) == "" {
		path += ".png"
	}
	return os.WriteFile(path, data, 0o644)
}
