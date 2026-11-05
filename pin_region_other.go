//go:build !windows

package main

// roundWindow does nothing here: only the Windows backend cuts a window to a
// shape, and the others keep their frames square.
func roundWindow(hwnd uintptr, w, h int, radius float32) {}
