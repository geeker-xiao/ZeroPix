// Talking to an OpenAI compatible chat service. Only the test of the
// settings dialog uses it for now; it is here so that a wrong address or
// token shows up at once, not at the first translation.

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"
)

const (
	// The defaults of the OpenAI compatible service: the official endpoint
	// and the cheapest model of it.
	defaultOpenAIURL   = "https://api.openai.com/v1/chat/completions"
	defaultOpenAIModel = "gpt-4o-mini"
	// translateOpenAI is what the service is called in the dialog; the
	// value lives in settings.json, so it keeps its wording.
	translateOpenAI = "OpenAI 兼容"
	// aiTimeout bounds a request: a service that does not answer in this
	// long is not one the dialog should wait on.
	aiTimeout = 25 * time.Second
)

// normalizeTranslate maps a stored service onto one the dialog offers. Names
// from older builds fall back to the offline dictionary.
func normalizeTranslate(v string) string {
	switch strings.TrimSpace(v) {
	case translateOpenAI, "OpenAI 兼容接口", "OpenAI", "openai", "openai-compatible":
		return translateOpenAI
	}
	return "未配置"
}

// goVersion is the language the binary was built with, as "Go 1.27.3".
func goVersion() string {
	v := runtime.Version()
	return "Go " + strings.TrimPrefix(v, "go")
}

// shortDuration writes an elapsed time as a test would report it:
// milliseconds under a second, tenths above it.
func shortDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// aiChat sends one exchange to the service and returns its answer.
func aiChat(cfg appSettings, system, user string) (string, error) {
	url := strings.TrimSpace(cfg.OpenAIURL)
	if url == "" {
		return "", errors.New("还没有填接口地址")
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}
	model := strings.TrimSpace(cfg.OpenAIModel)
	if model == "" {
		model = defaultOpenAIModel
	}
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		"temperature": 0,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", errors.New("接口地址无效：" + url)
	}
	req.Header.Set("Content-Type", "application/json")
	if key := strings.TrimSpace(cfg.OpenAIKey); key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	// The extra headers go last: a service that wants its own token header,
	// or a gateway that wants an organization, says so here.
	for name, value := range parseHeaderLines(cfg.OpenAIHeader) {
		req.Header.Set(name, value)
	}

	client := &http.Client{Timeout: aiTimeout}
	resp, err := client.Do(req)
	if err != nil {
		var ne interface{ Timeout() bool }
		if errors.As(err, &ne) && ne.Timeout() {
			return "", fmt.Errorf("等了 %s 没有回应", shortDuration(aiTimeout))
		}
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("HTTP %d：%s", resp.StatusCode, snippet(data))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", errors.New("返回的不是 JSON：" + snippet(data))
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", errors.New(out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", errors.New("返回里没有 choices：" + snippet(data))
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

// parseHeaderLines reads the extra headers written one a line, as
// "Name: Value" or "Name=Value". Blank lines and lines starting with # are
// ignored, so the field can carry a comment.
func parseHeaderLines(text string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sep := strings.IndexAny(line, ":=")
		if sep <= 0 {
			continue
		}
		name := strings.TrimSpace(line[:sep])
		value := strings.TrimSpace(line[sep+1:])
		if name == "" {
			continue
		}
		out[name] = value
	}
	return out
}

// snippet is the start of a response body, on one line, for an error message.
func snippet(data []byte) string {
	s := strings.Join(strings.Fields(string(data)), " ")
	if s == "" {
		return "（没有内容）"
	}
	r := []rune(s)
	if len(r) > 160 {
		return string(r[:160]) + "…"
	}
	return s
}
