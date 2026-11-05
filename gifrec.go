// Recording the chosen region to an animated GIF, or to an MP4 when ffmpeg
// is installed. Frames are grabbed with the same GDI capture the snip uses,
// quantized to one palette per frame, and kept within a fixed memory budget.

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type recOptions struct {
	x, y, w, h int
	video      bool
}

const (
	recMaxWidth  = 1280     // frames wider than this are scaled down
	recMaxFrames = 300      // hard cap on frames
	recMaxBudget = 96 << 20 // bytes of frame memory
	recMaxTime   = 30 * time.Second
)

type recorder struct {
	opt  recOptions
	fps  int
	max  int
	win  *mygo.Window
	stop chan struct{}
	done chan struct{}
	fin  sync.Once

	frames []*image.Paletted
	dir    string // PNG frames, for a video

	mu      sync.Mutex
	count   int
	elapsed time.Duration
	start   time.Time
	status  string
	errMsg  string
}

// startRecording records the region in physical screen pixels. A video needs
// ffmpeg; without it the recording is made as a GIF instead.
func startRecording(opt recOptions) {
	if app.rec != nil {
		return
	}
	if opt.w < 8 || opt.h < 8 {
		app.toast("选区太小，无法录制")
		return
	}
	if opt.video && ffmpegPath() == "" {
		app.toast("没有找到 ffmpeg，已改为录制 GIF")
		opt.video = false
	}
	r := &recorder{
		opt:   opt,
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
		start: time.Now(),
	}
	r.fps, r.max = recPlan(opt.w, opt.h)
	if opt.video {
		dir, err := os.MkdirTemp("", appSlug+"_rec_")
		if err != nil {
			app.toast("无法创建临时目录: " + err.Error())
			return
		}
		r.dir = dir
	}
	app.rec = r
	r.openControl()
	go r.loop()
}

// recPlan picks a frame rate and frame cap for a region that keep the
// recording inside the memory budget.
func recPlan(w, h int) (fps, maxFrames int) {
	factor := 1.0
	if w > recMaxWidth {
		factor = float64(recMaxWidth) / float64(w)
	}
	px := int(float64(w)*factor) * int(float64(h)*factor)
	budget := max(recMaxBudget, px*10)
	maxFrames = min(recMaxFrames, budget/px)
	fps = 10
	if maxFrames < 60 {
		fps = 6
	}
	return fps, max(10, maxFrames)
}

func (r *recorder) loop() {
	defer close(r.done)
	ticker := time.NewTicker(time.Second / time.Duration(r.fps))
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			if time.Since(r.start) > recMaxTime || r.framesLen() >= r.max {
				r.setStatus("限时结束，正在保存…")
				r.finalize(true)
				return
			}
			img, err := captureScreenRect(r.opt.x, r.opt.y, r.opt.w, r.opt.h)
			if err != nil || img == nil {
				continue
			}
			img = fitWidth(img, recMaxWidth)
			if r.opt.video {
				if err := writeFramePNG(r.dir, r.count, img); err != nil {
					r.setErr("写入帧失败: " + err.Error())
					r.finalize(false)
					return
				}
			} else {
				r.frames = append(r.frames, quantizeFrame(img))
			}
			r.count++
			if r.count%4 == 0 {
				r.mu.Lock()
				r.elapsed = time.Since(r.start)
				r.mu.Unlock()
				r.refresh()
			}
		}
	}
}

func (r *recorder) framesLen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

// finalize ends the recording once: the control bar goes away and what was
// captured is saved, or dropped.
func (r *recorder) finalize(save bool) {
	r.fin.Do(func() {
		select {
		case <-r.stop:
		default:
			close(r.stop)
		}
		go func() {
			<-r.done
			if r.win != nil && !r.win.IsDestroyed() {
				r.win.Destroy()
				r.win = nil
			}
			count := r.framesLen()
			if !save || count == 0 {
				r.cleanup()
				app.rec = nil
				if !save && count == 0 {
					app.toast("录制已取消")
				}
				return
			}
			r.save(count)
		}()
	})
}

// save writes the frames and reports where they went.
func (r *recorder) save(count int) {
	app.toast(fmt.Sprintf("正在生成 %d 帧…", count))
	go func() {
		defer func() { app.rec = nil }()
		var path string
		var err error
		if r.opt.video {
			path, err = r.saveVideo()
		} else {
			path, err = r.saveGIF()
		}
		r.cleanup()
		if err != nil {
			app.toast("保存失败: " + err.Error())
			return
		}
		if path == "" { // the file dialog was cancelled
			return
		}
		app.toast("已保存 " + filepath.Base(path))
	}()
}

func (r *recorder) cleanup() {
	r.frames = nil
	if r.dir != "" {
		os.RemoveAll(r.dir)
		r.dir = ""
	}
}

// saveGIF asks for a file and encodes the frames into it.
func (r *recorder) saveGIF() (string, error) {
	path, err := askPath("保存 GIF", "gif", "GIF 动图")
	if err != nil || path == "" {
		return "", err
	}
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	delays := make([]int, len(r.frames))
	for i := range delays {
		delays[i] = max(2, 100/r.fps)
	}
	anim := &gif.GIF{Image: r.frames, Delay: delays, LoopCount: 0}
	if err := gif.EncodeAll(f, anim); err != nil {
		return "", err
	}
	return path, nil
}

func (r *recorder) saveVideo() (string, error) {
	path, err := askPath("保存视频", "mp4", "MP4 视频")
	if err != nil || path == "" {
		return "", err
	}
	ff := ffmpegPath()
	if ff == "" {
		return "", fmt.Errorf("没有找到 ffmpeg")
	}
	cmd := exec.Command(ff, "-y",
		"-framerate", fmt.Sprint(r.fps),
		"-i", filepath.Join(r.dir, "frame_%05d.png"),
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-crf", "20",
		path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, trimMessage(string(out), 60))
	}
	return path, nil
}

func askPath(title, ext, label string) (string, error) {
	name := fmt.Sprintf("%s_%s.%s", appSlug, time.Now().Format("20060102_150405"), ext)
	return mygo.Dialog.Save(mygo.SaveDialogOptions{
		Title:       title,
		DefaultPath: filepath.Join(saveDir(), name),
		Filters:     []mygo.FileFilter{{Name: label, Extensions: []string{ext}}},
	})
}

func saveDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	if pics := filepath.Join(home, "Pictures"); dirExists(pics) {
		return pics
	}
	return home
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func ffmpegPath() string {
	p, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ""
	}
	return p
}

func writeFramePNG(dir string, n int, img *image.RGBA) error {
	name := filepath.Join(dir, fmt.Sprintf("frame_%05d.png", n))
	f, err := os.Create(name)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// fitWidth scales an image down to w pixels wide, if it is wider.
func fitWidth(src *image.RGBA, w int) *image.RGBA {
	if src.Bounds().Dx() <= w {
		return src
	}
	sh := float64(src.Bounds().Dy()) * float64(w) / float64(src.Bounds().Dx())
	dst := image.NewRGBA(image.Rect(0, 0, w, max(1, int(sh))))
	sx := float64(src.Bounds().Dx()) / float64(w)
	sy := float64(src.Bounds().Dy()) / float64(dst.Bounds().Dy())
	b := src.Bounds()
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < w; x++ {
			sxi := min(b.Min.X+int(float64(x)*sx), b.Max.X-1)
			syi := min(b.Min.Y+int(float64(y)*sy), b.Max.Y-1)
			i := src.PixOffset(sxi, syi)
			d := dst.PixOffset(x, y)
			copy(dst.Pix[d:d+3], src.Pix[i:i+3])
			dst.Pix[d+3] = 255
		}
	}
	return dst
}

// ---------------------------------------------------------------------------
// Quantization: a median cut over 15-bit color buckets, one palette per
// frame, which keeps a GIF of screen content readable.

type colorBox struct {
	lo, hi int // indexes into the bucket list
	r0, r1 int // channel ranges, for choosing the widest
	g0, g1 int
	b0, b1 int
}

func quantizeFrame(src *image.RGBA) *image.Paletted {
	const bits = 5
	const levels = 1 << bits
	const shift = 8 - bits
	const nBuckets = levels * levels * levels

	counts := make([]int32, nBuckets)
	sumR := make([]int32, nBuckets)
	sumG := make([]int32, nBuckets)
	sumB := make([]int32, nBuckets)
	b := src.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		row := src.Pix[src.PixOffset(b.Min.X, y):]
		for x := 0; x < b.Dx(); x++ {
			i := x * 4
			r, g, bl := int(row[i]), int(row[i+1]), int(row[i+2])
			k := (r>>shift)<<(2*bits) | (g>>shift)<<bits | (bl >> shift)
			counts[k]++
			sumR[k] += int32(r)
			sumG[k] += int32(g)
			sumB[k] += int32(bl)
		}
	}
	buckets := make([]int, 0, 4096)
	for k, c := range counts {
		if c > 0 {
			buckets = append(buckets, k)
		}
	}
	paletteIndex := make([]uint8, nBuckets)
	boxes := splitBoxes(buckets, counts, 256)
	pal := make(color.Palette, 0, len(boxes))
	for bi, box := range boxes {
		var r, g, bl, n int32
		for i := box.lo; i < box.hi; i++ {
			k := buckets[i]
			r += sumR[k]
			g += sumG[k]
			bl += sumB[k]
			n += counts[k]
			paletteIndex[k] = uint8(bi)
		}
		if n == 0 {
			n = 1
		}
		pal = append(pal, color.RGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(bl / n), A: 255})
	}
	if len(pal) == 0 {
		pal = color.Palette{color.Black}
	}
	dst := image.NewPaletted(image.Rect(0, 0, b.Dx(), b.Dy()), pal)
	for y := 0; y < b.Dy(); y++ {
		row := src.Pix[src.PixOffset(b.Min.X, b.Min.Y+y):]
		drow := dst.Pix[dst.PixOffset(0, y):]
		for x := 0; x < b.Dx(); x++ {
			i := x * 4
			k := (int(row[i])>>shift)<<(2*bits) | (int(row[i+1])>>shift)<<bits | (int(row[i+2]) >> shift)
			drow[x] = paletteIndex[k]
		}
	}
	return dst
}

// splitBoxes runs a median cut over the occupied buckets, returning at most
// n boxes.
func splitBoxes(buckets []int, counts []int32, n int) []colorBox {
	if len(buckets) == 0 {
		return nil
	}
	boxes := []colorBox{makeBox(buckets, 0, len(buckets))}
	for len(boxes) < n {
		best := -1
		bestScore := 0
		for i, box := range boxes {
			if box.hi-box.lo < 2 {
				continue
			}
			w := max(box.r1-box.r0, box.g1-box.g0, box.b1-box.b0)
			if w*2 > bestScore {
				best, bestScore = i, w*2
			}
		}
		if best < 0 {
			break
		}
		box := boxes[best]
		ch := 0
		if box.g1-box.g0 > box.r1-box.r0 && box.g1-box.g0 >= box.b1-box.b0 {
			ch = 1
		} else if box.b1-box.b0 > box.r1-box.r0 && box.b1-box.b0 > box.g1-box.g0 {
			ch = 2
		}
		sub := buckets[box.lo:box.hi]
		sortBuckets(sub, ch)
		total, acc := 0, 0
		for _, k := range sub {
			total += int(counts[k])
		}
		cut := box.lo + 1
		for i, k := range sub {
			acc += int(counts[k])
			if acc*2 >= total {
				cut = box.lo + i + 1
				break
			}
		}
		cut = min(max(cut, box.lo+1), box.hi-1)
		boxes[best] = makeBox(buckets, box.lo, cut)
		boxes = append(boxes, makeBox(buckets, cut, box.hi))
	}
	return boxes
}

func makeBox(buckets []int, lo, hi int) colorBox {
	box := colorBox{lo: lo, hi: hi, r0: 31, g0: 31, b0: 31}
	for i := lo; i < hi; i++ {
		k := buckets[i]
		r, g, b := (k>>10)&31, (k>>5)&31, k&31
		box.r0, box.r1 = min(box.r0, r), max(box.r1, r)
		box.g0, box.g1 = min(box.g0, g), max(box.g1, g)
		box.b0, box.b1 = min(box.b0, b), max(box.b1, b)
	}
	return box
}

func sortBuckets(s []int, ch int) {
	key := func(k int) int {
		switch ch {
		case 0:
			return (k >> 10) & 31
		case 1:
			return (k >> 5) & 31
		default:
			return k & 31
		}
	}
	for i := 1; i < len(s); i++ {
		v := s[i]
		j := i - 1
		for j >= 0 && key(s[j]) > key(v) {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = v
	}
}

// ---------------------------------------------------------------------------
// The control bar shown while recording.

func (r *recorder) openControl() {
	x, y := r.controlPos()
	r.win = mygo.NewWindow(mygo.WindowOptions{
		Title:           "录制中",
		X:               x,
		Y:               y,
		Width:           236,
		Height:          46,
		Frameless:       true,
		AlwaysOnTop:     true,
		SkipTaskbar:     true,
		DisableResize:   true,
		DisableMinimize: true,
		DisableMaximize: true,
		DisableShadow:   true,
		BackgroundColor: "#1E1F23",
		CornerRadius:    10,
		Content:         ui.View(r.view),
	})
	if r.win == nil || r.win.IsDestroyed() {
		app.rec = nil
		app.toast("无法创建录制控制条")
	}
}

// controlPos keeps the bar outside the recorded region where it can. The
// region is in physical pixels and the display in DIPs, so it converts.
func (r *recorder) controlPos() (int, int) {
	const w, h = 236, 46
	disp := displayAtPhysical(r.opt.x, r.opt.y)
	b := disp.Bounds
	scale := disp.ScaleFactor
	if scale <= 0 {
		scale = 1
	}
	rx, ry, rw, rh := dipRect(r.opt.x, r.opt.y, r.opt.w, r.opt.h, scale)
	if ry-h-10 > b.Y {
		return rx + rw - w, ry - h - 10
	}
	if ry+rh+h+10 < b.Y+b.Height {
		return rx + rw - w, ry + rh + 10
	}
	return max(b.X+8, rx+rw-w), ry + 10
}

func (r *recorder) setStatus(s string) {
	r.mu.Lock()
	r.status = s
	r.mu.Unlock()
	r.refresh()
}

func (r *recorder) setErr(s string) {
	r.mu.Lock()
	r.errMsg = s
	r.mu.Unlock()
	r.refresh()
}

func (r *recorder) refresh() {
	if w := r.win; w != nil && !w.IsDestroyed() {
		w.Update(func() {})
	}
}

func (r *recorder) view(c *ui.Context) {
	r.mu.Lock()
	msg := fmt.Sprintf("%s  %d 帧", fmtDuration(r.elapsed), r.count)
	if r.status != "" {
		msg = r.status
	}
	errMsg := r.errMsg
	r.mu.Unlock()

	// The bar paints its own background, as the window's would be covered by
	// a filled root.
	ui.Row(c).Fill().Background(ui.RGBA(0x1E, 0x1F, 0x23, 1)).
		Padding(12, 8).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(10).Height(10).Draw(func(p *ui.Painter, rect ui.Rect) {
			p.Fill(rect, ui.RGB(0xE5, 0x39, 0x35), rect.W/2)
		})
		ui.Box(c).FillWidth().Children(func() {
			if errMsg != "" {
				ui.Text(c, trimMessage(errMsg, 16)).FontSize(11.5).
					TextColor(ui.RGB(0xFF, 0x8A, 0x80))
				return
			}
			ui.Text(c, msg).FontSize(12.5).TextColor(ui.RGBA(255, 255, 255, 1))
		})
		ui.ButtonBase(c).Width(56).Height(26).Radius(6).
			Background(ui.RGB(0xE5, 0x39, 0x35)).Label("停止").
			OnClick(func() { go r.finalize(true) }).
			Children(func() {
				ui.Box(c).Fill().Center().Draw(func(p *ui.Painter, rect ui.Rect) {
					p.Text(rect.X, rect.Y, "停止", 12, ui.RGBA(255, 255, 255, 1))
				})
			})
		ui.ButtonBase(c).Width(26).Height(26).Radius(6).
			Background(ui.RGBA(255, 255, 255, 0.12)).Label("放弃").
			OnClick(func() { go r.finalize(false) }).
			Children(func() {
				ui.Box(c).Fill().Padding(6).Draw(func(p *ui.Painter, rect ui.Rect) {
					drawIcon(p, "close", rect, ui.RGBA(255, 255, 255, 0.85))
				})
			})
	})
}

func fmtDuration(d time.Duration) string {
	sec := int(d.Seconds())
	return fmt.Sprintf("%02d:%02d", sec/60, sec%60)
}
