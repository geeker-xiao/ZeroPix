// PP-OCRv5 mobile OCR (detection + recognition) running locally through
// onnxruntime. Models and the dictionary are looked up in the ocr/ folder
// next to the executable or the working directory.

package main

import (
	"fmt"
	"image"
	"math"
	"os"
	"strings"
	"sync"

	oni "github.com/yalue/onnxruntime_go"
)

// ocrLine is one recognized text line with its box in source pixels.
type ocrLine struct {
	Text string
	Box  image.Rectangle
	Conf float64
}

type ocrEngine struct {
	mu     sync.Mutex
	loaded bool
	det    *oni.DynamicAdvancedSession
	rec    *oni.DynamicAdvancedSession
	labels []string
}

var ocrEng = &ocrEngine{}

var (
	ortInitOnce sync.Once
	ortInitErr  error
)

func initOnnxRuntime() error {
	ortInitOnce.Do(func() {
		dll, err := findAsset("onnxruntime.dll")
		if err != nil {
			ortInitErr = fmt.Errorf("找不到 onnxruntime.dll: %w", err)
			return
		}
		oni.SetSharedLibraryPath(dll)
		if err := oni.InitializeEnvironment(); err != nil {
			ortInitErr = fmt.Errorf("初始化 onnxruntime 失败: %w", err)
		}
	})
	return ortInitErr
}

// Load initializes the OCR models lazily and is safe to call concurrently.
func (e *ocrEngine) Load() error {
	if err := initOnnxRuntime(); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.loaded {
		return nil
	}
	detPath, err := findAsset("PP-OCRv5_mobile_det_infer.onnx")
	if err != nil {
		return fmt.Errorf("缺少检测模型: %w", err)
	}
	recPath, err := findAsset("PP-OCRv5_mobile_rec_infer.onnx")
	if err != nil {
		return fmt.Errorf("缺少识别模型: %w", err)
	}
	dictPath, err := findAsset("ppocrv5_dict.txt")
	if err != nil {
		return fmt.Errorf("缺少字典: %w", err)
	}
	dict, err := os.ReadFile(dictPath)
	if err != nil {
		return fmt.Errorf("读取字典失败: %w", err)
	}
	text := strings.ReplaceAll(string(dict), "\r\n", "\n")
	labels := strings.Split(text, "\n")
	// The dictionary starts with the CTC blank entry; PP-OCRv5 mobile has
	// one extra output class at the end, which behaves as a space token.
	if len(labels) > 1 && labels[len(labels)-1] == "" {
		labels[len(labels)-1] = " "
	}
	det, err := oni.NewDynamicAdvancedSession(detPath, []string{"x"}, []string{"fetch_name_0"}, nil)
	if err != nil {
		return fmt.Errorf("加载检测模型失败: %w", err)
	}
	rec, err := oni.NewDynamicAdvancedSession(recPath, []string{"x"}, []string{"fetch_name_0"}, nil)
	if err != nil {
		det.Destroy()
		return fmt.Errorf("加载识别模型失败: %w", err)
	}
	e.det, e.rec, e.labels = det, rec, labels
	e.loaded = true
	return nil
}

// Recognize runs detection and recognition on img and returns text lines
// in reading order.
func (e *ocrEngine) Recognize(img *image.RGBA) ([]ocrLine, error) {
	if err := e.Load(); err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	groups, err := e.detectBoxes(img)
	if err != nil {
		return nil, err
	}
	lines := make([]ocrLine, 0, 16)
	for _, group := range groups {
		text := ""
		union := image.Rectangle{}
		var confSum float64
		var confN int
		for _, b := range group {
			t, conf := e.recognizeBox(img, b)
			if t == "" {
				continue
			}
			if text != "" {
				// Restore a space when neighbouring boxes have a gap.
				gap := b.Min.X - union.Max.X
				if gap > (union.Dy()+b.Dy())/4 {
					text += " "
				}
			}
			text += t
			union = union.Union(b)
			confSum += conf
			confN++
		}
		if text != "" {
			conf := 0.0
			if confN > 0 {
				conf = confSum / float64(confN)
			}
			lines = append(lines, ocrLine{Text: text, Box: union, Conf: conf})
		}
	}
	return lines, nil
}

// ---------------------------------------------------------------------------
// Detection

const detMaxSide = 1280

var detMean = [3]float32{0.485, 0.456, 0.406}
var detStd = [3]float32{0.229, 0.224, 0.225}

// detectBoxes returns text boxes grouped into lines, in reading order.
func (e *ocrEngine) detectBoxes(img *image.RGBA) ([][]image.Rectangle, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 4 || h < 4 {
		return nil, fmt.Errorf("图片太小")
	}
	scale := 1.0
	if m := max(w, h); m > detMaxSide {
		scale = float64(detMaxSide) / float64(m)
	}
	if m := min(w, h); m < 64 {
		if s := float64(64) / float64(m); s > scale {
			scale = s
		}
	}
	inW := max(32, int(math.Round(float64(w)*scale/32))*32)
	inH := max(32, int(math.Round(float64(h)*scale/32))*32)
	kx := float64(inW) / float64(w)
	ky := float64(inH) / float64(h)

	data := normalizeDet(img, inW, inH)
	in, err := oni.NewTensor[float32](oni.NewShape(1, 3, int64(inH), int64(inW)), data)
	if err != nil {
		return nil, err
	}
	out := []oni.Value{nil}
	if err := e.det.Run([]oni.Value{in}, out); err != nil {
		in.Destroy()
		return nil, fmt.Errorf("检测推理失败: %w", err)
	}
	in.Destroy()
	t, ok := out[0].(*oni.Tensor[float32])
	if !ok {
		return nil, fmt.Errorf("检测输出类型异常")
	}
	defer t.Destroy()
	shape := t.GetShape()
	if len(shape) != 4 {
		return nil, fmt.Errorf("检测输出维度异常: %v", shape)
	}
	probs := t.GetData()
	boxes := boxesFromProbMap(probs, int(shape[3]), int(shape[2]))

	// Map det-space boxes back to source pixels, with a DB-style unclip.
	outBoxes := make([]image.Rectangle, 0, len(boxes))
	for _, r := range boxes {
		bw := float64(r.Dx())
		bh := float64(r.Dy())
		pad := 2.0 * bw * bh / (2 * (bw + bh))
		x0 := int(math.Floor((float64(r.Min.X)-pad)/kx + 0.5))
		y0 := int(math.Floor((float64(r.Min.Y)-pad)/ky + 0.5))
		x1 := int(math.Ceil((float64(r.Max.X)+pad)/kx + 0.5))
		y1 := int(math.Ceil((float64(r.Max.Y)+pad)/ky + 0.5))
		r2 := image.Rect(x0, y0, x1, y1).Intersect(img.Bounds())
		if r2.Dx() < 3 || r2.Dy() < 3 {
			continue
		}
		outBoxes = append(outBoxes, r2)
	}
	return groupLines(outBoxes), nil
}

// normalizeDet resizes to w×h with bilinear sampling and normalizes to the
// ImageNet statistics PP-OCR detection expects, in CHW layout.
func normalizeDet(src *image.RGBA, w, h int) []float32 {
	b := src.Bounds()
	sw, sh := float64(b.Dx()), float64(b.Dy())
	data := make([]float32, 3*w*h)
	planes := [3][]float32{data[:w*h], data[w*h : 2*w*h], data[2*w*h:]}
	for y := 0; y < h; y++ {
		sy := (float64(y)+0.5)/float64(h)*sh - 0.5
		for x := 0; x < w; x++ {
			sx := (float64(x)+0.5)/float64(w)*sw - 0.5
			r, g, bl := sampleBilinear(src, sx, sy)
			i := y*w + x
			planes[0][i] = (r - detMean[0]) / detStd[0]
			planes[1][i] = (g - detMean[1]) / detStd[1]
			planes[2][i] = (bl - detMean[2]) / detStd[2]
		}
	}
	return data
}

// sampleBilinear samples src at fractional pixel coordinates (relative to
// the image origin) and returns linear RGB in 0..1.
func sampleBilinear(src *image.RGBA, x, y float64) (float32, float32, float32) {
	b := src.Bounds()
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	fx := x - float64(x0)
	fy := y - float64(y0)
	x1, y1 := x0+1, y0+1
	if x0 < 0 {
		x0, fx = 0, 0
	}
	if y0 < 0 {
		y0, fy = 0, 0
	}
	if x1 > b.Max.X-1 {
		x1 = b.Max.X - 1
	}
	if y1 > b.Max.Y-1 {
		y1 = b.Max.Y - 1
	}
	x0c := min(max(x0, b.Min.X), b.Max.X-1)
	y0c := min(max(y0, b.Min.Y), b.Max.Y-1)
	i00 := src.PixOffset(x0c, y0c)
	i10 := src.PixOffset(x1, y0c)
	i01 := src.PixOffset(x0c, y1)
	i11 := src.PixOffset(x1, y1)
	mix := func(a, b2 uint8, t float64) float32 {
		return (float32(a)*(1-float32(t)) + float32(b2)*float32(t)) / 255
	}
	var out [3]float32
	for c := 0; c < 3; c++ {
		top := mix(src.Pix[i00+c], src.Pix[i10+c], fx)
		bottom := mix(src.Pix[i01+c], src.Pix[i11+c], fx)
		out[c] = top*(1-float32(fy)) + bottom*float32(fy)
	}
	return out[0], out[1], out[2]
}

// boxesFromProbMap thresholds the DB probability map and extracts the
// bounding box of every connected component.
func boxesFromProbMap(probs []float32, w, h int) []image.Rectangle {
	const thresh = 0.3
	visited := make([]bool, w*h)
	boxes := make([]image.Rectangle, 0, 16)
	stack := make([]int, 0, 1024)
	for i := 0; i < w*h; i++ {
		if visited[i] || probs[i] <= thresh {
			continue
		}
		visited[i] = true
		stack = append(stack, i)
		minX, minY, maxX, maxY := w, h, -1, -1
		count := 0
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			px, py := p%w, p/w
			count++
			if px < minX {
				minX = px
			}
			if px > maxX {
				maxX = px
			}
			if py < minY {
				minY = py
			}
			if py > maxY {
				maxY = py
			}
			for _, n := range [4]int{p - 1, p + 1, p - w, p + w} {
				if n >= 0 && n < w*h && !visited[n] && probs[n] > thresh {
					// Guard against wrapping at row edges.
					if n == p-1 && px == 0 {
						continue
					}
					if n == p+1 && px == w-1 {
						continue
					}
					visited[n] = true
					stack = append(stack, n)
				}
			}
		}
		if count < 6 {
			continue
		}
		boxes = append(boxes, image.Rect(minX, minY, maxX+1, maxY+1))
	}
	return boxes
}

// groupLines sorts boxes into visual rows and orders each row left to
// right, top to bottom.
func groupLines(boxes []image.Rectangle) [][]image.Rectangle {
	if len(boxes) == 0 {
		return nil
	}
	sorted := make([]image.Rectangle, len(boxes))
	copy(sorted, boxes)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0; j-- {
			a, b := sorted[j-1], sorted[j]
			if a.Min.Y > b.Min.Y || (a.Min.Y == b.Min.Y && a.Min.X > b.Min.X) {
				sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
			} else {
				break
			}
		}
	}
	groups := make([][]image.Rectangle, 0, 8)
	lineMin, lineMax := sorted[0].Min.Y, sorted[0].Max.Y
	group := []image.Rectangle{sorted[0]}
	for _, b := range sorted[1:] {
		cy := (b.Min.Y + b.Max.Y) / 2
		if cy >= lineMin && cy <= lineMax {
			group = append(group, b)
			lineMin = min(lineMin, b.Min.Y)
			lineMax = max(lineMax, b.Max.Y)
			continue
		}
		groups = append(groups, orderRow(group))
		group = []image.Rectangle{b}
		lineMin, lineMax = b.Min.Y, b.Max.Y
	}
	groups = append(groups, orderRow(group))
	return groups
}

func orderRow(row []image.Rectangle) []image.Rectangle {
	for i := 1; i < len(row); i++ {
		for j := i; j > 0 && row[j-1].Min.X > row[j].Min.X; j-- {
			row[j-1], row[j] = row[j], row[j-1]
		}
	}
	return row
}

// ---------------------------------------------------------------------------
// Recognition

const recHeight = 48

// recognizeBox runs the recognition model on one crop and CTC-decodes it.
func (e *ocrEngine) recognizeBox(img *image.RGBA, box image.Rectangle) (string, float64) {
	r := box.Intersect(img.Bounds())
	w0, h0 := r.Dx(), r.Dy()
	if w0 < 2 || h0 < 2 {
		return "", 0
	}
	w := int(math.Round(float64(w0) * recHeight / float64(h0)))
	w = min(max(roundUpTo(w, 8), 32), 960)
	data := make([]float32, 3*recHeight*w)
	for y := 0; y < recHeight; y++ {
		sy := (float64(y)+0.5)/recHeight*float64(h0) - 0.5
		for x := 0; x < w; x++ {
			sx := (float64(x)+0.5)/float64(w)*float64(w0) - 0.5
			cr, cg, cb := sampleBilinearAt(img, float64(r.Min.X)+sx, float64(r.Min.Y)+sy)
			i := y*w + x
			data[i] = cr*2 - 1
			data[recHeight*w+i] = cg*2 - 1
			data[2*recHeight*w+i] = cb*2 - 1
		}
	}
	in, err := oni.NewTensor[float32](oni.NewShape(1, 3, recHeight, int64(w)), data)
	if err != nil {
		return "", 0
	}
	out := []oni.Value{nil}
	if err := e.rec.Run([]oni.Value{in}, out); err != nil {
		in.Destroy()
		return "", 0
	}
	in.Destroy()
	t, ok := out[0].(*oni.Tensor[float32])
	if !ok {
		return "", 0
	}
	defer t.Destroy()
	shape := t.GetShape()
	if len(shape) != 3 {
		return "", 0
	}
	steps, classes := int(shape[1]), int(shape[2])
	return ctcDecode(t.GetData(), steps, classes, e.labels)
}

// sampleBilinearAt is sampleBilinear with absolute source coordinates.
func sampleBilinearAt(src *image.RGBA, x, y float64) (float32, float32, float32) {
	return sampleBilinear(src, x-float64(src.Bounds().Min.X), y-float64(src.Bounds().Min.Y))
}

// ctcDecode greedy-decodes a [T][C] probability tensor. Index 0 is the CTC
// blank; empty dictionary entries also decode to nothing.
func ctcDecode(probs []float32, steps, classes int, labels []string) (string, float64) {
	var sb strings.Builder
	prev := -1
	var sum float64
	n := 0
	for t := 0; t < steps; t++ {
		row := probs[t*classes : (t+1)*classes]
		best, bestP := 0, float32(-1)
		for c, p := range row {
			if p > bestP {
				bestP, best = p, c
			}
		}
		if best != 0 && best != prev && best < len(labels) && labels[best] != "" {
			sb.WriteString(labels[best])
			sum += float64(bestP)
			n++
		}
		prev = best
	}
	if n == 0 {
		return "", 0
	}
	return sb.String(), sum / float64(n)
}

func roundUpTo(v, m int) int {
	return (v + m - 1) / m * m
}
