package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJournalLifecycle(t *testing.T) {
	dir, err := ioutil.TempDir("", "journal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	p := filepath.Join(dir, "ui.selfupdate.json")
	j := loadJournal(p)
	if j.shouldSkip(3, time.Minute) {
		t.Fatal("zero journal must not skip")
	}
	j.markAttempt()
	saveJournal(p, j)
	if got := loadJournal(p); !got.Pending {
		t.Fatalf("pending not persisted: %+v", got)
	}
	j.markFailure("disk full")
	saveJournal(p, j)
	got := loadJournal(p)
	if got.Count != 1 || got.Pending || got.LastError != "disk full" {
		t.Fatalf("bad failure: %+v", got)
	}
	j.markFailure("disk full")
	j.markFailure("disk full")
	if !j.shouldSkip(3, time.Hour) {
		t.Fatal("3 failures should skip within window")
	}
	j.markSuccess()
	if j.Count != 0 || j.Pending || j.LastTS != 0 {
		t.Fatalf("markSuccess: %+v", j)
	}
}

func TestJournalWindowExpires(t *testing.T) {
	j := updateJournal{Count: 3, LastTS: time.Now().Add(-2 * time.Hour).Unix()}
	if j.shouldSkip(3, 30*time.Minute) {
		t.Fatal("expired failures must not skip")
	}
}

func TestPathsCopyAndClean(t *testing.T) {
	dir, err := ioutil.TempDir("", "paths")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	if err := ioutil.WriteFile(src, []byte("v1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := copyVerify(src, dst); err != nil {
		t.Fatal(err)
	}
	same, err := sameFile(src, dst)
	if err != nil || !same {
		t.Fatalf("same=%v err=%v", same, err)
	}
	aside := nextAsidePath(dst, ".old")
	if aside != dst+".old" {
		t.Fatalf("aside=%q", aside)
	}
	if err := ioutil.WriteFile(dst+".new", []byte("junk"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(dst+".old", []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	cleanArtifacts(dst)
	if _, err := os.Stat(dst + ".new"); !os.IsNotExist(err) {
		t.Fatal(".new should be cleaned")
	}
	if _, err := os.Stat(dst + ".old"); !os.IsNotExist(err) {
		t.Fatal(".old should be cleaned when target exists")
	}
}

func TestUpdateSelfUISpawnsFinish(t *testing.T) {
	dir, err := ioutil.TempDir("", "selfui")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "Main")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(main, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(update, "aly-client-ui")
	src := filepath.Join(main, "aly-client-ui")
	writeFile(t, target, "OLD")
	writeFile(t, src, "NEW")
	l := layout{update: update, main: main, ui: target, updater: filepath.Join(update, "aly-client")}

	var gotPath string
	var gotArgs []string
	orig := spawnFinishFn
	spawnFinishFn = func(exePath string, args []string) error {
		gotPath, gotArgs = exePath, append([]string(nil), args...)
		return nil
	}
	defer func() { spawnFinishFn = orig }()

	replaced, err := updateSelfUI(l, []string{"-mode", "check"})
	if err != nil {
		t.Fatal(err)
	}
	if !replaced {
		t.Fatal("should replace when content differs")
	}
	if gotPath != target+".new" {
		t.Fatalf("spawned %q", gotPath)
	}
	if len(gotArgs) < 11 || gotArgs[0] != "-finish-self-update" || gotArgs[9] != "--" {
		t.Fatalf("args=%#v", gotArgs)
	}
	if got := readFile(t, target+".new"); got != "NEW" {
		t.Fatalf(".new=%q", got)
	}
	j := loadJournal(journalPath(target))
	if !j.Pending {
		t.Fatalf("journal pending not set: %+v", j)
	}
}

func TestUpdateSelfUISameIsNoop(t *testing.T) {
	dir, err := ioutil.TempDir("", "selfui-same")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "Main")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(main, 0755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(update, "aly-client-ui")
	src := filepath.Join(main, "aly-client-ui")
	writeFile(t, target, "SAME")
	writeFile(t, src, "SAME")
	l := layout{update: update, main: main, ui: target, updater: filepath.Join(update, "aly-client")}
	orig := spawnFinishFn
	spawnFinishFn = func(string, []string) error { t.Fatal("must not spawn"); return nil }
	defer func() { spawnFinishFn = orig }()
	if replaced, err := updateSelfUI(l, nil); err != nil || replaced {
		t.Fatalf("replaced=%v err=%v", replaced, err)
	}
}

func writeFile(t *testing.T, p, s string) {
	t.Helper()
	if err := ioutil.WriteFile(p, []byte(s), 0755); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
