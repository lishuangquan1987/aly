package tc

// 用例 18–25：中断与故障恢复
//
// 核心判据：**不留半成品状态**、**重启后自动收敛**。
// 故障注入手法（杀进程 / 停服务端 / 写坏状态）见文档附录 A。

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// waitStatus 轮询等待 version.json 的 version_status 变成 want。
func waitStatus(t *testing.T, c *Client, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c.ReadVersion(t).VersionStatus == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("等待 version_status=%s 超时(%s)，当前=%s",
		want, timeout, c.ReadVersion(t).VersionStatus)
}

// ────────────────────────────────────────────────────────────────
// 用例 18 — download 期间强杀进程：不留半成品状态
// ────────────────────────────────────────────────────────────────
func TestCase18(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-kill"
	e.EnsureProject(t, proj, false, nil, nil)
	BigFile(t, filepath.Join(e.DataDir(proj), "big.bin"), 50)
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Project: proj, NoMain: true,
	})

	cmd, _ := e.RunAsync(c, "download_update")
	snap := c.VerDir("1.1.0")
	MustWaitFile(t, filepath.Join(snap, "big.bin.part"), 60*time.Second)
	Kill(t, cmd) // 模拟任务管理器结束进程

	v := c.ReadVersion(t)
	Eq(t, "强杀后状态不变", v.VersionStatus, StatusApplied)
	Eq(t, "不应有正式文件", Exists(filepath.Join(snap, "big.bin")), false)

	again := e.RunLast(t, c, "download_update")
	True(t, again.IsSuccess, "重跑应成功: %s", again.ErrorMsg)
	Eq(t, "最终大小", SizeOf(filepath.Join(snap, "big.bin")), int64(50<<20))
	Eq(t, "状态", c.ReadVersion(t).VersionStatus, StatusDownloaded)
}

// ────────────────────────────────────────────────────────────────
// 用例 19 — download 残留半截 .part（模拟断电）
//
// 判据：脏 .part 要么被校验兜住整段重下，要么续传后校验失败自动重试；
// 绝不允许哈希不符却报成功。
// ────────────────────────────────────────────────────────────────
func TestCase19(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-power"
	e.EnsureProject(t, proj, false, nil, nil)
	bigSrc := filepath.Join(e.DataDir(proj), "big.bin")
	BigFile(t, bigSrc, 20)
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Project: proj, NoMain: true,
	})

	part := filepath.Join(c.VerDir("1.1.0"), "big.bin.part")
	WriteText(t, filepath.Join(c.VerDir("1.1.0"), ".keep"), "")
	// 8MB 的 0xFF 垃圾数据
	junk := bytes.Repeat([]byte{0xFF}, 1<<20)
	f, err := os.Create(part)
	True(t, err == nil, "建 .part 失败: %v", err)
	for i := 0; i < 8; i++ {
		_, _ = f.Write(junk)
	}
	f.Close()
	Eq(t, "前置：.part 应为 8MB", SizeOf(part), int64(8<<20))

	dl := e.RunLast(t, c, "download_update")
	final := filepath.Join(c.VerDir("1.1.0"), "big.bin")
	if dl.IsSuccess {
		Eq(t, "内容必须与源一致（校验必须兜住脏 .part）",
			MD5Of(final), MD5Of(bigSrc))
		Eq(t, "状态", c.ReadVersion(t).VersionStatus, StatusDownloaded)
	} else {
		// 失败也可接受，但必须明确报错且不写 downloaded 状态
		Eq(t, "失败时状态不能是 downloaded",
			c.ReadVersion(t).VersionStatus, StatusApplied)
		t.Logf("下载失败（可接受，脏 .part 被校验拦截）: %s", dl.ErrorMsg)
	}
}

// ────────────────────────────────────────────────────────────────
// 用例 20 — apply 复制阶段强杀：状态为 applying 且可恢复
// ────────────────────────────────────────────────────────────────
func TestCase20(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-apply-kill"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "lib.dll", "lib-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-100")

	e.PutFile(t, proj, "lib.dll", "lib-110")
	e.PutFile(t, proj, "new.txt", "new-in-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.1.0")

	cmd, _ := e.RunAsync(c, "apply_update")
	// 同步点：状态写 applying 即说明进入替换流程
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if c.ReadVersion(t).VersionStatus == StatusApplying {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	Kill(t, cmd)

	// apply 极快时可能已经跑完，此时属"杀在完工后"，仍应能从 applied 收敛
	st := c.ReadVersion(t).VersionStatus
	True(t, st == StatusApplying || st == StatusApplied,
		"强杀后状态应为 applying 或 applied（内容必须完整），实际 %q", st)

	after := e.RunLast(t, c, "apply_update")
	True(t, after.IsSuccess, "崩溃恢复失败: %s\n%s", after.ErrorMsg,
		c.Log(t, "update.log", 120))

	v := c.ReadVersion(t)
	Eq(t, "最终版本", v.Version, "1.1.0")
	Eq(t, "最终状态", v.VersionStatus, StatusApplied)
	Eq(t, "主目录为新内容",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-110")
	Eq(t, "备份目录为旧内容",
		strings.TrimSpace(ReadText(filepath.Join(c.VerDir("1.0.0"), "lib.dll"))), "lib-100")
	Eq(t, "备份目录不应含新版本才有的 new.txt",
		Exists(filepath.Join(c.VerDir("1.0.0"), "new.txt")), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 21 — apply 各崩溃点（6 个点位矩阵）
//
// ⚠ 方法学修正（实测 2026-11-01，与文档原写法不同）：
// 原设计是"异步启动 apply → 在某个同步点强杀"，其中点位 3/4/5 的同步条件是
// `WaitGone(t, c.Main(), ...)`（等主目录消失）。这个条件**永远抓不到**：
//   applyReplacement（apply_update.go:301-372）是三段 rename ——
//   旁移旧备份 → 主目录改名成备份 → 版本目录改名成主目录。
//   两次 rename 之间的"主目录不存在"窗口是**微秒级**（同分区 rename 是元数据操作），
//   而 WaitGone 是 50ms 轮询 → 必然落到"新版本已就位"之后。
// 于是"杀点"变成了"apply 已完工后杀"，重跑自然被 `no pending update to apply`
// 拒绝，用例 FAIL——但这不是产品缺陷，是**用例不可观测**。
//
// 修正后的做法：**不靠时序赌窗口，直接构造崩溃后的磁盘现场**。
// 崩溃恢复的正确性只取决于 (version.json 状态) × (磁盘目录组合)，这两者都能精确写入，
// 因此每个点位都能造出 100% 确定、可重复的现场，判据也从"杀得准"变成
// "给定现场，恢复逻辑是否收敛"——这才是用例真正要验证的东西。
//
// 每个点位都从同一干净起点（download 完 1.1.0、未 apply）开始，各自独立客户端。
// ────────────────────────────────────────────────────────────────

// applySnapshot 是一次 apply 之前的"干净起点"现场信息。
type applySnapshot struct {
	pkg    string // 包目录
	mainBk string // 主目录内容（1.0.0 的 lib.dll）
	upd    string // 待应用内容（1.1.0 的 lib.dll）
	vdir   string // 版本快照目录
}

// buildApplyPoint 造一个"apply 崩溃在点位 N"的磁盘现场。
//
// 所有点位共用同一条时间线（对应 applyReplacement 的执行顺序）：
//
//	① version.json: downloaded → applying          （apply_update.go:229）
//	② prevDir(1.0.0) → prevDir.old（若存在）
//	③ MainFolder    → prevDir(1.0.0)   备份改名
//	④ version.json: apply_stage = activating       （apply_update.go:350）
//	⑤ versionDir(1.1.0) → MainFolder   激活改名
//	⑥ version.json: applied + 清 transient
//
// 点位即"在第几步之后崩溃"，落盘状态照抄该步之后的真实磁盘/状态组合。
func (e *Env) buildApplyPoint(t *testing.T, point string, proj string) *Client {
	t.Helper()
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-100")
	// 起点：真实下载完 1.1.0（版本目录里是 1.1.0 的完整内容 + 下载时复制的 shared.json）
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 失败")

	v11 := c.VerDir("1.1.0")
	True(t, Exists(filepath.Join(v11, "lib.dll")), "前置：1.1.0 快照应含 lib.dll")
	oldLib := strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib.dll")))
	newLib := strings.TrimSpace(ReadText(filepath.Join(v11, "lib.dll")))
	Eq(t, "前置：主目录是 1.0.0 内容", oldLib, "lib-100")
	Eq(t, "前置：快照是 1.1.0 内容", newLib, "lib-110")

	vi := c.ReadVersion(t)
	Eq(t, "前置：下载完成后状态", vi.VersionStatus, StatusDownloaded)

	switch point {
	case "点位1-写状态前":
		// ① 之前崩溃：状态仍是 downloaded，磁盘没有被动过。
		// 重跑的语义就是"正常 apply 一次"（不是恢复）。
	case "点位2-写状态后":
		// ① 之后、②③ 之前崩溃：状态 applying，主目录与快照原样。
		vi.VersionStatus = StatusApplying
		c.WriteVersion(t, vi)
	case "点位3-备份改名后":
		// ③ 之后、④⑤ 之前崩溃：主目录已被改名成 1.0.0 备份 → MainFolder 不存在，
		// 快照 1.1.0 仍在。这是 apply_update 的 "MainFolder 缺失 + versionDir 存在"
		// 分支（apply_update.go:130-155）唯一能覆盖的窗口。
		vi.VersionStatus = StatusApplying
		c.WriteVersion(t, vi)
		renameDir(t, c.Main(), c.VerDir("1.0.0"))
	case "点位4-新版本改名之前":
		// ④ 之后、⑤ 之前崩溃：状态 applying + apply_stage=activating，
		// MainFolder 不存在、备份 1.0.0 与快照 1.1.0 都在。
		// 与点位3 的区别是"已有激活意图的证据"，恢复必须**继续激活**而不是回退。
		vi.VersionStatus = StatusApplying
		vi.ApplyStage = "activating"
		c.WriteVersion(t, vi)
		renameDir(t, c.Main(), c.VerDir("1.0.0"))
	case "点位5-新版本改名后":
		// ⑤ 之后、⑥ 之前崩溃：快照已被消耗（改名成 MainFolder），
		// 状态仍是 applying + activating，备份 1.0.0 在。
		// 恢复判据：versionDir 缺失 + 主目录在 → 补 applied，且**不得重做替换**
		// （重做会把备份 1.0.0 当垃圾删掉，回滚永久失效）。
		vi.VersionStatus = StatusApplying
		vi.ApplyStage = "activating"
		c.WriteVersion(t, vi)
		renameDir(t, c.Main(), c.VerDir("1.0.0"))
		renameDir(t, v11, c.Main())
	case "点位6-写applied之后":
		// ⑥ 已完成：applied + 1.1.0 内容，快照已消耗，备份在。
		vi.VersionStatus = StatusApplied
		vi.Version = "1.1.0"
		c.WriteVersion(t, vi)
		renameDir(t, c.Main(), c.VerDir("1.0.0"))
		renameDir(t, v11, c.Main())
	default:
		t.Fatalf("未知点位 %q", point)
	}
	return c
}

// renameDir 把目录 from 整个改名为 to（目标必须不存在）。
func renameDir(t *testing.T, from, to string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatalf("建父目录失败: %v", err)
	}
	if err := os.Rename(from, to); err != nil {
		t.Fatalf("改名前提出错(%s → %s): %v", from, to, err)
	}
}

func TestCase21(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-apply-matrix"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "lib.dll", "lib-100")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
	e.PutFile(t, proj, "lib.dll", "lib-110")
	e.PutFile(t, proj, "new.txt", "new-in-110")

	for _, point := range []string{
		"点位1-写状态前",
		"点位2-写状态后",
		"点位3-备份改名后",
		"点位4-新版本改名之前",
		"点位5-新版本改名后",
		"点位6-写applied之后",
	} {
		point := point
		t.Run(point, func(t *testing.T) {
			c := e.buildApplyPoint(t, point, proj)

			// 现场已精确构造，恢复入口只需跑一次 apply_update（幂等收敛）。
			out := e.RunLast(t, c, "apply_update")

			if point == "点位6-写applied之后" {
				// 已是终态，"恢复"没有语义：必须**明确拒绝**而不是挂死/假成功。
				Eq(t, point+" 完工后重跑应明确拒绝", out.IsSuccess, false)
				Contains(t, out.ErrorMsg, "no pending update", point+" errorMsg")
			} else {
				True(t, out.IsSuccess, "%s 恢复失败: %s\n%s", point, out.ErrorMsg,
					c.Log(t, "update.log", 120))
			}

			v := c.ReadVersion(t)
			Eq(t, point+" 最终版本", v.Version, "1.1.0")
			Eq(t, point+" 最终状态", v.VersionStatus, StatusApplied)
			Eq(t, point+" transient 字段应清空（apply_stage/rollback_*）", v.ApplyStage, "")
			Eq(t, point+" rollback_previous 应清空", v.RollbackPrevious, "")
			Eq(t, point+" 主目录新版内容",
				strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-110")
			Eq(t, point+" 主目录含 new.txt", Exists(filepath.Join(c.Main(), "new.txt")), true)
			Eq(t, point+" 备份旧版内容",
				strings.TrimSpace(ReadText(filepath.Join(c.VerDir("1.0.0"), "lib.dll"))), "lib-100")
			Eq(t, point+" 备份不含 new.txt",
				Exists(filepath.Join(c.VerDir("1.0.0"), "new.txt")), false)
			// 无 .old 旁移残留、无空版本名目录
			Eq(t, point+" 无 .old 残留",
				ListDirMatch(c.Pkg, "ApplicationFolder_*.old"), 0)
			Eq(t, point+" 无空版本名目录",
				Exists(filepath.Join(c.Pkg, "ApplicationFolder_")), false)
		})
	}
}

// ────────────────────────────────────────────────────────────────
// 用例 22 — applying 期间误触 download：跳过且状态不降级
// ────────────────────────────────────────────────────────────────
func TestCase22(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-applying-dl"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusApplying, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	e.PutFile(t, proj, "app.exe", "app-120")
	True(t, e.Publish(t, proj, "1.2.0", nil, "", "").IsSuccess, "发布 1.2.0")

	out := e.RunLast(t, c, "download_update")

	Eq(t, "isSuccess", out.IsSuccess, true)
	Eq(t, "完成行 data:null", string(out.Data) == "null" || len(out.Data) == 0, true)

	v := c.ReadVersion(t)
	Eq(t, "状态不得降级", v.VersionStatus, StatusApplying)
	Eq(t, "version 不变", v.Version, "1.1.0")
	Eq(t, "version_previous 不变", v.VersionPrevious, "1.0.0")
	Contains(t, c.Log(t, "download.log", 200), "applying 进行中，跳过 download", "download.log")
	Eq(t, "不应下 1.2.0", Exists(c.VerDir("1.2.0")), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 23 — rollback 崩溃现场：resumeRollback 四分支接管恢复
//
// ⚠ 方法学修正（实测 2026-11-01，同用例 21）：原设计"异步 rollback → 等主目录
// 消失 → 强杀"永远抓不到窗口——rollback 与 apply 是同一套三段 rename，
// 两次 rename 之间主目录不存在的窗口是微秒级，50ms 轮询必然错过，
// 结果杀在完工之后，续跑被 `no pending update to apply` 拒绝。
//
// 改为**直接构造 resumeRollback 的四个分支现场**（rollback.go:334-406），
// 每个分支都是 (version.json) × (MainFolder / targetDir / prevDir 是否存在) 的组合，
// 可精确写入，恢复逻辑是否走对分支从**结果**上就能判定。
// ────────────────────────────────────────────────────────────────

// rollbackSnapshot 是构造一次 rollback 崩溃现场所需的项目名。
type rollbackSnapshot struct {
	proj string
}

// buildRollbackPoint 造一个"rollback 崩在分支 N"的现场。
//
// rollback 的时间线（rollback.go:176-266）：
//
//	① version.json: applied → applying + rollback_previous/rollback_target
//	② prevDir(1.1.0 备份) → prevDir.old（若存在）
//	③ MainFolder → prevDir(1.1.0)          备份改名（主目录消失）
//	④ targetDir(1.0.0) → MainFolder        激活改名
//
// resumeRollback 按现场分四支：
//
//	分支1  MainFolder 在 + targetDir 不在 → 激活已完成，补 applied
//	分支2  MainFolder 在 + targetDir 在   → 备份改名前的崩溃，重做替换
//	分支3  MainFolder 不在 + targetDir 在 → 备份改名后、激活前，激活目标
//	分支4  MainFolder 不在 + targetDir 不在 + prevDir 在 → 目标已消耗，放弃回滚
func (e *Env) buildRollbackPoint(t *testing.T, branch string, s rollbackSnapshot) *Client {
	t.Helper()
	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusApplied, Previous: "1.1.0",
		Project: s.proj, MainFile: "app-110",
	})
	WriteText(t, filepath.Join(c.Main(), "new.txt"), "new-in-110")
	// 1.0.0 备份（含 .updator/shared.json，真实发布包结构 —— 主目录缺失时
	// loadFullConfig 会回退到版本目录读它）
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), "app.exe"), "app-100")
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), ".updator", "shared.json"),
		ReadText(filepath.Join(c.Main(), ".updator", "shared.json")))

	// 回滚现场状态：applying + 两个回滚字段（续跑的唯一依据）
	vi := c.ReadVersion(t)
	vi.VersionStatus = StatusApplying
	vi.RollbackPrevious = "1.1.0"
	vi.RollbackTarget = "1.0.0"
	c.WriteVersion(t, vi)

	switch branch {
	case "分支1-激活已完成":
		// ④ 已执行：targetDir 被消耗成 MainFolder（1.0.0 内容），prevDir 是 1.1.0 备份。
		renameDir(t, c.Main(), c.VerDir("1.1.0"))
		renameDir(t, c.VerDir("1.0.0"), c.Main())
	case "分支2-备份改名之前":
		// ① 之后、③ 之前：MainFolder 仍是 1.1.0、targetDir 也在。
		// 恢复必须重做"备份 1.1.0 → 激活 1.0.0"。
	case "分支3-备份改名后":
		// ③ 之后、④ 之前：MainFolder 不存在，prevDir(1.1.0) 与 targetDir(1.0.0) 都在。
		renameDir(t, c.Main(), c.VerDir("1.1.0"))
	case "分支4-目标已消耗":
		// ④ 已执行但 targetDir 也已不在（被别的操作吃掉）：MainFolder 不在、
		// targetDir 不在、prevDir(1.1.0) 在 → 只能安全放弃回滚、归位到回滚前版本。
		renameDir(t, c.Main(), c.VerDir("1.1.0"))
		if err := os.RemoveAll(c.VerDir("1.0.0")); err != nil {
			t.Fatalf("删除 target 目录失败: %v", err)
		}
	default:
		t.Fatalf("未知分支 %q", branch)
	}

	// 现场自检：确认构造出的组合与分支定义严格一致（否则"PASS"可能只是
	// 因为退化成另一个分支，用例就失去了验证力）。
	mainExists := Exists(c.Main())
	targetExists := Exists(c.VerDir("1.0.0"))
	prevExists := Exists(c.VerDir("1.1.0"))
	switch branch {
	case "分支1-激活已完成":
		Eq(t, "现场自检 MainFolder", mainExists, true)
		Eq(t, "现场自检 targetDir 已消耗", targetExists, false)
	case "分支2-备份改名之前":
		Eq(t, "现场自检 MainFolder", mainExists, true)
		Eq(t, "现场自检 targetDir 仍在", targetExists, true)
	case "分支3-备份改名后":
		Eq(t, "现场自检 MainFolder", mainExists, false)
		Eq(t, "现场自检 targetDir 仍在", targetExists, true)
	case "分支4-目标已消耗":
		Eq(t, "现场自检 MainFolder", mainExists, false)
		Eq(t, "现场自检 targetDir 已消耗", targetExists, false)
		Eq(t, "现场自检 回滚前版本备份仍在", prevExists, true)
	}
	return c
}

func TestCase23(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-roll-kill"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "new.txt", "new-in-110")

	for _, branch := range []string{
		"分支1-激活已完成",
		"分支2-备份改名之前",
		"分支3-备份改名后",
		"分支4-目标已消耗",
	} {
		branch := branch
		t.Run(branch, func(t *testing.T) {
			c := e.buildRollbackPoint(t, branch, rollbackSnapshot{proj: proj})

			// 现场已精确构造：续跑入口就是 rollback --version 1.0.0
			// （apply_update 也是等价入口，二者都委托 resumeRollback）。
			done := e.RunLast(t, c, "rollback", "--version", "1.0.0")
			True(t, done.IsSuccess, "%s 续跑失败: %s\n%s", branch, done.ErrorMsg,
				c.Log(t, "update.log", 120))

			v := c.ReadVersion(t)
			// 分支 4 是"安全放弃回滚"：版本归位到回滚前的 1.1.0，不是 1.0.0。
			wantVer := "1.0.0"
			if branch == "分支4-目标已消耗" {
				wantVer = "1.1.0"
			}
			Eq(t, branch+" 最终版本", v.Version, wantVer)
			Eq(t, branch+" 最终状态", v.VersionStatus, StatusApplied)
			Eq(t, branch+" rollback_previous 已清空", v.RollbackPrevious, "")
			Eq(t, branch+" rollback_target 已清空", v.RollbackTarget, "")
			Eq(t, branch+" apply_stage 已清空", v.ApplyStage, "")

			// 主目录内容必须与写入的版本号一致（不允许"目录名 V 装 W 内容"）
			wantApp := "app-100"
			if wantVer == "1.1.0" {
				wantApp = "app-110"
			}
			Eq(t, branch+" 主目录内容与版本号一致",
				strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), wantApp)

			if wantVer == "1.0.0" {
				// 回滚成功：主目录是旧版干净内容，1.1.0 成为备份
				Eq(t, branch+" 主目录不应含 new.txt",
					Exists(filepath.Join(c.Main(), "new.txt")), false)
				Eq(t, branch+" 1.1.0 成为备份", Exists(c.VerDir("1.1.0")), true)
				Eq(t, branch+" 备份是 1.1.0 内容",
					strings.TrimSpace(ReadText(filepath.Join(c.VerDir("1.1.0"), "app.exe"))), "app-110")
			}
			// 任何分支都不允许留下 .old 旁移残留
			Eq(t, branch+" 无 .old 残留",
				ListDirMatch(c.Pkg, "ApplicationFolder_*.old"), 0)
		})
	}
}

// ────────────────────────────────────────────────────────────────
// 用例 24 — version.json 写成半截（模拟断电）
// ────────────────────────────────────────────────────────────────
func TestCase24(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-corrupt"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	vp := filepath.Join(c.Up(), "version.json")
	full := ReadText(vp)
	True(t, len(full) > 20, "前置：version.json 应足够长")
	WriteText(t, vp, full[:20])                       // 半截
	WriteText(t, filepath.Join(c.Up(), "version.json.bak"), full) // 备份（若实现使用）

	out := e.RunLast(t, c, "check_update")

	// 不允许 panic/卡死：能走到这里说明进程正常退出
	if out.IsSuccess {
		// A：.bak 自愈 → 正常无更新
		Eq(t, "自愈后版本正确", c.ReadVersion(t).Version, "1.0.0")
		t.Logf("走 .bak 自愈路径")
	} else {
		// B：隔离重建 → 明确报错
		Contains(t, out.ErrorMsg, "read version", "errorMsg")
		True(t, Exists(vp), "应重建 version.json")
		t.Logf("走隔离重建路径，errorMsg=%s", out.ErrorMsg)
	}

	// 无论哪条路径，version.json 都不能还是那 20 字节的半截
	True(t, len(ReadText(vp)) != 20, "半截内容必须被处理掉")
}

// ────────────────────────────────────────────────────────────────
// 用例 25 — download 期间服务端被杀：失败可感知且可重试
// ────────────────────────────────────────────────────────────────
func TestCase25(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-dead"
	e.EnsureProject(t, proj, false, nil, nil)
	bigSrc := filepath.Join(e.DataDir(proj), "big.bin")
	BigFile(t, bigSrc, 50)
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Project: proj, NoMain: true,
	})

	port := e.Port // 必须先存：重启必须复用同一端口（shared.json 已固化 URL）
	cmd, buf := e.RunAsync(c, "download_update")
	MustWaitFile(t, filepath.Join(c.VerDir("1.1.0"), "big.bin.part"), 60*time.Second)

	e.StopServer() // 服务端断电
	time.Sleep(2 * time.Second)
	Kill(t, cmd)

	// 失败必须可感知：状态不能是 downloaded
	Eq(t, "下载失败必须可感知", c.ReadVersion(t).VersionStatus, StatusApplied)
	// 失败信息应明确（客户端可能已写出 isSuccess:false 行）
	if s := buf.String(); s != "" {
		t.Logf("被杀进程输出片段: %s", truncate(s, 600))
	}

	// 恢复：必须复用同一端口
	e.StartServer(t, port)
	again := e.RunLast(t, c, "download_update")
	True(t, again.IsSuccess, "恢复后重跑应成功: %s", again.ErrorMsg)
	Eq(t, "最终大小",
		SizeOf(filepath.Join(c.VerDir("1.1.0"), "big.bin")), int64(50<<20))
	Eq(t, "最终哈希与源一致", MD5Of(filepath.Join(c.VerDir("1.1.0"), "big.bin")), MD5Of(bigSrc))
}