// The recognition window: shows the text found in the image while it is
// still being recognized, lets the user edit and copy it, and can render the
// dictionary translation of it beside the original.

package main

import (
	"image"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type resultWindow struct {
	win       *mygo.Window
	status    string // busy | done | error
	errMsg    string
	lines     []ocrLine
	text      string
	trans     string
	transErr  string
	showTrans bool
}

// showOCRResult opens a window and recognizes img on a background goroutine.
func showOCRResult(img *image.RGBA) {
	rw := &resultWindow{status: "busy"}
	rw.win = dialogWindow("文字识别", 620, 528, 420, 360, "ocr-result-2", rw.view)
	go func() {
		lines, err := ocrEng.Recognize(img)
		if rw.win == nil || rw.win.IsDestroyed() {
			return
		}
		rw.win.Update(func() {
			if err != nil {
				rw.status, rw.errMsg = "error", err.Error()
				return
			}
			rw.lines = lines
			rw.text = joinLines(lines)
			rw.status = "done"
		})
	}()
}

func (rw *resultWindow) close() {
	if rw.win != nil && !rw.win.IsDestroyed() {
		rw.win.Destroy()
	}
}

func (rw *resultWindow) view(c *ui.Context) {
	t := c.Theme()
	c.OnShortcut(ui.Ctrl, ui.KeyC, func() { mygo.Clipboard.WriteText(rw.text) })
	c.OnShortcut(0, ui.KeyEscape, rw.close)
	ui.Column(c).Fill().Background(pageBackground(t)).Children(func() {
		dialogHeader(c, t, "scan", "文字识别",
			"在图片里找出的文字，可以直接改", "关闭文字识别", rw.close)
		dialogBody(c, t, func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				switch rw.status {
				case "busy":
					ui.Text(c, "正在识别…").Grow(1).TextColor(t.TextMuted)
					ui.Progress(c, -1).Width(140)
				case "error":
					ui.Text(c, "识别失败: "+rw.errMsg).Grow(1).TextColor(t.Danger)
				default:
					ui.Textf(c, "识别到 %d 行，可直接编辑", len(rw.lines)).Grow(1).TextColor(t.TextMuted)
				}
				ui.Button(c, "翻译").OnClick(rw.translate).Disabled(rw.status != "done")
				brandPrimaryButton(c, "复制全部", 120, rw.status != "done", func() {
					mygo.Clipboard.WriteText(rw.text)
					app.toast("已复制的文字已放进剪贴板")
				})
			})
			if rw.showTrans {
				ui.Row(c).Fill().Grow(1).Gap(12).Children(func() {
					rw.textPane(c, "识别文字")
					ui.Column(c).Fill().Grow(1).Gap(6).Children(func() {
						ui.Text(c, "词典释义").Bold().FontSize(12)
						if rw.transErr != "" {
							ui.Text(c, rw.transErr).TextColor(t.Danger).FontSize(12)
						}
						ui.TextArea(c, &rw.trans).Fill().Grow(1).
							Font("Consolas, 微软雅黑, monospace")
					})
				})
				return
			}
			rw.textPane(c, "")
		})
	})
}

func (rw *resultWindow) textPane(c *ui.Context, title string) {
	ui.Column(c).Fill().Grow(1).Gap(6).Children(func() {
		if title != "" {
			ui.Text(c, title).Bold().FontSize(12)
		}
		ui.TextArea(c, &rw.text).Fill().Grow(1).
			Placeholder("识别结果会显示在这里…").
			Font("Consolas, 微软雅黑, monospace")
	})
}

// translate renders the dictionary senses of the recognized text.
func (rw *resultWindow) translate() {
	rw.showTrans = true
	out, err := translateText(rw.text)
	if err != nil {
		rw.trans, rw.transErr = "", err.Error()
	} else {
		rw.trans, rw.transErr = out, ""
	}
	_ = strings.TrimSpace(rw.trans)
	rw.win.Update(func() {})
}
