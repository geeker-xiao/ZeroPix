// A small status bubble in the corner of the screen. Snips finish without
// any window of their own, so this is how they report what happened.

package main

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	toastW    = 300
	toastH    = 56
	toastLife = 3 * time.Second
)

// sleepFrame gives the compositor a moment to take a hidden window away
// before the screen is frozen.
func sleepFrame() { time.Sleep(170 * time.Millisecond) }

type toastWindow struct {
	win *mygo.Window
	msg string
	seq int
}

// toast shows msg in the bubble, replacing what it showed before.
func (a *zooApp) toast(msg string) {
	if t := a.toastWin; t != nil && !t.win.IsDestroyed() {
		t.seq++
		seq := t.seq
		t.msg = msg
		t.win.Update(func() {})
		time.AfterFunc(toastLife, func() { a.expireToast(seq) })
		return
	}
	disp := mygo.Screen.PrimaryDisplay()
	wa := disp.Bounds
	if disp.WorkArea.Width > 0 {
		wa = disp.WorkArea
	}
	t := &toastWindow{msg: msg, seq: 1}
	// Not DisableMove: mygo forces SWP_NOMOVE on such windows, which would
	// undo the X and Y above and keep every bubble in the top-left corner.
	t.win = mygo.NewWindow(mygo.WindowOptions{
		Title:           appName,
		X:               wa.X + wa.Width - toastW - 24,
		Y:               wa.Y + wa.Height - toastH - 24,
		Width:           toastW,
		Height:          toastH,
		Frameless:       true,
		AlwaysOnTop:     true,
		SkipTaskbar:     true,
		DisableResize:   true,
		DisableMinimize: true,
		DisableMaximize: true,
		DisableShadow:   true,
		BackgroundColor: appInk,
		CornerRadius:    12,
		Content:         ui.View(t.view),
	})
	a.toastWin = t
	seq := t.seq
	time.AfterFunc(toastLife, func() { a.expireToast(seq) })
}

// expireToast closes the bubble unless a newer message took its place.
func (a *zooApp) expireToast(seq int) {
	t := a.toastWin
	if t == nil {
		return
	}
	t.win.Update(func() {
		if t.seq != seq {
			return
		}
		t.close()
	})
}

func (t *toastWindow) close() {
	if t.win != nil && !t.win.IsDestroyed() {
		t.win.Destroy()
	}
	if app.toastWin == t {
		app.toastWin = nil
	}
}

func (t *toastWindow) view(c *ui.Context) {
	// The content paints the bubble itself: the window's BackgroundColor only
	// shows where nothing is drawn, and a filled root covers all of it.
	ui.Box(c).Fill().Background(ui.RGBA(0x23, 0x25, 0x2A, 1)).
		Padding(16, 12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, trimMessage(t.msg, 46)).
			FontSize(13).
			TextColor(ui.RGBA(255, 255, 255, 1))
	})
}

// trimMessage shortens a message to fit the bubble.
func trimMessage(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
