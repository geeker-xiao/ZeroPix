// Offline translation through the bundled ecdict word list. The dictionary
// maps an English word to its phonetics and Chinese senses; a reverse index
// turns Chinese text into English words. It is a dictionary, not a machine
// translation: the panel says so, and shows the senses it found.

package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"strings"
	"sync"
	"unicode"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type dictEntry struct {
	Phonetic string   `json:"p"`
	Senses   []string `json:"t"`
	Pos      []string `json:"pos"`
}

type dictionary struct {
	once    sync.Once
	words   map[string]dictEntry
	reverse map[string]revCand
	err     error
}

// revCand is a word whose Chinese senses contain a term, with how good a
// match it is: early senses and early terms win, then shorter words.
type revCand struct {
	word  string
	score int
}

var dict dictionary

// load reads the word list once and builds the reverse index with it.
func (d *dictionary) load() error {
	d.once.Do(func() {
		path, err := findAsset("ecdict-common.json")
		if err != nil {
			d.err = fmt.Errorf("找不到离线词典: %w", err)
			return
		}
		data, err := os.ReadFile(path)
		if err != nil {
			d.err = fmt.Errorf("读取离线词典失败: %w", err)
			return
		}
		words := make(map[string]dictEntry, 1<<16)
		if err := json.Unmarshal(data, &words); err != nil {
			d.err = fmt.Errorf("解析离线词典失败: %w", err)
			return
		}
		d.words = words
		d.buildReverse()
	})
	return d.err
}

// buildReverse indexes the Chinese senses back to their English words. Only
// terms of two characters or more are indexed: a single character has too
// many meanings to point at a word, and the first senses of an entry are the
// ones that say what it means.
func (d *dictionary) buildReverse() {
	rev := make(map[string]revCand, len(d.words)*3)
	for word, e := range d.words {
		size := min(len(word), 9)
		for si, sense := range e.Senses {
			for ti, term := range splitSenses(sense) {
				term = normalizeTerm(term)
				if len([]rune(term)) < 2 {
					continue
				}
				score := si*100 + ti*10 + size
				old, ok := rev[term]
				if ok && (old.score < score || (old.score == score && old.word <= word)) {
					continue
				}
				rev[term] = revCand{word: word, score: score}
			}
		}
	}
	d.reverse = rev
}

// splitSenses splits one dictionary sense into its Chinese terms.
func splitSenses(s string) []string {
	s = stripPOS(s)
	return strings.FieldsFunc(s, func(r rune) bool {
		switch r {
		case ',', '，', ';', '；', '/':
			return true
		}
		return false
	})
}

// stripPOS removes a leading part-of-speech marker such as "n." or "[计]".
func stripPOS(s string) string {
	for i := 0; i < 4; i++ {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "[") {
			if end := strings.Index(s, "]"); end > 0 {
				s = s[end+1:]
				continue
			}
		}
		if dot := strings.Index(s, "."); dot > 0 && dot <= 5 && isAbbr(s[:dot]) {
			s = s[dot+1:]
			continue
		}
		break
	}
	return s
}

func isAbbr(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) || r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// normalizeTerm keeps a term that is worth looking up: a short Chinese or
// latin phrase, without the parentheses and notes dictionaries add.
func normalizeTerm(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "地")
	for _, cut := range []string{"（", "(", "["} {
		if i := strings.Index(s, cut); i > 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	if s == "" || len([]rune(s)) > 10 {
		return ""
	}
	if strings.ContainsAny(s, "0123456789") {
		return ""
	}
	return s
}

// translate renders s through the dictionary: English words with their
// senses, Chinese runs as the English words their senses point back to.
func translateText(s string) (string, error) {
	if err := dict.load(); err != nil {
		return "", err
	}
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("没有可翻译的文字")
	}
	var sb strings.Builder
	for _, seg := range segments(s) {
		if seg.cjk {
			line := translateChinese(seg.text)
			if line != "" {
				sb.WriteString(line)
				sb.WriteString("\n")
			}
			continue
		}
		for _, w := range strings.Fields(seg.text) {
			sb.WriteString(translateWord(w))
			sb.WriteString("\n")
		}
	}
	out := strings.TrimRight(sb.String(), "\n")
	if out == "" {
		return "", fmt.Errorf("词典中没有这些文字")
	}
	return out, nil
}

type segment struct {
	text string
	cjk  bool
}

// segments splits text into latin words and CJK runs.
func segments(s string) []segment {
	var out []segment
	var cur strings.Builder
	curCJK := false
	flush := func() {
		if cur.Len() > 0 {
			out = append(out, segment{text: cur.String(), cjk: curCJK})
			cur.Reset()
		}
	}
	for _, r := range s {
		isCJK := unicode.Is(unicode.Han, r)
		if isCJK != curCJK && cur.Len() > 0 {
			flush()
		}
		curCJK = isCJK
		if !isCJK && !isWordRune(r) {
			flush()
			curCJK = false
			continue
		}
		cur.WriteRune(r)
	}
	flush()
	return out
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || r == '\'' || r == '-' || r == '_' || unicode.IsDigit(r)
}

// translateWord renders one english word.
func translateWord(w string) string {
	key := strings.ToLower(strings.Trim(w, "'-_"))
	e, ok := dict.words[key]
	if !ok {
		return w
	}
	var senses []string
	for _, s := range e.Senses {
		if t := stripPOS(s); t != "" {
			senses = append(senses, t)
		}
	}
	if len(senses) == 0 {
		return w
	}
	var sb strings.Builder
	sb.WriteString(w)
	if e.Phonetic != "" {
		sb.WriteString("  /" + e.Phonetic + "/")
	}
	sb.WriteString("  ")
	sb.WriteString(strings.Join(senses, "；"))
	return sb.String()
}

// translateChinese turns a run of Chinese into the English words its terms
// map to, longest term first, and keeps the original beside them.
func translateChinese(s string) string {
	runes := []rune(s)
	var words []string
	for i := 0; i < len(runes); {
		matched := false
		for n := min(len(runes)-i, 6); n >= 2; n-- {
			term := string(runes[i : i+n])
			if w, ok := dict.reverse[term]; ok {
				words = append(words, w.word)
				i += n
				matched = true
				break
			}
		}
		if !matched {
			i++
		}
	}
	if len(words) == 0 {
		return ""
	}
	var out []string
	for i, w := range words {
		if i > 0 && out[len(out)-1] == w {
			continue
		}
		out = append(out, w)
	}
	return strings.Join(out, " ") + "    ←  " + s
}

// ---------------------------------------------------------------------------
// The translation window.

type translationWindow struct {
	win    *mygo.Window
	status string
	errMsg string
	source string
	result string
}

// showTranslation recognizes img and shows the dictionary rendering of it.
func showTranslation(img *image.RGBA) {
	tw := &translationWindow{status: "busy"}
	tw.open()
	go func() {
		lines, err := ocrEng.Recognize(img)
		if err != nil {
			tw.update(func() { tw.status, tw.errMsg = "error", err.Error() })
			return
		}
		text := joinLines(lines)
		out, terr := translateText(text)
		tw.update(func() {
			tw.source = text
			if terr != nil {
				tw.status, tw.errMsg = "error", terr.Error()
				return
			}
			tw.result = out
			tw.status = "done"
		})
	}()
}

func (tw *translationWindow) open() {
	tw.win = dialogWindow("翻译", 780, 544, 520, 380, "translate-2", tw.view)
}

func (tw *translationWindow) close() {
	if tw.win != nil && !tw.win.IsDestroyed() {
		tw.win.Destroy()
	}
}

func (tw *translationWindow) update(fn func()) {
	if tw.win != nil && !tw.win.IsDestroyed() {
		tw.win.Update(fn)
	}
}

func joinLines(lines []ocrLine) string {
	var sb strings.Builder
	for _, l := range lines {
		sb.WriteString(l.Text)
		sb.WriteString("\n")
	}
	return strings.TrimRight(sb.String(), "\n")
}

func (tw *translationWindow) view(c *ui.Context) {
	t := c.Theme()
	c.OnShortcut(ui.Ctrl, ui.KeyC, func() { mygo.Clipboard.WriteText(tw.result) })
	c.OnShortcut(0, ui.KeyEscape, tw.close)
	ui.Column(c).Fill().Background(pageBackground(t)).Children(func() {
		dialogHeader(c, t, "translate", "翻译",
			"离线词典逐词释义，不是整句翻译", "关闭翻译", tw.close)
		dialogBody(c, t, func() {
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				switch tw.status {
				case "busy":
					ui.Text(c, "正在识别…").Grow(1).TextColor(t.TextMuted)
					ui.Progress(c, -1).Width(140)
				case "error":
					ui.Text(c, "失败: "+tw.errMsg).Grow(1).TextColor(t.Danger)
				default:
					ui.Text(c, "离线词典释义（ekdict 常用词库，逐词释义，非整句翻译）").
						Grow(1).TextColor(t.TextMuted)
				}
				ui.Button(c, "复制原文").OnClick(func() { mygo.Clipboard.WriteText(tw.source) }).
					Disabled(tw.status != "done")
				brandPrimaryButton(c, "复制结果", 120, tw.status != "done", func() {
					mygo.Clipboard.WriteText(tw.result)
				})
			})
			ui.Row(c).Fill().Grow(1).Gap(12).Children(func() {
				ui.Column(c).Fill().Grow(1).Gap(6).Children(func() {
					ui.Text(c, "原文").Bold().FontSize(12)
					ui.TextArea(c, &tw.source).Fill().Grow(1).
						Font("Consolas, 微软雅黑, monospace")
				})
				ui.Column(c).Fill().Grow(1).Gap(6).Children(func() {
					ui.Text(c, "释义").Bold().FontSize(12)
					ui.TextArea(c, &tw.result).Fill().Grow(1).
						Font("Consolas, 微软雅黑, monospace")
				})
			})
		})
	})
}
