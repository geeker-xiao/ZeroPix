// Annotation shapes drawn over the frozen screenshot: outlines, lines,
// freehand strokes, mosaic and blur, text, numbered markers and the
// measuring tools. A shape lives in overlay DIP coordinates and is painted
// twice: live through the mygo painter while the user draws it, and into the
// exported image by the raster helpers below.

package main

import (
	"fmt"
	"image"
	"math"

	"github.com/egoist/mygo/ui"
)

type tool int

const (
	toolSelect tool = iota
	toolRect
	toolEllipse
	toolCircle
	toolArrow
	toolLine
	toolBrush
	toolHighlighter
	toolMosaic
	toolBlur
	toolSpotlight
	toolText
	toolNumber
	toolDimension
	toolRuler
	toolEraser
	toolEyedropper
)

// shape is one annotation. a is where the drag started, b where it ended;
// text and numbered markers use a as their anchor, and a freehand stroke
// keeps every point it went through.
type shape struct {
	kind   tool
	a, b   ui.Point
	pts    []ui.Point // freehand strokes only
	text   string
	font   string // the family a text annotation is set in
	number int
	color  ui.Color
	width  float32
	size   float32

	// The blurred copy of the region, kept between frames so that a still
	// shape costs nothing to paint.
	blur    *ui.Bitmap
	blurKey string
}

func (s *shape) isPoint() bool { return s.kind == toolText || s.kind == toolNumber }

// rect returns the shape's bounds, normalized.
func (s *shape) rect() ui.Rect { return normalizeRect(s.a.X, s.a.Y, s.b.X, s.b.Y) }

// circleRect is the square the circle tool inscribes in the drag.
func (s *shape) circleRect() ui.Rect {
	r := s.rect()
	d := min(r.W, r.H)
	return ui.Rect{X: r.X + (r.W-d)/2, Y: r.Y + (r.H-d)/2, W: d, H: d}
}

// strokeWidth is the width a freehand stroke is drawn at; the highlighter
// writes wide and translucent, as a marker would.
func (s *shape) strokeWidth() float32 {
	if s.kind == toolHighlighter {
		return max(s.width*6, 12)
	}
	return s.width
}

// ink returns the colour a stroke is drawn in.
func (s *shape) ink() ui.Color {
	if s.kind == toolHighlighter {
		return ui.RGBA(s.color.R, s.color.G, s.color.B, 0.34)
	}
	return s.color
}

// ---------------------------------------------------------------------------
// Live painting through the mygo painter.

// paintShapes draws shapes over the overlay. kx and ky map DIPs to snapshot
// pixels, which the mosaic and the blur need to sample the screen
// underneath. bounds is the chosen region, which the spotlight darkens.
func paintShapes(p *ui.Painter, shapes []*shape, path *ui.Path, shot *image.RGBA, kx, ky float64, bounds ui.Rect) {
	for _, sh := range shapes {
		paintShape(p, sh, path, shot, kx, ky, bounds)
	}
}

func paintShape(p *ui.Painter, sh *shape, path *ui.Path, shot *image.RGBA, kx, ky float64, bounds ui.Rect) {
	switch sh.kind {
	case toolRect:
		r := sh.rect()
		if r.W < 1 || r.H < 1 {
			return
		}
		p.Stroke(r, sh.color, 0, sh.width)
	case toolEllipse:
		paintEllipseRect(p, sh, path, sh.rect())
	case toolCircle:
		paintEllipseRect(p, sh, path, sh.circleRect())
	case toolArrow:
		paintArrow(p, sh, path)
	case toolLine:
		paintLine(p, sh, path)
	case toolBrush, toolHighlighter:
		paintStroke(p, sh, path)
	case toolMosaic:
		paintMosaic(p, sh, shot, kx, ky)
	case toolBlur:
		paintBlur(p, sh, shot, kx, ky)
	case toolSpotlight:
		paintSpotlight(p, sh, bounds)
	case toolText:
		paintText(p, sh)
	case toolNumber:
		paintBadge(p, sh)
	case toolDimension:
		paintDimension(p, sh, kx)
	case toolRuler:
		paintRuler(p, sh, kx)
	}
}

// paintText draws a text annotation in its own family, so that the overlay
// shows what the export will.
func paintText(p *ui.Painter, sh *shape) {
	if sh.font == "" {
		p.Text(sh.a.X, sh.a.Y, sh.text, sh.size, sh.color)
		return
	}
	p.TextLayout(ui.ShapeText(sh.text, ui.Font{Family: sh.font, Size: sh.size}, 0),
		sh.a.X, sh.a.Y, sh.color)
}

// ellipseKappa turns the four cubic segments of an ellipse into the shape
// a circle of the same radius would be: 4/3·(√2−1).
const ellipseKappa = 0.5522848

// ellipsePath builds the outline of the ellipse inscribed in r.
func ellipsePath(path *ui.Path, r ui.Rect) *ui.Path {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	rx, ry := r.W/2, r.H/2
	kx, ky := rx*ellipseKappa, ry*ellipseKappa
	path.Reset()
	path.MoveTo(cx, cy-ry)
	path.CubeTo(cx+kx, cy-ry, cx+rx, cy-ky, cx+rx, cy)
	path.CubeTo(cx+rx, cy+ky, cx+kx, cy+ry, cx, cy+ry)
	path.CubeTo(cx-kx, cy+ry, cx-rx, cy+ky, cx-rx, cy)
	path.CubeTo(cx-rx, cy-ky, cx-kx, cy-ry, cx, cy-ry)
	path.Close()
	return path
}

func paintEllipseRect(p *ui.Painter, sh *shape, path *ui.Path, r ui.Rect) {
	if r.W < 1 || r.H < 1 || path == nil {
		return
	}
	p.StrokePath(ellipsePath(path, r), sh.width, sh.color)
}

// strokePath builds the polyline a freehand stroke went through.
func strokePath(path *ui.Path, sh *shape) *ui.Path {
	pts := sh.pts
	if len(pts) < 2 {
		pts = []ui.Point{sh.a, sh.b}
	}
	path.Reset()
	path.MoveTo(pts[0].X, pts[0].Y)
	for _, pt := range pts[1:] {
		path.LineTo(pt.X, pt.Y)
	}
	return path
}

func paintStroke(p *ui.Painter, sh *shape, path *ui.Path) {
	if path == nil {
		return
	}
	w := sh.strokeWidth()
	pts := sh.pts
	if len(pts) == 1 {
		// A click with the brush is a dot.
		p.Fill(ui.Rect{X: pts[0].X - w/2, Y: pts[0].Y - w/2, W: w, H: w}, sh.ink(), w/2)
		return
	}
	p.StrokePath(strokePath(path, sh), w, sh.ink())
}

func paintLine(p *ui.Painter, sh *shape, path *ui.Path) {
	if path == nil || math.Hypot(float64(sh.b.X-sh.a.X), float64(sh.b.Y-sh.a.Y)) < 1 {
		return
	}
	path.Reset()
	path.MoveTo(sh.a.X, sh.a.Y)
	path.LineTo(sh.b.X, sh.b.Y)
	p.StrokePath(path, sh.width, sh.color)
}

func paintArrow(p *ui.Painter, sh *shape, path *ui.Path) {
	dx, dy := sh.b.X-sh.a.X, sh.b.Y-sh.a.Y
	l := float32(math.Hypot(float64(dx), float64(dy)))
	if l < 2 || path == nil {
		return
	}
	ux, uy := dx/l, dy/l
	head := min(l*0.6, max(sh.width*4.2, 11))
	hw := head * 0.42
	baseX, baseY := sh.b.X-ux*head, sh.b.Y-uy*head
	// The shaft stops inside the head.
	path.Reset()
	path.MoveTo(sh.a.X, sh.a.Y)
	path.LineTo(baseX+ux*head*0.45, baseY+uy*head*0.45)
	p.StrokePath(path, sh.width, sh.color)
	// A solid triangular head, as Snipaste draws.
	path.Reset()
	path.MoveTo(sh.b.X, sh.b.Y)
	path.LineTo(baseX-uy*hw, baseY+ux*hw)
	path.LineTo(baseX+uy*hw, baseY-ux*hw)
	path.Close()
	p.FillPath(path, sh.color)
}

func paintBadge(p *ui.Painter, sh *shape) {
	d := sh.size * 1.7
	r := ui.Rect{X: sh.a.X - d/2, Y: sh.a.Y - d/2, W: d, H: d}
	p.Fill(r, sh.color, d/2)
	label := itoa(sh.number)
	tw := measureText(label, float64(sh.size))
	p.Text(sh.a.X-float32(tw)/2, sh.a.Y-sh.size*0.62, label, sh.size, ui.RGBA(255, 255, 255, 1))
}

// paintMosaic pixelates the region from the snapshot, block by block, which
// matches what the export does to the cropped pixels.
func paintMosaic(p *ui.Painter, sh *shape, shot *image.RGBA, kx, ky float64) {
	r := sh.rect()
	if r.W < 2 || r.H < 2 || shot == nil {
		return
	}
	block := max(sh.size, 4)
	// Keep the number of blocks sane on a big selection.
	for float64(r.W/block)*float64(r.H/block) > 9000 {
		block *= 1.4
	}
	sb := shot.Bounds()
	rad := max(1, int(float64(block)*kx/2))
	for y := r.Y; y < r.Y+r.H; y += block {
		for x := r.X; x < r.X+r.W; x += block {
			sx := int(float64(x+block/2) * kx)
			sy := int(float64(y+block/2) * ky)
			if sx < sb.Min.X || sx >= sb.Max.X || sy < sb.Min.Y || sy >= sb.Max.Y {
				continue
			}
			p.Fill(ui.Rect{X: x, Y: y, W: block + 0.4, H: block + 0.4},
				avgBlock(shot, sx, sy, rad), 0)
		}
	}
}

// blurRadius is how far the blur reaches, in DIPs.
const blurRadius = 7

// paintBlur softens the region: the snapshot underneath is blurred once and
// kept, so a shape that is not being dragged costs one image blit a frame.
func paintBlur(p *ui.Painter, sh *shape, shot *image.RGBA, kx, ky float64) {
	r := sh.rect()
	if r.W < 2 || r.H < 2 || shot == nil {
		return
	}
	key := fmt.Sprintf("%.0f:%.0f:%.0f:%.0f", r.X, r.Y, r.W, r.H)
	if sh.blur == nil || sh.blurKey != key {
		sh.blur = blurBitmap(shot, r, kx, ky)
		sh.blurKey = key
	}
	if sh.blur != nil {
		p.Image(sh.blur, r, ui.FillBox)
		return
	}
	// A copy taken at the very edge can come back empty; blocks still hide
	// what is underneath.
	paintMosaic(p, &shape{a: sh.a, b: sh.b, size: 8}, shot, kx, ky)
}

// paintSpotlight dims everything but the region, so that the eye goes to the
// part being explained.
func paintSpotlight(p *ui.Painter, sh *shape, bounds ui.Rect) {
	r := sh.rect()
	if r.W < 1 || r.H < 1 || bounds.W < 1 || bounds.H < 1 {
		return
	}
	dark := ui.RGBA(0, 0, 0, 0.55)
	if top := max(r.Y-bounds.Y, 0); top > 0 {
		p.Fill(ui.Rect{X: bounds.X, Y: bounds.Y, W: bounds.W, H: top}, dark, 0)
	}
	bottom := max(bounds.Y+bounds.H-r.Y-r.H, 0)
	if bottom > 0 {
		p.Fill(ui.Rect{X: bounds.X, Y: r.Y + r.H, W: bounds.W, H: bottom}, dark, 0)
	}
	if left := max(r.X-bounds.X, 0); left > 0 {
		p.Fill(ui.Rect{X: bounds.X, Y: r.Y, W: left, H: r.H}, dark, 0)
	}
	right := max(bounds.X+bounds.W-r.X-r.W, 0)
	if right > 0 {
		p.Fill(ui.Rect{X: r.X + r.W, Y: r.Y, W: right, H: r.H}, dark, 0)
	}
}

// paintDimension draws a line with a tick at each end and the distance in
// physical pixels above it.
func paintDimension(p *ui.Painter, sh *shape, kx float64) {
	dx, dy := sh.b.X-sh.a.X, sh.b.Y-sh.a.Y
	l := math.Hypot(float64(dx), float64(dy))
	if l < 2 {
		return
	}
	ux, uy := float32(float64(dx)/l), float32(float64(dy)/l)
	tx, ty := -uy, ux // across the line
	const tick = 7
	paintSeg(p, sh.a.X-tx*tick, sh.a.Y-ty*tick, sh.a.X+tx*tick, sh.a.Y+ty*tick, sh.width, sh.color)
	paintSeg(p, sh.b.X-tx*tick, sh.b.Y-ty*tick, sh.b.X+tx*tick, sh.b.Y+ty*tick, sh.width, sh.color)
	paintSeg(p, sh.a.X, sh.a.Y, sh.b.X, sh.b.Y, sh.width, sh.color)

	label := fmt.Sprintf("%d px", int(math.Round(l*kx)))
	mx, my := (sh.a.X+sh.b.X)/2, (sh.a.Y+sh.b.Y)/2
	tw := float32(measureText(label, 11.5))
	box := ui.Rect{X: mx - tw/2 - 5, Y: my - 20, W: tw + 10, H: 17}
	p.Fill(box, ui.RGBA(17, 17, 17, 0.82), 3)
	p.Text(box.X+5, box.Y+2, label, 11.5, ui.RGBA(255, 255, 255, 1))
}

// paintSeg strokes one straight segment; the painter has no line primitive,
// so a two-point path does the work.
func paintSeg(p *ui.Painter, x0, y0, x1, y1, w float32, c ui.Color) {
	path := &ui.Path{}
	path.MoveTo(x0, y0)
	path.LineTo(x1, y1)
	p.StrokePath(path, w, c)
}

// rulerThickness is the width of the ruler's bar, in DIPs.
const rulerThickness = 24

// rulerBar is the bar the ruler draws along the drag, and whether it lies
// horizontally.
func rulerBar(sh *shape) (ui.Rect, bool) {
	r := sh.rect()
	if r.W >= r.H {
		return ui.Rect{X: r.X, Y: sh.a.Y, W: r.W, H: rulerThickness}, true
	}
	return ui.Rect{X: sh.a.X, Y: r.Y, W: rulerThickness, H: r.H}, false
}

// paintRuler draws a translucent ruler with a tick every ten and a label
// every hundred physical pixels.
func paintRuler(p *ui.Painter, sh *shape, k float64) {
	bar, horiz := rulerBar(sh)
	if bar.W < 8 || bar.H < 8 {
		return
	}
	p.Fill(bar, ui.RGBA(255, 255, 255, 0.92), 3)
	p.Stroke(bar, ui.RGBA(0, 0, 0, 0.30), 3, 1)
	paintRulerTicks(p, bar, horiz, k, func(x, y float32, s string, size float32) {
		p.Text(x, y, s, size, ui.RGBA(40, 40, 40, 1))
	})
}

// paintRulerTicks marks the bar: a short tick every ten pixels, a long one
// and a number every hundred.
func paintRulerTicks(p *ui.Painter, bar ui.Rect, horiz bool, k float64, text func(x, y float32, s string, size float32)) {
	step := float32(10 / k)
	if step < 2 {
		step = 2
	}
	length := bar.W
	if !horiz {
		length = bar.H
	}
	i := 0
	for off := float32(0); off <= length; off += step {
		major := i%5 == 0
		h := float32(6)
		if major {
			h = 12
		}
		var tick ui.Rect
		if horiz {
			tick = ui.Rect{X: bar.X + off, Y: bar.Y, W: 1, H: h}
		} else {
			tick = ui.Rect{X: bar.X, Y: bar.Y + off, W: h, H: 1}
		}
		p.Fill(tick, ui.RGBA(20, 20, 20, 0.75), 0)
		if i%10 == 0 {
			label := itoa(i * 10)
			if horiz {
				text(bar.X+off+2, bar.Y+bar.H-13, label, 9.5)
			} else {
				text(bar.X+12, bar.Y+off+1, label, 9.5)
			}
		}
		i++
	}
}

// avgBlock averages the pixels around (x, y) in the snapshot for a mosaic.
func avgBlock(img *image.RGBA, x, y, rad int) ui.Color {
	b := img.Bounds()
	var r, g, bl, n int
	for yy := y - rad; yy <= y+rad; yy++ {
		if yy < b.Min.Y || yy >= b.Max.Y {
			continue
		}
		for xx := x - rad; xx <= x+rad; xx++ {
			if xx < b.Min.X || xx >= b.Max.X {
				continue
			}
			i := img.PixOffset(xx, yy)
			r += int(img.Pix[i])
			g += int(img.Pix[i+1])
			bl += int(img.Pix[i+2])
			n++
		}
	}
	if n == 0 {
		return ui.RGBA(0, 0, 0, 1)
	}
	return ui.RGBA(uint8(r/n), uint8(g/n), uint8(bl/n), 1)
}

// blurBitmap copies the region out of the snapshot, blurs it and hands back
// a bitmap the painter can blit in its place.
func blurBitmap(shot *image.RGBA, r ui.Rect, kx, ky float64) *ui.Bitmap {
	x0 := int(math.Floor(float64(r.X) * kx))
	y0 := int(math.Floor(float64(r.Y) * ky))
	x1 := int(math.Ceil(float64(r.X+r.W) * kx))
	y1 := int(math.Ceil(float64(r.Y+r.H) * ky))
	crop := cropRGBA(shot, x0, y0, x1, y1)
	if crop == nil {
		return nil
	}
	boxBlurRGBA(crop, blurRadiusPx(kx))
	return ui.NewBitmap(crop)
}

// blurRadiusPx is how far the blur reaches in snapshot pixels.
func blurRadiusPx(k float64) int {
	return max(2, int(math.Round(blurRadius*k/2)))
}

// boxBlurRGBA blurs an image in place with two box passes, which is close
// enough to a gaussian for a redaction.
func boxBlurRGBA(img *image.RGBA, radius int) {
	if radius < 1 {
		return
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 2 || h < 2 {
		return
	}
	radius = min(radius, max(w, h)/2)
	if radius < 1 {
		return
	}
	tmp := image.NewRGBA(image.Rect(0, 0, w, h))
	pass := func(src, dst *image.RGBA, horizontal bool) {
		n := 2*radius + 1
		if horizontal {
			for y := 0; y < h; y++ {
				var sr, sg, sb int
				for k := -radius; k <= radius; k++ {
					i := src.PixOffset(clampInt(k, 0, w-1), y)
					sr += int(src.Pix[i])
					sg += int(src.Pix[i+1])
					sb += int(src.Pix[i+2])
				}
				for x := 0; x < w; x++ {
					j := dst.PixOffset(x, y)
					dst.Pix[j] = uint8(sr / n)
					dst.Pix[j+1] = uint8(sg / n)
					dst.Pix[j+2] = uint8(sb / n)
					dst.Pix[j+3] = 255
					out := src.PixOffset(clampInt(x-radius, 0, w-1), y)
					in := src.PixOffset(clampInt(x+radius+1, 0, w-1), y)
					sr += int(src.Pix[in]) - int(src.Pix[out])
					sg += int(src.Pix[in+1]) - int(src.Pix[out+1])
					sb += int(src.Pix[in+2]) - int(src.Pix[out+2])
				}
			}
			return
		}
		for x := 0; x < w; x++ {
			var sr, sg, sb int
			for k := -radius; k <= radius; k++ {
				i := src.PixOffset(x, clampInt(k, 0, h-1))
				sr += int(src.Pix[i])
				sg += int(src.Pix[i+1])
				sb += int(src.Pix[i+2])
			}
			for y := 0; y < h; y++ {
				j := dst.PixOffset(x, y)
				dst.Pix[j] = uint8(sr / n)
				dst.Pix[j+1] = uint8(sg / n)
				dst.Pix[j+2] = uint8(sb / n)
				dst.Pix[j+3] = 255
				out := src.PixOffset(x, clampInt(y-radius, 0, h-1))
				in := src.PixOffset(x, clampInt(y+radius+1, 0, h-1))
				sr += int(src.Pix[in]) - int(src.Pix[out])
				sg += int(src.Pix[in+1]) - int(src.Pix[out+1])
				sb += int(src.Pix[in+2]) - int(src.Pix[out+2])
			}
		}
	}
	pass(img, tmp, true)
	pass(tmp, img, false)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------------------------------------------------------------------------
// Hit testing, for the eraser.

// shapeHit reports whether the eraser at (x, y) touches the shape.
func shapeHit(sh *shape, x, y float32) bool {
	switch sh.kind {
	case toolText:
		w := float32(measureText(sh.text, float64(sh.size)))
		return pointInRect(x, y, ui.Rect{X: sh.a.X, Y: sh.a.Y - sh.size, W: w, H: sh.size * 1.6})
	case toolNumber:
		return math.Hypot(float64(x-sh.a.X), float64(y-sh.a.Y)) <= float64(sh.size)
	case toolBrush, toolHighlighter:
		pts := sh.pts
		if len(pts) < 2 {
			pts = []ui.Point{sh.a, sh.b}
		}
		w := sh.strokeWidth()/2 + 3
		for i := 1; i < len(pts); i++ {
			if segDist(float64(x), float64(y),
				float64(pts[i-1].X), float64(pts[i-1].Y),
				float64(pts[i].X), float64(pts[i].Y)) <= float64(w) {
				return true
			}
		}
		return false
	case toolLine, toolArrow, toolDimension:
		return segDist(float64(x), float64(y), float64(sh.a.X), float64(sh.a.Y),
			float64(sh.b.X), float64(sh.b.Y)) <= float64(sh.width/2+5)
	case toolCircle, toolEllipse:
		r := sh.rect()
		if sh.kind == toolCircle {
			r = sh.circleRect()
		}
		// On the outline, or close to it.
		if r.W < 1 || r.H < 1 {
			return false
		}
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		f := (x-cx)*(x-cx)/((r.W/2)*(r.W/2)) + (y-cy)*(y-cy)/((r.H/2)*(r.H/2))
		return math.Abs(float64(f-1)) < 0.22
	case toolRuler:
		bar, _ := rulerBar(sh)
		return pointInRect(x, y, growRect(bar, 2))
	default:
		return pointInRect(x, y, growRect(sh.rect(), 3))
	}
}

// ---------------------------------------------------------------------------
// Export rasterization.

// rasterShapes draws shapes onto dst, which holds the selection cropped at
// physical pixel scale. ox/oy are the selection's top-left in DIPs, k the
// physical pixels per DIP.
func rasterShapes(dst *image.RGBA, shapes []*shape, ox, oy float32, k float64) {
	if dst == nil {
		return
	}
	// In the order they were drawn, as the overlay paints them: a spotlight
	// dims what is under it at the moment it is laid down.
	for _, sh := range shapes {
		rasterShape(dst, sh, ox, oy, k)
	}
}

func rasterShape(dst *image.RGBA, sh *shape, ox, oy float32, k float64) {
	toX := func(v float32) float64 { return float64(v-ox) * k }
	toY := func(v float32) float64 { return float64(v-oy) * k }
	col := rgbaColor(sh.color)
	switch sh.kind {
	case toolRect:
		strokeRect(dst, toX(sh.a.X), toY(sh.a.Y), toX(sh.b.X), toY(sh.b.Y),
			float64(sh.width)*k, col)
	case toolEllipse:
		r := sh.rect()
		strokeEllipse(dst,
			toX(r.X)+float64(r.W)*k/2, toY(r.Y)+float64(r.H)*k/2,
			float64(r.W)*k/2, float64(r.H)*k/2,
			float64(sh.width)*k, col)
	case toolCircle:
		r := sh.circleRect()
		strokeEllipse(dst,
			toX(r.X)+float64(r.W)*k/2, toY(r.Y)+float64(r.H)*k/2,
			float64(r.W)*k/2, float64(r.H)*k/2,
			float64(sh.width)*k, col)
	case toolArrow:
		rasterArrow(dst, toX(sh.a.X), toY(sh.a.Y), toX(sh.b.X), toY(sh.b.Y),
			float64(sh.width)*k, k, col)
	case toolLine:
		strokeLine(dst, toX(sh.a.X), toY(sh.a.Y), toX(sh.b.X), toY(sh.b.Y),
			float64(sh.width)*k, col)
	case toolBrush, toolHighlighter:
		rasterStroke(dst, sh, toX, toY, k)
	case toolMosaic:
		applyMosaic(dst, toX(sh.a.X), toY(sh.a.Y), toX(sh.b.X), toY(sh.b.Y),
			float64(sh.size)*k)
	case toolBlur:
		r := sh.rect()
		applyBlur(dst, toX(r.X), toY(r.Y), toX(r.X+r.W), toY(r.Y+r.H), float64(blurRadiusPx(k)))
	case toolText:
		drawText(dst, int(toX(sh.a.X)), int(toY(sh.a.Y)), sh.text,
			float64(sh.size)*k, sh.font, col)
	case toolNumber:
		size := float64(sh.size) * k
		d := size * 1.7
		cx, cy := toX(sh.a.X), toY(sh.a.Y)
		fillDisc(dst, cx, cy, d/2, col)
		label := itoa(sh.number)
		tw := measureText(label, size)
		drawText(dst, int(cx-tw/2), int(cy-size*0.62), label, size, "",
			colorRGBA{R: 255, G: 255, B: 255, A: 255})
	case toolDimension:
		rasterDimension(dst, sh, ox, oy, k)
	case toolRuler:
		rasterRuler(dst, sh, ox, oy, k)
	case toolSpotlight:
		rasterSpotlight(dst, sh, ox, oy, k)
	}
}

// rasterStroke draws a freehand stroke, keeping the highlighter see-through.
func rasterStroke(dst *image.RGBA, sh *shape, toX, toY func(float32) float64, k float64) {
	c := rgbaColor(sh.color)
	if sh.kind == toolHighlighter {
		c.A = 87 // about a third
	}
	w := float64(sh.strokeWidth()) * k
	pts := sh.pts
	if len(pts) < 2 {
		pts = []ui.Point{sh.a, sh.b}
	}
	if len(pts) == 1 {
		fillDisc(dst, toX(pts[0].X), toY(pts[0].Y), w/2, c)
		return
	}
	for i := 1; i < len(pts); i++ {
		strokeLine(dst, toX(pts[i-1].X), toY(pts[i-1].Y), toX(pts[i].X), toY(pts[i].Y), w, c)
	}
}

// rasterDimension draws the measuring line: the ticks, the line and the
// number over it.
func rasterDimension(dst *image.RGBA, sh *shape, ox, oy float32, k float64) {
	x0, y0 := float64(sh.a.X-ox)*k, float64(sh.a.Y-oy)*k
	x1, y1 := float64(sh.b.X-ox)*k, float64(sh.b.Y-oy)*k
	l := math.Hypot(x1-x0, y1-y0)
	if l < 2 {
		return
	}
	ux, uy := (x1-x0)/l, (y1-y0)/l
	tx, ty := -uy*7*k, ux*7*k
	w := float64(sh.width) * k
	c := rgbaColor(sh.color)
	strokeLine(dst, x0-tx, y0-ty, x0+tx, y0+ty, w, c)
	strokeLine(dst, x1-tx, y1-ty, x1+tx, y1+ty, w, c)
	strokeLine(dst, x0, y0, x1, y1, w, c)

	label := fmt.Sprintf("%d px", int(math.Round(l)))
	tw := measureText(label, 11.5*k)
	mx, my := (x0+x1)/2, (y0+y1)/2
	box := image.Rect(int(mx-tw/2-5*k), int(my-20*k), int(mx+tw/2+5*k), int(my-3*k))
	fillRectPx(dst, box, colorRGBA{R: 17, G: 17, B: 17, A: 209})
	drawText(dst, int(mx-tw/2), int(my-18.5*k), label, 11.5*k, "",
		colorRGBA{R: 255, G: 255, B: 255, A: 255})
}

// rasterRuler draws the ruler bar with its ticks.
func rasterRuler(dst *image.RGBA, sh *shape, ox, oy float32, k float64) {
	bar, horiz := rulerBar(sh)
	x0, y0 := float64(bar.X-ox)*k, float64(bar.Y-oy)*k
	x1, y1 := float64(bar.X+bar.W-ox)*k, float64(bar.Y+bar.H-oy)*k
	if x1-x0 < 4 || y1-y0 < 4 {
		return
	}
	fillRectPx(dst, image.Rect(int(x0), int(y0), int(x1), int(y1)),
		colorRGBA{R: 255, G: 255, B: 255, A: 235})
	strokeRect(dst, x0, y0, x1, y1, 1, colorRGBA{R: 0, G: 0, B: 0, A: 76})
	step := 10.0
	length := x1 - x0
	if !horiz {
		length = y1 - y0
	}
	i := 0
	for off := 0.0; off <= length; off += step {
		major := i%5 == 0
		h := 6 * k
		if major {
			h = 12 * k
		}
		if horiz {
			strokeLine(dst, x0+off, y0, x0+off, y0+h, 1, colorRGBA{A: 190, R: 20, G: 20, B: 20})
		} else {
			strokeLine(dst, x0, y0+off, x0+h, y0+off, 1, colorRGBA{A: 190, R: 20, G: 20, B: 20})
		}
		if i%10 == 0 {
			label := itoa(i * 10)
			if horiz {
				drawText(dst, int(x0+off+2*k), int(y1-13*k), label, 9.5*k, "",
					colorRGBA{R: 40, G: 40, B: 40, A: 255})
			} else {
				drawText(dst, int(x0+12*k), int(y0+off+1*k), label, 9.5*k, "",
					colorRGBA{R: 40, G: 40, B: 40, A: 255})
			}
		}
		i++
	}
}

// rasterSpotlight darkens the whole crop but the region of the shape.
func rasterSpotlight(dst *image.RGBA, sh *shape, ox, oy float32, k float64) {
	r := sh.rect()
	x0 := int(math.Round(float64(r.X-ox) * k))
	y0 := int(math.Round(float64(r.Y-oy) * k))
	x1 := int(math.Round(float64(r.X+r.W-ox) * k))
	y1 := int(math.Round(float64(r.Y+r.H-oy) * k))
	dark := colorRGBA{R: 0, G: 0, B: 0, A: 140}
	b := dst.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if x >= x0 && x < x1 && y >= y0 && y < y1 {
				continue
			}
			setPix(dst, x, y, dark)
		}
	}
}

// colorRGBA is an rgb color with straight alpha, in the raster helpers.
type colorRGBA struct{ R, G, B, A uint8 }

func rgbaColor(c ui.Color) colorRGBA {
	return colorRGBA{R: c.R, G: c.G, B: c.B, A: c.A}
}

// setPix blends c over the pixel at (x, y).
func setPix(img *image.RGBA, x, y int, c colorRGBA) {
	b := img.Bounds()
	if x < b.Min.X || x >= b.Max.X || y < b.Min.Y || y >= b.Max.Y {
		return
	}
	i := img.PixOffset(x, y)
	if c.A == 255 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2] = c.R, c.G, c.B
		img.Pix[i+3] = 255
		return
	}
	a := float64(c.A) / 255
	d := img.Pix
	d[i] = uint8(float64(c.R)*a + float64(d[i])*(1-a))
	d[i+1] = uint8(float64(c.G)*a + float64(d[i+1])*(1-a))
	d[i+2] = uint8(float64(c.B)*a + float64(d[i+2])*(1-a))
	d[i+3] = 255
}

// fillRectPx fills an axis-aligned rectangle with c.
func fillRectPx(img *image.RGBA, r image.Rectangle, c colorRGBA) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			setPix(img, x, y, c)
		}
	}
}

// strokeRect outlines the rectangle (x0, y0)-(x1, y1) with a line w wide,
// antialiased by sampling the distance to the outline.
func strokeRect(img *image.RGBA, x0, y0, x1, y1, w float64, c colorRGBA) {
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	half := max(w, 1) / 2
	cx0, cx1 := x0-half, x1+half
	cy0, cy1 := y0-half, y1+half
	for y := int(math.Floor(cy0)) - 1; y <= int(math.Ceil(cy1))+1; y++ {
		for x := int(math.Floor(cx0)) - 1; x <= int(math.Ceil(cx1))+1; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			dx := max(cx0-px, px-cx1, 0)
			dy := max(cy0-py, py-cy1, 0)
			if dx*dx+dy*dy <= half*half {
				// The inside of the rectangle stays clear.
				if px > x0+half && px < x1-half && py > y0+half && py < y1-half {
					continue
				}
				setPix(img, x, y, c)
			}
		}
	}
}

// strokeEllipse outlines the ellipse centered at (cx, cy) with the radii rx
// and ry, in a line w wide. The distance to the outline comes from the
// gradient of its implicit equation, which is close enough for the widths
// an annotation uses.
func strokeEllipse(img *image.RGBA, cx, cy, rx, ry, w float64, c colorRGBA) {
	if rx < 0.5 || ry < 0.5 {
		return
	}
	half := max(w, 1) / 2
	minX := int(math.Floor(cx - rx - half - 1))
	maxX := int(math.Ceil(cx + rx + half + 1))
	minY := int(math.Floor(cy - ry - half - 1))
	maxY := int(math.Ceil(cy + ry + half + 1))
	a2, b2 := rx*rx, ry*ry
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x)+0.5-cx, float64(y)+0.5-cy
			f := px*px/a2 + py*py/b2 - 1
			gx, gy := 2*px/a2, 2*py/b2
			grad := math.Hypot(gx, gy)
			if grad < 1e-9 {
				continue
			}
			if math.Abs(f)/grad <= half {
				setPix(img, x, y, c)
			}
		}
	}
}

func strokeLine(img *image.RGBA, x0, y0, x1, y1, w float64, c colorRGBA) {
	half := max(w, 1) / 2
	minX := int(math.Floor(math.Min(x0, x1) - half - 1))
	maxX := int(math.Ceil(math.Max(x0, x1) + half + 1))
	minY := int(math.Floor(math.Min(y0, y1) - half - 1))
	maxY := int(math.Ceil(math.Max(y0, y1) + half + 1))
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			if segDist(float64(x)+0.5, float64(y)+0.5, x0, y0, x1, y1) <= half {
				setPix(img, x, y, c)
			}
		}
	}
}

func segDist(px, py, x0, y0, x1, y1 float64) float64 {
	dx, dy := x1-x0, y1-y0
	l2 := dx*dx + dy*dy
	if l2 == 0 {
		return math.Hypot(px-x0, py-y0)
	}
	t := ((px-x0)*dx + (py-y0)*dy) / l2
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(x0+t*dx), py-(y0+t*dy))
}

// rasterArrow draws the shaft and the solid head of an arrow, in pixels.
func rasterArrow(img *image.RGBA, x0, y0, x1, y1, w, k float64, c colorRGBA) {
	dx, dy := x1-x0, y1-y0
	l := math.Hypot(dx, dy)
	if l < 2 {
		return
	}
	ux, uy := dx/l, dy/l
	head := math.Min(l*0.6, math.Max(w*4.2, 11*k))
	hw := head * 0.42
	bx, by := x1-ux*head, y1-uy*head
	strokeLine(img, x0, y0, bx+ux*head*0.45, by+uy*head*0.45, w, c)
	fillTriangle(img, x1, y1,
		bx-uy*hw, by+ux*hw,
		bx+uy*hw, by-ux*hw, c)
}

// fillTriangle fills the triangle (x0, y0), (x1, y1), (x2, y2).
func fillTriangle(img *image.RGBA, x0, y0, x1, y1, x2, y2 float64, c colorRGBA) {
	minX := int(math.Floor(math.Min(x0, math.Min(x1, x2)))) - 1
	maxX := int(math.Ceil(math.Max(x0, math.Max(x1, x2)))) + 1
	minY := int(math.Floor(math.Min(y0, math.Min(y1, y2)))) - 1
	maxY := int(math.Ceil(math.Max(y0, math.Max(y1, y2)))) + 1
	edge := func(ax, ay, bx, by, px, py float64) float64 {
		return (bx-ax)*(py-ay) - (by-ay)*(px-ax)
	}
	area := edge(x0, y0, x1, y1, x2, y2)
	if area == 0 {
		return
	}
	for y := minY; y <= maxY; y++ {
		for x := minX; x <= maxX; x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			e0 := edge(x0, y0, x1, y1, px, py) / area
			e1 := edge(x1, y1, x2, y2, px, py) / area
			e2 := edge(x2, y2, x0, y0, px, py) / area
			if e0 >= -0.02 && e1 >= -0.02 && e2 >= -0.02 {
				setPix(img, x, y, c)
			}
		}
	}
}

func fillDisc(img *image.RGBA, cx, cy, rad float64, c colorRGBA) {
	for y := int(math.Floor(cy - rad)); y <= int(math.Ceil(cy+rad)); y++ {
		for x := int(math.Floor(cx - rad)); x <= int(math.Ceil(cx+rad)); x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			if dx*dx+dy*dy <= rad*rad {
				setPix(img, x, y, c)
			}
		}
	}
}

// applyMosaic pixelates the rectangle of dst in place from its own pixels.
func applyMosaic(img *image.RGBA, x0, y0, x1, y1, block float64) {
	b := img.Bounds()
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	block = math.Max(block, 4)
	xa, xb := int(math.Floor(x0)), int(math.Ceil(x1))
	ya, yb := int(math.Floor(y0)), int(math.Ceil(y1))
	xa, xb = max(xa, b.Min.X), min(xb, b.Max.X)
	ya, yb = max(ya, b.Min.Y), min(yb, b.Max.Y)
	step := int(block)
	for by := ya; by < yb; by += step {
		for bx := xa; bx < xb; bx += step {
			var sr, sg, sb, n int
			ex := min(bx+step, xb)
			ey := min(by+step, yb)
			for y := by; y < ey; y++ {
				for x := bx; x < ex; x++ {
					i := img.PixOffset(x, y)
					sr += int(img.Pix[i])
					sg += int(img.Pix[i+1])
					sb += int(img.Pix[i+2])
					n++
				}
			}
			if n == 0 {
				continue
			}
			col := colorRGBA{R: uint8(sr / n), G: uint8(sg / n), B: uint8(sb / n), A: 255}
			for y := by; y < ey; y++ {
				for x := bx; x < ex; x++ {
					setPix(img, x, y, col)
				}
			}
		}
	}
}

// applyBlur blurs a rectangle of dst in place, for the blur annotation.
func applyBlur(img *image.RGBA, x0, y0, x1, y1, radius float64) {
	if x1 < x0 {
		x0, x1 = x1, x0
	}
	if y1 < y0 {
		y0, y1 = y1, y0
	}
	b := img.Bounds()
	xa, xb := int(math.Floor(x0)), int(math.Ceil(x1))
	ya, yb := int(math.Floor(y0)), int(math.Ceil(y1))
	xa, xb = max(xa, b.Min.X), min(xb, b.Max.X)
	ya, yb = max(ya, b.Min.Y), min(yb, b.Max.Y)
	sub := cropRGBA(img, xa, ya, xb, yb)
	if sub == nil {
		return
	}
	boxBlurRGBA(sub, int(radius))
	for y := ya; y < yb; y++ {
		for x := xa; x < xb; x++ {
			i := img.PixOffset(x, y)
			j := sub.PixOffset(x-xa, y-ya)
			img.Pix[i], img.Pix[i+1], img.Pix[i+2] = sub.Pix[j], sub.Pix[j+1], sub.Pix[j+2]
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
