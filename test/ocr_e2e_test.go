package main

import (
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// renderText draws text on a white image for OCR testing.
func renderText(t *testing.T, text string, w, h, size, x, y int) *image.RGBA {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	drawWhite := image.NewUniform(color.White)
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 255, 255, 255, 255
	}
	_ = drawWhite
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: float64(size), DPI: 96})
	if err != nil {
		t.Fatal(err)
	}
	d := &font.Drawer{Dst: img, Src: image.Black, Face: face, Dot: fixed.P(x, y)}
	d.DrawString(text)
	return img
}

// TestOCREndToEnd runs the real PP-OCRv5 models against rendered text.
func TestOCREndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping onnxruntime e2e test in short mode")
	}
	img := renderText(t, "HELLO OCR", 360, 120, 42, 30, 80)
	img2 := renderText(t, "SECOND LINE", 460, 120, 42, 30, 80)
	combined := image.NewRGBA(image.Rect(0, 0, 480, 430))
	for y := 0; y < 120; y++ {
		copy(combined.Pix[combined.PixOffset(0, y):], img.Pix[img.PixOffset(0, y):img.PixOffset(360, y)])
		copy(combined.Pix[combined.PixOffset(0, y+150):], img2.Pix[img2.PixOffset(0, y):img2.PixOffset(460, y)])
	}
	// Add a wide-gap word pair on the third row.
	third := renderText(t, "AAA", 200, 120, 42, 30, 80)
	fourth := renderText(t, "BBB", 200, 120, 42, 30, 80)
	for y := 0; y < 120; y++ {
		copy(combined.Pix[combined.PixOffset(20, y+300):], third.Pix[third.PixOffset(0, y):third.PixOffset(200, y)])
		copy(combined.Pix[combined.PixOffset(240, y+300):], fourth.Pix[fourth.PixOffset(0, y):fourth.PixOffset(200, y)])
	}

	lines, err := ocrEng.Recognize(combined)
	if err != nil {
		t.Skipf("onnxruntime unavailable here: %v", err)
	}
	full := ""
	for _, l := range lines {
		full += l.Text + "\n"
	}
	t.Logf("recognized: %q", full)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d: %q", len(lines), full)
	}
	foundHello, foundSecond, foundGap := false, false, false
	for _, l := range lines {
		if containsFold(l.Text, "HELLO") || containsFold(l.Text, "OCR") {
			foundHello = true
		}
		if containsFold(l.Text, "SECOND") {
			foundSecond = true
		}
		if containsFold(l.Text, "AAA BBB") {
			foundGap = true
		}
	}
	if !foundHello {
		t.Errorf("missing HELLO/OCR in %q", full)
	}
	if !foundSecond {
		t.Errorf("missing SECOND in %q", full)
	}
	if !foundGap {
		t.Errorf("wide-gap words not separated by a space: %q", full)
	}
}

func containsFold(s, sub string) bool {
	if len(sub) == 0 || len(s) < len(sub) {
		return false
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			a, b := s[i+j], sub[j]
			if 'A' <= a && a <= 'Z' {
				a += 'a' - 'A'
			}
			if 'A' <= b && b <= 'Z' {
				b += 'a' - 'A'
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}
