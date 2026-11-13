package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestParsePushLine(t *testing.T) {
	progress := `{"isSuccess":true,"errorMsg":"","data":{"index":1,"total":3,"file":"a.exe","status":"START","file_size":10}}`
	ev := parsePushLine(progress)
	if ev.Kind != pushProgress || ev.Progress.File != "a.exe" || ev.Progress.Status != "START" {
		t.Fatalf("progress: %+v", ev)
	}

	fail := `{"isSuccess":false,"errorMsg":"a.exe: boom","data":{"index":1,"total":3,"file":"a.exe","status":"FAIL","file_size":10,"error":"boom"}}`
	ev = parsePushLine(fail)
	if ev.Kind != pushProgress || ev.Progress.Status != "FAIL" || ev.Error == "" {
		t.Fatalf("fail progress: %+v", ev)
	}

	success := `{"isSuccess":true,"errorMsg":"","data":{"version":"1.0.1","files":"1"}}`
	if ev = parsePushLine(success); ev.Kind != pushSuccess {
		t.Fatalf("success: %+v", ev)
	}

	done := `{"isSuccess":true,"errorMsg":"","data":null}`
	if ev = parsePushLine(done); ev.Kind != pushIgnored {
		t.Fatalf("done: %+v", ev)
	}

	noise := "Uploading a.exe"
	if ev = parsePushLine(noise); ev.Kind != pushIgnored {
		t.Fatalf("noise: %+v", ev)
	}

	failure := `{"isSuccess":false,"errorMsg":"upload failed","data":null}`
	if ev = parsePushLine(failure); ev.Kind != pushFailure || ev.Error != "upload failed" {
		t.Fatalf("failure: %+v", ev)
	}
}

func TestFormatSize(t *testing.T) {
	cases := map[int64]string{
		-1:      "0 B",
		0:       "0 B",
		1:       "1 B",
		1024:    "1 KB",
		1536:    "1.5 KB",
		1048576: "1 MB",
	}
	for in, want := range cases {
		if got := formatSize(in); got != want {
			t.Errorf("formatSize(%d)=%q want %q", in, got, want)
		}
	}
}

func TestSuggestVersion(t *testing.T) {
	cases := map[string]string{
		"V1.0.1": "V1.0.2",
		"1.2":    "1.3",
		"v2":     "v3",
		"":       "",
		"abc":    "",
	}
	for in, want := range cases {
		if got := suggestVersion(in); got != want {
			t.Errorf("suggestVersion(%q)=%q want %q", in, got, want)
		}
	}
}

func TestDiffList(t *testing.T) {
	added, removed := diffList([]string{"a", "b"}, []string{"b", "c"})
	if len(added) != 1 || added[0] != "c" || len(removed) != 1 || removed[0] != "a" {
		t.Fatalf("added=%v removed=%v", added, removed)
	}
}

func TestRelativeToProject(t *testing.T) {
	project := filepath.Join(t.TempDir(), "app")
	inside := filepath.Join(project, "bin", "a.exe")
	outside := filepath.Join(t.TempDir(), "other", "a.exe")
	if got := relativeToProject(project, inside); got != "bin/a.exe" {
		t.Fatalf("relative=%q", got)
	}
	if got := relativeToProject(project, outside); got != outside {
		t.Fatalf("absolute=%q want %q", got, outside)
	}
}

func TestParseJSONOutput(t *testing.T) {
	var out cliOutput
	if err := json.Unmarshal([]byte(`{"isSuccess":true,"data":{"staged":[],"unstaged":[]}}`), &out); err != nil {
		t.Fatal(err)
	}
	if !out.IsSuccess || len(out.Data) == 0 {
		t.Fatalf("out=%+v", out)
	}
}
