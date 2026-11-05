// The settings dialog: one frameless window with a header of its own, three
// cards of rows, and the values stored in settings.json. Each row is a name
// and a muted line on the left, its control on the right.

package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	appName    = "ZeroPix"
	appSlug    = "zeropix"
	appTagline = "取景 · 标注 · 识别 · 贴图"

	appInk    = "#111820"
	appAccent = "#00C9A0"
	appPaper  = "#F5F7F9"
)

// legacySlugs are the names this app shipped under, newest first. Each one's
// data directory is harvested once so a rename keeps the user's preferences
// instead of starting over.
var legacySlugs = []string{"pixmint", "zoo_pix"}

//go:embed resources/icon-mark.png
var appMarkPNG []byte

// appSettings is what the dialog edits and settings.json keeps.
type appSettings struct {
	SnipHotkey string `json:"snipHotkey"` // region capture
	FullHotkey string `json:"fullHotkey"` // whole screen
	PinHotkey  string `json:"pinHotkey"`  // pin the clipboard
	PickHotkey string `json:"pickHotkey"` // pick a colour off the screen
	Translate  string `json:"translateService"`
	TargetLang string `json:"targetLang"`
	// The OpenAI compatible service the translation may go through: where
	// it answers, the model asked for, the token, and any extra headers as
	// "Name: Value" a line.
	OpenAIURL    string `json:"openaiUrl"`
	OpenAIModel  string `json:"openaiModel"`
	OpenAIKey    string `json:"openaiKey"`
	OpenAIHeader string `json:"openaiHeader"`
	Theme        string `json:"theme"` // system, dark or light
	AutoStart    bool   `json:"autoStart"`
	// Toolbar is the buttons the snip bar carries, by id, comma separated;
	// empty means the default set.
	Toolbar string `json:"toolbar"`
}

func defaultSettings() appSettings {
	return appSettings{
		SnipHotkey:  defaultSnipHotkey,
		FullHotkey:  defaultFullHotkey,
		PinHotkey:   defaultPinHotkey,
		PickHotkey:  defaultPickHotkey,
		Translate:   "未配置",
		TargetLang:  "简体中文",
		OpenAIURL:   defaultOpenAIURL,
		OpenAIModel: defaultOpenAIModel,
		Theme:       "system",
	}
}

var settings = defaultSettings()

// The choices of the dialog's selects and its segmented control. The
// interface itself is Chinese only, so there is no language choice.
var (
	translateServices = []string{"未配置", translateOpenAI}
	targetLanguages   = []string{"简体中文", "English"}
	themeLabels       = []string{"跟随系统", "深色", "浅色"}
	themeSources      = []string{"system", "dark", "light"}
)

// themeSegmentW is the width of each of the theme's three segments: one for
// all of them, so that the labels do not make the control uneven. It fits
// the longest label with room to spare.
const themeSegmentW = 88

// The shortcuts the app ships with; the dialog offers one field per action.
const (
	defaultSnipHotkey = "F1"
	defaultFullHotkey = "F3"
	defaultPinHotkey  = "F4"
	defaultPickHotkey = "F6"
)

// hotkeyAction is one shortcut: the row the dialog shows for it, the field
// it is stored in, and what pressing it does.
type hotkeyAction struct {
	name  string         // the row's name in the dialog
	label string         // what the field is called to assistive tech
	def   string         // the shortcut used when the stored one is taken
	value func() *string // the accelerator in settings
	run   func()
}

// hotkeyActions are the shortcuts in the order the dialog lists them.
var hotkeyActions = []hotkeyAction{
	{"区域截图", "区域截图快捷键输入", defaultSnipHotkey,
		func() *string { return &settings.SnipHotkey }, func() { startSnip(false) }},
	{"全屏截图", "全屏截图快捷键输入", defaultFullHotkey,
		func() *string { return &settings.FullHotkey }, func() { startSnip(true) }},
	{"贴出剪贴板图片", "贴图快捷键输入", defaultPinHotkey,
		func() *string { return &settings.PinHotkey }, pinFromClipboard},
	{"颜色拾取", "颜色拾取快捷键输入", defaultPickHotkey,
		func() *string { return &settings.PickHotkey }, startPick},
}

// ---------------------------------------------------------------------------
// settings.json

// settingsFileOverride points at the file in tests; empty means the user's
// data directory.
var settingsFileOverride string

func settingsFile() string {
	if settingsFileOverride != "" {
		return settingsFileOverride
	}
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil || dir == "" {
		if exe, err := os.Executable(); err == nil {
			dir = filepath.Dir(exe)
		} else {
			dir = "."
		}
	}
	return filepath.Join(dir, "settings.json")
}

// legacySettingsFiles are the settings files left by earlier names, newest
// first. Tests leave the override alone, so they never read a real one.
func legacySettingsFiles() []string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	var out []string
	for _, slug := range legacySlugs {
		out = append(out, filepath.Join(dir, slug, "settings.json"))
	}
	return out
}

// loadSettings reads settings.json over the defaults, keeping the defaults
// for whatever the file leaves out. A file left behind by the old name is
// copied over once, so renaming the app keeps the user's preferences.
func loadSettings() {
	settings = defaultSettings()
	path := settingsFile()
	data, err := os.ReadFile(path)
	if err != nil && settingsFileOverride == "" {
		for _, old := range legacySettingsFiles() {
			if old == path {
				continue
			}
			if stale, rerr := os.ReadFile(old); rerr == nil {
				data = stale
				_ = os.MkdirAll(filepath.Dir(path), 0o755)
				_ = os.WriteFile(path, stale, 0o644)
				break
			}
		}
	}
	if data == nil {
		return
	}
	var s appSettings
	if json.Unmarshal(data, &s) != nil {
		return
	}
	def := defaultSettings()
	if s.SnipHotkey == "" {
		s.SnipHotkey = def.SnipHotkey
	}
	if s.FullHotkey == "" {
		s.FullHotkey = def.FullHotkey
	}
	if s.PinHotkey == "" {
		s.PinHotkey = def.PinHotkey
	}
	if s.Translate == "" {
		s.Translate = def.Translate
	}
	// A service named by an older build is renamed, and the file is written
	// back so that it stops carrying the old name.
	service := s.Translate
	s.Translate = normalizeTranslate(service)
	if s.OpenAIURL == "" {
		s.OpenAIURL = def.OpenAIURL
	}
	if s.OpenAIModel == "" {
		s.OpenAIModel = def.OpenAIModel
	}
	if s.TargetLang == "" {
		s.TargetLang = def.TargetLang
	}
	if s.Theme == "" {
		s.Theme = def.Theme
	}
	settings = s
	if settings.Translate != service {
		_ = settings.save()
	}
}

func (s appSettings) save() error {
	path := settingsFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// applyTheme makes the window chrome follow the stored choice.
func (s appSettings) applyTheme() {
	switch s.Theme {
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// themeIndex is where theme sits in themeSources.
func themeIndex(theme string) int {
	for i, s := range themeSources {
		if s == theme {
			return i
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// The window

// The dialog is a page of its own: a column of section names on the left,
// the section itself on the right, as wide as a dialog can afford.
const (
	settingsWinW = 720
	settingsWinH = 640
	settingsNavW = 168
)

// settingsSection is one entry of the left column: its icon, the name it
// goes by in the column and the card title it opens.
type settingsSection struct {
	icon string
	name string
}

// settingsSections are the sections in the order the column lists them.
var settingsSections = []settingsSection{
	{"fullscreen", "快捷操作"},
	{"settings", "工具栏"},
	{"keyboard", "快捷键"},
	{"scan", "OCR 与翻译"},
	{"palette", "外观与启动"},
	{"info", "关于软件"},
}

type settingsWindow struct {
	win   *mygo.Window
	theme int // the segmented control's choice, as themeSources indexes it
	saved appSettings
	// section is the entry of the left column that is open.
	section int
	// recRow is the hotkey row that takes the next combination, plus one;
	// 0 means none of them is recording.
	recRow int
	recTxt string
	note   string
	// noteOK turns the note green on success.
	noteOK bool
	// aiBusy is true while the test request of the OpenAI row is in flight,
	// and aiNote says how it went.
	aiBusy bool
	aiNote string
	aiOK   bool
	// lastTranslate is the service the page was last built for, so that
	// choosing another one brings its rows without a click elsewhere.
	lastTranslate string
	// hidden records that the dialog was up when a snip started, so that it
	// comes back when the overlay closes.
	hidden bool
}

var settingsWin *settingsWindow

// openSettings shows the dialog, or brings up the one already open.
func openSettings() {
	if w := settingsWin; w != nil {
		if w.win != nil && !w.win.IsDestroyed() {
			w.win.Show()
			w.win.Focus()
			return
		}
		settingsWin = nil
	}
	w := &settingsWindow{theme: themeIndex(settings.Theme), saved: settings}
	w.win = dialogWindow(appName+" 设置", settingsWinW, settingsWinH, 620, 480, "", w.view)
	if w.win == nil || w.win.IsDestroyed() {
		app.toast("无法打开设置")
		return
	}
	settingsWin = w
	w.win.Focus()
}

func (w *settingsWindow) close() {
	if w.win != nil && !w.win.IsDestroyed() {
		w.win.Destroy()
	}
	if settingsWin == w {
		settingsWin = nil
	}
}

// hideForShot takes the dialog away so that it is not in the screenshot.
func (w *settingsWindow) hideForShot() {
	if w.win == nil || w.win.IsDestroyed() || !w.win.IsVisible() {
		return
	}
	w.hidden = true
	w.win.Hide()
}

// restoreAfterShot puts back a dialog that a snipping session took away.
func (w *settingsWindow) restoreAfterShot() {
	if !w.hidden {
		return
	}
	w.hidden = false
	if w.win != nil && !w.win.IsDestroyed() {
		w.win.Show()
	}
}

func (w *settingsWindow) view(c *ui.Context) {
	t := c.Theme()
	c.OnShortcut(0, ui.KeyEscape, func() {
		if w.recRow != 0 { // the first Escape leaves the hotkey field
			w.recRow, w.recTxt, w.note, w.noteOK = 0, "", "", false
			return
		}
		w.close()
	})
	w.syncControls()
	ui.Column(c).Fill().Background(pageBackground(t)).Children(func() {
		w.header(c, t)
		ui.Row(c).Fill().Grow(1).Children(func() {
			w.nav(c, t)
			w.page(c, t)
		})
	})
}

// nav is the brand column: the mark above, then the numbered sections. It
// stays ink in both appearances, so the dialog is recognisable at a glance.
func (w *settingsWindow) nav(c *ui.Context, t *ui.Theme) {
	ui.Column(c).Width(settingsNavW).FillHeight().Shrink(0).Gap(0).
		Background(inkColor()).Children(func() {
		ui.Row(c).FillWidth().Padding(18, 14).Gap(10).AlignItems(ui.Center).Children(func() {
			if bmp := appIconBitmap(); bmp != nil {
				ui.Image(c, bmp).Width(32).Height(32)
			}
			ui.Column(c).Grow(1).Gap(3).Children(func() {
				ui.Text(c, appName).FontSize(16).Bold().TextColor(onInkColor())
				ui.Text(c, "SCREEN TOOLKIT").FontSize(10.5).TextColor(onInkMutedColor())
			})
		})
		ui.Box(c).FillWidth().Height(1).Background(onInkSoftColor())
		ui.Box(c).Height(10)
		ui.Column(c).FillWidth().Padding(0, 8).Gap(3).Children(func() {
			for i, sec := range settingsSections {
				idx, s := i, sec
				open := w.section == idx
				b := ui.ButtonBase(c).FillWidth().Height(38).Radius(8).
					Label("转到" + s.name)
				if open {
					b.Background(onInkSoftColor())
				} else if b.Hovered() {
					b.Background(ui.RGBA(255, 255, 255, 0.045))
				}
				if b.Clicked() {
					w.section = idx
					w.refresh()
				}
				b.Children(func() {
					col := onInkMutedColor()
					if open {
						col = onInkColor()
					}
					ui.Row(c).FillWidth().Padding(10, 0).Gap(8).AlignItems(ui.Center).Children(func() {
						bar := ui.Box(c).Width(3).Height(18).Radius(2)
						if open {
							bar.Background(accentColor())
						}
						ui.Text(c, fmt.Sprintf("%02d", idx+1)).FontSize(11).
							Font("Consolas, 微软雅黑, monospace").TextColor(ui.RGB(0x7E, 0xAB, 0xA1))
						ui.Box(c).Width(15).Height(15).Draw(func(p *ui.Painter, r ui.Rect) {
							ic := onInkMutedColor()
							if open {
								ic = accentColor()
							}
							drawIcon(p, s.icon, r, ic)
						})
						ui.Text(c, s.name).FontSize(12.5).TextColor(col)
					})
				})
			}
		})
		ui.Box(c).Grow(1)
		ui.Row(c).FillWidth().Padding(14, 16).Gap(7).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Width(6).Height(6).Radius(3).Background(accentColor())
			ui.Text(c, "本机运行 · 无需联网").FontSize(10.5).TextColor(onInkMutedColor())
		})
	})
}

// page is the open section: one sheet of paper, scrolled on its own.
func (w *settingsWindow) page(c *ui.Context, t *ui.Theme) {
	ui.Scroll(c).FillHeight().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(16).Padding(20).Children(func() {
			switch w.section {
			case 0:
				w.actionsCard(c, t)
			case 1:
				w.card(c, t, "settings", "工具栏", func() { w.toolbarCard(c, t) })
			case 2:
				w.hotkeysCard(c, t)
			case 3:
				w.ocrCard(c, t)
			case 4:
				w.appearanceCard(c, t)
			case 5:
				w.aboutCard(c, t)
			}
		})
	})
}

// actionsCard is the first section: the things the app does, one press
// each, since the dialog itself is just an entry point.
// actionsCard is the first section. One capture is the protagonist; the
// other six are peers in a grid, instead of seven equal-height full-width
// buttons that give the eye nothing to land on.
func (w *settingsWindow) actionsCard(c *ui.Context, t *ui.Theme) {
	w.card(c, t, "fullscreen", "快捷操作", func() {
		actions := []struct {
			label   string
			primary bool
			act     func()
		}{
			{"区域截图  " + settings.SnipHotkey, true, func() { startSnip(false) }},
			{"全屏截图  " + settings.FullHotkey, false, func() { startSnip(true) }},
			{"贴出剪贴板图片  " + settings.PinHotkey, false, pinFromClipboard},
			{"颜色拾取  " + settings.PickHotkey, false, startPick},
			{"识别剪贴板图片", false, func() { ocrClipboard(false) }},
			{"翻译剪贴板图片", false, func() { ocrClipboard(true) }},
			{"预加载文字识别模型", false, loadOCRModel},
		}
		main := actions[0]
		mb := ui.ButtonBase(c).FillWidth().Height(54).Radius(10).
			Label(main.label).Background(inkColor())
		if mb.Hovered() {
			mb.Background(inkHoverColor())
		}
		if mb.Clicked() {
			main.act()
		}
		mb.Children(func() {
			ui.Row(c).Fill().Padding(14, 0).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Width(3).Height(24).Radius(2).Background(accentColor())
				ui.Column(c).Grow(1).Gap(3).Children(func() {
					ui.Text(c, main.label).FontSize(14).Bold().TextColor(onInkColor())
					ui.Text(c, "随时按热键也能唤起").FontSize(11.5).TextColor(onInkMutedColor())
				})
				ui.Box(c).Width(22).Height(22).Draw(func(p *ui.Painter, r ui.Rect) {
					drawIcon(p, "fullscreen", r, accentColor())
				})
			})
		})
		ui.Box(c).Height(2)
		for i := 1; i < len(actions); i += 2 {
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				for j := i; j < min(i+2, len(actions)); j++ {
					a := actions[j]
					b := ui.ButtonBase(c).Grow(1).Height(34).Radius(8).Label(a.label).
						Background(t.Surface).Border(1, groupBorder(t))
					if b.Hovered() {
						b.Background(t.SurfaceHover)
					}
					if b.Clicked() {
						a.act()
					}
					b.Children(func() {
						ui.Row(c).Fill().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
							ui.Text(c, a.label).FontSize(12).TextColor(t.Text)
						})
					})
				}
				if i+1 >= len(actions) {
					ui.Box(c).Grow(1)
				}
			})
		}
		ui.Text(c, "模型与词典在程序目录的 ocr/ 文件夹").
			FontSize(11.5).TextColor(mutedOnPaper(t))
		if n := app.noteText(); n != "" {
			ui.Text(c, n).FontSize(11).TextColor(mutedOnPaper(t))
		}
	})
}

// hotkeysCard is the section of shortcuts, one field each.
// resetHotkeys puts every shortcut back to its shipped key and re-registers
// them, so a wrong combination can always be undone from this page.
func resetHotkeys() {
	for _, act := range hotkeyActions {
		cur := *act.value()
		if cur == act.def {
			continue
		}
		mygo.GlobalShortcut.Unregister(cur)
		*act.value() = act.def
		_ = mygo.GlobalShortcut.Register(act.def, act.run)
	}
	_ = settings.save()
	if app.tray != nil {
		app.tray.SetMenu(app.menu())
	}
}

// hotkeysCard is the section of shortcuts, with feedback where it is used.
func (w *settingsWindow) hotkeysCard(c *ui.Context, t *ui.Theme) {
	w.card(c, t, "keyboard", "快捷键", func() {
		for i, a := range hotkeyActions {
			w.row(c, t, a.name, "点击输入框后直接按下组合键", func() {
				w.hotkeyField(c, t, i)
			})
		}
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "录入时按 Esc 取消 · 冲突或无效的组合键会给出原因").
				Grow(1).FontSize(11.5).TextColor(mutedOnPaper(t))
			ui.Button(c, "恢复默认").Height(28).OnClick(func() {
				resetHotkeys()
				w.note, w.noteOK = "已恢复默认快捷键", true
			})
		})
		if w.note != "" {
			col := t.Danger
			if w.noteOK {
				col = accentInkColor()
			}
			ui.Text(c, w.note).FontSize(12).TextColor(col)
		}
	})
}

// ocrCard is where the recognition and translation services are chosen.
func (w *settingsWindow) ocrCard(c *ui.Context, t *ui.Theme) {
	w.card(c, t, "scan", "OCR 与翻译", func() {
		w.row(c, t, "翻译服务", "不配置时使用内置离线词典", func() {
			ui.Select(c, &settings.Translate, translateServices).Width(168)
		})
		w.row(c, t, "目标语言", "单词卡片不受此项影响", func() {
			ui.Select(c, &settings.TargetLang, targetLanguages).Width(168)
		})
		if w.lastTranslate != settings.Translate {
			w.lastTranslate = settings.Translate
			w.refresh()
		}
		if settings.Translate == translateOpenAI {
			w.openAIRows(c, t)
		}
		w.hint(c, t, "OCR 使用内置 PP-OCRv5 模型与 ECDICT 常用词库。")
	})
}

// openAIRows are the settings of the OpenAI compatible service, shown only
// while it is the chosen one.
func (w *settingsWindow) openAIRows(c *ui.Context, t *ui.Theme) {
	ui.Divider(c)
	w.row(c, t, "接口地址", "到 /chat/completions 的完整地址", func() {
		ui.TextInput(c, &settings.OpenAIURL).Width(268).Height(30)
	})
	w.row(c, t, "模型", "服务端支持的模型名", func() {
		ui.TextInput(c, &settings.OpenAIModel).Width(232).Height(30)
	})
	w.row(c, t, "Token", "作为 Bearer 发送，可留空", func() {
		ui.TextInput(c, &settings.OpenAIKey).Width(232).Height(30).Password()
	})
	w.row(c, t, "自定义 Header", "每行一条 Name: Value", func() {
		ui.TextArea(c, &settings.OpenAIHeader).Width(232).Height(58).
			Font("Consolas, 微软雅黑, monospace").FontSize(11.5)
	})
	w.row(c, t, "测试效果", "用当前配置发一次最小请求", func() {
		label := "测试"
		if w.aiBusy {
			label = "测试中…"
		}
		ui.Button(c, label).Width(96).Height(28).Disabled(w.aiBusy).OnClick(w.testAI)
	})
	if w.aiNote != "" {
		col := t.TextMuted
		if !w.aiOK {
			col = t.Danger
		}
		ui.Row(c).FillWidth().Children(func() {
			ui.Box(c).Width(3).FillHeight().Background(accentColor())
			ui.Box(c).FillWidth().Padding(9, 10).Children(func() {
				ui.Text(c, w.aiNote).FontSize(11.5).TextColor(col)
			})
		})
	}
}

// testAI asks the service for one short answer, so that a wrong address or
// token shows up here instead of at the first translation.
func (w *settingsWindow) testAI() {
	if w.aiBusy {
		return
	}
	w.aiBusy, w.aiNote, w.aiOK = true, "", false
	w.refresh()
	cfg := settings
	go func() {
		start := time.Now()
		_, err := aiChat(cfg, "你是一个连通性测试。", "只回复两个字：正常")
		w.aiBusy = false
		w.aiOK = err == nil
		if err != nil {
			w.aiNote = "失败：" + err.Error()
		} else {
			w.aiNote = "连接正常，用时 " + shortDuration(time.Since(start))
		}
		w.refresh()
	}()
}

// appearanceCard is the theme and the start-at-login switch.
func (w *settingsWindow) appearanceCard(c *ui.Context, t *ui.Theme) {
	w.card(c, t, "palette", "外观与启动", func() {
		w.row(c, t, "主题", "跟随系统或固定深浅色", func() {
			w.themeSwitch(c)
		})
		w.row(c, t, "开机自启", "登录系统后自动驻留托盘", func() {
			ui.Switch(c, &settings.AutoStart)
		})
	})
}

// aboutCard says where the app keeps its files and what it is written in.
// Nothing about the app itself: the header above already says its name.
func (w *settingsWindow) aboutCard(c *ui.Context, t *ui.Theme) {
	w.card(c, t, "info", "关于软件", func() {
		configDir := filepath.Dir(settingsFile())
		w.row(c, t, "配置文件", configDir, func() {
			ui.Button(c, "打开目录").Height(28).OnClick(func() { openFolder(configDir) })
		})
		w.row(c, t, "识别资源", assetDir, func() {
			ui.Button(c, "打开目录").Height(28).OnClick(func() { openFolder(assetDir) })
		})
		w.row(c, t, "开发语言", goVersion(), func() {})
	})
}

// refresh asks for a frame after the dialog changed what it shows.
func (w *settingsWindow) refresh() {
	if w.win != nil && !w.win.IsDestroyed() {
		w.win.Update(func() {})
	}
}

// openFolder shows dir in the system's file manager.
func openFolder(dir string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		app.toast("无法打开目录: " + err.Error())
	}
}

// toolbarCard lists every button the bar can carry, with the three presets
// above them. A change is stored the next time the dialog syncs.
// toolbarCard lists every button the bar can carry, with the three presets
// above them. The grid is tight enough for the whole catalogue at 640 DIP.
func (w *settingsWindow) toolbarCard(c *ui.Context, t *ui.Theme) {
	ui.Text(c, "勾选截图工具条上出现的按钮，改动立即生效").FontSize(12).TextColor(mutedOnPaper(t))
	ui.Row(c).Gap(8).Children(func() {
		for _, p := range barPresets {
			pr := p
			ui.Button(c, pr.name).Height(26).OnClick(func() { applyBarPreset(pr.name) })
		}
	})
	ids := enabledBarIDs()
	ui.Column(c).FillWidth().Gap(3).Children(func() {
		for i := 0; i < len(barCatalog); i += barChecksPerRow {
			line := barCatalog[i:min(i+barChecksPerRow, len(barCatalog))]
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				for _, it := range line {
					w.barCheck(c, t, it, barIDsContain(ids, it.id))
				}
				for j := len(line); j < barChecksPerRow; j++ {
					ui.Box(c).Grow(1)
				}
			})
		}
	})
}

// barChecksPerRow is how many checkboxes share a line of the card.
const barChecksPerRow = 3

// barCheck is one button of the bar as a checkbox: the box on the left, the
// name on the right, the whole line clickable.
func (w *settingsWindow) barCheck(c *ui.Context, t *ui.Theme, it *barItem, on bool) {
	b := ui.ButtonBase(c).Grow(1).Height(27).Radius(7).
		Label("工具条：" + it.name).Tooltip(it.tip)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	if b.Clicked() {
		setBarEnabled(it.id, !on)
		w.refresh()
	}
	b.Children(func() {
		ui.Row(c).FillWidth().Padding(9, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Width(15).Height(15).Draw(func(p *ui.Painter, r ui.Rect) {
				if on {
					p.Fill(r, accentColor(), 4)
					tick := ui.Rect{X: r.X + 2, Y: r.Y + 2, W: r.W - 4, H: r.H - 4}
					drawIcon(p, "check", tick, inkColor())
					return
				}
				p.Stroke(r, t.Border, 4, 1.2)
			})
			ui.Text(c, it.name).FontSize(12.5).SingleLine()
		})
	})
}

// barIDsContain reports whether id is in ids.
func barIDsContain(ids []string, id string) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

// setBarEnabled adds or removes one button from the bar.
func setBarEnabled(id string, on bool) {
	ids := append([]string(nil), enabledBarIDs()...)
	kept := ids[:0]
	for _, v := range ids {
		if v != id {
			kept = append(kept, v)
		}
	}
	if on {
		kept = append(kept, id)
	}
	settings.Toolbar = strings.Join(kept, ",")
}

// applyBarPreset switches the bar to one of the presets.
func applyBarPreset(name string) {
	for _, p := range barPresets {
		if p.name != name {
			continue
		}
		if p.ids == nil {
			all := make([]string, 0, len(barCatalog))
			for _, it := range barCatalog {
				all = append(all, it.id)
			}
			settings.Toolbar = strings.Join(all, ",")
			return
		}
		settings.Toolbar = strings.Join(p.ids, ",")
		return
	}
}

// themeSwitch is the three-way choice of the theme. The segments keep one
// width, unlike the labels, so the control stays even.
func (w *settingsWindow) themeSwitch(c *ui.Context) {
	ui.ToggleGroup(c, func() {
		for i, label := range themeLabels {
			on := w.theme == i
			seg := ui.Toggle(c, &on, label).Width(themeSegmentW).Height(28)
			if seg.Changed() {
				w.theme = i
				settings.Theme = themeSources[i]
				settings.applyTheme()
			}
		}
	})
}

// syncControls applies what the controls changed this frame and stores it.
func (w *settingsWindow) syncControls() {
	if w.theme >= 0 && w.theme < len(themeSources) {
		settings.Theme = themeSources[w.theme]
	}
	if settings == w.saved {
		return
	}
	if settings.Theme != w.saved.Theme {
		settings.applyTheme()
	}
	if settings.AutoStart != w.saved.AutoStart {
		if err := setAutoStart(settings.AutoStart); err != nil {
			settings.AutoStart = w.saved.AutoStart
			app.toast("开机自启设置失败: " + err.Error())
		}
	}
	if err := settings.save(); err != nil {
		app.toast("保存设置失败: " + err.Error())
	}
	w.saved = settings
}

func (w *settingsWindow) header(c *ui.Context, t *ui.Theme) {
	dialogHeader(c, t, "app", appName, appTagline, "关闭设置", w.close)
}

// card is a white rounded group with an icon and a title.
// sectionSubtitle gives each sheet one concrete line under its title.
func sectionSubtitle(title string) string {
	switch title {
	case "快捷操作":
		return "一键触发，不用离开当前窗口"
	case "工具栏":
		return "决定截图时浮出的那一排按钮"
	case "快捷键":
		return "全局生效，按下后立即保存"
	case "OCR 与翻译":
		return "模型在程序内，翻译可接外部服务"
	case "外观与启动":
		return "主题与登录时的行为"
	case "关于软件":
		return "文件位置与构建信息"
	}
	return ""
}

// card is one sheet: a numbered-looking header, the accent rule that signs
// the brand, a hairline, then the rows. There is one per page, not a stack.
func (w *settingsWindow) card(c *ui.Context, t *ui.Theme, icon, title string, body func()) {
	ui.Column(c).FillWidth().Radius(12).Background(t.Surface).
		Border(1, groupBorder(t)).Padding(18, 16).Gap(0).Children(func() {
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Width(30).Height(30).Radius(9).
				Background(accentSoftColor(t)).Children(func() {
				ui.Box(c).Fill().Padding(7).Draw(func(p *ui.Painter, r ui.Rect) {
					ic := accentInkColor()
					if t.Dark {
						ic = accentColor()
					}
					drawIcon(p, icon, r, ic)
				})
			})
			ui.Column(c).Grow(1).Gap(3).Children(func() {
				ui.Text(c, title).FontSize(16).Bold()
				ui.Text(c, sectionSubtitle(title)).FontSize(11.5).TextColor(mutedOnPaper(t))
			})
		})
		ui.Box(c).Width(32).Height(3).Radius(2).MarginY(13).Background(accentColor())
		ui.Divider(c)
		ui.Box(c).Height(13)
		ui.Column(c).FillWidth().Gap(9).Children(body)
	})
}

// row is a setting: its name over a muted line, its control on the right.
// row is a setting: its name over a muted line, its control on the right,
// and a hairline that keeps a long page readable as a table.
func (w *settingsWindow) row(c *ui.Context, t *ui.Theme, name, desc string, control func()) {
	ui.Column(c).FillWidth().Gap(0).Children(func() {
		ui.Row(c).FillWidth().Gap(16).Padding(3, 2).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Gap(3).Children(func() {
				ui.Text(c, name).FontSize(13).Bold()
				ui.Text(c, desc).FontSize(11.5).TextColor(mutedOnPaper(t))
			})
			control()
		})
		ui.Box(c).FillWidth().Height(1).MarginY(7).Background(groupBorder(t))
	})
}

// hint is the full width note at the bottom of a group.
// hint is a note with the accent down its left edge.
// hint is a note with the accent down its left edge and no second fill.
func (w *settingsWindow) hint(c *ui.Context, t *ui.Theme, text string) {
	ui.Row(c).FillWidth().Children(func() {
		ui.Box(c).Width(3).FillHeight().Background(accentColor())
		ui.Box(c).FillWidth().Padding(9, 10).Children(func() {
			ui.Text(c, text).FontSize(11.5).TextColor(mutedOnPaper(t))
		})
	})
}

// hotkeyField is the input of one shortcut that takes the next combination
// pressed in it.
func (w *settingsWindow) hotkeyField(c *ui.Context, t *ui.Theme, row int) {
	act := hotkeyActions[row]
	recording := w.recRow == row+1
	label, muted := *act.value(), false
	if recording {
		label, muted = w.recTxt, true
		if label == "" {
			label = "按下组合键…"
		}
	}
	b := ui.ButtonBase(c).Width(168).Height(32).Radius(8).Label(act.label).
		Tooltip("点击后直接按下组合键").
		// Focusable: HandleInput sees keys only while the element has the
		// keyboard focus, which Focus asks for on the click below.
		Focusable().FocusRing(true)
	if b.Clicked() {
		b.Focus()
		w.recRow, w.recTxt, w.note, w.noteOK = row+1, "", "", false
	}
	b.HandleInput(func(ev ui.InputEvent) bool { return w.recordKey(row, ev) })
	b.Background(t.Surface)
	if recording {
		b.Border(1, accentColor())
	} else {
		b.Border(1, t.Border)
		if b.Hovered() {
			b.Background(t.SurfaceHover)
		}
	}
	b.Children(func() {
		// Centred on both axes, like the buttons above: the padding only
		// keeps a long combination off the border.
		ui.Row(c).Fill().Padding(8, 0).Justify(ui.Center).AlignItems(ui.Center).
			Children(func() {
				col := t.Text
				if muted {
					col = t.TextMuted
				}
				ui.Text(c, label).FontSize(12.5).TextColor(col)
			})
	})
}

// recordKey turns the next key pressed in the field of row into that
// shortcut.
func (w *settingsWindow) recordKey(row int, ev ui.InputEvent) bool {
	if w.recRow != row+1 || ev.Kind != ui.InputKeyDown {
		return false
	}
	if ev.Key == ui.KeyEscape { // give up on the field, keep the hotkey
		w.recRow, w.recTxt, w.note, w.noteOK = 0, "", "", false
		return true
	}
	name := accelKeyName(ev.Key)
	if name == "" { // a modifier on its own, waiting for the rest
		w.recTxt = modsLabel(ev.Mods)
		return true
	}
	acc := accelString(ev.Mods, name)
	if !accelUsable(ev.Mods, name) {
		w.recTxt = acc
		w.note, w.noteOK = "全局快捷键需要配合 Ctrl / Alt / Win 使用", false
		return true
	}
	if err := setHotkey(row, acc); err != nil {
		w.recTxt = acc
		w.note, w.noteOK = err.Error(), false
		return true
	}
	w.note, w.noteOK = "已保存 · "+acc+"，下次按这个键即可唤起", true
	w.recRow, w.recTxt = 0, ""
	return true
}

// pageBackground is the soft gray the cards sit on, which the theme's own
// background does not give in a light appearance: it is white, as they are.
func pageBackground(t *ui.Theme) ui.Color {
	if t.Dark {
		return t.Background
	}
	return ui.RGB(0xF5, 0xF7, 0xF9)
}

// groupBorder outlines the cards: a hairline that suits either appearance.
func groupBorder(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.08)
	}
	return ui.RGBA(0x11, 0x18, 0x20, 0.07)
}

// hintFill tints the notes of a group.
func hintFill(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.05)
	}
	return ui.RGBA(0, 0, 0, 0.035)
}

// ---------------------------------------------------------------------------
// The application icon

var (
	appIconOnce sync.Once
	appIconBmp  *ui.Bitmap
)

func appIconBitmap() *ui.Bitmap {
	appIconOnce.Do(func() {
		img, err := png.Decode(bytes.NewReader(appMarkPNG))
		if err != nil {
			return
		}
		bounds := img.Bounds()
		rgba := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		for y := 0; y < bounds.Dy(); y++ {
			for x := 0; x < bounds.Dx(); x++ {
				rgba.Set(x, y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		appIconBmp = ui.NewBitmap(rgba)
	})
	return appIconBmp
}

// ---------------------------------------------------------------------------
// Accelerators: the field's keys as the strings GlobalShortcut takes.

// accelKeyNames maps the keys worth registering; "" means the key cannot be
// one, as with the modifiers.
var accelKeyNames = buildAccelKeyNames()

func buildAccelKeyNames() map[ui.Key]string {
	m := map[ui.Key]string{
		ui.KeyEnter: "Enter", ui.KeyTab: "Tab", ui.KeySpace: "Space",
		ui.KeyBackspace: "Backspace", ui.KeyDelete: "Delete", ui.KeyInsert: "Insert",
		ui.KeyHome: "Home", ui.KeyEnd: "End", ui.KeyPageUp: "PageUp",
		ui.KeyPageDown: "PageDown", ui.KeyUp: "Up", ui.KeyDown: "Down",
		ui.KeyLeft: "Left", ui.KeyRight: "Right",
		ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeyComma: ",", ui.KeyPeriod: ".",
		ui.KeySlash: "/", ui.KeySemicolon: ";", ui.KeyQuote: "'",
		ui.KeyBracketLeft: "[", ui.KeyBracketRight: "]", ui.KeyBackslash: "\\",
		ui.KeyBackquote: "`",
	}
	for i := 0; i < 12; i++ {
		m[ui.KeyF1+ui.Key(i)] = "F" + itoa(i+1)
	}
	for i := 0; i < 10; i++ {
		m[ui.Key0+ui.Key(i)] = itoa(i)
	}
	for i := 0; i < 26; i++ {
		m[ui.KeyA+ui.Key(i)] = string(rune('a' + i))
	}
	return m
}

// accelKeyName is the key inside an accelerator, or "" for keys that cannot
// be part of one.
func accelKeyName(k ui.Key) string { return accelKeyNames[k] }

// accelString joins modifiers and a key as GlobalShortcut spells them. A
// single character reads better in upper case, and parses the same.
func accelString(mods ui.Modifiers, key string) string {
	if len([]rune(key)) == 1 {
		key = strings.ToUpper(key)
	}
	var parts []string
	if mods&ui.Super != 0 {
		parts = append(parts, "Super")
	}
	if mods&ui.Ctrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if mods&ui.Alt != 0 {
		parts = append(parts, "Alt")
	}
	if mods&ui.Shift != 0 {
		parts = append(parts, "Shift")
	}
	return strings.Join(append(parts, key), "+")
}

// modsLabel is a combination of modifiers alone, as the field shows it while
// it waits for a key.
func modsLabel(mods ui.Modifiers) string {
	parts := make([]string, 0, 4)
	for _, m := range []struct {
		mod  ui.Modifiers
		name string
	}{{ui.Super, "Win"}, {ui.Ctrl, "Ctrl"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}} {
		if mods&m.mod != 0 {
			parts = append(parts, m.name)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " + ")
}

// accelUsable reports whether a combination is fit to be a global shortcut:
// on its own a letter or a digit would take the key away from every app.
func accelUsable(mods ui.Modifiers, key string) bool {
	if mods&(ui.Ctrl|ui.Alt|ui.Super) != 0 {
		return true
	}
	return strings.HasPrefix(key, "F") && len(key) > 1
}
