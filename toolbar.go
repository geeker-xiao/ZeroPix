// The toolbar that floats under the snip, its buttons and the in-place text
// editor. Which buttons it carries is up to the user: the settings dialog
// picks from the catalogue below, and the bar lays them out in as many rows
// as the screen needs.

package main

import (
	"image/color"
	"strings"

	"github.com/egoist/mygo/ui"
)

const (
	tbPadX      float32 = 6
	tbPadY      float32 = 5
	tbGap       float32 = 2
	tbRowGap    float32 = 1
	tbBtnW      float32 = 30
	tbLabelW    float32 = 58
	tbSwatchW   float32 = 20
	tbDivW      float32 = 1
	tbDivMargin float32 = 5
	tbRowH      float32 = 36
	tbInset     float32 = 10
)

// The bar is a sheet of its own over the capture, so it carries the
// appearance: ink in the dark theme, near-white in the light one, where a
// black slab would be the only dark thing on the screen. Every colour below
// follows the theme for the same reason.

func tbBarColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(0x14, 0x1B, 0x23, 0.97)
	}
	return ui.RGBA(0xFF, 0xFF, 0xFF, 0.98)
}

func tbBarBorderColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.12)
	}
	return ui.RGBA(0x14, 0x1B, 0x23, 0.12)
}

func tbBarShadowColor(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(0, 0, 0, 0.30)
	}
	return ui.RGBA(0x14, 0x1B, 0x23, 0.16)
}

// tbFore is an icon or a label at rest.
func tbFore(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGB(0xE8, 0xEE, 0xF3)
	}
	return inkColor()
}

// tbMuted is a button that cannot be pressed, such as undo with nothing to
// undo.
func tbMuted(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGB(0x5C, 0x6B, 0x79)
	}
	return ui.RGB(0x9F, 0xAB, 0xB6)
}

func tbHover(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.10)
	}
	return ui.RGBA(0x14, 0x1B, 0x23, 0.06)
}

// tbActive is the face of the tool in use.
func tbActive(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(0, 0xC9, 0xA0, 0.18)
	}
	return ui.RGB(0xE4, 0xF8, 0xF2)
}

// tbAccent draws the tool in use: the pure teal on ink, the darker one on
// the white sheet.
func tbAccent(t *ui.Theme) ui.Color {
	if t.Dark {
		return accentColor()
	}
	return accentInkColor()
}

func tbDivider(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGBA(255, 255, 255, 0.14)
	}
	return ui.RGBA(0x14, 0x1B, 0x23, 0.10)
}

// tbCaption is the small print beside the swatches: the colour in hex.
func tbCaption(t *ui.Theme) ui.Color {
	if t.Dark {
		return ui.RGB(0x8F, 0xA0, 0xB0)
	}
	return ui.RGB(0x5A, 0x68, 0x75)
}

// barItem is one button of the bar: a drawing tool, or an action.
type barItem struct {
	id      string // the name the settings dialog stores
	name    string // what the settings dialog calls it
	icon    string
	label   string // the text beside the icon, empty for the icon alone
	tip     string
	drawing bool // a drawing tool rather than an action
	tool    tool
	run     func(*snipSession)
}

// barCatalog is every button the bar can carry, tools first and then the
// actions, in the order the settings dialog lists them. It is filled in an
// init so that the actions, which reach back into the overlay and the
// settings dialog, do not make a cycle out of the package variables.
var barCatalog []*barItem

func init() {
	barCatalog = []*barItem{
		{id: "rect", name: "矩形", icon: "rect", tip: "矩形描边 (1)", drawing: true, tool: toolRect},
		{id: "ellipse", name: "椭圆", icon: "ellipse", tip: "椭圆描边 (2)", drawing: true, tool: toolEllipse},
		{id: "circle", name: "圆形", icon: "circle", tip: "圆形 (3)", drawing: true, tool: toolCircle},
		{id: "line", name: "直线", icon: "line", tip: "直线 (4)", drawing: true, tool: toolLine},
		{id: "arrow", name: "箭头", icon: "arrow", tip: "箭头 (5)", drawing: true, tool: toolArrow},
		{id: "brush", name: "画笔", icon: "brush", tip: "画笔 (6)", drawing: true, tool: toolBrush},
		{id: "highlighter", name: "荧光笔", icon: "highlighter", tip: "荧光笔 (7)", drawing: true, tool: toolHighlighter},
		{id: "text", name: "文字", icon: "text", tip: "文字 (8)", drawing: true, tool: toolText},
		{id: "number", name: "序号", icon: "number", tip: "序号 (9)", drawing: true, tool: toolNumber},
		{id: "mosaic", name: "马赛克", icon: "mosaic", tip: "马赛克 (0)", drawing: true, tool: toolMosaic},
		{id: "blur", name: "模糊", icon: "blur", tip: "模糊", drawing: true, tool: toolBlur},
		{id: "spotlight", name: "聚光灯", icon: "spotlight", tip: "聚光灯", drawing: true, tool: toolSpotlight},
		{id: "dimension", name: "尺寸标注", icon: "dimension", tip: "尺寸标注", drawing: true, tool: toolDimension},
		{id: "ruler", name: "标尺", icon: "ruler", tip: "标尺", drawing: true, tool: toolRuler},
		{id: "eraser", name: "橡皮擦", icon: "eraser", tip: "橡皮擦", drawing: true, tool: toolEraser},
		{id: "eyedropper", name: "取色", icon: "pipette", tip: "取色（从截图取颜色）", drawing: true, tool: toolEyedropper},

		{id: "undo", name: "撤销", icon: "undo", tip: "撤销 (Ctrl+Z)", run: func(s *snipSession) { s.undo() }},
		{id: "redo", name: "重做", icon: "redo", tip: "重做 (Ctrl+Shift+Z)", run: func(s *snipSession) { s.redo() }},
		{id: "ocr", name: "文字识别", icon: "scan", label: "识别", tip: "文字识别", run: func(s *snipSession) { s.finish(actOCR) }},
		{id: "translate", name: "翻译", icon: "translate", label: "翻译", tip: "翻译选中的文字", run: func(s *snipSession) { s.finish(actTranslate) }},
		{id: "redact", name: "智能脱敏", icon: "mask", label: "脱敏", tip: "智能脱敏：识别文字并打码", run: func(s *snipSession) { s.redact() }},
		{id: "gif", name: "GIF 截图", icon: "gif", label: "GIF", tip: "录制选中区域为 GIF", run: func(s *snipSession) { s.finish(actGIF) }},
		{id: "video", name: "录屏", icon: "video", label: "视频", tip: "录制选中区域为视频", run: func(s *snipSession) { s.finish(actVideo) }},
		{id: "pin", name: "贴图", icon: "pin", label: "钉图", tip: "贴到屏幕上", run: func(s *snipSession) { s.finish(actPin) }},
		{id: "save", name: "保存", icon: "save", label: "保存", tip: "保存为 PNG (Ctrl+S)", run: func(s *snipSession) { s.finish(actSave) }},
		{id: "copy", name: "复制", icon: "copy", label: "复制", tip: "复制到剪贴板 (Enter)", run: func(s *snipSession) { s.finish(actCopy) }},
		{id: "settings", name: "设置", icon: "settings", tip: "打开设置", run: func(s *snipSession) { s.openSettings() }},
		{id: "close", name: "取消", icon: "close", label: "取消", tip: "取消截图 (Esc)", run: func(s *snipSession) { s.finish(actClose) }},
	}
}

// The three toolbars the settings dialog offers as one-click presets.
var barPresets = []struct {
	name string
	ids  []string
}{
	{"个性极简", []string{"rect", "text", "number", "mosaic", "pin", "copy", "close"}},
	{"默认实用", []string{
		"rect", "ellipse", "arrow", "brush", "text", "number", "mosaic", "blur",
		"undo", "redo", "ocr", "translate", "save", "pin", "copy", "close",
	}},
	{"火力全开", nil}, // nil stands for every button in the catalogue
}

func barItemByID(id string) *barItem {
	for _, it := range barCatalog {
		if it.id == id {
			return it
		}
	}
	return nil
}

// enabledBarIDs is the set the bar shows right now: what the settings store,
// or every button when it has never been set, so that nothing new is hidden
// from someone who has not opened the settings yet.
func enabledBarIDs() []string {
	if settings.Toolbar != "" {
		var ids []string
		for _, id := range strings.Split(settings.Toolbar, ",") {
			if id = strings.TrimSpace(id); id != "" {
				ids = append(ids, id)
			}
		}
		if len(ids) > 0 {
			return ids
		}
	}
	all := make([]string, 0, len(barCatalog))
	for _, it := range barCatalog {
		all = append(all, it.id)
	}
	return all
}

// barEnabled reports whether id is on the bar.
func barEnabled(id string) bool {
	for _, v := range enabledBarIDs() {
		if v == id {
			return true
		}
	}
	return false
}

// barActionWidth is what a button takes on the bar: a label makes it wider.
func barActionWidth(it *barItem) float32 {
	if it.isTool() {
		return tbBtnW
	}
	if it.label != "" {
		return tbLabelW
	}
	return tbBtnW
}

// isTool tells the tool buttons from the actions.
func (it *barItem) isTool() bool { return it.drawing }

// barGroups is the bar's content, split into the runs a divider separates:
// the tools, undo and redo, the capture actions, the actions that finish the
// snip, the settings button, and the swatches, which take the last line.
func (s *snipSession) barGroups() []barGroup {
	var tools, capture, finish []*barItem
	for _, it := range barCatalog {
		if !barEnabled(it.id) {
			continue
		}
		switch it.id {
		case "undo", "redo":
			// Always on the bar, whichever preset is in use.
		case "ocr", "translate", "redact", "gif", "video":
			capture = append(capture, it)
		case "pin", "save", "copy", "settings", "close":
			finish = append(finish, it)
		default:
			tools = append(tools, it)
		}
	}
	groups := []barGroup{}
	if len(tools) > 0 {
		groups = append(groups, barGroup{items: tools})
	}
	// Undo and redo keep their place even when a preset leaves them out.
	var edit []*barItem
	for _, id := range []string{"undo", "redo"} {
		edit = append(edit, barItemByID(id))
	}
	groups = append(groups, barGroup{items: edit})
	if len(capture) > 0 {
		groups = append(groups, barGroup{items: capture})
	}
	groups = append(groups, barGroup{items: finish})
	// The swatches keep a line of their own, under the buttons that have
	// nothing to do with colour. They stay there whichever tool is in use,
	// so that picking one never makes the bar change shape under the hand.
	groups = append(groups, barGroup{items: s.swatchGroup(), breakBefore: true})
	return groups
}

// swatchGroup is the colour dots, counted as buttons so that the layout
// knows how much room they take: the four presets and the mixed colour.
func (s *snipSession) swatchGroup() []*barItem {
	out := make([]*barItem, len(annotationColors)+1)
	for i := range out {
		out[i] = &barItem{id: "swatch"}
	}
	return out
}

// barGroup is a run of buttons a divider sets apart from the next, and
// whether it has to start a row of its own.
type barGroup struct {
	items       []*barItem
	breakBefore bool
}

// barRow is one row of the finished bar.
type barRow struct {
	groups [][]*barItem
	width  float32
}

// groupWidth is what a group takes, gaps included.
func groupWidth(g []*barItem) float32 {
	var w float32
	for i, it := range g {
		if i > 0 {
			w += tbGap
		}
		w += barActionWidth(it)
	}
	return w
}

// barRowGap is the gap a divider adds between two groups.
const barRowGap = tbDivW + 2*tbDivMargin + tbGap

// layoutBar packs the groups into rows no wider than maxW, honouring the
// groups that ask for a row of their own.
func (s *snipSession) layoutBar(maxW float32) []barRow {
	groups := s.barGroups()
	rows := []barRow{}
	cur := barRow{}
	for _, g := range groups {
		if len(g.items) == 0 {
			continue
		}
		w := groupWidth(g.items)
		add := w
		if len(cur.groups) > 0 {
			add += barRowGap
		}
		if len(cur.groups) > 0 && (g.breakBefore || cur.width+add > maxW) {
			rows = append(rows, cur)
			cur = barRow{}
			add = w
		}
		if len(cur.groups) > 0 {
			cur.width += barRowGap
		}
		cur.groups = append(cur.groups, g.items)
		cur.width += w
	}
	if len(cur.groups) > 0 {
		rows = append(rows, cur)
	}
	return rows
}

func (s *snipSession) buildToolbar(c *ui.Context) {
	maxW := min(float32(1240), s.frame.W-24)
	rows := s.layoutBar(maxW - tbPadX*2)
	barW := float32(0)
	for _, r := range rows {
		barW = max(barW, r.width)
	}
	barW += tbPadX * 2
	// The box includes its 1px hairline, so the rows plus the padding plus
	// both edges fill it exactly; leaving the border out of the sum made the
	// rows overflow by two pixels and pushed the buttons off centre.
	barH := float32(len(rows))*tbRowH + float32(len(rows)-1)*tbRowGap + tbPadY*2 + 2

	x, y := s.toolbarPos(barW, barH)
	s.barRect = ui.Rect{X: x, Y: y, W: barW, H: barH}
	t := c.Theme()
	ui.Column(c).Absolute().Left(x).Top(y).Width(barW).Height(barH).
		Radius(10).Padding(tbPadY, tbPadX).Gap(tbRowGap).
		Background(tbBarColor(t)).
		Border(1, tbBarBorderColor(t)).
		Shadow(0, 6, 22, 0, tbBarShadowColor(t)).
		Children(func() {
			for _, row := range rows {
				s.buildRow(c, row)
			}
		})
	s.colorPanelView(c)
}

// buildRow lays one row out: the swatches first, then each group behind a
// divider.
func (s *snipSession) buildRow(c *ui.Context, row barRow) {
	if swatchRow(row) {
		s.buildSwatchRow(c)
		return
	}
	ui.Row(c).FillWidth().Height(tbRowH).Gap(tbGap).AlignItems(ui.Center).Children(func() {
		for gi, g := range row.groups {
			if gi > 0 {
				s.toolbarDivider(c)
			}
			if len(g) > 0 && g[0].id == "swatch" {
				s.swatches(c)
				continue
			}
			for _, it := range g {
				s.barButton(c, it)
			}
		}
	})
}

// swatchRow reports whether a row holds nothing but the colour dots.
func swatchRow(row barRow) bool {
	return len(row.groups) == 1 && len(row.groups[0]) > 0 && row.groups[0][0].id == "swatch"
}

// buildSwatchRow draws the line of colours: the four presets and the mixed
// swatch at its left, the colour in hex at its right. Both ends carry
// something, so the line does not end in a blank patch of sheet.
func (s *snipSession) buildSwatchRow(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).FillWidth().Height(tbRowH).Gap(tbGap).AlignItems(ui.Center).
		Justify(ui.SpaceBetween).Children(func() {
		ui.Row(c).Gap(tbGap).AlignItems(ui.Center).Children(func() {
			s.swatches(c)
		})
		ui.Text(c, strings.ToUpper(hexOf(color.RGBA{R: s.color.R, G: s.color.G, B: s.color.B, A: 255}))).
			FontSize(12).Font("Consolas, 微软雅黑, monospace").TextColor(tbCaption(t))
	})
}

// swatches draws the colour dots at the left of their own line: the four the
// app ships with, then the mixed colour, which opens a picker.
func (s *snipSession) swatches(c *ui.Context) {
	for i, col := range annotationColors {
		s.colorButton(c, "color-"+itoa(i), col, annotationColorNames[i])
	}
	s.customButton(c)
}

// customButton is the last dot: it opens a picker for a colour the four
// presets do not have, and wears that colour once one is chosen.
func (s *snipSession) customButton(c *ui.Context) {
	t := c.Theme()
	on := s.hasCustom && s.color == s.customColor
	b := ui.ButtonBase(c.Key("color-custom")).Width(tbSwatchW).Height(30).Radius(6).
		Center().Label("自定义颜色").Tooltip("自定义颜色")
	if b.Hovered() {
		b.Background(tbHover(t))
	}
	if b.Clicked() {
		s.colorPanel = !s.colorPanel
		if s.colorPanel {
			x := hsvOf(color.RGBA{R: s.customColor.R, G: s.customColor.G, B: s.customColor.B, A: 255})
			if x.S == 0 {
				x.H = s.panelH
			}
			s.panelH, s.panelS, s.panelV = x.H, x.S, x.V
		}
	}
	b.Children(func() {
		ui.Box(c).Width(16).Height(16).Draw(func(p *ui.Painter, r ui.Rect) {
			if !s.hasCustom {
				drawIcon(p, "palette", r, tbFore(t))
				return
			}
			if on {
				p.Fill(r, ui.RGBA(s.customColor.R, s.customColor.G, s.customColor.B, 0.24), 8)
				p.Fill(ui.Rect{X: r.X + 2, Y: r.Y + 2, W: 12, H: 12}, s.customColor, 6)
				p.Stroke(ui.Rect{X: r.X + 2, Y: r.Y + 2, W: 12, H: 12}, s.customColor, 6, 1)
				return
			}
			p.Fill(ui.Rect{X: r.X + 3, Y: r.Y + 3, W: 10, H: 10}, s.customColor, 5)
		})
	})
}

// colorPanelView floats a picker beside the bar while the mixed swatch is
// open: a square of saturation and brightness, a bar of hues, and the
// colour in hex, in the bar's own sheet.
func (s *snipSession) colorPanelView(c *ui.Context) {
	if !s.colorPanel {
		return
	}
	t := c.Theme()
	const w, h = float32(248), float32(190)
	x := min(max(s.barRect.X, 8), s.frame.W-w-8)
	y := s.barRect.Y + s.barRect.H + 8
	if y+h > s.frame.H-8 {
		y = s.barRect.Y - h - 8
	}
	y = max(y, 8)
	s.panelRect = ui.Rect{X: x, Y: y, W: w, H: h}
	ui.Box(c).Absolute().Left(x).Top(y).Width(w).Height(h).Radius(10).Padding(12).
		Background(tbBarColor(t)).Border(1, tbBarBorderColor(t)).
		Shadow(0, 8, 26, 0, tbBarShadowColor(t)).
		Children(func() {
			ui.Column(c).Fill().Gap(8).Children(func() {
				s.panelSquare(c)
				s.panelHueBar(c)
				ui.Text(c, strings.ToUpper(hexOf(color.RGBA{
					R: s.customColor.R, G: s.customColor.G, B: s.customColor.B, A: 255,
				}))).FontSize(12).Font("Consolas, 微软雅黑, monospace").TextColor(tbCaption(t))
			})
		})
}

// panelSquare is the square that takes saturation across and brightness up.
func (s *snipSession) panelSquare(c *ui.Context) {
	hue := hsvColor{H: s.panelH, S: 1, V: 1}.rgba()
	sat, val := s.panelS, s.panelV
	ui.Box(c).Width(224).Height(118).Radius(8).Cursor(ui.CursorCrosshair).
		HandleInput(s.panelDrag(panelDragSquare)).
		Draw(func(p *ui.Painter, r ui.Rect) {
			s.panelSq = r
			p.FillGradient(r, ui.LinearGradient{
				From: ui.RGB(255, 255, 255), To: ui.RGBA(hue.R, hue.G, hue.B, 1), Angle: 90,
			}, 8)
			p.FillGradient(r, ui.LinearGradient{
				From: ui.RGBA(0, 0, 0, 0), To: ui.RGB(0, 0, 0), Angle: 180,
			}, 8)
			cx := r.X + r.W*float32(sat)
			cy := r.Y + r.H*float32(1-val)
			p.Stroke(ui.Rect{X: cx - 7, Y: cy - 7, W: 14, H: 14}, ui.RGBA(0, 0, 0, 0.35), 7, 1)
			p.Stroke(ui.Rect{X: cx - 5.5, Y: cy - 5.5, W: 11, H: 11}, ui.RGBA(255, 255, 255, 1), 5.5, 2)
		})
}

// panelHueBar is the strip of hues under the square, red at either end.
func (s *snipSession) panelHueBar(c *ui.Context) {
	hues := []ui.Color{
		ui.RGB(255, 0, 0), ui.RGB(255, 255, 0), ui.RGB(0, 255, 0),
		ui.RGB(0, 255, 255), ui.RGB(0, 0, 255), ui.RGB(255, 0, 255), ui.RGB(255, 0, 0),
	}
	at := s.panelH / 360
	ui.Box(c).Width(224).Height(12).Radius(6).Cursor(ui.CursorCrosshair).
		HandleInput(s.panelDrag(panelDragHue)).
		Draw(func(p *ui.Painter, r ui.Rect) {
			s.panelHueRect = r
			p.Clip(r, 6, func() {
				seg := r.W / 6
				for i := 0; i < 6; i++ {
					p.FillGradient(ui.Rect{X: r.X + seg*float32(i), Y: r.Y, W: seg + 0.5, H: r.H},
						ui.LinearGradient{From: hues[i], To: hues[i+1], Angle: 90}, 0)
				}
			})
			x := min(max(r.X+r.W*float32(at), r.X+2), r.X+r.W-2)
			p.Stroke(ui.Rect{X: x - 2, Y: r.Y - 2, W: 4, H: r.H + 4}, ui.RGBA(0, 0, 0, 0.30), 2, 1)
			p.Stroke(ui.Rect{X: x - 1, Y: r.Y - 1, W: 2, H: r.H + 2}, ui.RGBA(255, 255, 255, 0.95), 1, 1)
		})
}

// Where the pointer holds the picker, if anywhere.
const (
	panelDragNone = iota
	panelDragSquare
	panelDragHue
)

// panelDrag hands the square and the hue bar their drags.
func (s *snipSession) panelDrag(target int) func(ui.InputEvent) bool {
	return func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button != 0 {
				return false
			}
			s.panelHeld = target
			s.panelPick(target, ev.X, ev.Y)
			return true
		case ui.InputPointerMove:
			if s.panelHeld != target {
				return false
			}
			s.panelPick(target, ev.X, ev.Y)
			return true
		case ui.InputPointerUp:
			if s.panelHeld != target {
				return false
			}
			s.panelHeld = panelDragNone
			return true
		}
		return false
	}
}

// panelPick takes the colour under a point of the picker, the coordinates
// being those of the piece the press landed on.
func (s *snipSession) panelPick(target int, x, y float32) {
	switch target {
	case panelDragSquare:
		r := s.panelSq
		if r.W <= 0 || r.H <= 0 {
			return
		}
		s.panelS = clamp01(float64(x) / float64(r.W))
		s.panelV = 1 - clamp01(float64(y)/float64(r.H))
	case panelDragHue:
		r := s.panelHueRect
		if r.W <= 0 {
			return
		}
		h := float64(x) / float64(r.W) * 360
		if h >= 360 {
			h = 359.9
		}
		s.panelH = h
	default:
		return
	}
	s.applyPanelColor()
}

// applyPanelColor puts the picker's colour on the snip.
func (s *snipSession) applyPanelColor() {
	c := hsvColor{H: s.panelH, S: s.panelS, V: s.panelV}.rgba()
	s.customColor = ui.RGB(c.R, c.G, c.B)
	s.hasCustom = true
	s.color = s.customColor
	lastColor = s.color
	s.refresh()
}

func (s *snipSession) toolbarPos(w, h float32) (float32, float32) {
	x := s.sel.X + (s.sel.W-w)/2
	x = min(max(x, 8), s.frame.W-w-8)
	y := s.sel.Y + s.sel.H + tbInset
	if y+h > s.frame.H-8 {
		y = s.sel.Y - h - tbInset
	}
	return x, max(y, 8)
}

// inFloating reports whether a point falls on the toolbar or the text
// editor, which the overlay's canvas leaves alone.
func (s *snipSession) inFloating(x, y float32) bool {
	if s.barRect.W > 0 && pointInRect(x, y, growRect(s.barRect, 2)) {
		return true
	}
	if s.colorPanel && s.panelRect.W > 0 && pointInRect(x, y, s.panelRect) {
		return true
	}
	if s.editorRect.W > 0 && pointInRect(x, y, s.editorRect) {
		return true
	}
	return false
}

func (s *snipSession) toolbarDivider(c *ui.Context) {
	ui.Box(c).Width(tbDivW).Height(22).MarginX(tbDivMargin).Background(tbDivider(c.Theme()))
}

// colorButton is one swatch: a dot in its colour, ringed while it is the one
// in use.
func (s *snipSession) colorButton(c *ui.Context, key string, col ui.Color, name string) {
	on := s.color == col
	b := ui.ButtonBase(c.Key(key)).Width(tbSwatchW).Height(30).Radius(6).
		Center().Label(name).Tooltip(name + "（描边与文字）")
	if b.Hovered() {
		b.Background(tbHover(c.Theme()))
	}
	if b.Clicked() {
		s.colorPanel = false
		s.color = col
		lastColor = col
	}
	b.Children(func() {
		ui.Box(c).Width(16).Height(16).Draw(func(p *ui.Painter, r ui.Rect) {
			if on {
				p.Fill(r, ui.RGBA(col.R, col.G, col.B, 0.24), 8)
				p.Fill(ui.Rect{X: r.X + 2, Y: r.Y + 2, W: 12, H: 12}, col, 6)
				p.Stroke(ui.Rect{X: r.X + 2, Y: r.Y + 2, W: 12, H: 12}, col, 6, 1)
			} else {
				p.Fill(ui.Rect{X: r.X + 3, Y: r.Y + 3, W: 10, H: 10}, col, 5)
			}
		})
	})
}

// barButton draws one catalogue entry: a tool that stays pressed while it is
// the one in use, or an action.
func (s *snipSession) barButton(c *ui.Context, it *barItem) {
	if !it.isTool() && it.run == nil {
		return
	}
	active := it.isTool() && s.active == it.tool
	disabled := it.id == "undo" && len(s.history) == 0
	if it.id == "redo" {
		disabled = len(s.redone) == 0
	}
	t := c.Theme()
	fg := tbFore(t)
	if active {
		fg = tbAccent(t)
	}
	if disabled {
		fg = tbMuted(t)
	}
	key := "bar-" + it.id
	b := ui.ButtonBase(c.Key(key)).Width(barActionWidth(it)).Height(30).Radius(6).
		Center().Gap(4).Label(it.tip).Tooltip(it.tip).Disabled(disabled)
	switch {
	case active:
		b.Background(tbActive(t))
	case !disabled && b.Hovered():
		b.Background(tbHover(t))
	}
	if !disabled && b.Clicked() {
		s.colorPanel = false
		if it.isTool() {
			s.setTool(it.tool)
		} else {
			it.run(s)
		}
	}
	b.Children(func() {
		iconW := float32(18)
		if it.icon == "gif" {
			iconW = 22
		}
		ui.Box(c).Width(iconW).Height(18).Draw(func(p *ui.Painter, r ui.Rect) {
			drawIcon(p, it.icon, r, fg)
		})
		if !it.isTool() && it.label != "" {
			ui.Box(c).Height(18).Width(measureLabel(it.label)).Draw(func(p *ui.Painter, r ui.Rect) {
				p.Text(r.X, r.Y+1.5, it.label, 12.5, fg)
			})
		}
	})
}

// measureLabel is the width of a two-character (or three-latin) label at the
// bar's font size.
func measureLabel(s string) float32 {
	return float32(approxTextWidth(s, 12.5)) + 1
}

// setTool switches the drawing tool; choosing the same one again returns to
// selecting.
func (s *snipSession) setTool(t tool) {
	if s.active == t {
		s.active = toolSelect
	} else {
		s.active = t
	}
	if s.active == toolSelect {
		s.want = ui.CursorMove
	} else {
		s.want = ui.CursorCrosshair
	}
}

// buildTextEditor floats the text field where the text annotation starts,
// with the family and the size the annotation is set in above it.
func (s *snipSession) buildTextEditor(c *ui.Context) {
	const w, h = float32(380), float32(84)
	x := min(max(s.editAt.X, 4), s.frame.W-w-4)
	y := min(max(s.editAt.Y-6, 4), s.frame.H-h-4)
	s.editorRect = ui.Rect{X: x, Y: y, W: w, H: h}
	// A bare white card: no border around the field, and TextInputBase so the
	// field does not draw one of its own either. The shadow alone tells the
	// user where the editor is.
	ui.Box(c).Absolute().Left(x).Top(y).Width(w).Height(h).Radius(8).
		Background(ui.RGBA(255, 255, 255, 0.99)).
		Shadow(0, 4, 14, 0, ui.RGBA(0, 0, 0, 0.28)).
		Padding(8).Children(func() {
		ui.Column(c).Fill().Gap(6).Children(func() {
			ui.Row(c).FillWidth().Gap(6).AlignItems(ui.Center).Children(func() {
				s.fontField(c)
				s.sizeField(c)
			})
			ui.TextInputBase(c.Key("text-input-"+itoa(s.editFocus)), &s.editText).
				FillWidth().Height(26).
				Font(s.font).
				FontSize(s.fontSize).
				Placeholder("输入文字，回车确认").
				AutoFocus().
				OnSubmit(s.commitText)
		})
	})
}

// fontField is the drop-down of the families the machine has, narrowed by
// typing into it. It carries the family onto the session, keeping the field
// below in the chosen family.
func (s *snipSession) fontField(c *ui.Context) {
	if s.fontPick == "" {
		s.fontPick = fontLabel(s.font)
	}
	ui.Combobox(c, &s.fontPick, fontLabels()).Label("字体").
		Grow(1).Height(28).FontSize(12.5)
	if af, ok := fontByLabel(s.fontPick); ok && af.Family != s.font {
		s.font = af.Family
		s.editFocus++
	}
}

// sizeField takes the size of the text annotation, typed or stepped: it goes
// from 8 to 96 points.
func (s *snipSession) sizeField(c *ui.Context) {
	if s.sizeValue == 0 {
		s.sizeValue = float64(s.fontSize)
	}
	el := ui.NumberInput(c, &s.sizeValue, 8, 96, 1).Label("字号").Width(150)
	if el.Changed() {
		s.fontSize = float32(s.sizeValue)
		s.editFocus++ // the field takes the keyboard back after the click
	}
}
