//go:build windows

// A pin window is cut to the corners its image was cut with, with a window
// region: what lies outside them is not part of the window at all, so the
// desktop shows there without relying on a transparent window.

package main

var (
	procSetWindowRgn       = modUser32.NewProc("SetWindowRgn")
	procCreateRoundRectRgn = modGdi32.NewProc("CreateRoundRectRgn")
)

// roundWindow cuts hwnd, w by h physical pixels, to a rectangle with corners
// of radius: zero leaves it rectangular.
func roundWindow(hwnd uintptr, w, h int, radius float32) {
	if hwnd == 0 || w <= 0 || h <= 0 {
		return
	}
	var rgn uintptr
	if radius >= 1 {
		// A round rectangle region takes the width of the ellipses, which
		// is the diameter of the corners.
		d := uintptr(radius*2 + 0.5)
		rgn, _, _ = procCreateRoundRectRgn.Call(0, 0, uintptr(w)+1, uintptr(h)+1, d, d)
	}
	// SetWindowRgn owns the region from here: the system frees it with the
	// window, so it is not deleted on the way out.
	procSetWindowRgn.Call(hwnd, rgn, 1)
}
