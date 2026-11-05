// The dialogs of the app all wear the same title bar, since none of them
// has the frame of Windows: this keeps them from drifting apart.

package main

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/ui"
)

// Each dialog shows its title and the close button of the shared header.
func TestDialogsShareTheTitleBar(t *testing.T) {
	settings = defaultSettings()
	t.Cleanup(func() { settings = defaultSettings() })

	cases := []struct {
		name       string
		view       func(*ui.Context)
		title      string
		closeLabel string
		w, h       int
	}{
		{
			"settings",
			(&settingsWindow{theme: themeIndex(settings.Theme), saved: settings}).view,
			appName, "关闭设置", settingsWinW, settingsWinH,
		},
		{
			"colour",
			(&colorWindow{col: color.RGBA{R: 0x20, G: 0x40, B: 0x60, A: 255}}).view,
			colorWinTitle, "关闭颜色拾取", colorWinW, colorWinH,
		},
		{
			"ocr",
			(&resultWindow{status: "done", text: "hello"}).view,
			"文字识别", "关闭文字识别", 620, 528,
		},
		{
			"translate",
			(&translationWindow{status: "done", source: "hello", result: "你好"}).view,
			"翻译", "关闭翻译", 780, 544,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tt := ui.NewTester(c.view, c.w, c.h)
			tt.Frame()
			for _, want := range []string{c.title, c.closeLabel} {
				if !tt.HasText(want) {
					t.Errorf("no %q in the window, have %q", want, tt.Texts())
				}
			}
			r, ok := tt.Find(c.closeLabel)
			if !ok {
				t.Fatalf("the close button has no box")
			}
			if int(r.X+r.W) > c.w || int(r.Y+r.H) > c.h {
				t.Errorf("the close button leaves the window: %v", r)
			}
		})
	}
}

// Escape closes a dialog without a frame to close it with the mouse, and the
// shortcut must not panic when the window is already gone.
func TestDialogsCloseOnEscape(t *testing.T) {
	for _, view := range []func(*ui.Context){
		(&resultWindow{status: "done"}).view,
		(&translationWindow{status: "done"}).view,
		(&colorWindow{col: color.RGBA{R: 1, G: 2, B: 3, A: 255}}).view,
	} {
		tt := ui.NewTester(view, 620, 560)
		tt.Frame()
		tt.Key(0, ui.KeyEscape)
	}
}
