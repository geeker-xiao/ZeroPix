package main

import (
	"image"
	"testing"

	"github.com/egoist/mygo"
)

// The app has no panel of its own: everything is reachable from the tray
// icon, whose menu names the current hotkey and keeps the actions.
func TestTrayMenu(t *testing.T) {
	settings = defaultSettings()
	var labels []string
	for _, it := range app.menu().Items() {
		if it.Type == mygo.MenuItemSeparator {
			continue
		}
		if it.Role == mygo.RoleQuit {
			labels = append(labels, "quit")
			continue
		}
		labels = append(labels, it.Label)
	}
	want := []string{
		"区域截图  F1", "全屏截图  F3", "定时截图", "贴出剪贴板图片  F4", "颜色拾取  F6",
		"设置…", "quit",
	}
	if len(labels) != len(want) {
		t.Fatalf("menu items = %q, want %q", labels, want)
	}
	for i, w := range want {
		if labels[i] != w {
			t.Errorf("item %d = %q, want %q", i, labels[i], w)
		}
	}
	// The panel that used to hold these entries is gone for good.
	for _, gone := range []string{"显示面板", "截图窗口里可以做什么", "预加载文字识别模型"} {
		for _, l := range labels {
			if l == gone {
				t.Errorf("menu still offers %q", gone)
			}
		}
	}
}

func TestGroupLines(t *testing.T) {
	boxes := []image.Rectangle{
		{Min: image.Point{80, 10}, Max: image.Point{120, 30}}, // row 1 right
		{Min: image.Point{10, 10}, Max: image.Point{50, 30}},  // row 1 left
		{Min: image.Point{10, 60}, Max: image.Point{100, 80}}, // row 2
	}
	groups := groupLines(boxes)
	if len(groups) != 2 {
		t.Fatalf("groups = %v", groups)
	}
	if len(groups[0]) != 2 || groups[0][0].Min.X != 10 || groups[0][1].Min.X != 80 {
		t.Fatalf("row 1 not left-to-right: %v", groups[0])
	}
	if len(groups[1]) != 1 {
		t.Fatalf("row 2 = %v", groups[1])
	}
}

func TestCTCDecode(t *testing.T) {
	labels := []string{"", "你", "好", "", "世"} // index 0 and 3 are blanks
	// T=5 steps: 你 你 _ 好 世 -> "你好世"
	probs := []float32{
		0.0, 0.9, 0.0, 0.1, 0.0,
		0.0, 0.8, 0.1, 0.1, 0.0,
		0.1, 0.0, 0.0, 0.8, 0.1,
		0.0, 0.1, 0.8, 0.1, 0.0,
		0.0, 0.0, 0.1, 0.1, 0.8,
	}
	text, conf := ctcDecode(probs, 5, len(labels), labels)
	if text != "你好世" {
		t.Fatalf("ctcDecode text = %q", text)
	}
	if conf <= 0.5 || conf > 1 {
		t.Fatalf("ctcDecode conf = %f", conf)
	}
}

func TestBoxesFromProbMap(t *testing.T) {
	// A 20x10 map with one bright blob at (2..6, 3..6).
	w, h := 20, 10
	probs := make([]float32, w*h)
	for y := 3; y <= 6; y++ {
		for x := 2; x <= 6; x++ {
			probs[y*w+x] = 0.9
		}
	}
	boxes := boxesFromProbMap(probs, w, h)
	if len(boxes) != 1 {
		t.Fatalf("boxes = %v", boxes)
	}
	if boxes[0] != (image.Rect(2, 3, 7, 7)) {
		t.Fatalf("box = %v", boxes[0])
	}
}

func TestRoundUpTo(t *testing.T) {
	for in, want := range map[int]int{1: 8, 8: 8, 9: 16, 33: 40} {
		if got := roundUpTo(in, 8); got != want {
			t.Fatalf("roundUpTo(%d) = %d, want %d", in, got, want)
		}
	}
}
