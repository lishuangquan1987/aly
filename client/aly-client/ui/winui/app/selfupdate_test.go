// +build windows

package app

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func mkTmp(t *testing.T) (string, func()) {
	dir, err := ioutil.TempDir("", "alyself")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	return dir, func() { os.RemoveAll(dir) }
}

func writeF(t *testing.T, p, content string) {
	if err := ioutil.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile %s: %v", p, err)
	}
}

func readF(t *testing.T, p string) string {
	b, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", p, err)
	}
	return string(b)
}

func TestSha256OfAndSameFile(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	a := filepath.Join(dir, "a.bin")
	b := filepath.Join(dir, "b.bin")
	c := filepath.Join(dir, "c.bin")
	writeF(t, a, "hello")
	writeF(t, b, "hello")
	writeF(t, c, "world")

	same, err := sameFile(a, b)
	if err != nil {
		t.Fatalf("sameFile: %v", err)
	}
	if !same {
		t.Fatal("identical content must compare equal")
	}

	same, err = sameFile(a, c)
	if err != nil {
		t.Fatalf("sameFile: %v", err)
	}
	if same {
		t.Fatal("different content must not compare equal")
	}

	if _, err := sha256Of(filepath.Join(dir, "nope")); err == nil {
		t.Fatal("sha256Of on missing file must error")
	}
}

func TestReplaceFromSourceReplaces(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	src := filepath.Join(dir, "src.exe")
	dst := filepath.Join(dir, "dst.exe")
	writeF(t, src, "NEW")
	writeF(t, dst, "OLD")

	if err := replaceFromSource(src, dst, 2, 10*time.Millisecond); err != nil {
		t.Fatalf("replaceFromSource: %v", err)
	}
	if got := readF(t, dst); got != "NEW" {
		t.Fatalf("dst should be replaced, got %q", got)
	}
	// 临时文件必须清理干净
	if _, err := os.Stat(dst + ".new"); !os.IsNotExist(err) {
		t.Fatal("dst.new should be removed after success")
	}
}

// 关键：源文件缺失时**必须保留旧目标文件**，不能把它删掉。
// 这正是 SDK bug #8 的语义，也是「先删后改名」写法会踩的坑。
func TestReplaceFromSourceMissingSrcKeepsDst(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	src := filepath.Join(dir, "missing.exe")
	dst := filepath.Join(dir, "dst.exe")
	writeF(t, dst, "OLD")

	if err := replaceFromSource(src, dst, 2, 10*time.Millisecond); err == nil {
		t.Fatal("expected error when src is missing")
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("dst must be preserved, stat err=%v", err)
	}
	if got := readF(t, dst); got != "OLD" {
		t.Fatalf("dst content must be untouched, got %q", got)
	}
}

func TestUpdateUpdaterCopies(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	writeF(t, filepath.Join(main, "aly-client.exe"), "UPDATER-V2")
	writeF(t, filepath.Join(update, "aly-client.exe"), "UPDATER-V1")

	if err := UpdateUpdater(update, main); err != nil {
		t.Fatalf("UpdateUpdater: %v", err)
	}
	if got := readF(t, filepath.Join(update, "aly-client.exe")); got != "UPDATER-V2" {
		t.Fatalf("updater should be updated, got %q", got)
	}
}

// UpdateUpdater 与 UpdateSelfUI 口径一致（见 selfupdate.go）：
// 内容相同不重写、源缺失不算错误。
func TestUpdateUpdaterNoChangeIsNoop(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	dst := filepath.Join(update, "aly-client.exe")
	writeF(t, filepath.Join(main, "aly-client.exe"), "SAME")
	writeF(t, dst, "SAME")

	// 把目标 mtime 拨到一小时前：若发生重写，mtime 会变
	old := time.Now().Add(-time.Hour).Truncate(time.Second)
	if err := os.Chtimes(dst, old, old); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	if err := UpdateUpdater(update, main); err != nil {
		t.Fatalf("UpdateUpdater: %v", err)
	}
	st, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !st.ModTime().Equal(old) {
		t.Fatal("内容一致时不应改写目标文件（与 UpdateSelfUI 口径一致）")
	}
}

// 主程序目录没有随包分发的 aly-client.exe 时：跳过且保留 UpdateFolder 里的旧文件。
func TestUpdateUpdaterMissingSrcIsNotError(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	writeF(t, filepath.Join(update, "aly-client.exe"), "UPDATER-V1")

	if err := UpdateUpdater(update, main); err != nil {
		t.Fatalf("源缺失不应报错: %v", err)
	}
	if got := readF(t, filepath.Join(update, "aly-client.exe")); got != "UPDATER-V1" {
		t.Fatalf("目标应保持旧内容，实际 %q", got)
	}
}

// UpdateSelfUI 的 spawn 用注入桩替换，避免测试真的去 move/start 进程。
func TestUpdateSelfUIReplacesAndSpawns(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	writeF(t, filepath.Join(main, "aly-client-ui.exe"), "UI-V2")
	writeF(t, filepath.Join(update, "aly-client-ui.exe"), "UI-V1")

	var spawned string
	orig := spawnDetachedFn
	spawnDetachedFn = func(cmdLine string) error {
		spawned = cmdLine
		return nil
	}
	defer func() { spawnDetachedFn = orig }()

	// 重启必须带上原参数：否则手动「检查更新」重启后会退化为静默 auto 模式
	replaced, err := UpdateSelfUI(update, main, []string{"-mode", "check", "-lang", "zh-CN"})
	if err != nil {
		t.Fatalf("UpdateSelfUI: %v", err)
	}
	if !replaced {
		t.Fatal("replaced should be true when content differs")
	}
	// .new 已就绪且内容为新版
	if got := readF(t, filepath.Join(update, "aly-client-ui.exe.new")); got != "UI-V2" {
		t.Fatalf(".new content wrong: %q", got)
	}
	// 延迟脚本必须包含 move 与 start，且路径带引号；并透传原启动参数
	for _, want := range []string{
		"move /y", "start \"\"", "aly-client-ui.exe.new",
		`"-mode" "check"`, `"-lang" "zh-CN"`,
	} {
		if !containsStr(spawned, want) {
			t.Fatalf("spawn cmdline %q should contain %q", spawned, want)
		}
	}
}

func TestUpdateSelfUINoChangeIsNoop(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	writeF(t, filepath.Join(main, "aly-client-ui.exe"), "SAME")
	writeF(t, filepath.Join(update, "aly-client-ui.exe"), "SAME")

	orig := spawnDetachedFn
	spawnDetachedFn = func(string) error {
		t.Fatal("must not spawn when content is identical")
		return nil
	}
	defer func() { spawnDetachedFn = orig }()

	replaced, err := UpdateSelfUI(update, main, nil)
	if err != nil {
		t.Fatalf("UpdateSelfUI: %v", err)
	}
	if replaced {
		t.Fatal("replaced should be false when content is identical")
	}
}

func TestUpdateSelfUIMissingSrcIsNotError(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	// 主程序目录没有随包分发的 UI
	replaced, err := UpdateSelfUI(update, main, nil)
	if err != nil {
		t.Fatalf("missing src should not be an error, got %v", err)
	}
	if replaced {
		t.Fatal("replaced should be false")
	}
}

func TestAcquireSingleInstance(t *testing.T) {
	first, release1 := AcquireSingleInstance()
	if !first {
		t.Fatal("first acquire must succeed")
	}

	second, release2 := AcquireSingleInstance()
	if second {
		release2()
		t.Fatal("second acquire in the same process must fail")
	}
	release2()

	release1()

	third, release3 := AcquireSingleInstance()
	if !third {
		t.Fatal("acquire after release must succeed")
	}
	release3()
}

// 直接验证 spawnDetached 的命令行传递是**原样**的。
// 若退回 exec.Command("cmd.exe","/c",cmdLine)，Go 会把引号转义成 \"
// 而 cmd.exe 不认，下面这条重定向就会失败。
func TestSpawnDetachedPassesQuotesVerbatim(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	// 目录名带空格，确保引号真的被用上
	sub := filepath.Join(dir, "dir with space")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	target := filepath.Join(sub, "out.txt")

	if err := spawnDetached(`echo spawned> "` + target + `"`); err != nil {
		t.Fatalf("spawnDetached: %v", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := ioutil.ReadFile(target); err == nil {
			if containsStr(string(b), "spawned") {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("spawned command did not write %s (cmdline quoting broken?)", target)
}

func containsStr(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
