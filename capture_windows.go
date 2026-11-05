//go:build windows

// Screen capture through the classic GDI BitBlt path. It runs on any
// goroutine and captures layered windows as well (CAPTUREBLT).

package main

import (
	"fmt"
	"image"
	"syscall"
	"unsafe"
)

var (
	modUser32 = syscall.NewLazyDLL("user32.dll")
	modGdi32  = syscall.NewLazyDLL("gdi32.dll")

	procGetDC     = modUser32.NewProc("GetDC")
	procReleaseDC = modUser32.NewProc("ReleaseDC")

	procCreateCompatibleDC     = modGdi32.NewProc("CreateCompatibleDC")
	procCreateCompatibleBitmap = modGdi32.NewProc("CreateCompatibleBitmap")
	procSelectObject           = modGdi32.NewProc("SelectObject")
	procBitBlt                 = modGdi32.NewProc("BitBlt")
	procDeleteObject           = modGdi32.NewProc("DeleteObject")
	procDeleteDC               = modGdi32.NewProc("DeleteDC")
	procGetDIBits              = modGdi32.NewProc("GetDIBits")
)

const (
	srccopy      = 0x00CC0020
	captureblt   = 0x40000000
	dibRGBColors = 0
)

type bitmapInfoHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

// captureScreenRect captures a physical-pixel rectangle of the virtual
// screen and returns it as an RGBA image.
func captureScreenRect(x, y, w, h int) (*image.RGBA, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("capture: empty rect %dx%d", w, h)
	}
	screenDC, _, _ := procGetDC.Call(0)
	if screenDC == 0 {
		return nil, fmt.Errorf("capture: GetDC failed")
	}
	defer procReleaseDC.Call(0, screenDC)

	memDC, _, _ := procCreateCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil, fmt.Errorf("capture: CreateCompatibleDC failed")
	}
	defer procDeleteDC.Call(memDC)

	bmp, _, _ := procCreateCompatibleBitmap.Call(screenDC, uintptr(w), uintptr(h))
	if bmp == 0 {
		return nil, fmt.Errorf("capture: CreateCompatibleBitmap failed")
	}
	defer procDeleteObject.Call(bmp)

	prev, _, _ := procSelectObject.Call(memDC, bmp)
	defer procSelectObject.Call(memDC, prev)

	r, _, _ := procBitBlt.Call(memDC, 0, 0, uintptr(w), uintptr(h),
		screenDC, uintptr(x), uintptr(y), srccopy|captureblt)
	if r == 0 {
		return nil, fmt.Errorf("capture: BitBlt failed")
	}

	var hdr bitmapInfoHeader
	hdr.Size = uint32(unsafe.Sizeof(hdr))
	hdr.Width = int32(w)
	hdr.Height = int32(-h) // top-down rows
	hdr.Planes = 1
	hdr.BitCount = 32
	hdr.Compression = 0 // BI_RGB

	buf := make([]byte, 4*w*h)
	res, _, _ := procGetDIBits.Call(memDC, bmp, 0, uintptr(h),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&hdr)), dibRGBColors)
	if res == 0 {
		return nil, fmt.Errorf("capture: GetDIBits failed")
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, j := 0, 0; i < len(buf); i, j = i+4, j+4 {
		img.Pix[j] = buf[i+2]   // R
		img.Pix[j+1] = buf[i+1] // G
		img.Pix[j+2] = buf[i]   // B
		img.Pix[j+3] = 255
	}
	return img, nil
}
