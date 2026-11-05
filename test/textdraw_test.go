package main

import (
	"bytes"
	"image"
	"testing"
)

// Drawing the same text in two families must give two different images, and
// an unknown family must fall back instead of drawing nothing.
func TestDrawTextUsesTheFamily(t *testing.T) {
	render := func(family string) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, 140, 44))
		drawText(img, 2, 2, "国kW", 26, family, colorRGBA{R: 0, G: 0, B: 0, A: 255})
		return img
	}
	yahei := render(annotationFonts[0].Family)
	simsun := render(annotationFonts[2].Family)
	if bytes.Equal(yahei.Pix, simsun.Pix) {
		t.Fatal("雅黑 and 宋体 rasterized identically")
	}

	// The fallback draws the text too: a load failure must not blank the
	// annotation.
	fallback := render("no such family")
	if bytes.Equal(fallback.Pix, image.NewRGBA(image.Rect(0, 0, 140, 44)).Pix) {
		t.Fatal("an unknown family drew nothing")
	}
}

// The editor offers the machine's own families, and every label it shows
// leads back to its family and its file.
func TestSystemFontList(t *testing.T) {
	list := systemFontList()
	if len(list) == 0 {
		t.Fatal("no fonts listed")
	}
	for _, f := range list[:min(len(list), 8)] {
		af, ok := fontByLabel(f.Name)
		if !ok || af.Family != f.Family {
			t.Fatalf("%q does not lead back to its family: %+v", f.Name, af)
		}
		if fontLabel(f.Family) != f.Name {
			t.Fatalf("%q labels as %q", f.Family, fontLabel(f.Family))
		}
	}
	if font := fontForFamily(defaultFamily()); font == nil {
		t.Fatalf("the default family %q has no font", defaultFamily())
	}
}
