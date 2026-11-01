package tc

// 用例 105 — 更新锁的空窗期与崩溃残留（钉住缺陷）
//
// 背景：`AcquireUpdateLock`（cmd/lock.go:46）为了保证"O_EXCL 占位 → 原子写入内容"
// 之间的空窗不被误判成残留，给**空/坏内容且 mtime 在 1 分钟内**的锁做了宽限
// （lock.go:207-215：`return false // 可能是刚创建的在途锁`）。这个宽限本身是对的
// ——它防的是"另一进程刚建锁还没写内容时被我删掉"的双实例并发。
//
// 但它有一个必然的副作用：**持有者在"建锁成功、内容尚未落盘"这段窗口内被强杀，
// 留下的空锁会被宽限 1 分钟**，期间所有 download/apply/rollback 都被
// `另一更新正在进行中，请稍后重试` 拒绝，且**没有任何自愈路径**：
//   - 宽限期内 cleanStaleLock 返回 false → 直接报错退出；
//   - 1 分钟后才按残留清理。
//
// 空窗有多长？`tryAcquireLock` 里 O_EXCL 之后立刻 `f.Close()` + `writeLockFile`，
// 实测在毫秒级 —— 但崩溃恰好落在这几毫秒内就是 1 分钟的更新不可用。
// 这是**真实产品缺陷的第 5 项**（本用例钉住当前行为；修复方向见文末"结论"）。

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCase105 分三段：
//   - ① 空锁（mtime=now、内容为空）= "O_EXCL 占位后立即被强杀" 的等价现场
//   - ② 空锁超过 1 分钟宽限 = 应被清理并放行
//   - ③ 有内容但持有者已死 = 应被立即清理（不受宽限影响）
func TestCase105(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-lock"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "lib.dll", "lib-100")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
	e.PutFile(t, proj, "lib.dll", "lib-110")

	newReady := func(t *testing.T) (*Client, string) {
		t.Helper()
		c := e.NewClient(t, NewClientOpts{
			Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
			Project: proj, MainFile: "app-100",
		})
		WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-100")
		True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 失败")
		return c, filepath.Join(c.Up(), ".aly.lock")
	}

	// ── ① 空锁 + 刚创建：被 1 分钟宽限挡住（钉住缺陷）
	t.Run("①空锁落在宽限期内", func(t *testing.T) {
		c, lock := newReady(t)
		WriteText(t, lock, "") // 空内容 = 占位已建、内容未落盘
		Eq(t, "前置：空锁文件存在", Exists(lock), true)

		out := e.RunLast(t, c, "apply_update")
		Eq(t, "已知缺陷：空锁（持有者已死）仍被当作在途锁拒绝", out.IsSuccess, false)
		Contains(t, out.ErrorMsg, "另一更新正在进行中", "errorMsg")
		// 注意：download 完成时 version 已是 1.1.0（status=downloaded），
		// 所以"被挡住"要看状态是否停留在 downloaded（apply 没生效）。
		v := c.ReadVersion(t)
		Eq(t, "状态停在 downloaded（apply 完全没生效）", v.VersionStatus, StatusDownloaded)
		Eq(t, "主目录仍是 1.0.0 内容（没被替换）",
			strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-100")
		Eq(t, "锁文件仍残留（拒绝路径不清理，无自愈）", Exists(lock), true)

		// 同一时刻的重试也一样被挡（无退避、无重试次数上限之外的自愈）
		again := e.RunLast(t, c, "apply_update")
		Eq(t, "已知缺陷：立即重试仍被挡（无自愈窗口）", again.IsSuccess, false)

		// 收拾现场，避免影响后续子测试
		_ = os.Remove(lock)
	})

	// ── ② 空锁超过宽限期：应被清理并放行
	t.Run("②空锁超过宽限期", func(t *testing.T) {
		c, lock := newReady(t)
		WriteText(t, lock, "")
		// 把 mtime 拨回 2 分钟前（模拟"崩溃已过去一段时间"）
		old := time.Now().Add(-2 * time.Minute)
		if err := os.Chtimes(lock, old, old); err != nil {
			t.Fatalf("调整 mtime 失败: %v", err)
		}

		out := e.RunLast(t, c, "apply_update")
		True(t, out.IsSuccess, "超过宽限期的空锁应被清理后放行: %s\n%s",
			out.ErrorMsg, c.Log(t, "update.log", 60))
		Eq(t, "版本已推进", c.ReadVersion(t).Version, "1.1.0")
		Eq(t, "锁已被释放", Exists(lock), false)
	})

	// ── ③ 有内容但持有者已死：应被立即清理（不走宽限）
	t.Run("③持有者已死的有效锁", func(t *testing.T) {
		c, lock := newReady(t)

		// 造一个"确定已退出"的 PID：起一个立即结束的进程，等它退出后取其 PID。
		// 不能用 pid=1（Windows 上 PROCESS_QUERY_INFORMATION 失败会保守判"存活"）。
		var deadPID int
		if cmd := exec.Command("cmd", "/c", "exit", "0"); cmd != nil {
			if err := cmd.Start(); err == nil {
				deadPID = cmd.Process.Pid
				_, _ = cmd.Process.Wait() // 等它真的退出，PID 才确定失效
			}
		}
		True(t, deadPID > 0, "前置：需拿到一个已退出的 PID")

		WriteText(t, lock, fmt.Sprintf(
			`{"pid":%d,"ts":"%d","cmd":"apply_update","token":"dead-token"}`,
			deadPID, time.Now().Unix()))

		out := e.RunLast(t, c, "apply_update")
		True(t, out.IsSuccess, "持有者已死的锁应立即清理并放行: %s\n%s",
			out.ErrorMsg, c.Log(t, "update.log", 60))
		Eq(t, "版本已推进", c.ReadVersion(t).Version, "1.1.0")
		Eq(t, "锁已被释放", Exists(lock), false)
	})
}