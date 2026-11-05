package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// The service names of older builds map onto the two the dialog offers now.
func TestNormalizeTranslate(t *testing.T) {
	for _, in := range []string{"OpenAI 兼容", "OpenAI 兼容接口", "OpenAI", "openai"} {
		if got := normalizeTranslate(in); got != translateOpenAI {
			t.Errorf("normalizeTranslate(%q) = %q, want %q", in, got, translateOpenAI)
		}
	}
	for _, in := range []string{"", "未配置", "微软翻译", "DeepL"} {
		if got := normalizeTranslate(in); got != "未配置" {
			t.Errorf("normalizeTranslate(%q) = %q, want 未配置", in, got)
		}
	}
	if len(translateServices) != 2 || translateServices[1] != translateOpenAI {
		t.Errorf("services = %q", translateServices)
	}
}

// A file written by a build that knew other services loads with its name
// brought up to date, and the file is written back with it.
func TestLoadSettingsRenamesTheService(t *testing.T) {
	path := t.TempDir() + "/settings.json"
	settingsFileOverride = path
	t.Cleanup(func() { settingsFileOverride = ""; settings = defaultSettings() })
	if err := os.WriteFile(path, []byte(`{"translateService":"OpenAI 兼容接口","targetLang":"English"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	loadSettings()
	if settings.Translate != translateOpenAI {
		t.Errorf("service = %q, want %q", settings.Translate, translateOpenAI)
	}
	if settings.TargetLang != "English" {
		t.Errorf("the rest of the file was lost: %q", settings.TargetLang)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "兼容接口") {
		t.Errorf("the file still carries the old name: %s", data)
	}
	if !strings.Contains(string(data), "OpenAI 兼容") {
		t.Errorf("the file was not written back: %s", data)
	}
}

// The rows of the service are there only while it is the chosen one.
func TestOpenAIRowsShowWhenChosen(t *testing.T) {
	settingsFileOverride = t.TempDir() + "/settings.json"
	t.Cleanup(func() { settingsFileOverride = ""; settings = defaultSettings() })

	render := func(service string) *ui.Tester {
		t.Helper()
		settings = defaultSettings()
		settings.Translate = service
		w := &settingsWindow{theme: themeIndex(settings.Theme), saved: settings, section: 3}
		tt := ui.NewTester(w.view, settingsWinW, settingsWinH)
		tt.Frame()
		return tt
	}

	off := render("未配置")
	for _, gone := range []string{"接口地址", "Token", "自定义 Header", "测试效果"} {
		if off.HasText(gone) {
			t.Errorf("offline service still shows %q, have %q", gone, off.Texts())
		}
	}

	on := render(translateOpenAI)
	for _, want := range []string{
		"接口地址", "到 /chat/completions 的完整地址",
		"模型", "Token", "自定义 Header", "每行一条 Name: Value",
		"测试效果", "测试",
	} {
		if !on.HasText(want) {
			t.Errorf("service rows are missing %q, have %q", want, on.Texts())
		}
	}
	// The defaults are filled in, so a test can be run at once.
	if settings.OpenAIURL != defaultOpenAIURL || settings.OpenAIModel != defaultOpenAIModel {
		t.Errorf("defaults = %q %q", settings.OpenAIURL, settings.OpenAIModel)
	}
}

// The extra headers are one a line, and a comment or a typo is skipped
// rather than sent.
func TestParseHeaderLines(t *testing.T) {
	got := parseHeaderLines(strings.Join([]string{
		"# the gateway wants its own key",
		"",
		"X-Api-Key: secret",
		"  X-Org = team-a  ",
		"broken",
		": nope",
	}, "\n"))
	want := map[string]string{"X-Api-Key": "secret", "X-Org": "team-a"}
	if len(got) != len(want) {
		t.Fatalf("headers = %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("header %q = %q, want %q", k, got[k], v)
		}
	}
}

// The request carries what the dialog holds, and the answer comes back.
func TestAIChatSendsTheSettings(t *testing.T) {
	var gotPath, gotAuth, gotOrg, gotModel, gotUser string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		gotOrg = r.Header.Get("X-Org")
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel = body.Model
		if len(body.Messages) > 0 {
			gotUser = body.Messages[len(body.Messages)-1].Content
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":" 正常 "}}]}`))
	}))
	defer srv.Close()

	cfg := defaultSettings()
	cfg.OpenAIURL = srv.URL + "/v1/chat/completions"
	cfg.OpenAIKey = "sk-test"
	cfg.OpenAIHeader = "X-Org: team-a"
	answer, err := aiChat(cfg, "system", "只回复两个字：正常")
	if err != nil {
		t.Fatalf("aiChat: %v", err)
	}
	if answer != "正常" {
		t.Errorf("answer = %q", answer)
	}
	if gotPath != "/v1/chat/completions" || gotAuth != "Bearer sk-test" || gotOrg != "team-a" {
		t.Errorf("request: path %q auth %q org %q", gotPath, gotAuth, gotOrg)
	}
	if gotModel != defaultOpenAIModel || gotUser != "只回复两个字：正常" {
		t.Errorf("body: model %q user %q", gotModel, gotUser)
	}
}

// A refused request says why, in one line, so that the dialog can show it.
func TestAIChatReportsTheReason(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("{\"error\":{\"message\":\"Incorrect API key\"}}"))
	}))
	defer srv.Close()

	cfg := defaultSettings()
	cfg.OpenAIURL = srv.URL
	_, err := aiChat(cfg, "system", "hi")
	if err == nil {
		t.Fatal("aiChat accepted a 401")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "Incorrect API key") {
		t.Errorf("error = %v, want the status and the reason", err)
	}
	// Without an address there is nothing to send to.
	cfg.OpenAIURL = ""
	if _, err := aiChat(cfg, "s", "u"); err == nil {
		t.Error("aiChat accepted an empty address")
	}
}
