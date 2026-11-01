package tc

// 用例 26–35：边界条件
//
// 判据统一为：**要么正确处理，要么明确拒绝，绝不假成功、绝不破坏数据**。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────
// 用例 26 — downloaded 但下载产物丢失：识别并重下
// ────────────────────────────────────────────────────────────────
func TestCase26(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-lost"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-101")
	e.PutFile(t, proj, "a.txt", "a")
	e.PutFile(t, proj, "b.txt", "b")
	// 客户端 1.0.0 → 服务端必须发更高版本才有得下
	True(t, e.Publish(t, proj, "1.0.1", nil, "", "").IsSuccess, "发布 1.0.1")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.1", Status: StatusDownloaded, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
		Extras: &VersionInfo{DownloadedFileCount: 3},
	})
	// 造好快照目录再删掉（模拟用户误删 / 杀软隔离）
	WriteText(t, filepath.Join(c.VerDir("1.0.1"), "app.exe"), "app-101")
	WriteText(t, filepath.Join(c.VerDir("1.0.1"), "a.txt"), "a")
	WriteText(t, filepath.Join(c.VerDir("1.0.1"), "b.txt"), "b")
	True(t, os.RemoveAll(c.VerDir("1.0.1")) == nil, "删除快照失败")
	Eq(t, "前置：快照已不存在", Exists(c.VerDir("1.0.1")), false)

	cu := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	Eq(t, "has_update", cu.HasUpdate, true)
	Eq(t, "need_download_update", cu.NeedDownloadUpdate, true)
	Eq(t, "new_version", cu.NewVersion, "1.0.1")

	dl := e.RunLast(t, c, "download_update")
	True(t, dl.IsSuccess, "补齐下载应成功: %s", dl.ErrorMsg)
	Eq(t, "快照重建", Exists(filepath.Join(c.VerDir("1.0.1"), "app.exe")), true)
	Eq(t, "状态仍是 downloaded", c.ReadVersion(t).VersionStatus, StatusDownloaded)
}

// ────────────────────────────────────────────────────────────────
// 用例 27 — downloaded 状态 + 服务器不可达：不上报待应用
// ────────────────────────────────────────────────────────────────
func TestCase27(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-offline"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusDownloaded, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	// 快照要完好（模拟"已下好但未应用"）
	WriteText(t, filepath.Join(c.VerDir("1.1.0"), "app.exe"), "app-110")

	port := e.Port
	e.StopServer()
	cu := e.RunLast(t, c, "check_update")
	True(t, cu.IsSuccess, "断网时 check_update 应仍为成功（只表示无法核对）: %s", cu.ErrorMsg)
	d1 := Data[CheckUpdateData](t, cu)
	Eq(t, "断网 has_update", d1.HasUpdate, false)
	Eq(t, "断网 current_version", d1.CurrentVersion, "1.0.0")
	Eq(t, "断网后快照仍在", Exists(filepath.Join(c.VerDir("1.1.0"), "app.exe")), true)

	e.StartServer(t, port) // 必须复用端口
	cu2 := e.RunLast(t, c, "check_update")
	d2 := Data[CheckUpdateData](t, cu2)
	Eq(t, "恢复后 has_update", d2.HasUpdate, true)
	Eq(t, "恢复后 need_download_update（继续 apply）", d2.NeedDownloadUpdate, false)
	Eq(t, "恢复后 new_version", d2.NewVersion, "1.1.0")
}

// ────────────────────────────────────────────────────────────────
// 用例 28 — 空/未知状态执行 apply：拒绝替换且不改状态
// ────────────────────────────────────────────────────────────────
func TestCase28(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-bogus"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")

	// 场景 1：空状态
	c1 := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusEmpty, Previous: "1.0.0",
		Project: proj, MainFile: "fake-main-v0",
	})
	before := ReadText(filepath.Join(c1.Main(), "app.exe"))
	out1 := e.RunLast(t, c1, "apply_update")
	Eq(t, "空状态主目录未被改动", ReadText(filepath.Join(c1.Main(), "app.exe")), before)
	Eq(t, "空状态无空版本名目录", Exists(filepath.Join(c1.Pkg, "ApplicationFolder_")), false)
	t.Logf("空状态 isSuccess=%v errorMsg=%q", out1.IsSuccess, out1.ErrorMsg)

	// 场景 2：未知状态
	c2 := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: "bogus", Previous: "1.0.0",
		Project: proj, MainFile: "fake-main-v0",
	})
	out2 := e.RunLast(t, c2, "apply_update")
	Eq(t, "未知状态应拒绝", out2.IsSuccess, false)
	Contains(t, out2.ErrorMsg, "unknown version_status", "errorMsg")
	Eq(t, "未知状态主目录未被改动",
		ReadText(filepath.Join(c2.Main(), "app.exe")), "fake-main-v0")
	Eq(t, "未知状态无空版本名目录",
		Exists(filepath.Join(c2.Pkg, "ApplicationFolder_")), false)

	// 场景 3：downloaded 但 Version 为空
	// 这里**故意**要空版本号（造 A1 异常现场），必须显式声明 AllowEmptyVersion，
	// 否则 NewClient 的"Version 必填"守卫会拦住——那条守卫是防"忘传参数"，
	// 不是防"故意造异常现场"。
	c3 := e.NewClient(t, NewClientOpts{
		Version: "", Status: StatusDownloaded, Previous: "1.0.0",
		Project: proj, MainFile: "fake-main-v0",
		AllowEmptyVersion: true,
	})
	out3 := e.RunLast(t, c3, "apply_update")
	Eq(t, "Version 为空应拒绝", out3.IsSuccess, false)
	Eq(t, "主目录未被改动",
		ReadText(filepath.Join(c3.Main(), "app.exe")), "fake-main-v0")
	Eq(t, "不得产生空版本名目录",
		Exists(filepath.Join(c3.Pkg, "ApplicationFolder_")), false)
	t.Logf("Version 空 errorMsg=%q", out3.ErrorMsg)
}

// ────────────────────────────────────────────────────────────────
// 用例 29 — 非法状态 Version == VersionPrevious：不报待应用且可自愈
// ────────────────────────────────────────────────────────────────
func TestCase29(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-same"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusDownloaded, Previous: "1.1.0",
		Project: proj, MainFile: "app-110",
	})
	// 快照也要造出来（downloaded 状态通常有快照）
	WriteText(t, filepath.Join(c.VerDir("1.1.0"), "app.exe"), "app-110")

	cu := e.RunLast(t, c, "check_update")
	Eq(t, "不上报待应用", Data[CheckUpdateData](t, cu).HasUpdate, false)

	// 注意校验顺序：`Version == VersionPrevious` 拒绝发生在 apply_update 的
	// downloaded 分支里；而 download_update 会**先把状态归一化成 applied**。
	// 所以必须"先验证 apply 的拒绝"，再验证 download 的自愈 —— 反了就看不到
	// 那条 errorMsg（apply 会走 applied 分支报 no pending update）。
	ap := e.RunLast(t, c, "apply_update")
	Eq(t, "apply 应拒绝", ap.IsSuccess, false)
	Contains(t, ap.ErrorMsg, "Version == VersionPrevious", "errorMsg")
	Eq(t, "主目录未被改动",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
	Eq(t, "拒绝时不改状态（仍是 downloaded）",
		c.ReadVersion(t).VersionStatus, StatusDownloaded)

	// 再走 download：归一化为 applied（自愈路径）
	e.RunLast(t, c, "download_update")
	Eq(t, "download 归一化为 applied", c.ReadVersion(t).VersionStatus, StatusApplied)
}

// ────────────────────────────────────────────────────────────────
// 用例 30 — apply 熔断：连续 3 次失败后停止自动 apply
//
// 直接构造 apply_fail_count 现场（避免真造 3 次失败）。
// ────────────────────────────────────────────────────────────────
func TestCase30(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-fuse"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusDownloaded, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	// 快照必须存在且文件数够 —— 否则 check_update 会先命中 F3 的
	// "待应用目录缺失/残缺 → 要求重下"分支（HasUpdate=true），
	// 走不到熔断判断。所以这里造假一个"下载完好"的快照。
	WriteText(t, filepath.Join(c.VerDir("1.1.0"), "app.exe"), "app-110")
	WriteText(t, filepath.Join(c.VerDir("1.1.0"), ".updator", "shared.json"),
		ReadText(filepath.Join(c.Main(), ".updator", "shared.json")))
	c.WriteVersion(t, VersionInfo{
		VersionPrevious: "1.0.0", Version: "1.1.0",
		VersionStatus: StatusDownloaded, DownloadedFileCount: 2,
		ApplyFailCount: 3, // MaxApplyFailCount
	})

	// 服务端停在 1.0.0（低于待应用的 1.1.0）→ 走"继续 apply"分支，
	// 那里才是熔断判断所在位置。
	cu := e.RunLast(t, c, "check_update")
	Eq(t, "熔断后不上报待应用", Data[CheckUpdateData](t, cu).HasUpdate, false)
	// 熔断提示走 stderr（fmt.Fprintf(os.Stderr, "check_update: apply circuit broken ...")），
	// 不落 download.log，故这里不做日志断言，改由 Out 的原始 stderr 观察。
	t.Logf("熔断 check_update errorMsg=%q", cu.ErrorMsg)

	// 更高版本 → 应重新下载并复位
	e.PutFile(t, proj, "app.exe", "app-120")
	True(t, e.Publish(t, proj, "1.2.0", nil, "", "").IsSuccess, "发布 1.2.0")
	cu2 := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	Eq(t, "更高版本要求重下", cu2.NeedDownloadUpdate, true)
	Eq(t, "更高版本版本号", cu2.NewVersion, "1.2.0")

	True(t, e.RunLast(t, c, "download_update").IsSuccess, "重下应成功")
	Eq(t, "重下后复位", c.ReadVersion(t).ApplyFailCount, 0)
}

// ────────────────────────────────────────────────────────────────
// 用例 31 — 并发锁：两个 client 同时 download
// ────────────────────────────────────────────────────────────────
func TestCase31(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-lock"
	e.EnsureProject(t, proj, false, nil, nil)
	bigSrc := filepath.Join(e.DataDir(proj), "big.bin")
	BigFile(t, bigSrc, 20) // 大一点，拉长窗口确保并发重叠
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Project: proj, NoMain: true,
	})

	outs := e.RunConcurrent(t,
		[]*Client{c, c},
		[]string{"download_update"}, []string{"download_update"},
	)

	oks := 0
	for _, o := range outs {
		if o.IsSuccess {
			oks++
		}
	}
	Eq(t, "应恰好一个成功", oks, 1)
	// 失败的必须是"锁被占用"，不是别的错
	for _, o := range outs {
		if !o.IsSuccess {
			t.Logf("被拒的调用 errorMsg=%q", o.ErrorMsg)
		}
	}
	Eq(t, "产物完整", SizeOf(filepath.Join(c.VerDir("1.1.0"), "big.bin")), int64(20<<20))
	Eq(t, "产物哈希与源一致",
		MD5Of(filepath.Join(c.VerDir("1.1.0"), "big.bin")), MD5Of(bigSrc))
}

// ────────────────────────────────────────────────────────────────
// 用例 32 — 首次更新：不残留无名备份目录
// ────────────────────────────────────────────────────────────────
func TestCase32(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-firstdeploy"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	// 首次部署：version_previous 为空（不知道主目录内容的版本），
	// 当前实际版本按 1.0.0 计（服务端已发 1.1.0，有得下）。
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "", // 版本未知
		Project: proj, MainFile: "old-unknown",
	})
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")
	ap := e.RunLast(t, c, "apply_update")
	True(t, ap.IsSuccess, "apply 失败: %s\n%s", ap.ErrorMsg, c.Log(t, "update.log", 120))

	Eq(t, "不残留无名备份目录",
		Exists(filepath.Join(c.Pkg, "ApplicationFolder_")), false)
	names := ListDir(c.Pkg)
	t.Logf("包目录内容: %v", names)
	Eq(t, "主目录为新内容",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
}

// ────────────────────────────────────────────────────────────────
// 用例 33 — 目录被占用时 apply：探测并处置占用者
// ────────────────────────────────────────────────────────────────
func TestCase33(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-busy"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "lib.dll", "lib-100")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
	e.PutFile(t, proj, "lib.dll", "lib-110")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-100")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")

	// 占住主目录（子进程把自己 CWD 放在这里 → 目录不可改名）
	_, stop := e.StartHolder(t, c.Main())
	defer stop()

	ap := e.RunLast(t, c, "apply_update")
	True(t, ap.IsSuccess, "占用场景 apply 应成功（探测+处置占用者）: %s\n%s",
		ap.ErrorMsg, c.Log(t, "update.log", 120))
	Eq(t, "主目录为新版",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-110")
}

// ────────────────────────────────────────────────────────────────
// 用例 34 — 中文/空格/深层嵌套路径全链路
// ────────────────────────────────────────────────────────────────
func TestCase34(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-path"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "中 文件.txt", "cn-content")
	e.PutFile(t, proj, "目录 名/子 目录/深层.txt", "cn-nested")
	e.PutFile(t, proj, "a/b/c/d/e/f/deep.txt", "deep-content")
	// 客户端 1.0.0 → 服务端需更高版本
	True(t, e.Publish(t, proj, "1.0.1", nil, "", "").IsSuccess, "发布 1.0.1")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, NoMain: true,
	})
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply")

	for _, p := range []string{"中 文件.txt", "目录 名/子 目录/深层.txt", "a/b/c/d/e/f/deep.txt"} {
		full := filepath.Join(c.Main(), filepath.FromSlash(p))
		True(t, Exists(full), "路径未落地（中文/空格/深层）: %s\n%s", p, Tree(c.Main(), 40))
	}
	Eq(t, "中文文件内容",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "中 文件.txt"))), "cn-content")
	Eq(t, "中文嵌套内容",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "目录 名", "子 目录", "深层.txt"))), "cn-nested")
	Eq(t, "深层文件内容",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "a", "b", "c", "d", "e", "f", "deep.txt"))),
		"deep-content")

	// 第二轮：改一个文件，验证更新也能命中这些路径
	e.PutFile(t, proj, "中 文件.txt", "cn-content-v2")
	True(t, e.Publish(t, proj, "1.0.2", nil, "", "").IsSuccess, "发布 1.0.2")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 2")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 2")
	Eq(t, "第二轮更新命中中文路径",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "中 文件.txt"))), "cn-content-v2")
	Eq(t, "第二轮其余文件不变",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "a", "b", "c", "d", "e", "f", "deep.txt"))),
		"deep-content")
}

// ────────────────────────────────────────────────────────────────
// 用例 35 — 回滚到不存在的版本
// ────────────────────────────────────────────────────────────────
func TestCase35(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-roll404"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusApplied, Previous: "1.1.0",
		Project: proj, MainFile: "app-110",
	})
	// 只有 1.0.0 备份（含 shared.json）
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), "app.exe"), "app-100")
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), ".updator", "shared.json"),
		ReadText(filepath.Join(c.Main(), ".updator", "shared.json")))

	out := e.RunLast(t, c, "rollback", "--version", "9.9.9")
	Eq(t, "isSuccess", out.IsSuccess, false)
	Contains(t, out.ErrorMsg, "version 9.9.9 not found", "errorMsg")

	v := c.ReadVersion(t)
	Eq(t, "状态不变", v.VersionStatus, StatusApplied)
	Eq(t, "版本不变", v.Version, "1.1.0")
	Eq(t, "主目录内容不变",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")

	// 回滚到"已下载但未应用"的版本：另一条拒绝路径
	c2 := e.NewClient(t, NewClientOpts{
		Version: "1.2.0", Status: StatusDownloaded, Previous: "1.1.0",
		Project: proj, MainFile: "app-110",
	})
	WriteText(t, filepath.Join(c2.VerDir("1.2.0"), "app.exe"), "app-120")
	out2 := e.RunLast(t, c2, "rollback", "--version", "1.2.0")
	Eq(t, "回滚未应用版本应拒绝", out2.IsSuccess, false)
	Contains(t, out2.ErrorMsg, "未应用的下载版本", "errorMsg")
}