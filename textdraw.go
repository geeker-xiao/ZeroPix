// Text rasterization for exported annotations. A CJK-capable system font is
// preferred so Chinese annotations survive the export; the Go font is the
// fallback.

package main

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

var (
	fontOnce sync.Once
	uiFont   *opentype.Font

	fontMu    sync.Mutex
	fontCache = map[string]*opentype.Font{}
)

// annotationFont is one of the families a text annotation can be set in:
// the name the editor shows, the family the UI engine knows, and the file
// that carries it for the export.
type annotationFont struct {
	Name   string // 微软雅黑 (Microsoft YaHei), ...
	Family string // Microsoft YaHei, ...
	File   string // msyh.ttc, ...
}

// annotationFonts is what the editor offers when the system's own list
// cannot be read, and it decides the order the system list starts with: the
// Chinese families people reach for first.
var annotationFonts = []annotationFont{
	{"微软雅黑 (Microsoft YaHei)", "Microsoft YaHei", "msyh.ttc"},
	{"黑体 (SimHei)", "SimHei", "simhei.ttf"},
	{"宋体 (SimSun)", "SimSun", "simsun.ttc"},
	{"楷体 (KaiTi)", "KaiTi", "simkai.ttf"},
}

var (
	fontListOnce sync.Once
	fontList     []annotationFont
)

// systemFontList returns every family the machine has, the curated Chinese
// ones first.
func systemFontList() []annotationFont {
	fontListOnce.Do(func() {
		fontList = installedFonts()
		if len(fontList) == 0 {
			fontList = annotationFonts
			return
		}
		// The curated families lead, whether or not Windows names them the
		// same way, since they are the ones Chinese annotations use.
		var head []annotationFont
		for _, af := range annotationFonts {
			for i, got := range fontList {
				if got.Family == af.Family {
					got.Name = af.Name
					head = append(head, got)
					fontList = append(fontList[:i], fontList[i+1:]...)
					break
				}
			}
		}
		fontList = append(head, fontList...)
	})
	return fontList
}

// fontLabels are the families as the drop-down lists them.
func fontLabels() []string {
	list := systemFontList()
	out := make([]string, len(list))
	for i, f := range list {
		out[i] = f.Name
	}
	return out
}

// defaultFamily is the family a new text annotation starts in.
func defaultFamily() string {
	if list := systemFontList(); len(list) > 0 {
		return list[0].Family
	}
	return annotationFonts[0].Family
}

// fontLabel names a family as the drop-down does.
func fontLabel(family string) string {
	for _, f := range systemFontList() {
		if f.Family == family {
			return f.Name
		}
	}
	return family
}

// fontByLabel finds the family a label of the drop-down stands for.
func fontByLabel(label string) (annotationFont, bool) {
	for _, f := range systemFontList() {
		if f.Name == label {
			return f, true
		}
	}
	return annotationFont{}, false
}

// fontFileFor returns the file that carries a family, if the list knows it.
func fontFileFor(family string) string {
	for _, f := range systemFontList() {
		if f.Family == family {
			if filepath.IsAbs(f.File) {
				return f.File
			}
			return fontPath(f.File)
		}
	}
	return ""
}

// loadUIFont finds a font that covers CJK text and latin text alike.
func loadUIFont() *opentype.Font {
	fontOnce.Do(func() {
		cands := []string{"msyh.ttc", "simhei.ttf", "simsun.ttc", "segoeui.ttf"}
		for _, name := range cands {
			if f := parseFontFile(fontPath(name)); f != nil {
				uiFont = f
				return
			}
		}
		f, err := opentype.Parse(goregular.TTF)
		if err == nil {
			uiFont = f
		}
	})
	return uiFont
}

func fontPath(name string) string {
	dir := os.Getenv("WINDIR")
	if dir == "" {
		dir = `C:\Windows`
	}
	return filepath.Join(dir, "Fonts", name)
}

// fontForFamily returns the family's font, or the default one when the
// family is unknown, so a missing font never loses an annotation.
func fontForFamily(family string) *opentype.Font {
	if family == "" {
		return loadUIFont()
	}
	fontMu.Lock()
	defer fontMu.Unlock()
	if f, ok := fontCache[family]; ok {
		return f
	}
	var f *opentype.Font
	if path := fontFileFor(family); path != "" {
		f = parseFontFamily(path, family)
	}
	if f == nil {
		f = loadUIFont()
	}
	fontCache[family] = f
	return f
}

// parseFontFamily reads a font file, picking the face of a collection that
// carries family, and the first one when none does.
func parseFontFamily(path, family string) *opentype.Font {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	faces := []*opentype.Font{}
	if strings.EqualFold(filepath.Ext(path), ".ttc") {
		coll, err := sfnt.ParseCollection(data)
		if err != nil || coll.NumFonts() == 0 {
			return nil
		}
		for i := 0; i < coll.NumFonts(); i++ {
			if f, err := coll.Font(i); err == nil {
				faces = append(faces, f)
			}
		}
	} else if f, err := opentype.Parse(data); err == nil {
		faces = append(faces, f)
	}
	if len(faces) == 0 {
		return nil
	}
	for _, f := range faces {
		var buf sfnt.Buffer
		if name, err := f.Name(&buf, sfnt.NameIDFamily); err == nil && name == family {
			return f
		}
	}
	return faces[0]
}

func parseFontFile(path string) *opentype.Font {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if strings.EqualFold(filepath.Ext(path), ".ttc") {
		coll, err := sfnt.ParseCollection(data)
		if err != nil || coll.NumFonts() == 0 {
			return nil
		}
		f, err := coll.Font(0)
		if err != nil {
			return nil
		}
		return f
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return nil
	}
	return f
}

func newFace(family string, size float64) font.Face {
	f := fontForFamily(family)
	if f == nil {
		return nil
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		return nil
	}
	return face
}

// measureText returns the width of s at the given pixel size.
func measureText(s string, size float64) float64 {
	face := newFace("", size)
	if face == nil {
		return float64(approxTextWidth(s, float32(size)))
	}
	defer face.Close()
	return float64(font.MeasureString(face, s).Ceil())
}

// drawText draws s with its box's top-left at (x, y) and its first baseline
// y + ascent, in the given family.
func drawText(dst *image.RGBA, x, y int, s string, size float64, family string, c colorRGBA) {
	face := newFace(family, size)
	if face == nil {
		return
	}
	defer face.Close()
	d := &font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(color.NRGBA{R: c.R, G: c.G, B: c.B, A: c.A}),
		Face: face,
	}
	lineH := (face.Metrics().Ascent + face.Metrics().Descent).Ceil()
	for i, line := range strings.Split(s, "\n") {
		d.Dot = fixed.P(x, y+i*lineH+face.Metrics().Ascent.Ceil())
		d.DrawString(line)
	}
}

// approxTextWidth estimates the width without rasterizing, for the live
// overlay.
func approxTextWidth(s string, size float32) float32 {
	var w float32
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r), unicode.Is(unicode.Hangul, r),
			unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r),
			r == '　':
			w += size
		case r == ' ':
			w += size * 0.3
		case r < 128:
			w += size * 0.56
		default:
			w += size * 0.8
		}
	}
	return w
}

// drawMaskedText is unused now but kept close to drawText for callers that
// want an offscreen mask; remove it when the pipeline stabilizes.
var _ = draw.Over
