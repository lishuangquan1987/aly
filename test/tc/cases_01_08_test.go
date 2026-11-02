package tc

// 用例 1–8：aly-client 正常流程主干
//
// 覆盖"一台干净机器从零走完一次完整更新"：check → download → apply → 回滚。
// 每条用例自造前置现场（不依赖前一条的遗留状态），可单独执行：
//
//	go test -tags purego -run TestCase01 ./test/tc -v
//
// 运行前须按本机情况核对 env.go 的 Go126 / Go1108 路径，或设环境变量
// ALY_GO126 / ALY_GO1108 覆盖。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────
// 用例 1 — check_update：服务器版本与本地相同，判定无更新
//
// 前置：服务端有项目 tc-basic（**创建即默认版本 V1.0.0**，
// CreateProjectWithFirstLog 会写一条 V1.0.0 变更日志），数据目录内有 app.exe；
// 客户端 1.0.0/applied。
//
// 注意：这里**不能**再 Publish("1.0.0")。PublishVersion 的防倒退校验是
// CompareVersion(new, 已发布最高版本) <= 0 即拒，而 projects 表的初始
// Version 与首条 changelog 都是 V1.0.0 —— 重发 1.0.0 会被判"不大于最新版本"。
// 用例 1 要验证的是"两端版本相同 → 无更新"，创建项目的默认状态正好满足。
//
// 重点：check 是**纯只读**命令，version.json 必须逐字节不变。
// ────────────────────────────────────────────────────────────────
func TestCase01(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-basic"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	before := ReadText(filepath.Join(c.Up(), "version.json"))

	out := e.RunLast(t, c, "check_update")
	True(t, out.IsSuccess, "check_update 应成功，实际 errorMsg=%q", out.ErrorMsg)

	d := Data[CheckUpdateData](t, out)
	Eq(t, "has_update", d.HasUpdate, false)
	Eq(t, "current_version", d.CurrentVersion, "1.0.0")
	// 无更新时不下发 need_download_update / new_version（omitempty 语义）
	Eq(t, "need_download_update", d.NeedDownloadUpdate, false)
	Eq(t, "new_version", d.NewVersion, "")

	after := ReadText(filepath.Join(c.Up(), "version.json"))
	Eq(t, "version.json 是否被改写", after == before, true)
}

// ────────────────────────────────────────────────────────────────
// 用例 2 — check_update：服务器版本更高，上报可更新并带出变更说明
//
// 前置：客户端 1.0.0/applied；服务端已发 1.1.0（logs 两条）。
// 重点：new_version 必须已剥离 v 前缀；force_update 指针必下发；
//       release_notes 带出本次变更内容。
//
// 注意：项目创建即 V1.0.0（首条 changelog 也是 V1.0.0），故基线 1.0.0
// 不需要、也不能再 Publish 一次（防倒退校验会拒）。
// ────────────────────────────────────────────────────────────────
func TestCase02(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-basic"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "lib.dll", "lib-110")
	r := e.Publish(t, proj, "1.1.0", []string{"新增导出功能", "修复登录超时"}, "", "")
	True(t, r.IsSuccess, "发布 1.1.0 失败: %s", r.ErrorMsg)

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	out := e.RunLast(t, c, "check_update")
	True(t, out.IsSuccess, "check_update 失败: %s", out.ErrorMsg)

	d := Data[CheckUpdateData](t, out)
	Eq(t, "has_update", d.HasUpdate, true)
	Eq(t, "need_download_update", d.NeedDownloadUpdate, true)
	Eq(t, "current_version", d.CurrentVersion, "1.0.0")
	Eq(t, "new_version", d.NewVersion, "1.1.0") // 必须不带 v 前缀
	True(t, d.ForceUpdate != nil, "force_update 字段必须下发（applied 分支用指针 + omitempty）")
	Eq(t, "force_update", *d.ForceUpdate, false)
	Eq(t, "release_notes 条数", len(d.ReleaseNotes), 2)
	Eq(t, "release_notes[0]", d.ReleaseNotes[0], "新增导出功能")
	Eq(t, "release_notes[1]", d.ReleaseNotes[1], "修复登录超时")

	// 幂等：重复检查结果一致，且不改 version.json
	before := ReadText(filepath.Join(c.Up(), "version.json"))
	out2 := e.RunLast(t, c, "check_update")
	d2 := Data[CheckUpdateData](t, out2)
	Eq(t, "二次检查 has_update", d2.HasUpdate, d.HasUpdate)
	Eq(t, "二次检查 new_version", d2.NewVersion, d.NewVersion)
	Eq(t, "version.json 是否被改写", ReadText(filepath.Join(c.Up(), "version.json")) == before, true)
}

// ────────────────────────────────────────────────────────────────
// 用例 3 — download_update：首次下载，产物落版本快照目录并写 downloaded 状态
//
// 前置：服务端 tc-dl 从 1.0.0（app.exe/lib.dll/sub/res.txt）演进到 1.1.0
//       （lib.dll 改内容 + new.txt 新增）。客户端 1.0.0/applied。
// 重点：
//   - 完成行 data 必须为 null（SDK/UI 靠它判完成）
//   - 变化文件重新下载、未变化文件由 materialize 从 MainFolder 拷入
//   - version_previous 保持 1.0.0（MainFolder 真实内容版本，不能写成新版）
//   - downloaded_file_count = 快照目录常规文件数（apply 的 F3 守卫依赖它）
//   - MainFolder 不被改动（download 只写快照目录）
// ────────────────────────────────────────────────────────────────
func TestCase03(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-dl"
	e.EnsureProject(t, proj, false, nil, nil)

	// 1.0.0 的内容基线（项目创建即 V1.0.0，无需 Publish —— 重发会被防倒退校验拒）
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "lib.dll", "lib-100")
	e.PutFile(t, proj, "sub/res.txt", "res-100")

	// 客户端当前跑 1.0.0（主目录内容对齐）
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-100")
	WriteText(t, filepath.Join(c.Main(), "sub", "res.txt"), "res-100")

	// 演进到 1.1.0：lib.dll 改、new.txt 新增，app.exe 与 sub/res.txt 不变
	e.PutFile(t, proj, "lib.dll", "lib-110")
	e.PutFile(t, proj, "new.txt", "new-in-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"v1.1.0"}, "", "").IsSuccess, "发布 1.1.0 失败")

	outs := e.Run(t, c, "download_update")
	last := outs[len(outs)-1]
	True(t, last.IsSuccess, "download_update 失败: %s", last.ErrorMsg)
	Eq(t, "完成行 data 是否为 null", string(last.Data) == "null" || len(last.Data) == 0, true)

	// 过程行：至少 1 条 BEGIN（携带 total_bytes）
	var hasBegin bool
	for _, o := range outs {
		if o.Progress != nil && o.Progress.Status == "BEGIN" {
			hasBegin = true
			True(t, o.Progress.TotalBytes > 0, "BEGIN 行 total_bytes 应 > 0，实际 %d", o.Progress.TotalBytes)
		}
	}
	True(t, hasBegin, "缺少 BEGIN 进度行（字节级进度的预告行）")

	// 快照目录产物
	snap := c.VerDir("1.1.0")
	for _, want := range []string{"app.exe", "lib.dll", "new.txt", "sub/res.txt"} {
		p := filepath.Join(snap, filepath.FromSlash(want))
		True(t, Exists(p), "快照目录缺少 %s\n现场:\n%s", want, Tree(snap, 40))
	}
	Eq(t, "快照 lib.dll 内容", strings.TrimSpace(ReadText(filepath.Join(snap, "lib.dll"))), "lib-110")
	Eq(t, "快照 new.txt 内容", strings.TrimSpace(ReadText(filepath.Join(snap, "new.txt"))), "new-in-110")
	Eq(t, "快照 app.exe 内容（materialize 拷入）",
		strings.TrimSpace(ReadText(filepath.Join(snap, "app.exe"))), "app-100")
	Eq(t, "快照 sub/res.txt 内容（materialize 拷入）",
		strings.TrimSpace(ReadText(filepath.Join(snap, "sub", "res.txt"))), "res-100")

	// version.json 状态
	v := c.ReadVersion(t)
	Eq(t, "version_previous", v.VersionPrevious, "1.0.0")
	Eq(t, "version", v.Version, "1.1.0")
	Eq(t, "version_status", v.VersionStatus, StatusDownloaded)
	// downloaded_file_count = 快照目录**全部常规文件**数，含 materialize 一并拷入的
	// `.updator/shared.json`（发布端把它算在清单内：util/file.go 注释明确
	// 「.updator/ 需参与扫描（shared.json 随版本下发），不可排除」）。
	// 故本用例是 4 个业务文件 + 1 个 shared.json = 5；apply 的 F3 守卫
	// 比对的是同一个数，写 4 会误判"文件数不符"。
	Eq(t, "downloaded_file_count（4 业务文件 + .updator/shared.json）",
		v.DownloadedFileCount, 5)

	// MainFolder 不被改动
	Eq(t, "MainFolder app.exe 未被改动",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")
	Eq(t, "MainFolder 不应出现 new.txt", Exists(filepath.Join(c.Main(), "new.txt")), false)

	// 物化日志
	Contains(t, c.Log(t, "download.log", 80), "materialized unchanged files", "download.log")
}

// ────────────────────────────────────────────────────────────────
// 用例 4 — download_update：已是最新时不重复下载
//
// 前置：客户端 applied/1.1.0，服务端最新也是 1.1.0。
// 期望：isSuccess=false、errorMsg="already at latest version"、状态不变。
// ────────────────────────────────────────────────────────────────
func TestCase04(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-dl"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"v1.1.0"}, "", "").IsSuccess, "发布 1.1.0 失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-110",
	})
	before := ReadText(filepath.Join(c.Up(), "version.json"))

	out := e.RunLast(t, c, "download_update")
	Eq(t, "isSuccess", out.IsSuccess, false)
	Contains(t, out.ErrorMsg, "already at latest version", "errorMsg")

	Eq(t, "version.json 是否被改写", ReadText(filepath.Join(c.Up(), "version.json")) == before, true)
	Eq(t, "不应产生快照目录", Exists(c.VerDir("1.1.0")), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 5 — apply_update：原子替换后主目录为新版，旧版本成为备份目录
//
// 前置：客户端 downloaded/1.1.0，快照目录完好（由 download 生成）。
// 重点（原子替换的三段 rename 语义）：
//   - ApplicationFolder      = 新版本内容
//   - ApplicationFolder_1.0.0= 旧版本备份（**不含**新版本才有的 new.txt）
//   - ApplicationFolder_1.1.0 消失（已被改名成主目录）
//   - 无 ApplicationFolder_1.0.0.old 残留（成功路径会清理旁移临时目录）
//   - 过渡字段（rollback_previous/rollback_target/apply_stage）被清空
// ────────────────────────────────────────────────────────────────
func TestCase05(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-dl"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "lib.dll", "lib-100")
	e.PutFile(t, proj, "sub/res.txt", "res-100")
	// 项目创建即 V1.0.0：基线无需 Publish（重发 1.0.0 会被防倒退校验拒）

	e.PutFile(t, proj, "lib.dll", "lib-110")
	e.PutFile(t, proj, "new.txt", "new-in-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"v1.1.0"}, "", "").IsSuccess, "发布 1.1.0 失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-100")
	WriteText(t, filepath.Join(c.Main(), "sub", "res.txt"), "res-100")

	dl := e.RunLast(t, c, "download_update")
	True(t, dl.IsSuccess, "download_update 失败: %s", dl.ErrorMsg)

	ap := e.RunLast(t, c, "apply_update")
	True(t, ap.IsSuccess, "apply_update 失败: %s\nupdate.log:\n%s", ap.ErrorMsg,
		c.Log(t, "update.log", 60))
	Eq(t, "apply 完成行 data", string(ap.Data) == "null" || len(ap.Data) == 0, true)

	// 主目录 = 新内容
	Eq(t, "主目录 lib.dll", strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-110")
	Eq(t, "主目录 new.txt 存在", Exists(filepath.Join(c.Main(), "new.txt")), true)
	Eq(t, "主目录 app.exe 存在", Exists(filepath.Join(c.Main(), "app.exe")), true)

	// 备份 = 旧内容，且不含新版本才有的文件
	bak := c.VerDir("1.0.0")
	Eq(t, "备份目录存在", Exists(bak), true)
	Eq(t, "备份 lib.dll（旧）", strings.TrimSpace(ReadText(filepath.Join(bak, "lib.dll"))), "lib-100")
	Eq(t, "备份不应含 new.txt", Exists(filepath.Join(bak, "new.txt")), false)
	Eq(t, "备份 sub/res.txt", strings.TrimSpace(ReadText(filepath.Join(bak, "sub", "res.txt"))), "res-100")

	// 快照目录已消失；无旁移残留
	Eq(t, "ApplicationFolder_1.1.0 应已消失", Exists(c.VerDir("1.1.0")), false)
	Eq(t, "不应残留 .old 旁移目录", Exists(c.VerDir("1.0.0")+".old"), false)

	// version.json 终态
	v := c.ReadVersion(t)
	Eq(t, "version_previous", v.VersionPrevious, "1.0.0")
	Eq(t, "version", v.Version, "1.1.0")
	Eq(t, "version_status", v.VersionStatus, StatusApplied)
	Eq(t, "apply_fail_count 应为 0", v.ApplyFailCount, 0)
	Eq(t, "rollback_previous 应清空", v.RollbackPrevious, "")
	Eq(t, "rollback_target 应清空", v.RollbackTarget, "")
	Eq(t, "apply_stage 应清空", v.ApplyStage, "")

	NotContains(t, c.Log(t, "update.log", 80), "apply attempt", "update.log 不应有重试失败")
}

// ────────────────────────────────────────────────────────────────
// 用例 6 — list_rollback_versions：列出可回滚版本，排除非版本项
//
// 前置：当前 applied/1.1.0，存在 ApplicationFolder_1.0.0 与 _0.9.0，
//       另有干扰目录 ApplicationFolder_backup / ApplicationFolder_1.2.0.tmp。
// 重点：非版本号目录必须被 isLikelyVersion 过滤掉。
// ────────────────────────────────────────────────────────────────
func TestCase06(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-roll"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"v1.1.0"}, "", "").IsSuccess, "发布失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-110",
	})
	// 造历史备份目录
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), "app.exe"), "app-100")
	WriteText(t, filepath.Join(c.VerDir("0.9.0"), "app.exe"), "app-090")
	// 造非版本号干扰目录
	WriteText(t, filepath.Join(c.Pkg, "ApplicationFolder_backup", "x.txt"), "x")
	WriteText(t, filepath.Join(c.Pkg, "ApplicationFolder_1.2.0.tmp", "x.txt"), "x")

	out := e.RunLast(t, c, "list_rollback_versions")
	True(t, out.IsSuccess, "list_rollback_versions 失败: %s", out.ErrorMsg)

	d := Data[RollbackListData](t, out)
	Eq(t, "current_version", d.CurrentVersion, "1.1.0")

	got := strings.Join(d.Versions, ",")
	Contains(t, got, "1.0.0", "versions 应含 1.0.0")
	Contains(t, got, "0.9.0", "versions 应含 0.9.0")
	NotContains(t, got, "backup", "versions 不应含非版本项 backup")
	NotContains(t, got, "1.2.0.tmp", "versions 不应含 .tmp 目录")
}

// ────────────────────────────────────────────────────────────────
// 用例 7 — rollback：回滚到指定版本，主目录与备份目录互换
//
// 前置：当前 applied/1.1.0，存在 ApplicationFolder_1.0.0 备份。
// 重点（第四批回归点）：
//   - 主目录内容回到旧版
//   - 回滚前的运行版本被备份为 ApplicationFolder_1.1.0
//   - **release_manifest.json 缓存必须被删除**（否则下轮差量删除会误删文件）
//   - 重复回滚同名版本被拒绝
// ────────────────────────────────────────────────────────────────
func TestCase07(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-roll"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "new.txt", "new-in-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"v1.1.0"}, "", "").IsSuccess, "发布失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-110",
	})
	WriteText(t, filepath.Join(c.Main(), "new.txt"), "new-in-110")
	// 造 1.0.0 备份（旧内容，不含 new.txt）
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), "app.exe"), "app-100")
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), "lib.dll"), "lib-100")
	// 关键：真实发布包里 `.updator/` 是随版本下发的（util/file.go 明确注释
	// 「.updator/ 需参与扫描（shared.json 随版本下发），不可排除」）。
	// rollback 走的是三段 rename，MainFolder 连同其内部的 .updator/ 一起被
	// 改名成备份，回滚后的新主目录来自 1.0.0 快照 —— 快照里若没有 shared.json，
	// 回滚后任何命令都会报 "load shared.json ... cannot find the path"。
	// 所以这里必须把 shared.json 一并放进 1.0.0 快照，才是真实现场。
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), ".updator", "shared.json"),
		ReadText(filepath.Join(c.Main(), ".updator", "shared.json")))
	// 造清单缓存（回滚成功必须清掉）。真实格式是纯字符串数组，不是对象。
	WriteText(t, filepath.Join(c.Up(), "release_manifest.json"),
		`["app.exe","lib.dll","new.txt"]`)
	True(t, Exists(filepath.Join(c.Up(), "release_manifest.json")), "前置：清单缓存应已存在")

	out := e.RunLast(t, c, "rollback", "--version", "1.0.0")
	True(t, out.IsSuccess, "rollback 失败: %s\nupdate.log:\n%s", out.ErrorMsg,
		c.Log(t, "update.log", 60))

	d := Data[RollbackData](t, out)
	Eq(t, "data.version", d.Version, "1.0.0")

	// 主目录回到旧内容
	Eq(t, "主目录 lib.dll（旧）", strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-100")
	Eq(t, "主目录不应再有 new.txt", Exists(filepath.Join(c.Main(), "new.txt")), false)

	// 回滚前的运行版本成为备份
	Eq(t, "ApplicationFolder_1.1.0 备份应出现", Exists(c.VerDir("1.1.0")), true)
	Eq(t, "备份 1.1.0 内含 new.txt", Exists(filepath.Join(c.VerDir("1.1.0"), "new.txt")), true)
	Eq(t, "ApplicationFolder_1.0.0 应已消失", Exists(c.VerDir("1.0.0")), false)

	// 状态
	v := c.ReadVersion(t)
	Eq(t, "version_previous", v.VersionPrevious, "1.1.0")
	Eq(t, "version", v.Version, "1.0.0")
	Eq(t, "version_status", v.VersionStatus, StatusApplied)

	// 第四批回归点：回滚成功清清单缓存
	Eq(t, "release_manifest.json 应被删除", Exists(filepath.Join(c.Up(), "release_manifest.json")), false)

	// 幂等：再回滚同一版本应被拒绝。
	// 注意报错是 `version 1.0.0 not found` 而**不是**「当前已处于版本 1.0.0」：
	// rollback.go 的检查顺序是 ①versionDir stat（L102）→ ②downloaded 守卫 →
	// ③oldVersion 相等守卫（L131）。回滚成功后 ApplicationFolder_1.0.0 已被
	// 改名成 MainFolder 而消失，所以第二次必然先在 ① 被拦。源码注释也点明
	// 「该场景仅当活动版本目录残留存在时可达」。
	out2 := e.RunLast(t, c, "rollback", "--version", "1.0.0")
	Eq(t, "重复回滚 isSuccess", out2.IsSuccess, false)
	Contains(t, out2.ErrorMsg, "not found", "重复回滚 errorMsg")
}

// ────────────────────────────────────────────────────────────────
// 用例 8 — check_diff：输出与服务端不一致的文件清单
//
// 前置：服务端有 app.exe / lib.dll / same.txt；客户端缺 app.exe、
//       lib.dll 内容不同、same.txt 完全一致。
// 重点（单向语义 + SHA256 计算条件）：
//   - 只列"服务端有而本地缺失/不一致"的文件，本地独有文件不出现
//   - 本地缺失：local_md5 空、local_sha256 空（不计算）
//   - 本地存在但不同：local_sha256 **非空**（仅对 MD5 不同且本地存在的并行计算）
//   - 完全一致：不出现在差异列表
// ────────────────────────────────────────────────────────────────
func TestCase08(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-diff"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-server")
	e.PutFile(t, proj, "lib.dll", "lib-server")
	e.PutFile(t, proj, "same.txt", "identical")
	True(t, e.Publish(t, proj, "1.1.0", []string{"v1.1.0"}, "", "").IsSuccess, "发布失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, NoMain: true, // 不放主程序：app.exe 缺失
	})
	WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-local-different")
	WriteText(t, filepath.Join(c.Main(), "same.txt"), "identical")
	WriteText(t, filepath.Join(c.Main(), "local-only.txt"), "only-local")

	out := e.RunLast(t, c, "check_diff")
	True(t, out.IsSuccess, "check_diff 失败: %s", out.ErrorMsg)

	d := Data[DiffData](t, out)
	Eq(t, "new_version", d.NewVersion, "1.1.0")

	byPath := map[string]DiffItem{}
	for _, f := range d.Files {
		byPath[strings.ReplaceAll(f.Path, "\\", "/")] = f
	}

	// app.exe：本地缺失
	f, ok := byPath["app.exe"]
	True(t, ok, "差异列表应含 app.exe（本地缺失）\n实际: %v", keys(byPath))
	Eq(t, "app.exe local_md5", f.LocalMD5, "")
	Eq(t, "app.exe local_size", f.LocalSize, int64(0))
	Eq(t, "app.exe local_sha256（缺失不应计算）", f.LocalSHA256, "")
	True(t, f.ServerMD5 != "" && f.ServerSize > 0 && f.ServerSHA256 != "",
		"app.exe 的 server 侧字段应完整: %+v", f)

	// lib.dll：MD5 不同
	f, ok = byPath["lib.dll"]
	True(t, ok, "差异列表应含 lib.dll（内容不同）\n实际: %v", keys(byPath))
	True(t, f.LocalMD5 != "" && f.LocalMD5 != f.ServerMD5,
		"lib.dll local_md5 应非空且不同于 server_md5: %+v", f)
	True(t, f.LocalSHA256 != "", "lib.dll local_sha256 应被计算（MD5 不同且本地存在）: %+v", f)

	// same.txt：完全一致 → 不出现
	_, ok = byPath["same.txt"]
	Eq(t, "same.txt 出现在差异列表", ok, false)

	// 本地独有文件：单向语义 → 不出现
	_, ok = byPath["local-only.txt"]
	Eq(t, "local-only.txt 出现在差异列表", ok, false)
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// 保持 os 引用（部分用例后续会用到环境变量控制）
var _ = os.Getenv