// +build windows

package app

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aly/client/aly-client/model"
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

// withSelfUpdateCmd 替换 UpdateUpdater 的命令注入点，便于单测断言“是否委派”。
// 返回恢复函数，调用方写成 `defer withSelfUpdateCmd(t, fn)()`：
// Go 1.10 没有 t.Cleanup（生产包按 1.10 编译，测试也保持同一工具链可跑）。
func withSelfUpdateCmd(t *testing.T,
	fn func(string) (model.SelfUpdateData, bool, error)) func() {
	t.Helper()
	orig := selfUpdateViaCmd
	selfUpdateViaCmd = fn
	return func() { selfUpdateViaCmd = orig }
}

// ────────────────────────────────────────────────────────────────
// UpdateUpdater：正常路径委派 self_update；缺失/旧命令才直接复制
// ────────────────────────────────────────────────────────────────

// TestUpdateUpdaterDelegatesToSelfUpdate 内容不同 → 必须调 self_update 命令，
// 由命令完成替换；UI 不再自己写盘。
func TestUpdateUpdaterDelegatesToSelfUpdate(t *testing.T) {
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
	writeF(t, filepath.Join(main, "aly-client.exe"), "UPDATER-V2")
	writeF(t, dst, "UPDATER-V1")
	// 预置历史失败：委派成功后应清零
	journalFixture(t, dst, 2, time.Now().Unix(), "previous failure")

	var calledWith string
	defer withSelfUpdateCmd(t, func(exePath string) (model.SelfUpdateData, bool, error) {
		calledWith = exePath
		return model.SelfUpdateData{Updated: true, Reason: "replaced"}, true, nil
	})()

	if err := UpdateUpdater(update, main); err != nil {
		t.Fatalf("UpdateUpdater: %v", err)
	}
	if calledWith != dst {
		t.Fatalf("self_update called with %q; want %q", calledWith, dst)
	}
	if j := readJournal(t, dst); j.Count != 0 || j.Pending {
		t.Fatalf("journal must be cleared after delegated success, got %+v", j)
	}
}

// TestUpdateUpdaterNoChangeIsNoop 内容相同 → 不启动进程，也不重写目标。
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

	called := false
	defer withSelfUpdateCmd(t, func(string) (model.SelfUpdateData, bool, error) {
		called = true
		return model.SelfUpdateData{}, true, nil
	})()

	if err := UpdateUpdater(update, main); err != nil {
		t.Fatalf("UpdateUpdater: %v", err)
	}
	if called {
		t.Fatal("内容一致时不得启动 self_update 进程")
	}
	st, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !st.ModTime().Equal(old) {
		t.Fatal("内容一致时不应改写目标文件")
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

// TestUpdateUpdaterMissingTargetRecovers 目标缺失（崩溃/被删）时命令起不来，
// 必须由共享 ReplaceFromFile 从主目录直接落位。
func TestUpdateUpdaterMissingTargetRecovers(t *testing.T) {
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

	called := false
	defer withSelfUpdateCmd(t, func(string) (model.SelfUpdateData, bool, error) {
		called = true
		return model.SelfUpdateData{}, true, nil
	})()

	if err := UpdateUpdater(update, main); err != nil {
		t.Fatalf("UpdateUpdater: %v", err)
	}
	if called {
		t.Fatal("目标缺失时无法启动命令，不应调用 self_update")
	}
	dst := filepath.Join(update, "aly-client.exe")
	if got := readF(t, dst); got != "UPDATER-V2" {
		t.Fatalf("目标应由直接落位恢复为 V2，实际 %q", got)
	}
	if _, err := os.Stat(dst + ".new"); !os.IsNotExist(err) {
		t.Fatalf(".new 不应残留 (err=%v)", err)
	}
	if j := readJournal(t, dst); j.Count != 0 {
		t.Fatalf("恢复成功应清 journal, got %+v", j)
	}
}

// TestUpdateUpdaterFallsBackWhenCommandUnavailable 旧版更新器不认识
// self_update（进程级失败）→ 直接复制兜底，升级链不能断。
func TestUpdateUpdaterFallsBackWhenCommandUnavailable(t *testing.T) {
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
	writeF(t, filepath.Join(main, "aly-client.exe"), "UPDATER-V2")
	writeF(t, dst, "UPDATER-V1")

	defer withSelfUpdateCmd(t, func(string) (model.SelfUpdateData, bool, error) {
		return model.SelfUpdateData{}, false, errors.New("exited with code 1: 未知命令")
	})()

	if err := UpdateUpdater(update, main); err != nil {
		t.Fatalf("旧命令应回退直接复制成功: %v", err)
	}
	if got := readF(t, dst); got != "UPDATER-V2" {
		t.Fatalf("兜底后目标应为 V2，实际 %q", got)
	}
	if j := readJournal(t, dst); j.Count != 0 {
		t.Fatalf("兜底成功应清 journal, got %+v", j)
	}
}

// TestUpdateUpdaterNoFallbackOnBusinessError 命令已执行但业务失败（锁占用）：
// 绝不能绕过 .aly.lock 直接复制。
func TestUpdateUpdaterNoFallbackOnBusinessError(t *testing.T) {
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
	writeF(t, filepath.Join(main, "aly-client.exe"), "UPDATER-V2")
	writeF(t, dst, "UPDATER-V1")

	defer withSelfUpdateCmd(t, func(string) (model.SelfUpdateData, bool, error) {
		return model.SelfUpdateData{}, true, errors.New("另一更新正在进行中，请稍后重试")
	})()

	err := UpdateUpdater(update, main)
	if err == nil {
		t.Fatal("业务失败必须返回错误")
	}
	if got := readF(t, dst); got != "UPDATER-V1" {
		t.Fatalf("锁占用时不得绕过锁改写目标，实际 %q", got)
	}
	j := readJournal(t, dst)
	if j.Count != 1 {
		t.Fatalf("journal count = %d; want 1", j.Count)
	}
	if j.LastError == "" {
		t.Fatal("journal last_error 必须记录失败原因")
	}
}

// ────────────────────────────────────────────────────────────────
// UpdateSelfUI：UI 自身仍走延迟脚本；替换/校验复用共享包
// ────────────────────────────────────────────────────────────────

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
	// 已交给延迟脚本：journal 必须是 pending（下一次启动据此判断收敛）
	if j := readJournal(t, filepath.Join(update, "aly-client-ui.exe")); !j.Pending || j.Count != 0 {
		t.Fatalf("journal should be pending with count 0, got %+v", j)
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

func TestCheckRequestEventRoundTrip(t *testing.T) {
	// 无监听者时通知应失败（事件不存在）
	if NotifyCheckRequested() {
		t.Fatal("无监听者时 NotifyCheckRequested 应返回 false")
	}

	// 起监听者：命名事件是会话级全局的，测试期间独占
	ch, stop := ListenCheckRequested()
	defer stop()

	// 有监听者时通知应成功
	if !NotifyCheckRequested() {
		t.Fatal("有监听者时 NotifyCheckRequested 应返回 true")
	}

	select {
	case <-ch:
		// 收到信号：通道投递正常
	case <-time.After(2 * time.Second):
		t.Fatal("2 秒内未收到转发信号")
	}
}

func TestCheckRequestEventStop(t *testing.T) {
	ch, stop := ListenCheckRequested()
	stop()
	select {
	case <-ch:
		// stop 关闭通道：正常退出
	case <-time.After(500 * time.Millisecond):
		t.Fatal("stop 后通道应在短时间內关闭")
	}
}
