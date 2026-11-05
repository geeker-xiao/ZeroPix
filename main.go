// ZeroPix — a Snipaste-style screen capture, pin and OCR tool built on
// mygo's native UI. It lives in the notification area: F1 takes a region,
// F3 the whole screen, F4 pins the image on the clipboard.

package main

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

//go:embed resources/tray.png
var trayPNG []byte

type zooApp struct {
	tray     *mygo.Tray
	snip     *snipSession
	pick     *pickSession
	colorWin *colorWindow
	pins     []*pinWindow
	toastWin *toastWindow
	rec      *recorder

	mu      sync.Mutex
	note    string
	ocrNote string
}

var app = &zooApp{}

// hideForShot hides the app's own windows so they are not in the screenshot.
// The settings dialog is the only one that can be on screen: the app lives in
// the notification area and opens nothing else by itself.
func (a *zooApp) hideForShot() {
	if w := settingsWin; w != nil {
		w.hideForShot()
	}
	if w := a.colorWin; w != nil {
		w.hideForShot()
	}
	if t := a.toastWin; t != nil {
		t.close()
	}
	sleepFrame()
}

// restoreAfterShot brings back the windows a snip took away.
func (a *zooApp) restoreAfterShot() {
	if w := settingsWin; w != nil {
		w.restoreAfterShot()
	}
	if w := a.colorWin; w != nil {
		w.restoreAfterShot()
	}
}

// setNote shows a message in the settings dialog from any goroutine.
func (a *zooApp) setNote(s string) {
	a.mu.Lock()
	a.note = s
	a.mu.Unlock()
	if w := settingsWin; w != nil && w.win != nil && !w.win.IsDestroyed() {
		w.win.Update(func() {})
	}
}

func (a *zooApp) noteText() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.note
}

// loadOCRModel reads the models ahead of the first snip. The result goes to
// the note rather than a toast: a toast opens a window, and this runs on a
// goroutine that does not own the message loop.
func loadOCRModel() {
	app.setNote("正在加载文字识别模型…")
	go func() {
		if err := ocrEng.Load(); err != nil {
			app.setNote("文字识别: " + err.Error())
			return
		}
		app.setNote("文字识别模型已就绪")
	}()
}

// startTimedSnip waits a few seconds and then takes the whole screen, so
// that menus and tooltips have time to close.
func startTimedSnip(seconds int) {
	if app.snip != nil {
		return
	}
	app.toast(fmt.Sprintf("%d 秒后截取全屏", seconds))
	time.AfterFunc(time.Duration(seconds)*time.Second, func() {
		startSnip(true)
	})
}

// pinFromClipboard pins the image currently on the clipboard at the cursor.
func pinFromClipboard() {
	img := clipboardImage()
	if img == nil {
		app.toast("剪贴板中没有图片")
		return
	}
	pt := mygo.Screen.CursorScreenPoint()
	disp := mygo.Screen.DisplayNearestPoint(pt)
	scale := disp.ScaleFactor
	if scale <= 0 {
		scale = 1
	}
	x := int(math.Round(float64(pt.X)*scale)) - img.Bounds().Dx()/2
	y := int(math.Round(float64(pt.Y)*scale)) - img.Bounds().Dy()/2
	app.pinImageAt(img, x, y, scale, lastColor, 0)
}

// ocrClipboard recognizes (or translates) the image on the clipboard.
func ocrClipboard(translate bool) {
	img := clipboardImage()
	if img == nil {
		app.toast("剪贴板中没有图片")
		return
	}
	if translate {
		showTranslation(img)
	} else {
		showOCRResult(img)
	}
}

// clipboardImage decodes the PNG the clipboard holds, if any.
func clipboardImage() *image.RGBA {
	data := mygo.Clipboard.ReadImage()
	if len(data) == 0 {
		return nil
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	rgba := image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	return rgba
}

// trayIcon is the small viewfinder mark, baked at the size the tray needs.
func trayIcon() []byte { return trayPNG }

func (a *zooApp) menu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "区域截图  " + settings.SnipHotkey, Click: func(*mygo.MenuItem, *mygo.Window) { startSnip(false) }},
		{Label: "全屏截图  " + settings.FullHotkey, Click: func(*mygo.MenuItem, *mygo.Window) { startSnip(true) }},
		{Label: "定时截图", Submenu: []*mygo.MenuItem{
			{Label: "3 秒后", Click: func(*mygo.MenuItem, *mygo.Window) { startTimedSnip(3) }},
			{Label: "5 秒后", Click: func(*mygo.MenuItem, *mygo.Window) { startTimedSnip(5) }},
			{Label: "10 秒后", Click: func(*mygo.MenuItem, *mygo.Window) { startTimedSnip(10) }},
		}},
		{Label: "贴出剪贴板图片  " + settings.PinHotkey, Click: func(*mygo.MenuItem, *mygo.Window) { pinFromClipboard() }},
		{Label: "颜色拾取  " + settings.PickHotkey, Click: func(*mygo.MenuItem, *mygo.Window) { startPick() }},
		mygo.Separator(),
		{Label: "设置…", Click: func(*mygo.MenuItem, *mygo.Window) { openSettings() }},
		mygo.Separator(),
		{Label: "退出", Role: mygo.RoleQuit},
	})
}

func main() {
	// A tray app: no Dock icon on macOS, and no taskbar button anywhere,
	// since the app opens windows only when asked.
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.WhenReady(func() {
		loadSettings()
		settings.applyTheme()
		// The switch follows the registry: a rename leaves a stale entry and
		// a moved executable leaves one pointing at nothing. Both settle here.
		if adoptLegacyAutoStart() {
			_ = setAutoStart(true)
		}
		settings.AutoStart = autoStartEnabled()
		if settings.AutoStart {
			_ = setAutoStart(true)
		}
		registerHotkeys()

		tray, err := mygo.NewTray(mygo.TrayOptions{
			Icon:    trayIcon(),
			ToolTip: appName + " · 取景 / 标注 / 文字识别",
			Menu:    app.menu(),
		})
		if err != nil {
			log.Println("创建托盘失败:", err)
		} else {
			app.tray = tray
			// Left click opens the dialog, which is the only window the app
			// has; right click shows the menu.
			tray.OnClick(openSettings)
		}
	})
	// Closing the dialog leaves the app running in the notification area.
	mygo.App.OnWindowAllClosed(func() {})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// registerHotkeys registers the three shortcuts. One whose stored
// combination another program holds goes back to its default.
func registerHotkeys() {
	changed := false
	for _, act := range hotkeyActions {
		acc := *act.value()
		err := mygo.GlobalShortcut.Register(acc, act.run)
		if err == nil {
			continue
		}
		if acc == act.def {
			app.setNote("热键 " + acc + " 注册失败：" + err.Error())
			continue
		}
		*act.value() = act.def
		changed = true
		if err := mygo.GlobalShortcut.Register(act.def, act.run); err != nil {
			app.setNote("热键 " + act.def + " 注册失败：" + err.Error())
			continue
		}
		app.setNote("热键 " + acc + " 注册失败，已回到 " + act.def)
	}
	if changed {
		_ = settings.save()
	}
}

// setHotkey moves the shortcut of action i to acc, keeping the old one when
// another program, or another action, holds the new one.
func setHotkey(i int, acc string) error {
	acc = strings.TrimSpace(acc)
	cur := *hotkeyActions[i].value()
	if acc == "" || acc == cur {
		return nil
	}
	for j, other := range hotkeyActions {
		if j != i && *other.value() == acc {
			return errors.New("该组合键已用于" + other.name)
		}
	}
	mygo.GlobalShortcut.Unregister(cur)
	if err := mygo.GlobalShortcut.Register(acc, hotkeyActions[i].run); err != nil {
		_ = mygo.GlobalShortcut.Register(cur, hotkeyActions[i].run)
		return errors.New("该组合键已被其它程序占用")
	}
	*hotkeyActions[i].value() = acc
	if err := settings.save(); err != nil {
		app.toast("保存设置失败: " + err.Error())
	}
	if app.tray != nil {
		app.tray.SetMenu(app.menu()) // the labels name the shortcuts
	}
	if w := settingsWin; w != nil {
		w.saved = settings
	}
	return nil
}
