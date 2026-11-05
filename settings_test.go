package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The keys a hotkey field can record map to the strings GlobalShortcut
// takes; the ones that cannot are left out.
func TestAccelKeys(t *testing.T) {
	cases := map[ui.Key]string{
		ui.KeyF1: "F1", ui.KeyF5: "F5", ui.KeyF12: "F12",
		ui.KeyA: "a", ui.KeyZ: "z", ui.Key3: "3",
		ui.KeyEnter: "Enter", ui.KeySlash: "/", ui.KeyPageDown: "PageDown",
	}
	for k, want := range cases {
		if got := accelKeyName(k); got != want {
			t.Errorf("accelKeyName(%v) = %q, want %q", k, got, want)
		}
	}
	if got := accelKeyName(ui.KeyEscape); got != "" {
		t.Errorf("Escape must stay free for the field, got %q", got)
	}
	if got := accelString(ui.Ctrl|ui.Shift, "a"); got != "Ctrl+Shift+A" {
		t.Errorf("accelString = %q", got)
	}
	if got := accelString(ui.Ctrl|ui.Alt|ui.Super, "4"); got != "Super+Ctrl+Alt+4" {
		t.Errorf("accelString = %q", got)
	}
}

// A global shortcut needs a modifier, or a function key on its own.
func TestAccelUsable(t *testing.T) {
	cases := []struct {
		mods ui.Modifiers
		key  string
		want bool
	}{
		{0, "a", false},
		{0, "1", false},
		{ui.Shift, "a", false},
		{ui.Ctrl, "a", true},
		{ui.Alt, "F1", true},
		{0, "F2", true},
		{0, "Enter", false},
	}
	for _, c := range cases {
		if got := accelUsable(c.mods, c.key); got != c.want {
			t.Errorf("accelUsable(%v, %q) = %v, want %v", c.mods, c.key, got, c.want)
		}
	}
	if got := modsLabel(ui.Ctrl | ui.Alt); got != "Ctrl + Alt" {
		t.Errorf("modsLabel = %q", got)
	}
}

// settings.json round trips, and a file that leaves fields out keeps the
// defaults for them.
func TestSettingsRoundTrip(t *testing.T) {
	settingsFileOverride = filepath.Join(t.TempDir(), "settings.json")
	t.Cleanup(func() { settingsFileOverride = ""; settings = defaultSettings() })

	loadSettings()
	if settings != defaultSettings() {
		t.Fatalf("without a file got %+v", settings)
	}
	settings.SnipHotkey = "Ctrl+Shift+1"
	settings.FullHotkey = "Ctrl+Shift+2"
	settings.Theme = "dark"
	settings.AutoStart = true
	if err := settings.save(); err != nil {
		t.Fatal(err)
	}
	settings = defaultSettings()
	loadSettings()
	if settings.SnipHotkey != "Ctrl+Shift+1" || settings.FullHotkey != "Ctrl+Shift+2" ||
		settings.Theme != "dark" || !settings.AutoStart {
		t.Fatalf("after a round trip got %+v", settings)
	}

	// A hand-written file with only one field leaves the rest as defaults.
	path := settingsFile()
	if err := os.WriteFile(path, []byte(`{"snipHotkey":"F4"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loadSettings()
	if settings.SnipHotkey != "F4" {
		t.Fatalf("hotkey = %q", settings.SnipHotkey)
	}
	if settings.FullHotkey != "F3" || settings.PinHotkey != "F4" {
		t.Fatalf("hotkey defaults not kept: %+v", settings)
	}
	if settings.TargetLang != "简体中文" || settings.Theme != "system" || settings.Translate != "未配置" {
		t.Fatalf("defaults not kept: %+v", settings)
	}
}

// The dialog is a column of sections on the left and the open one on the
// right: every entry is listed, in the order the column should keep, and
// each opens its own card with its rows, explanations and values.
func TestSettingsWindowRows(t *testing.T) {
	settingsFileOverride = filepath.Join(t.TempDir(), "settings.json")
	t.Cleanup(func() { settingsFileOverride = ""; settings = defaultSettings() })
	settings = defaultSettings()

	w := &settingsWindow{theme: themeIndex(settings.Theme), saved: settings}
	tt := ui.NewTester(w.view, settingsWinW, settingsWinH)
	tt.Frame()

	// The column, top to bottom.
	var last float32
	for _, sec := range settingsSections {
		if !tt.HasText(sec.name) {
			t.Errorf("the column is missing %q, have %q", sec.name, tt.Texts())
		}
		r, ok := tt.Find("转到" + sec.name)
		if !ok {
			t.Fatalf("no entry for %q", sec.name)
		}
		if r.Y <= last {
			t.Errorf("%q is not below the entry before it: %v", sec.name, r)
		}
		last = r.Y
	}
	if len(settingsSections) != 6 {
		t.Fatalf("the column has %d sections, want 6", len(settingsSections))
	}

	open := func(section string, want ...string) {
		t.Helper()
		if section != "" {
			if err := tt.Click("转到" + section); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
		}
		for _, s := range want {
			if !tt.HasText(s) {
				t.Errorf("%q section is missing %q, have %q", section, s, tt.Texts())
			}
		}
	}

	// The first section is open without a click, and carries the actions
	// with their shortcuts.
	open("", appName, appTagline, "关闭设置",
		"区域截图  F1", "全屏截图  F3", "贴出剪贴板图片  F4", "颜色拾取  F6",
		"识别剪贴板图片", "翻译剪贴板图片", "预加载文字识别模型",
		"模型与词典在程序目录的 ocr/ 文件夹")

	open("工具栏", "勾选截图工具条上出现的按钮，改动立即生效",
		"个性极简", "默认实用", "火力全开")

	open("快捷键", "区域截图快捷键输入", "全屏截图快捷键输入",
		"贴图快捷键输入", "颜色拾取快捷键输入", "点击输入框后直接按下组合键")

	open("OCR 与翻译", "翻译服务", "不配置时使用内置离线词典",
		"目标语言", "单词卡片不受此项影响", "未配置", "简体中文",
		"OCR 使用内置 PP-OCRv5 模型与 ECDICT 常用词库。")

	open("外观与启动", "主题", "跟随系统或固定深浅色", "开机自启",
		"登录系统后自动驻留托盘", "跟随系统", "深色", "浅色")

	// About says where the files are and what the app is written in, and
	// nothing it already says in the header.
	open("关于软件", "配置文件", "识别资源", "开发语言", goVersion())
	for _, gone := range []string{"运行环境", runtime.GOOS + "/" + runtime.GOARCH} {
		if tt.HasText(gone) {
			t.Errorf("about still shows %q", gone)
		}
	}

	// The target language list keeps just the two the app translates into.
	if len(targetLanguages) != 2 || targetLanguages[0] != "简体中文" || targetLanguages[1] != "English" {
		t.Errorf("target languages = %q", targetLanguages)
	}
	// The interface is Chinese only, so there is no language choice, and the
	// target language list keeps just the two the app can translate into.
	for _, gone := range []string{"界面语言", "繁体中文", "日本語"} {
		if tt.HasText(gone) {
			t.Errorf("dialog still shows %q", gone)
		}
	}
}

// The toolbar is a list of checkboxes: three to a line, the box on the left
// and the name on the right, and a click on either takes the button off the
// bar or puts it back.
func TestToolbarChecksThreePerRow(t *testing.T) {
	settingsFileOverride = filepath.Join(t.TempDir(), "settings.json")
	t.Cleanup(func() { settingsFileOverride = ""; settings = defaultSettings() })
	settings = defaultSettings()

	w := &settingsWindow{theme: themeIndex(settings.Theme), saved: settings, section: 1}
	tt := ui.NewTester(w.view, settingsWinW, settingsWinH)
	tt.Frame()

	// The first three names sit on one line, the fourth on the next.
	var boxes []ui.Rect
	for _, id := range []string{"rect", "ellipse", "circle", "line"} {
		it := barItemByID(id)
		r, ok := tt.Find("工具条：" + it.name)
		if !ok {
			t.Fatalf("no checkbox for %q", it.name)
		}
		boxes = append(boxes, r)
	}
	for i := 1; i < barChecksPerRow; i++ {
		if boxes[i].Y != boxes[0].Y {
			t.Errorf("%q is not on the first line: %v", barCatalog[i].name, boxes[i])
		}
		if boxes[i].X <= boxes[i-1].X {
			t.Errorf("%q is not to the right of the one before it: %v", barCatalog[i].name, boxes[i])
		}
	}
	if boxes[3].Y <= boxes[0].Y {
		t.Errorf("the fourth checkbox is not below the first line: %v", boxes[3])
	}

	// Every button starts on, since the stored toolbar is empty.
	if !barIDsContain(enabledBarIDs(), "rect") {
		t.Fatal("rect is off before the first click")
	}
	if err := tt.Click("工具条：矩形"); err != nil {
		t.Fatal(err)
	}
	if barIDsContain(enabledBarIDs(), "rect") {
		t.Error("the click did not take the button off the bar")
	}
	tt.Frame()
	if err := tt.Click("工具条：矩形"); err != nil {
		t.Fatal(err)
	}
	if !barIDsContain(enabledBarIDs(), "rect") {
		t.Error("the second click did not put the button back")
	}
}

// Clicking a section of the column opens it, and no other: the cards are
// one page each.
func TestSettingsColumnSwitchesPage(t *testing.T) {
	settingsFileOverride = filepath.Join(t.TempDir(), "settings.json")
	t.Cleanup(func() { settingsFileOverride = ""; settings = defaultSettings() })
	settings = defaultSettings()

	w := &settingsWindow{theme: themeIndex(settings.Theme), saved: settings}
	tt := ui.NewTester(w.view, settingsWinW, settingsWinH)
	tt.Frame()
	if !tt.HasText("区域截图  F1") {
		t.Fatalf("the actions are not the first page, have %q", tt.Texts())
	}
	if tt.HasText("开机自启") {
		t.Error("the first page already shows the appearance card")
	}

	if err := tt.Click("转到外观与启动"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if w.section != 4 {
		t.Fatalf("the column opened section %d, want 4", w.section)
	}
	if !tt.HasText("开机自启") {
		t.Errorf("the appearance card did not open, have %q", tt.Texts())
	}
	if tt.HasText("区域截图  F1") {
		t.Error("the actions page is still there under the appearance one")
	}
}

// Clicking a hotkey field puts that row into recording, and a modifier
// alone waits for the key that follows.
func TestHotkeyFieldRecords(t *testing.T) {
	settings = defaultSettings()
	w := &settingsWindow{theme: themeIndex(settings.Theme), saved: settings}
	tt := ui.NewTester(w.view, settingsWinW, settingsWinH)
	tt.Frame()
	if err := tt.Click("转到快捷键"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()

	if err := tt.Click("区域截图快捷键输入"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if w.recRow != 1 {
		t.Fatalf("clicking the field set recRow = %d, want 1", w.recRow)
	}
	if !tt.HasText("按下组合键…") {
		t.Fatalf("recording field shows %q", tt.Texts())
	}
	if settings.SnipHotkey != "F1" {
		t.Errorf("recording must not commit anything yet, got %q", settings.SnipHotkey)
	}

	// Escape leaves the field without changing the hotkey.
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if w.recRow != 0 || settings.SnipHotkey != "F1" {
		t.Fatalf("after Escape: recRow=%d hotkey=%q", w.recRow, settings.SnipHotkey)
	}

	// Each row records on its own: the second field takes keys for itself.
	if err := tt.Click("贴图快捷键输入"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if w.recRow != 3 {
		t.Fatalf("clicking the pin field set recRow = %d, want 3", w.recRow)
	}
}
