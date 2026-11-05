//go:build !windows

// Placeholder screen capture for platforms where only the UI shell runs.

package main

import (
	"fmt"
	"image"
)

func captureScreenRect(x, y, w, h int) (*image.RGBA, error) {
	return nil, fmt.Errorf("截屏功能目前只实现了 Windows 版本")
}
