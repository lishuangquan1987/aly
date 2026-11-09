// +build windows

package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aly/client/aly-client/model"
	"aly/client/aly-client/selfupdate"
)

// journalFixture 直接预置 journal 文件（模拟历史失败现场）。
func journalFixture(t *testing.T, target string, count int, lastTS int64, lastErr string) {
	t.Helper()
	selfupdate.SaveJournal(selfupdate.JournalPath(target), selfupdate.Journal{
		Count:     count,
		LastTS:    lastTS,
		LastError: lastErr,
	})
}

func readJournal(t *testing.T, target string) selfupdate.Journal {
	t.Helper()
	return selfupdate.LoadJournal(selfupdate.JournalPath(target))
}

// TestUpdateUpdaterFailureMarksJournal 替换失败必须留痕（journal 计数 +1，
// 错误原因可读）——此前失败被调用方 `_ = UpdateUpdater(...)` 静默吞掉。
func TestUpdateUpdaterFailureMarksJournal(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	writeF(t, filepath.Join(update, "aly-client.exe"), "UI-V1")
	writeF(t, filepath.Join(main, "aly-client.exe"), "UI-V2")
	// 命令已执行但业务失败（磁盘满/锁占用等）：UpdateUpdater 必须留痕且不兜底。
	defer withSelfUpdateCmd(t, func(string) (model.SelfUpdateData, bool, error) {
		return model.SelfUpdateData{}, true, errors.New("copy failed: disk full")
	})()

	err := UpdateUpdater(update, main)
	if err == nil {
		t.Fatal("命令业务失败必须返回错误")
	}
	j := readJournal(t, filepath.Join(update, "aly-client.exe"))
	if j.Count != 1 {
		t.Fatalf("journal count = %d; want 1", j.Count)
	}
	if j.LastError == "" {
		t.Fatal("journal last_error must carry the failure reason")
	}
	// 现场保持：dst 原样
	if got := readF(t, filepath.Join(update, "aly-client.exe")); got != "UI-V1" {
		t.Fatalf("dst must keep old content, got %q", got)
	}
}

// TestUpdateUpdaterNoopClearsJournal 一致（收敛）时清零计数。
func TestUpdateUpdaterNoopClearsJournal(t *testing.T) {
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
	writeF(t, dst, "SAME-CONTENT")
	writeF(t, filepath.Join(main, "aly-client.exe"), "SAME-CONTENT")
	// 预置历史失败 + 残留
	journalFixture(t, dst, 2, time.Now().Unix(), "previous failure")
	writeF(t, dst+".old", "junk")

	if err := UpdateUpdater(update, main); err != nil {
		t.Fatalf("UpdateUpdater noop: %v", err)
	}
	if j := readJournal(t, dst); j.Count != 0 {
		t.Fatalf("journal must be cleared on convergence, count=%d", j.Count)
	}
	if _, err := os.Stat(dst + ".old"); !os.IsNotExist(err) {
		t.Fatalf("stale .old should be cleaned on convergence (err=%v)", err)
	}
}

// TestUpdateSelfUISkipsAfterMaxFailures 连续失败达到上限后跳过自更新：
// 不复制、不派生收尾进程（防无限重启循环的核心断言）。
func TestUpdateSelfUISkipsAfterMaxFailures(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	self := filepath.Join(update, "aly-client-ui.exe")
	writeF(t, self, "UI-V1")
	writeF(t, filepath.Join(main, "aly-client-ui.exe"), "UI-V2")
	journalFixture(t, self, maxSelfUpdateAttempts, time.Now().Unix(), "disk full")

	spawned := false
	orig := spawnFinishFn
	spawnFinishFn = func(string, []string) error {
		spawned = true
		return nil
	}
	defer func() { spawnFinishFn = orig }()

	replaced, err := UpdateSelfUI(update, main, []string{"-mode", "check"})
	if err != nil {
		t.Fatalf("skip path must not error, got %v", err)
	}
	if replaced {
		t.Fatal("must skip self update after max consecutive failures")
	}
	if spawned {
		t.Fatal("delayed replacement script must not be spawned in skip path")
	}
	// 现场保持：self 原样、无 .new
	if got := readF(t, self); got != "UI-V1" {
		t.Fatalf("self must be untouched in skip path, got %q", got)
	}
	if _, err := os.Stat(self + ".new"); !os.IsNotExist(err) {
		t.Fatalf(".new must not be created in skip path (err=%v)", err)
	}
}

// TestUpdateSelfUISkipExpiresAfterWindow 窗口过后跳过解除、允许再试。
func TestUpdateSelfUISkipExpiresAfterWindow(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	self := filepath.Join(update, "aly-client-ui.exe")
	writeF(t, self, "UI-V1")
	writeF(t, filepath.Join(main, "aly-client-ui.exe"), "UI-V2")
	// 失败 3 次，但最后一次已在窗口外（31 分钟前）
	journalFixture(t, self, maxSelfUpdateAttempts,
		time.Now().Unix()-int64((selfUpdateRetryWindow+time.Minute)/time.Second), "disk full")

	orig := spawnFinishFn
	spawnFinishFn = func(string, []string) error { return nil }
	defer func() { spawnFinishFn = orig }()

	replaced, err := UpdateSelfUI(update, main, []string{})
	if err != nil {
		t.Fatalf("expired window must allow retry: %v", err)
	}
	if !replaced {
		t.Fatal("expired window must not skip")
	}
}

// TestUpdateSelfUIFailureIncrementsJournal 复制失败路径：计数累加且原因留痕。
func TestUpdateSelfUIFailureIncrementsJournal(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	self := filepath.Join(update, "aly-client-ui.exe")
	writeF(t, self, "UI-V1")
	// src 是目录 → copyFile 失败
	if err := os.MkdirAll(filepath.Join(main, "aly-client-ui.exe"), 0755); err != nil {
		t.Fatalf("构造目录型源失败: %v", err)
	}

	replaced, err := UpdateSelfUI(update, main, []string{})
	if err == nil || replaced {
		t.Fatalf("copy failure must return (false, err), got (%v, %v)", replaced, err)
	}
	j := readJournal(t, self)
	if j.Count != 1 || j.LastError == "" {
		t.Fatalf("journal = %+v; want count=1 with last_error", j)
	}
}

// TestUpdateSelfUINoopClearsJournal 重启后收敛（self==src）：清零计数 + 清残留。
func TestUpdateSelfUINoopClearsJournal(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	self := filepath.Join(update, "aly-client-ui.exe")
	writeF(t, self, "SAME-CONTENT")
	writeF(t, filepath.Join(main, "aly-client-ui.exe"), "SAME-CONTENT")
	journalFixture(t, self, 2, time.Now().Unix(), "previous failure")
	writeF(t, self+".new", "stale")

	replaced, err := UpdateSelfUI(update, main, []string{})
	if err != nil || replaced {
		t.Fatalf("noop must be (false, nil), got (%v, %v)", replaced, err)
	}
	if j := readJournal(t, self); j.Count != 0 {
		t.Fatalf("journal must be cleared on convergence, count=%d", j.Count)
	}
	if _, err := os.Stat(self + ".new"); !os.IsNotExist(err) {
		t.Fatalf("stale .new must be cleaned on convergence (err=%v)", err)
	}
}

// TestJournalSurvivalAcrossRestarts 端到端口径：失败累计 → 达到上限跳过 →
// （模拟重启后）收敛清零 → 再次可更新。覆盖 journal 的完整生命周期。
func TestJournalSurvivalAcrossRestarts(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	self := filepath.Join(update, "aly-client-ui.exe")
	writeF(t, self, "UI-V1")
	if err := os.MkdirAll(filepath.Join(main, "aly-client-ui.exe"), 0755); err != nil {
		t.Fatalf("构造目录型源失败: %v", err)
	}

	orig := spawnFinishFn
	spawnFinishFn = func(string, []string) error { return nil }
	defer func() { spawnFinishFn = orig }()

	// "重启" 1/3：失败计数 1
	if _, err := UpdateSelfUI(update, main, []string{}); err == nil {
		t.Fatal("attempt 1 must fail")
	}
	// "重启" 2/3：计数 2（journal 从盘上读回，跨进程存活）
	if _, err := UpdateSelfUI(update, main, []string{}); err == nil {
		t.Fatal("attempt 2 must fail")
	}
	if j := readJournal(t, self); j.Count != 2 {
		t.Fatalf("count across restarts = %d; want 2", j.Count)
	}

	// 修复现场（源恢复正常）后一次成功替换 → 下次启动收敛清零
	if err := os.RemoveAll(filepath.Join(main, "aly-client-ui.exe")); err != nil {
		t.Fatalf("移除目录型源: %v", err)
	}
	writeF(t, filepath.Join(main, "aly-client-ui.exe"), "UI-V2")
	if _, err := UpdateSelfUI(update, main, []string{}); err != nil {
		t.Fatalf("recovery attempt must succeed: %v", err)
	}
	writeF(t, self, "UI-V2") // 模拟收尾进程完成替换 + 重启
	if replaced, err := UpdateSelfUI(update, main, []string{}); err != nil || replaced {
		t.Fatalf("post-restart noop must be (false, nil), got (%v, %v)", replaced, err)
	}
	if j := readJournal(t, self); j.Count != 0 {
		t.Fatalf("journal must be cleared after convergence, count=%d", j.Count)
	}
}

// TestUpdateSelfUIPendingAttemptCounted 上一次已交给收尾进程但未收敛（Pending）：
// 本次启动必须先补记一次失败，再继续尝试；尝试本身重新置 Pending。
func TestUpdateSelfUIPendingAttemptCounted(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	self := filepath.Join(update, "aly-client-ui.exe")
	writeF(t, self, "UI-V1")
	writeF(t, filepath.Join(main, "aly-client-ui.exe"), "UI-V2")
	// 上一次的收尾进程没有收敛（例如等待/改名失败）
	selfupdate.SaveJournal(selfupdate.JournalPath(self), selfupdate.Journal{Pending: true})

	spawned := false
	orig := spawnFinishFn
	spawnFinishFn = func(string, []string) error { spawned = true; return nil }
	defer func() { spawnFinishFn = orig }()

	replaced, err := UpdateSelfUI(update, main, []string{})
	if err != nil {
		t.Fatalf("pending retry must not error: %v", err)
	}
	if !replaced {
		t.Fatal("pending retry should attempt replacement")
	}
	if !spawned {
		t.Fatal("pending retry should spawn delayed script")
	}
	j := readJournal(t, self)
	if j.Count != 1 {
		t.Fatalf("pending non-convergence must count 1, got %+v", j)
	}
	if !j.Pending {
		t.Fatal("new attempt must leave pending=true for the next startup")
	}
}

// TestUpdateSelfUISkipsAfterPendingFailures 连续未收敛达到上限后必须跳过：
// 不复制、不派生、不产生 .new。
func TestUpdateSelfUISkipsAfterPendingFailures(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	self := filepath.Join(update, "aly-client-ui.exe")
	writeF(t, self, "UI-V1")
	writeF(t, filepath.Join(main, "aly-client-ui.exe"), "UI-V2")
	// 已失败 2 次，且上一次替换仍未收敛 → 本次补记后达到上限
	selfupdate.SaveJournal(selfupdate.JournalPath(self), selfupdate.Journal{
		Count: 2, LastTS: time.Now().Unix(), LastError: "move failed", Pending: true,
	})

	spawned := false
	orig := spawnFinishFn
	spawnFinishFn = func(string, []string) error { spawned = true; return nil }
	defer func() { spawnFinishFn = orig }()

	replaced, err := UpdateSelfUI(update, main, []string{})
	if err != nil {
		t.Fatalf("skip path must not error: %v", err)
	}
	if replaced {
		t.Fatal("must skip after max non-converged attempts")
	}
	if spawned {
		t.Fatal("skip path must not spawn")
	}
	if _, err := os.Stat(self + ".new"); !os.IsNotExist(err) {
		t.Fatalf("skip path must not create .new (err=%v)", err)
	}
	j := readJournal(t, self)
	if j.Count != maxSelfUpdateAttempts {
		t.Fatalf("journal count = %d; want %d", j.Count, maxSelfUpdateAttempts)
	}
}

// TestUpdateSelfUIPendingConvergenceClears 重启后内容一致：Pending 与计数一起清零。
func TestUpdateSelfUIPendingConvergenceClears(t *testing.T) {
	dir, cleanup := mkTmp(t)
	defer cleanup()

	update := filepath.Join(dir, "UpdateFolder")
	main := filepath.Join(dir, "win-x64")
	for _, d := range []string{update, main} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	self := filepath.Join(update, "aly-client-ui.exe")
	writeF(t, self, "SAME")
	writeF(t, filepath.Join(main, "aly-client-ui.exe"), "SAME")
	selfupdate.SaveJournal(selfupdate.JournalPath(self), selfupdate.Journal{
		Count: 2, LastTS: time.Now().Unix(), LastError: "move failed", Pending: true,
	})

	orig := spawnFinishFn
	spawnFinishFn = func(string, []string) error {
		t.Fatal("converged path must not spawn")
		return nil
	}
	defer func() { spawnFinishFn = orig }()

	replaced, err := UpdateSelfUI(update, main, []string{})
	if err != nil || replaced {
		t.Fatalf("converged path must be (false,nil), got (%v,%v)", replaced, err)
	}
	j := readJournal(t, self)
	if j.Count != 0 || j.Pending {
		t.Fatalf("convergence must clear count/pending, got %+v", j)
	}
}
