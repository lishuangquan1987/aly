package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/lishuangquan1987/aly/client/aly-client-ui/aly-client-linuxui/client"
)

func TestDecide(t *testing.T) {
	forced := true
	notForced := false
	tests := []struct {
		name   string
		chk    client.CheckUpdate
		action string
		ask    bool
	}{
		{"无更新", client.CheckUpdate{HasUpdate: false}, "none", false},
		{"需要下载-不强制", client.CheckUpdate{HasUpdate: true, NeedDownloadUpdate: true}, "download", true},
		{"需要下载-强制", client.CheckUpdate{HasUpdate: true, NeedDownloadUpdate: true, ForceUpdate: &forced}, "download", false},
		{"已下载-不强制", client.CheckUpdate{HasUpdate: true}, "apply", true},
		{"已下载-强制", client.CheckUpdate{HasUpdate: true, ForceUpdate: &forced}, "apply", false},
		{"force_update=false 视为询问", client.CheckUpdate{HasUpdate: true, ForceUpdate: &notForced}, "apply", true},
	}
	for _, tc := range tests {
		action, ask := decide(tc.chk)
		if action != tc.action || ask != tc.ask {
			t.Errorf("%s: decide=%q,%v; want %q,%v", tc.name, action, ask, tc.action, tc.ask)
		}
	}
}

func TestI18nKeysMatch(t *testing.T) {
	if len(zhCN) != len(enUS) {
		t.Fatalf("zh keys=%d en keys=%d", len(zhCN), len(enUS))
	}
	for k, v := range zhCN {
		if v == "" {
			t.Errorf("zh key %q empty", k)
		}
		if enUS[k] == "" {
			t.Errorf("en missing/empty key %q", k)
		}
	}
	if normalizeLang("en-US") != "en" || normalizeLang("zh-CN") != "zh-Hans" ||
		normalizeLang("fr") != "zh-Hans" {
		t.Fatal("language normalization mismatch")
	}
}

func TestModeFromArgs(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-mode", "check"}, "check"},
		{[]string{"-mode=rollback"}, "rollback"},
		{[]string{"-lang", "en-US", "-mode", "update"}, "update"},
		{[]string{"-lang", "en-US"}, ""},
		{[]string{"-mode", "unknown"}, "auto"},
	} {
		if got := modeFromArgs(tc.args); got != tc.want {
			t.Errorf("%v: got %q want %q", tc.args, got, tc.want)
		}
	}
}

func TestViewUpdateCard(t *testing.T) {
	a := &app{
		lang:    "zh-Hans",
		project: "Demo",
		current: "1.0.0",
		newer:   "1.0.1",
		status:  "发现新版本 v1.0.1",
		buttons: []uiButton{
			{id: btnUpdate, label: "立即更新", primary: true},
			{id: btnLater, label: "稍后"},
		},
	}
	tt := ui.NewTester(a.view, 420, 250)
	if !tt.HasText("Demo") || !tt.HasText("发现新版本 v1.0.1") ||
		!tt.HasText("立即更新") || !tt.HasText("稍后") {
		t.Fatalf("view texts = %q", tt.Texts())
	}
}

func TestViewEnglishNoUpdate(t *testing.T) {
	a := &app{
		lang:    "en",
		project: "Demo",
		current: "1.0.0",
		status:  tr("en", keyNoUpdate),
		detail:  tr("en", keyNoUpdateBox),
		buttons: []uiButton{{id: btnClose, label: tr("en", keyClose), primary: true}},
	}
	tt := ui.NewTester(a.view, 420, 250)
	if !tt.HasText("You are up to date") || !tt.HasText("Close") {
		t.Fatalf("view texts = %q", tt.Texts())
	}
}
