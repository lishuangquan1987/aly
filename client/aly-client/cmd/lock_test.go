package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"aly/client/aly-client/config"
)

// setupLockTemp 创建临时 UpdateFolder 并设置 ExeDir 覆盖
func setupLockTemp(t *testing.T) string {
	t.Helper()
	root, err := ioutil.TempDir("", "lock-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })
	return update
}

// TestAcquireUpdateLock 验证：首次获取成功、锁被持有（本进程存活）时获取失败、
// 释放后可再次获取、释放后锁文件被删除。
func TestAcquireUpdateLock(t *testing.T) {
	update := setupLockTemp(t)

	release1, err := AcquireUpdateLock("apply_update")
	if err != nil {
		t.Fatalf("首次获取锁应成功: %v", err)
	}
	if _, err := os.Stat(filepath.Join(update, ".aly.lock")); err != nil {
		t.Fatalf("获取锁后应存在锁文件: %v", err)
	}

	// 第二次获取（持有者=本测试进程，存活）→ 应失败
	if _, err := AcquireUpdateLock("apply_update"); err == nil {
		t.Fatal("锁被存活进程持有时应获取失败")
	}

	// 释放后可再获取
	release1()
	if _, err := os.Stat(filepath.Join(update, ".aly.lock")); !os.IsNotExist(err) {
		t.Errorf("释放后锁文件应被删除")
	}
	release2, err := AcquireUpdateLock("download_update")
	if err != nil {
		t.Fatalf("释放后应可再次获取: %v", err)
	}
	release2()
}

// TestCleanStaleLockInFlightEmpty 验证 #9 写入竞态修复：
// 空内容锁（O_EXCL 占位刚建立、原子 rename 尚未发生）且 mtime 在 1 分钟内
// → 不当残留删除（旧实现会删掉刚创建的新锁导致双实例并发）。
func TestCleanStaleLockInFlightEmpty(t *testing.T) {
	update := setupLockTemp(t)
	lockPath := filepath.Join(update, ".aly.lock") // updateLockPath() 即此文件
	if err := ioutil.WriteFile(lockPath, []byte(""), 0644); err != nil {
		t.Fatalf("写空锁失败: %v", err)
	}

	if cleanStaleLock(lockPath) {
		t.Fatal("1 分钟内的空锁应视为在途写入，不应清理")
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Errorf("在途空锁不应被删除: %v", err)
	}
}

// TestCleanStaleLockStaleEmpty 验证：空/坏内容锁且超过 1 分钟 → 按残留清理。
func TestCleanStaleLockStaleEmpty(t *testing.T) {
	update := setupLockTemp(t)
	lockPath := filepath.Join(update, ".aly.lock")
	if err := ioutil.WriteFile(lockPath, []byte("not-json"), 0644); err != nil {
		t.Fatalf("写坏锁失败: %v", err)
	}
	// mtime 回拨到 5 分钟前
	old := time.Now().Add(-5 * time.Minute)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("回拨 mtime 失败: %v", err)
	}

	if !cleanStaleLock(lockPath) {
		t.Fatal("超过 1 分钟的坏锁应视为残留清理")
	}
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Errorf("残留坏锁应被删除")
	}
}

// TestAcquireUpdateLockStale 验证崩溃残留锁（持有者 PID 已不存在）自动清理后可获取。
func TestAcquireUpdateLockStale(t *testing.T) {
	update := setupLockTemp(t)
	stale := `{"pid":99999999,"ts":"2020-01-01 00:00:00","cmd":"apply_update"}`
	if err := ioutil.WriteFile(filepath.Join(update, ".aly.lock"), []byte(stale), 0644); err != nil {
		t.Fatalf("写残留锁失败: %v", err)
	}

	release, err := AcquireUpdateLock("apply_update")
	if err != nil {
		t.Fatalf("残留锁应被清理并获取成功: %v", err)
	}
	release()
}

// TestAcquireUpdateLockHeldByAlive 验证持有者为存活进程时获取失败（不抢占）。
func TestAcquireUpdateLockHeldByAlive(t *testing.T) {
	setupLockTemp(t)
	// ts 用当前时间（旧格式），持有者 = 本测试进程（存活）→ 不应抢占
	alive := fmt.Sprintf(`{"pid":%d,"ts":%q,"cmd":"apply_update"}`,
		os.Getpid(), time.Now().Format("2006-01-02 15:04:05"))
	if err := ioutil.WriteFile(filepath.Join(updateLockPath()), []byte(alive), 0644); err != nil {
		t.Fatalf("写锁失败: %v", err)
	}

	if _, err := AcquireUpdateLock("apply_update"); err == nil {
		t.Fatal("持有者为存活进程时应获取失败")
	}
}

// TestAcquireUpdateLockTTLAliveKept 验证 #9 新语义：锁持有超过 lockTTL 但
// 持有者 PID 仍存活（正在工作/卡死未退出）→ 不抢占（touch 机制下存活持有者的
// TS 会持续刷新；真正超 TTL 无 touch 且存活的属于锁泄漏，用 clean 手工清理）。
func TestAcquireUpdateLockTTLAliveKept(t *testing.T) {
	setupLockTemp(t)
	staleTS := time.Now().Add(-lockTTL - time.Minute).Unix()
	alive := fmt.Sprintf(`{"pid":%d,"ts":%q,"cmd":"download_update","token":"t1"}`,
		os.Getpid(), strconv.FormatInt(staleTS, 10))
	if err := ioutil.WriteFile(filepath.Join(updateLockPath()), []byte(alive), 0644); err != nil {
		t.Fatalf("写锁失败: %v", err)
	}

	if _, err := AcquireUpdateLock("download_update"); err == nil {
		t.Fatal("超 TTL 但持有者存活时不应抢占（#9：存活不抢占）")
	}
}

// TestAcquireUpdateLockTTLTakeover 验证 #9 新语义：超 TTL 且持有者进程已不存在
// （崩溃 + PID 未复用）→ 抢占成功。
func TestAcquireUpdateLockTTLTakeover(t *testing.T) {
	setupLockTemp(t)
	staleTS := time.Now().Add(-lockTTL - time.Minute).Unix()
	dead := fmt.Sprintf(`{"pid":99999999,"ts":%q,"cmd":"download_update","token":"t1"}`,
		strconv.FormatInt(staleTS, 10))
	if err := ioutil.WriteFile(filepath.Join(updateLockPath()), []byte(dead), 0644); err != nil {
		t.Fatalf("写锁失败: %v", err)
	}

	release, err := AcquireUpdateLock("download_update")
	if err != nil {
		t.Fatalf("超 TTL 且持有者已死时应被抢占: %v", err)
	}
	release()
}

// TestReleaseLockTokenMismatch 验证释放时 token 不匹配不删除他人锁：
// 获取锁后由另一持有者（改写 token）接管，原释放函数不得删除新锁。
func TestReleaseLockTokenMismatch(t *testing.T) {
	update := setupLockTemp(t)
	release, err := AcquireUpdateLock("apply_update")
	if err != nil {
		t.Fatalf("首次获取锁应成功: %v", err)
	}
	// 模拟被他人接管：改写锁文件 token
	replaced := `{"pid":123,"ts":"1700000000","cmd":"rollback","token":"other-token"}`
	if err := ioutil.WriteFile(filepath.Join(update, ".aly.lock"), []byte(replaced), 0644); err != nil {
		t.Fatalf("改写锁失败: %v", err)
	}
	release() // token 不匹配 → 不应删除
	if _, err := os.Stat(filepath.Join(update, ".aly.lock")); err != nil {
		t.Errorf("token 不匹配时不应删除新持有者的锁")
	}
}
