package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aly/client/aly-client/config"
)

// newSelfUpdateTestEnv 搭建 self_update 测试环境：
// root/UpdateFolder（ExeDir，模拟"正在运行的更新器所在目录"）
// + root/ApplicationFolder（MainFolder，模拟"随版本分发的权威副本"）。
// 返回 root 与 fc；selfExePathFn 由各用例自行注入。
func newSelfUpdateTestEnv(t *testing.T, name string) (string, *FullConfig) {
	t.Helper()
	root, err := ioutil.TempDir("", name)
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })

	if err := ioutil.WriteFile(filepath.Join(update, "client.json"),
		[]byte(`{"main_exe_relative_path":"../ApplicationFolder/app.exe","must_close_process_name":[]}`), 0644); err != nil {
		t.Fatalf("写 client.json 失败: %v", err)
	}
	main := filepath.Join(root, "ApplicationFolder")
	if err := os.MkdirAll(main, 0755); err != nil {
		t.Fatalf("创建 MainFolder 失败: %v", err)
	}
	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: main,
	}
	return root, fc
}

// withSelfPath 注入 self 路径（模拟 os.Executable 的返回）。
func withSelfPath(t *testing.T, selfPath string) {
	t.Helper()
	orig := selfExePathFn
	selfExePathFn = func() (string, error) { return selfPath, nil }
	t.Cleanup(func() { selfExePathFn = orig })
}

// TestRunSelfUpdateAlreadyUpToDate 一致时跳过 + 顺带清理 .new/.old 残留。
func TestRunSelfUpdateAlreadyUpToDate(t *testing.T) {
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-same")
	self := filepath.Join(fc.MainFolder, "..", "UpdateFolder", config.UpdaterFileName())
	self = filepath.Clean(self)
	withSelfPath(t, self)

	src := filepath.Join(fc.MainFolder, config.UpdaterFileName())
	if err := ioutil.WriteFile(src, []byte("V1"), 0755); err != nil {
		t.Fatalf("写源: %v", err)
	}
	if err := ioutil.WriteFile(self, []byte("V1"), 0755); err != nil {
		t.Fatalf("写 self: %v", err)
	}
	// 预置残留：一致路径下必须被清理
	if err := ioutil.WriteFile(self+".new", []byte("junk"), 0755); err != nil {
		t.Fatalf("写 .new: %v", err)
	}
	if err := ioutil.WriteFile(self+".old", []byte("older"), 0755); err != nil {
		t.Fatalf("写 .old: %v", err)
	}

	data, err := runSelfUpdate(fc)
	if err != nil {
		t.Fatalf("runSelfUpdate: %v", err)
	}
	if data.Updated {
		t.Fatal("identical content must not trigger replace")
	}
	if data.Reason != "already_up_to_date" {
		t.Fatalf("reason = %q; want already_up_to_date", data.Reason)
	}
	if _, err := os.Stat(self + ".new"); !os.IsNotExist(err) {
		t.Fatalf(".new should be cleaned in up-to-date path (err=%v)", err)
	}
	if _, err := os.Stat(self + ".old"); !os.IsNotExist(err) {
		t.Fatalf(".old should be cleaned in up-to-date path (err=%v)", err)
	}
}

// TestRunSelfUpdateReplaces 差异时执行舞步替换：self 换新、.old 备份留存旧内容。
func TestRunSelfUpdateReplaces(t *testing.T) {
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-replace")
	self := filepath.Clean(filepath.Join(fc.MainFolder, "..", "UpdateFolder", config.UpdaterFileName()))
	withSelfPath(t, self)

	src := filepath.Join(fc.MainFolder, config.UpdaterFileName())
	if err := ioutil.WriteFile(src, []byte("V2-from-package"), 0755); err != nil {
		t.Fatalf("写源: %v", err)
	}
	if err := ioutil.WriteFile(self, []byte("V1-running"), 0755); err != nil {
		t.Fatalf("写 self: %v", err)
	}

	data, err := runSelfUpdate(fc)
	if err != nil {
		t.Fatalf("runSelfUpdate: %v", err)
	}
	if !data.Updated || data.Reason != "replaced" {
		t.Fatalf("data = %+v; want updated=true reason=replaced", data)
	}
	if data.OldPath == "" {
		t.Fatal("replaced path must carry old_path")
	}
	if got := readFileOrFail(t, self); got != "V2-from-package" {
		t.Fatalf("self content = %q; want new version", got)
	}
	if got := readFileOrFail(t, data.OldPath); got != "V1-running" {
		t.Fatalf(".old content = %q; want old version", got)
	}
	// .new 中转物不得残留
	if _, err := os.Stat(self + ".new"); !os.IsNotExist(err) {
		t.Fatalf(".new should not remain after replace (err=%v)", err)
	}
}

// TestRunSelfUpdateSourceMissing 主目录没有随包分发更新器：跳过不算错误。
func TestRunSelfUpdateSourceMissing(t *testing.T) {
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-missingsrc")
	self := filepath.Clean(filepath.Join(fc.MainFolder, "..", "UpdateFolder", config.UpdaterFileName()))
	withSelfPath(t, self)
	// self 存在、src 缺失
	if err := ioutil.WriteFile(self, []byte("V1"), 0755); err != nil {
		t.Fatalf("写 self: %v", err)
	}

	data, err := runSelfUpdate(fc)
	if err != nil {
		t.Fatalf("source missing must not be an error: %v", err)
	}
	if data.Updated {
		t.Fatal("must not replace when source missing")
	}
	if data.Reason != "source_missing" {
		t.Fatalf("reason = %q; want source_missing", data.Reason)
	}
	// self 必须原样保留
	if got := readFileOrFail(t, self); got != "V1" {
		t.Fatalf("self must be untouched, got %q", got)
	}
}

// TestRunSelfUpdateRefusesWhenLockHeld 另一更新（apply/download/rollback）
// 持锁时 self_update 必须拒绝，不能读半换位的主目录。
func TestRunSelfUpdateRefusesWhenLockHeld(t *testing.T) {
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-locked")
	self := filepath.Clean(filepath.Join(fc.MainFolder, "..", "UpdateFolder", config.UpdaterFileName()))
	withSelfPath(t, self)
	if err := ioutil.WriteFile(filepath.Join(fc.MainFolder, config.UpdaterFileName()), []byte("V2"), 0755); err != nil {
		t.Fatalf("写源: %v", err)
	}
	if err := ioutil.WriteFile(self, []byte("V1"), 0755); err != nil {
		t.Fatalf("写 self: %v", err)
	}

	release, err := AcquireUpdateLock("test-holder")
	if err != nil {
		t.Fatalf("获取测试锁失败: %v", err)
	}
	defer release()

	_, err = runSelfUpdate(fc)
	if err == nil {
		t.Fatal("锁被占用时 self_update 必须拒绝")
	}
	if !strings.Contains(err.Error(), "另一更新正在进行中") {
		t.Fatalf("error = %v; want lock contention message", err)
	}
	if got := readFileOrFail(t, self); got != "V1" {
		t.Fatalf("被拒绝时 self 必须原样, got %q", got)
	}
}

// TestRunSelfUpdateSelfMissing self 缺失（异常现场）：应报错而不是静默跳过。
func TestRunSelfUpdateSelfMissing(t *testing.T) {
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-missingself")
	self := filepath.Clean(filepath.Join(fc.MainFolder, "..", "UpdateFolder", config.UpdaterFileName()))
	withSelfPath(t, self)
	if err := ioutil.WriteFile(filepath.Join(fc.MainFolder, config.UpdaterFileName()), []byte("V1"), 0755); err != nil {
		t.Fatalf("写源: %v", err)
	}

	_, err := runSelfUpdate(fc)
	if err == nil {
		t.Fatal("missing self must be an error (cannot silently 'update' a nonexistent file)")
	}
	if !strings.Contains(err.Error(), "compare self vs source") {
		t.Fatalf("error = %v; want compare stage failure", err)
	}
}
