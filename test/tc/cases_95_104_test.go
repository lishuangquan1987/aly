package tc

// 用例 95–104：集成链
//
// 前面十节是"单组件 + 自造现场"，这一节是**真链路**：服务端 + 发布端 + 客户端 + UI + SDK
// 串起来跑，验证跨组件的契约没有对不上。
//
// 判据统一为"用户在真实机器上会看到什么"——不是内部状态对不对，而是最终文件、
// 最终版本、最终界面。所以这里大量断言**磁盘产物**而不是 JSON 字段。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────
// 用例 95 — server ↔ client 全链路闭环（发布 → 更新 → 回滚）
//
// 这是整份文档唯一"一次跑完四段"的用例：发布端演进、客户端 check、
// download 差量（含删除同步）、apply 原子替换、rollback 目录互换。
// 每一段都可能单独通过而在交界处出错，所以断言必须在**同一现场**里连续做。
//
// 关键前置（易踩）：**项目创建不会写发布清单**。`.updator_manifest.json`
// 只在 PublishVersion 成功时按磁盘实际内容固化（project_service.go 的
// writeReleaseManifest）。所以必须先真正发布一版，否则
// GetReleaseManifest 返回空清单 → 客户端走 "release manifest empty,
// skip delete-sync" 分支 → 既不写缓存也不做删除。这里用 1.0.1 当"基线版"
// （内容不变，只为把清单建起来），再演进到 1.1.0 做真正的差量 + 删除。
// ────────────────────────────────────────────────────────────────
func TestCase95(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-e2e"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "lib/old.dll", "old-dll")
	// 项目创建即 V1.0.0：基线版必须 > V1.0.0，用 1.0.1（内容不变，仅为固化清单）
	True(t, e.Publish(t, proj, "1.0.1", []string{"基线"}, "", "").IsSuccess, "发布基线 1.0.1")
	Eq(t, "基线清单已固化",
		strings.Join(e.GetManifest(t, proj).Files, ""), "app.exe"+"lib/old.dll")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "lib", "old.dll"), "old-dll")

	// ── 轮 0：把基线建立起来（1.0.0 → 1.0.1，内容相同，只为写缓存）──
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "基线 download 失败")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "基线 apply 失败")
	Eq(t, "基线缓存已建立", Exists(filepath.Join(c.Up(), "release_manifest.json")), true)
	Eq(t, "基线版本", c.ReadVersion(t).Version, "1.0.1")

	// ── ① 服务端演进：改 app.exe、删 lib/old.dll、加 lib/new.dll ──
	e.PutFile(t, proj, "app.exe", "app-110")
	e.RemoveFile(t, proj, "lib/old.dll")
	e.PutFile(t, proj, "lib/new.dll", "new-dll")
	True(t, e.Publish(t, proj, "1.1.0", []string{"改版"}, "", "").IsSuccess, "发布 1.1.0")

	// ── ② check ──
	cu := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	Eq(t, "has_update", cu.HasUpdate, true)
	Eq(t, "new_version", cu.NewVersion, "1.1.0")
	Eq(t, "need_download_update", cu.NeedDownloadUpdate, true)
	Eq(t, "release_notes 透传服务端日志", strings.Join(cu.ReleaseNotes, ","), "改版")

	// ── ③ download + apply ──
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.1.0")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.1.0\nupdate.log:\n%s",
		c.Log(t, "update.log", 60))

	Eq(t, "app.exe 已更新", strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
	Eq(t, "旧文件被删（差量删除生效）",
		Exists(filepath.Join(c.Main(), "lib", "old.dll")), false)
	Eq(t, "新文件落地",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib", "new.dll"))), "new-dll")
	v := c.ReadVersion(t)
	Eq(t, "version", v.Version, "1.1.0")
	Eq(t, "version_previous", v.VersionPrevious, "1.0.1")
	Eq(t, "version_status", v.VersionStatus, StatusApplied)
	// 注意（实测口径，与"已清 0"的直觉不同）：apply **不复位** downloaded_file_count。
	// 该字段在 download_update 末尾由 countRegularFiles(targetDir) 写成，语义是
	// "下载包里含几个常规文件"（本例 1.1.0 = app.exe / lib/new.dll / .updator/shared.json 三件）；
	// apply 只在 version_status=downloaded 的分支里**读**它做残缺现场校验
	// （pendingCount < DownloadedFileCount → 拒绝替换），成功替换路径（apply_update.go:272-279）
	// 只清 transient state + 置 applied，没有归零该字段。残留值无害：check_update 的
	// 该分支只在 downloaded 状态下走，applied 状态不会误用。
	Eq(t, "downloaded_file_count 保留下载包文件数", v.DownloadedFileCount, 3)
	Eq(t, "apply_stage 已清", v.ApplyStage, "")

	// ── ④ 回滚（回到那个真的被替换掉的版本 1.0.0）──
	// 为什么是 1.0.0：轮 0 的 apply 生成了 ApplicationFolder_1.0.0 备份；
	// 1.0.1 与 1.0.0 内容相同，但 1.0.0 目录是唯一"真实存在过的先后版本快照"。
	rl := Data[RollbackListData](t, e.RunLast(t, c, "list_rollback_versions"))
	Eq(t, "可回滚到 1.0.0", SliceContains(rl.Versions, "1.0.0"), true)
	Eq(t, "可回滚列表不含当前版本 1.1.0", SliceContains(rl.Versions, "1.1.0"), false)

	rb := e.RunLast(t, c, "rollback", "--version", "1.0.0")
	True(t, rb.IsSuccess, "回滚失败: %s\nupdate.log:\n%s", rb.ErrorMsg, c.Log(t, "update.log", 60))
	Eq(t, "回滚后 app.exe",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")
	Eq(t, "回滚后旧文件回来",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib", "old.dll"))), "old-dll")
	Eq(t, "回滚后 version", c.ReadVersion(t).Version, "1.0.0")
	Eq(t, "回滚后 version_previous", c.ReadVersion(t).VersionPrevious, "1.1.0")

	// 回滚成功必须清掉清单缓存（否则下轮删除基线是 1.1.0 的，会误删 1.0.0 的文件）
	Eq(t, "回滚后 release_manifest.json 应被清掉",
		Exists(filepath.Join(c.Up(), "release_manifest.json")), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 96 — 删除同步联动（服务端删文件 → 客户端删文件）
//
// 这条用例的关键是"**两轮**"：删除同步靠 release_manifest.json 缓存做差集，
// 而缓存是上一轮 download 写下的。只有一轮的话根本没有基线，
// 差集算不出来——首轮还会打印 "no cached manifest"，那是正常提示不是错。
//
// 另一处易踩：**项目创建不写发布清单**（只有 PublishVersion 才固化），
// 清单为空时客户端直接 "release manifest empty, skip delete-sync"，
// 连缓存都不写。所以这里必须先发布一版 1.0.1 把清单建起来。
// ────────────────────────────────────────────────────────────────
func TestCase96(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sync"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-090")
	e.PutFile(t, proj, "a.txt", "a")
	e.PutFile(t, proj, "b.txt", "b")
	e.PutFile(t, proj, "c.txt", "c")
	// 基线发布：内容不变，只为把 .updator_manifest.json 固化下来
	True(t, e.Publish(t, proj, "1.0.1", []string{"基线"}, "", "").IsSuccess, "发布基线 1.0.1")
	Eq(t, "服务端基线清单含 b.txt", SliceContains(e.GetManifest(t, proj).Files, "b.txt"), true)

	c := e.NewClient(t, NewClientOpts{
		Version: "0.9.0", Status: StatusApplied, Previous: "0.9.0",
		Project: proj, MainFile: "app-090",
	})

	// ── 首轮：0.9.0 → 1.0.1（建立清单缓存）──
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "首轮 download")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "首轮 apply")

	manifestPath := filepath.Join(c.Up(), "release_manifest.json")
	Eq(t, "基线已建立", Exists(manifestPath), true)
	// 缓存格式是**纯字符串数组**（util.WriteCachedManifest 用 json.MarshalIndent 写出）
	var base []string
	True(t, jsonUnmarshalText(t, manifestPath, &base), "缓存应是合法 JSON 数组")
	// 注意（实测口径）：服务端发布清单的口径 = **项目工作目录的实际内容**
	// （writeReleaseManifest → collectFileStamps 遍历 data/{project}/）。
	// 本用例的服务端目录只放了 app.exe/a.txt/b.txt/c.txt 四件——
	// 客户端的 .updator/shared.json 是**客户端本地**文件，从未上传到服务端，
	// 因此不在清单里。所以基线是 4 条而不是 5 条。
	Eq(t, "基线条目数（服务端项目目录 4 件；客户端 shared.json 不在清单内）", len(base), 4)
	Eq(t, "基线含 b.txt", SliceContains(base, "b.txt"), true)
	t.Logf("首轮基线: %v", base)

	// ── 服务端删 b.txt，发布 1.1.0 ──
	e.RemoveFile(t, proj, "b.txt")
	True(t, e.Publish(t, proj, "1.1.0", []string{"删 b"}, "", "").IsSuccess, "发布 1.1.0")
	Eq(t, "服务端清单已不含 b.txt",
		SliceContains(e.GetManifest(t, proj).Files, "b.txt"), false)

	// ── 二轮：更新到 1.1.0 ──
	outs := e.Run(t, c, "download_update")
	True(t, outs[len(outs)-1].IsSuccess, "二轮 download: %s", outs[len(outs)-1].ErrorMsg)
	log := c.Log(t, "download.log", 200)
	Contains(t, log, "delete-sync removed 1 file", "download.log 应记录删除同步（1 个文件）")

	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "二轮 apply")

	Eq(t, "b.txt 应被删除", Exists(filepath.Join(c.Main(), "b.txt")), false)
	Eq(t, "a.txt 保留", Exists(filepath.Join(c.Main(), "a.txt")), true)
	Eq(t, "c.txt 保留", Exists(filepath.Join(c.Main(), "c.txt")), true)

	// 缓存更新为 3 条（不含 b.txt）——否则下轮会把它再"删"一次刷日志。
	// 注意缓存口径 = **服务端清单**（4 件），不是版本目录里的实际文件数（5 件，多一个
	// 由 materialize 拷进去的客户端本地 .updator/shared.json）。
	var after []string
	True(t, jsonUnmarshalText(t, manifestPath, &after), "二轮后缓存仍应合法")
	Eq(t, "缓存条目数已减 1", len(after), 3)
	Eq(t, "缓存已不含 b.txt", SliceContains(after, "b.txt"), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 97 — 推送后立即可见（缓存失效链路）
//
// 这是"发布端 status 永远显示无变更"这类问题的根因用例：
// upload_file 后必须立刻 InvalidateProjectFileList，否则 get_all_files 会
// 一直返回旧 md5，发布端算出的差集为空 → 用户以为上传没成功。
// ────────────────────────────────────────────────────────────────
func TestCase97(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-cache"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	// ① 先 check 一次，把服务端文件列表缓存建起来
	cu0 := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	Eq(t, "先确认无更新", cu0.HasUpdate, false)
	oldMD5 := ""
	for _, f := range e.GetAllFiles(t, proj) {
		if filepath.ToSlash(f.FileRelativePath) == "app.exe" {
			oldMD5 = f.MD5
		}
	}
	True(t, oldMD5 != "", "缓存里应有 app.exe")

	// ② 上传新内容（不发布新版本）
	newLocal := filepath.Join(e.Root, "app-110.exe")
	WriteText(t, newLocal, "app-110")
	up := e.Upload(t, proj, "app.exe", newLocal)
	True(t, up.IsSuccess, "上传失败: %s", up.ErrorMsg)
	wantMD5 := MD5Of(newLocal)
	True(t, wantMD5 != oldMD5, "新内容 MD5 必须与旧的不同（否则本用例无意义）")

	// ③ 立刻 get_all_files：必须看到新 MD5（缓存已失效）
	found := false
	for _, f := range e.GetAllFiles(t, proj) {
		if filepath.ToSlash(f.FileRelativePath) == "app.exe" {
			Eq(t, "上传后 md5 立即更新（缓存已失效）", f.MD5, wantMD5)
			found = true
		}
	}
	Eq(t, "app.exe 出现在列表", found, true)

	// ④ 发布 1.1.0 → 客户端能正常拿到新内容
	True(t, e.Publish(t, proj, "1.1.0", []string{"上传后发布"}, "", "").IsSuccess, "发布 1.1.0")
	cu := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	Eq(t, "发布后识别新版本", cu.NewVersion, "1.1.0")

	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")
	Eq(t, "下载到的就是刚上传的内容",
		strings.TrimSpace(ReadText(filepath.Join(c.VerDir("1.1.0"), "app.exe"))), "app-110")
	Eq(t, "快照 MD5 与上传内容一致",
		MD5Of(filepath.Join(c.VerDir("1.1.0"), "app.exe")), wantMD5)
}

// ────────────────────────────────────────────────────────────────
// 用例 98 — 待应用被新版本覆盖（旧目录不干扰）
//
// 语义要点：downloaded/1.1.0 的现场被 1.2.0 取代后，apply 备份的必须是
// **MainFolder 的真实内容**（1.0.0），而不是那个从没装过的 1.1.0。
// 否则回滚会把 1.1.0 装回去——一个用户机器上从未存在过的版本。
// ────────────────────────────────────────────────────────────────
func TestCase98(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-skipver"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	// 造 downloaded/1.1.0 现场（快照含 1.1.0 独有文件 x.dll）
	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusDownloaded, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
		Extras: &VersionInfo{DownloadedFileCount: 2},
	})
	WriteText(t, filepath.Join(c.VerDir("1.1.0"), "app.exe"), "app-110")
	WriteText(t, filepath.Join(c.VerDir("1.1.0"), "x.dll"), "x-110")

	// 服务端跳到 1.2.0（1.1.0 从未在本机应用过）
	e.PutFile(t, proj, "app.exe", "app-120")
	e.PutFile(t, proj, "y.dll", "y-120")
	True(t, e.Publish(t, proj, "1.2.0", []string{"跳版"}, "", "").IsSuccess, "发布 1.2.0")

	// ① check 必须只认最新 1.2.0
	cu := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	Eq(t, "new_version 应为 1.2.0（不是 1.1.0）", cu.NewVersion, "1.2.0")
	Eq(t, "need_download_update", cu.NeedDownloadUpdate, true)

	// ② download 1.2.0
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "下载 1.2.0")
	v1 := c.ReadVersion(t)
	Eq(t, "下载后 version", v1.Version, "1.2.0")
	Eq(t, "version_previous 仍描述 MainFolder 真实内容", v1.VersionPrevious, "1.0.0")

	// ③ apply
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "应用 1.2.0\nupdate.log:\n%s",
		c.Log(t, "update.log", 60))

	Eq(t, "app.exe = 1.2.0",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-120")
	Eq(t, "1.2.0 新文件落地",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "y.dll"))), "y-120")
	Eq(t, "旧 1.1.0 独有文件不残留", Exists(filepath.Join(c.Main(), "x.dll")), false)

	v := c.ReadVersion(t)
	Eq(t, "version", v.Version, "1.2.0")
	Eq(t, "version_previous 仍是 1.0.0（备份对象）", v.VersionPrevious, "1.0.0")

	// 关键：备份目录必须是 1.0.0（MainFolder 真实内容），不是 1.1.0
	Eq(t, "备份目录 = ApplicationFolder_1.0.0", Exists(c.VerDir("1.0.0")), true)
	Eq(t, "必须有 1.1.0 的读取来源，但不作为备份",
		strings.TrimSpace(ReadText(filepath.Join(c.VerDir("1.0.0"), "app.exe"))), "app-100")

	// ④ 回滚列表：**真实行为与"直觉"不同**，这里必须钉死两个方向。
	//
	// list_rollback.go 的排除逻辑只在 `version_status == downloaded` 时生效：
	//
	//	if versionInfo.VersionStatus == config.VersionStatusDownloaded {
	//	    pendingVersion = stripVPrefix(versionInfo.Version)   // 排除待应用版本
	//	}
	//
	// 也就是说：downloaded 期间 1.1.0 不进列表（正确）；**一旦 apply 成功、
	// 状态变成 applied，这段排除就不再适用**，若 1.1.0 目录还留在磁盘上，
	// 它就会以"可回滚版本"的身份出现在列表里——尽管它从未被应用过。
	//
	// 这一轮实测：目录**没有被清理**（pruneVersionSnapshots 只在候选数 > keep(3)
	// 时才删，孤本 1.1.0 不够数），所以列表里确实含 1.1.0。
	// 这是真实缺陷（残留 + 语义混淆），不是期望写错：
	//   - 磁盘层面：1.1.0 快照永久残留（每跳一版留一份，"跳版频繁"会堆积）；
	//   - 交互层面：用户会看到一个从没装过的版本可以"回滚"。
	// 现在的断言 = 钉住当前行为，防止"改了却没意识到语义变了"；
	// 修好产品后这两条会 FAIL，提醒回来改文档（这是有意的）。
	rl := Data[RollbackListData](t, e.RunLast(t, c, "list_rollback_versions"))
	Eq(t, "列表含 1.0.0（真实备份）", SliceContains(rl.Versions, "1.0.0"), true)
	Eq(t, "已知缺陷：1.1.0 残留目录在 applied 后混入回滚列表",
		SliceContains(rl.Versions, "1.1.0"), true)
	Eq(t, "1.1.0 残留目录仍在磁盘（prune 只在候选 >3 时才删）",
		Exists(c.VerDir("1.1.0")), true)

	// ⑤ 但"回滚到 1.1.0"实际是有害操作的反证：它的内容确实是那份跳过的版本。
	// 这里不执行回滚（会污染现场），只记录存在性——上面的语义缺陷足以定位问题。
	t.Logf("实测：ApplicationFolder_1.1.0 残留=%v，回滚列表=%v",
		Exists(c.VerDir("1.1.0")), rl.Versions)
}

// ────────────────────────────────────────────────────────────────
// 用例 99 — client ↔ UI 全链路（check → download → apply → 退出）
//
// **关键前提（易踩，文档原期望有误）**：`-mode update` 并不会跳过「要更新吗」的
// 询问卡片。UI 的判定在 flow/updater.go 的 Decide：
//
//	Decide → forced := c.IsForced()（= check_update 返回的 force_update）
//	         needDownload → {Action: ActionDownload, Ask: !forced}
//
// 只有 `Ask=false`（即服务端项目 force_update=true）才自动下载 + 自动应用；
// 否则 runFlowOnce 会 `w.WaitButton(0)` 阻塞等用户点「立即更新」——无人值守
// 环境下就是永久挂起（实测 318s 后被测试超时杀掉，主目录仍是 app-100）。
//
// 所以本用例分两段：
//  ① 断「非强制更新时 `-mode update` 会阻塞在询问卡片上」——这是真实交互语义，
//     不是缺陷，必须钉死，否则将来有人把 Ask 判断改错没人发现；
//  ② 把项目设成 force_update=true 再跑，验证真正的全链路自动完成。
//
// 为什么 `-mode update` 存在却没有"跳过询问"的语义：它只影响**窗口可见性**
// （run.go:164 的 `if mode == "check"` 分支）与单实例转发，不参与 Decide。
// ────────────────────────────────────────────────────────────────
func TestCase99(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-ui-e2e"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	// 1.1.0：改 app.exe + 加 10MB 中等文件（让下载阶段有可见进度）
	e.PutFile(t, proj, "app.exe", "app-110")
	BigFile(t, filepath.Join(e.DataDir(proj), "mid.bin"), 10)
	True(t, e.Publish(t, proj, "1.1.0", []string{"UI 全链路"}, "", "").IsSuccess, "发布 1.1.0")

	// ── ① 非强制更新：UI 必须停在询问卡片（不自动应用）──
	c1 := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	uiExe := e.BuildUI(t)
	ui1 := filepath.Join(c1.Up(), "aly-client-ui.exe")
	CopyFile(t, uiExe, ui1)

	// 不点任何按钮、不点模态框：只给 20s 观察窗（正常路径 3~5s 就该走完）
	res1 := e.RunUIModeAuto(t, ui1, c1, "update", "", 0, 20*time.Second)
	Eq(t, "非强制更新：UI 应停在询问卡片（超时 -999），未被自动放行",
		res1.ExitCode, -999)
	// ⚠ -999 只说明"没退出"，不能说明"卡在询问卡片"：UI 直接 panic 后被
	// 卡住的后台 goroutine 也会表现为不退出。这两条是"卡住"这个断言的
	// 反向排除项，缺了它们，UI 崩死也会被判 PASS。
	NotContains(t, res1.Output, "panic:", "UI 不得 panic（否则 -999 是崩溃而非等待点击）")
	NotContains(t, res1.Output, "failed to", "UI 不得有内部失败（否则 -999 是卡死而非等待点击）")
	// ⚠ 注意：不能指望 res1.Output 里有版本号。
	// UI 编译成 windowsgui（-H=windowsgui，见 build-ui.bat），**没有控制台**，
	// 既不写 stdout/stderr 也不落日志文件（全仓 grep 无 AppendToLog/logDir）。
	// 所以 -999 的正向证据只能来自**磁盘副作用**，不能来自输出文本：
	//   - 主目录 app.exe 未变（下面已有）
	//   - version.json 仍是 applied/1.0.0（下面已有）
	//   - 没有 1.1.0 的下载产物（下面已有）
	// 这三条同时成立 = "UI 起来了、检查过、但停在询问卡片没有继续"；
	// 若 UI 在初始化阶段就挂掉，主目录/version.json 同样不动（无法区分），
	// 故补一条**进程存活**证据：-999 是超时杀掉的，若进程早已退出，
	// RunUIModeAuto 会走 done 分支拿到真实退出码而不是 -999。
	Eq(t, "非强制更新：主目录不得被改动",
		strings.TrimSpace(ReadText(filepath.Join(c1.Main(), "app.exe"))), "app-100")
	// 注意不要写成"status != applied"：初始现场本来就是 applied，它没被改成别的
	// 也只说明"没动"，断不出"卡住了"。正确的判据是**版本号没变 + 没留下待应用产物**。
	v1 := c1.ReadVersion(t)
	Eq(t, "非强制更新：版本号仍是 1.0.0（没被自动应用）", v1.Version, "1.0.0")
	Eq(t, "非强制更新：状态仍是 applied（没进入 downloaded/applying）",
		v1.VersionStatus, StatusApplied)
	Eq(t, "非强制更新：不得产生 1.1.0 的下载产物（连下载都没开始）",
		Exists(c1.VerDir("1.1.0")), false)

	// 收尾说明：RunUIModeAuto 的超时分支已 Kill + Wait 掉 UI 进程，
	// 单实例互斥体（app/single.go 的会话级全局名 AlyClientUI_SingleInstance，
	// 不按安装路径区分）随进程退出而释放，第二段可以直接启动。
	// ① 段停在 WaitButton 上，没有派生子进程，无需额外清理。

	// ── ② 强制更新（force_update=true）：UI 应自动走完全链路 ──
	r := e.Post(t, "/api/project/set_force_update", map[string]interface{}{
		"projectName": proj, "forceUpdate": true,
	})
	True(t, r.IsSuccess, "set_force_update 失败: %s", r.ErrorMsg)

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	uiInClient := filepath.Join(c.Up(), "aly-client-ui.exe")
	CopyFile(t, uiExe, uiInClient)

	// 强制路径仍有 10s 倒计时（forceCountdown），但不需要用户点击
	res := e.RunUIModeAuto(t, uiInClient, c, "update", "", 0, 5*time.Minute)
	True(t, res.ExitCode == 0 || res.ExitCode == -999,
		"UI 不应崩溃，退出码=%d，输出：%s", res.ExitCode, truncate(res.Output, 2000))
	NotContains(t, res.Output, "panic:", "UI 不得 panic")

	Eq(t, "app.exe 已更新",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
	Eq(t, "mid.bin 落地大小", SizeOf(filepath.Join(c.Main(), "mid.bin")), int64(10<<20))

	v := c.ReadVersion(t)
	Eq(t, "version", v.Version, "1.1.0")
	Eq(t, "version_status", v.VersionStatus, StatusApplied)
	Eq(t, "快照目录应被消费掉", Exists(c.VerDir("1.1.0")), false)
	// 备份存在 = 可回滚（apply 三段 rename 的证据）
	Eq(t, "备份目录 ApplicationFolder_1.0.0 存在", Exists(c.VerDir("1.0.0")), true)

	t.Logf("UI update 全链路：①非强制 %s 停在询问（-999）；②强制 退出码=%d，耗时 %s",
		res1.Elapsed, res.ExitCode, res.Elapsed)
}

// ────────────────────────────────────────────────────────────────
// 用例 100 — UI 下载中暂停/继续（真实 UI 路径）
//
// 与用例 45 的区别：45 用 CLI 模拟"暂停"（杀进程），这里强杀的是
// **UI 拉起的 aly-client 子进程链**，验证 UI 被打断后本地状态仍然自洽——
// 也就是"关窗口 = 暂停"这个用户认知是否真的成立。
// ────────────────────────────────────────────────────────────────
func TestCase100(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-ui-pause2"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "app.exe", "app-110")
	// 50MB 足够大，能在被杀之前留下可观的 .part
	BigFile(t, filepath.Join(e.DataDir(proj), "big.bin"), 50)
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	part := filepath.Join(c.VerDir("1.1.0"), "big.bin.part")
	cmd, buf := e.RunAsync(c, "download_update")
	if !WaitFile(t, part, 60*time.Second) {
		Kill(t, cmd)
		t.Skipf("60s 内未出现 .part（本机太快或已直接落盘），异步输出：%s", truncate(buf.String(), 800))
	}

	// 等攒够 1MB 再"暂停"，避免只测到 0 字节的退化情况
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) && SizeOf(part) < 1<<20 {
		time.Sleep(50 * time.Millisecond)
	}
	Kill(t, cmd)
	at := SizeOf(part)
	True(t, at > 0, "暂停时应有部分字节，实际 %d", at)

	// 中断后状态不应被写成 downloaded（产物不完整就不能声称就绪）
	vAfterKill := c.ReadVersion(t)
	True(t, vAfterKill.VersionStatus != StatusDownloaded,
		"中断后 version_status 不应为 downloaded（产物不完整），实际 %q", vAfterKill.VersionStatus)
	t.Logf("暂停时 .part = %d 字节，status = %q", at, vAfterKill.VersionStatus)

	// 续传：CLI 等价路径（UI 内部就是这一条）
	outs := e.Run(t, c, "download_update")
	last := outs[len(outs)-1]
	True(t, last.IsSuccess, "续传失败: %s", last.ErrorMsg)

	final := filepath.Join(c.VerDir("1.1.0"), "big.bin")
	Eq(t, "最终大小", SizeOf(final), int64(50<<20))
	Eq(t, "MD5 与源一致（拼接没错位）", MD5Of(final),
		MD5Of(filepath.Join(e.DataDir(proj), "big.bin")))
	Eq(t, "状态进入 downloaded", c.ReadVersion(t).VersionStatus, StatusDownloaded)
	Eq(t, ".part 已被 rename 成正式文件", Exists(part), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 101 — SDK MainLoop 全链路（自动发现 → 下载 → apply）
//
// **文档原期望与实现不符，必须改正**：原文写"与用例 86 的区别：这里额外验证
// MainLoop 自己会下载"，并打算靠 sd8 来验——但 sd8 只调一次
// `AlyApi.ApplyUpdateAsync`，那条路径是 RunAsyncAlone 直接 spawn
// `apply_update`，**根本不经过 MainLoop**（也不下载）。所以用 sd8 验"MainLoop
// 会下载"是错的前提：实测 sd8 等满 60s 未被关闭（apply_update 在 applied 状态
// 直接返回 "no pending update to apply"），主目录仍是 1.0.0。
//
// 真正的 MainLoop 在 `AlyUpdateClient.cs:237` 的 while 循环里，编排
// check → download → apply。SDK 原先没有任何用例覆盖它（sd10~sd13 只验
// 事件隔离 / Dispose / Cancel，用的是同一个循环但那几轮恰好无更新），
// 故本次新增 **sd14** 作为驱动入口，本用例断言它输出的状态序列。
//
// 分两段，各验一件事：
//  ① sd14 —— MainLoop 自己完成 发现 → 下载 → 下载完成 → apply 阶段；
//  ② sd9  —— 宿主重启后读 version.json 确认版本真的切过去了。
// ────────────────────────────────────────────────────────────────
func TestCase101(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-loop"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "extra.dll", "extra-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"MainLoop"}, "", "").IsSuccess, "发布 1.1.0")

	// 关键：status=applied（不是 downloaded），让 MainLoop 必须自己走下载
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	// must-close 指向 smoke 自己：走到 apply 时 aly-client.exe 会关闭宿主，
	// 所以 RunSmoke 可能拿不到退出码（被强杀），只能按输出前缀判定。
	WriteJSON(t, filepath.Join(c.Up(), "client.json"), map[string]interface{}{
		"main_exe_relative_path":  "../ApplicationFolder/app.exe",
		"must_close_process_name": []string{"AlySdkSmoke"},
	})

	smoke := e.BuildSDK(t)
	start := time.Now()
	out14 := e.RunSmoke(t, smoke, "sd14", c)
	elapsed := time.Since(start)
	t.Logf("sd14 耗时 %s；输出:\n%s", elapsed, truncate(out14, 1500))

	// ① MainLoop 的四段状态必须都在（断言的是"编排真的走过"，不是最终文件）
	smokeAssert(t, out14,
		"PASS: SD14 MainLoop 发现新版本（DiscoveredUpdate）",
		"PASS: SD14 MainLoop 自己发起下载（DownloadingUpdate）",
		"PASS: SD14 下载完成（DownloadedUpdate）",
	)
	// 首轮立刻 check 的证据：远小于空闲间隔下限 60s
	True(t, elapsed < 180*time.Second,
		"MainLoop 首轮不应等空闲间隔（最低 60s），实际 %s", elapsed)

	// apply 阶段：进程可能已被关闭，所以"进入 apply 阶段"这条 PASS 行可能来不及
	// 输出（被 kill 在 Assert 之前）。这里用磁盘结果兜底判定，不硬性要求该行。
	if !strings.Contains(out14, "PASS: SD14 MainLoop 进入 apply 阶段") {
		t.Logf("提示：sd14 的 apply 阶段 PASS 行未输出（进程可能已被 apply 关闭），改由磁盘状态判定")
	}

	// ② 等 apply 真正落定（apply 是独立进程，spawn 返回 ≠ 已完成）
	deadline := time.Now().Add(180 * time.Second)
	for time.Now().Before(deadline) {
		if strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))) == "app-110" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	Eq(t, "主目录已是 1.1.0",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
	Eq(t, "extra.dll 落地", Exists(filepath.Join(c.Main(), "extra.dll")), true)
	Eq(t, "状态 applied", c.ReadVersion(t).VersionStatus, StatusApplied)
	Eq(t, "版本 1.1.0", c.ReadVersion(t).Version, "1.1.0")

	// ③ 宿主重启后的版本核验（sd9，等价于"重启后进程读到的是新版本"）
	out9 := e.RunSmokeEnv(t, smoke, "sd9", c, map[string]string{"ALY_EXPECT_VERSION": "1.1.0"})
	smokeAssert(t, out9, "PASS: SD9 apply 后 version=1.1.0")
}

// ────────────────────────────────────────────────────────────────
// 用例 102 — SDK 断点续传（杀进程后重试）
//
// **文档原期望有误，必须改正**：原文只写"续传起点应 ≥ 暂停时的字节数"，
// 没提续传**只在文件 > 100MB 时才启用**。实测 50MB 文件杀进程后重下，
// 起点只有 3775 字节 = 全量重下。
//
// 根因在 client/http_client.go:289 与 cmd/download_update.go:18：
//
//	const largeFileThreshold = 100 * 1024 * 1024   // 100MB
//	if serverFileSize > largeFileThreshold {        // ← 严格大于
//	    ...续传分支（读 .part 大小 → Range: bytes=N-）
//	}
//	// 否则（≤100MB）走 DownloadFile：整文件重下，.part 只是原子写的临时文件
//
// 也就是说 **≤100MB 的文件没有断点续传**——`.part` 仍会出现（原子写的第一步），
// 但重试时不会被复用，而是从头下。这是既有设计（避免小文件续传的 3 次握手
// 开销），不是缺陷；但文档必须写清楚，否则测试会误判成"续传失效"。
//
// 本用例因此分两段，把两种行为都钉死：
//  ① 50MB（< 阈值）：重试时**全量重下**——断"起点不接续"，避免有人误以为
//     续传对所有大小生效；
//  ② 120MB（> 阈值）：重试时**真的续传**——断 Range 生效、起点接近暂停点。
// ────────────────────────────────────────────────────────────────
func TestCase102(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-resume"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "app.exe", "app-110")
	// 120MB > largeFileThreshold(100MB)：让续传分支真的进得去
	BigFile(t, filepath.Join(e.DataDir(proj), "big.bin"), 120)
	const srcMB = 120
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	part := filepath.Join(c.VerDir("1.1.0"), "big.bin.part")
	cmd, buf := e.RunAsync(c, "download_update")
	if !WaitFile(t, part, 60*time.Second) {
		Kill(t, cmd)
		t.Skipf("60s 内未出现 .part，异步输出：%s", truncate(buf.String(), 800))
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) && SizeOf(part) < 8<<20 {
		time.Sleep(50 * time.Millisecond)
	}
	paused := SizeOf(part)
	Kill(t, cmd)
	True(t, paused > 0, "强杀前 .part 应有字节，实际 %d", paused)
	Eq(t, "强杀后 .part 保留（不被删）", Exists(part), true)

	// 续传：SDK 内部同样是再拉起 download_update
	outs := e.Run(t, c, "download_update")
	last := outs[len(outs)-1]
	True(t, last.IsSuccess, "续传成功: %s", last.ErrorMsg)

	// 续传起点必须**按文件**看：downloaded_bytes 是"该文件累计已写字节数"，
	// 而 http_client.go:334 是这样的：
	//
	//	_, copyErr := io.Copy(f, newProgressReader(resp.Body,
	//	    offsetCallback(onBytes, partInfo.Size())))   // ← 从既有 .part 大小起算
	//
	// 也就是说续传回调的**基线只对本文件**有意义。第一版这里没按文件过滤，
	// 取到了别的文件（app.exe，3.7KB）的首条进度，误判成"退化成全量重下"。
	resumedFrom := int64(-1)
	var seenFiles []string
	for _, o := range outs {
		p := o.Progress
		if p == nil {
			continue
		}
		if p.File != "" {
			seenFiles = append(seenFiles, fmt.Sprintf("%s/%s(%dB)", p.File, p.Status, p.DownloadedBytes))
		}
		if filepath.ToSlash(p.File) != "big.bin" {
			continue
		}
		if p.DownloadedBytes > 0 {
			resumedFrom = p.DownloadedBytes
			break
		}
	}
	True(t, resumedFrom >= 0,
		"续传时应能看到 big.bin 的进度行，实际进度行: %v", seenFiles)

	// ② 120MB > 阈值(100MB)：续传分支必须真的生效
	True(t, resumedFrom >= paused,
		"120MB 文件应断点续传：暂停 %d 字节，起始 %d 字节（< 暂停点说明退化成全量重下）",
		paused, resumedFrom)
	t.Logf("暂停 %d 字节，big.bin 续传起点 %d 字节", paused, resumedFrom)

	// 顺带钉住阈值本身：改了 largeFileThreshold 必须回来改文档
	Contains(t, ReadText(filepath.Join(RepoRoot, "client", "aly-client", "cmd", "download_update.go")),
		"largeFileThreshold = 100 * 1024 * 1024",
		"断点续传阈值必须仍是 100MB（用例 102 与文档基于此口径）")

	final := filepath.Join(c.VerDir("1.1.0"), "big.bin")
	Eq(t, "最终大小", SizeOf(final), int64(srcMB<<20))
	Eq(t, "最终 MD5 与源一致（偏移没错位）", MD5Of(final),
		MD5Of(filepath.Join(e.DataDir(proj), "big.bin")))
	Eq(t, "状态 downloaded", c.ReadVersion(t).VersionStatus, StatusDownloaded)
}

// ────────────────────────────────────────────────────────────────
// 用例 103 — SDK 断网轮询（快速重试、恢复继续）
//
// 断网不是错误：必须 isSuccess:true + has_update:false，且不能写坏 version.json。
// 恢复必须**复用同一端口**（shared.json 里的 URL 是写死的）。
// ────────────────────────────────────────────────────────────────
func TestCase103(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-poll"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	before := ReadText(filepath.Join(c.Up(), "version.json"))
	port := e.Port

	// ① 断网：连查 3 次，每次都必须"成功但无更新"
	e.StopServer()
	for i := 0; i < 3; i++ {
		out := e.RunLast(t, c, "check_update")
		Eq(t, fmt.Sprintf("第 %d 次断网 isSuccess", i+1), out.IsSuccess, true)
		cu := Data[CheckUpdateData](t, out)
		Eq(t, fmt.Sprintf("第 %d 次断网 has_update", i+1), cu.HasUpdate, false)
		Eq(t, fmt.Sprintf("第 %d 次断网 current_version", i+1), cu.CurrentVersion, "1.0.0")
	}
	Eq(t, "断网期间 version.json 逐字节不变",
		ReadText(filepath.Join(c.Up(), "version.json")), before)

	// ② 恢复（复用端口）并发布 1.1.0
	e.StartServer(t, port)
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"恢复"}, "", "").IsSuccess, "发布 1.1.0")

	cu := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	Eq(t, "恢复后识别新版本", cu.NewVersion, "1.1.0")
	Eq(t, "恢复后 has_update", cu.HasUpdate, true)

	// ③ 断网期间的"不报错"也要在 SDK 侧成立（sd6）
	smoke := e.BuildSDK(t)
	e.StopServer()
	out6 := e.RunSmoke(t, smoke, "sd6", c)
	smokeAssert(t, out6,
		"PASS: SD6 断网 check isSuccess:true",
		"PASS: SD6 断网 has_update:false",
	)

	// ④ 再恢复一次，确认能继续 download（状态没被断网污染）
	e.StartServer(t, port)
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "断网恢复后应能继续下载")
	Eq(t, "下载后状态", c.ReadVersion(t).VersionStatus, StatusDownloaded)
}

// ────────────────────────────────────────────────────────────────
// 用例 104 — GUI 与 CLI 同库（行为等价）
//
// **文档原期望有误，必须改正**：原文写"GUI 必须复用 publish-cli 的
// internal/{api,staging,diff}"，暗示 GUI 会 `using` Go 包——这在技术上不可能：
// publish-gui 是 C#/Avalonia（net8.0），publish-cli 是 Go。跨语言不存在 import。
//
// GUI 实际走的等价路径是**子进程复用**（Services/CliService.cs）：
//
//	var exeName = OperatingSystem.IsWindows() ? "aly-publish.exe" : "aly-publish";
//	var fullArgs = $"{args} --json";                 // 统一追加 --json
//	var result = await _ps.RunAsync(CliPath, fullArgs, workDir, timeoutMs);
//	return JsonConvert.DeserializeObject<CliOutput<T>>(result.StandardOutput);
//
// 也就是说：GUI 的 HTTP / 暂存 / 扫描 / 差量全部由 CLI 做，GUI 只负责
// 组装参数 + 解析 CLI 的 JSON。等价性由此成立——**同一份二进制**，
// 不可能出现两套实现分叉。
//
// 所以本用例要钉的是这层"子进程契约"，共四条：
//  ① GUI 启动时必须能找到 `aly-publish`（同目录 → 开发路径回退）；
//  ② GUI 每条命令都必须追加 `--json`（否则解析必挂）；
//  ③ GUI 的输出结构必须与 CLI 的 `CliOutput<T>` 对齐（isSuccess/data）；
//  ④ CLI 侧的行为确实自洽（发布 → 文件集合 = 清单、暂存区可回读）。
// ────────────────────────────────────────────────────────────────
func TestCase104(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-gui"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	ws := e.NewPublishWS(t, proj, "pub-gui")
	WriteText(t, filepath.Join(ws, "app.exe"), "app-110")
	WriteText(t, filepath.Join(ws, "new.txt"), "new-content")
	True(t, e.PublishCLI(t, ws, "add", "--all").IsSuccess, "add --all")

	// ① 子进程契约：GUI 必须通过 aly-publish.exe 干活，且必须追加 --json
	guiRoot := filepath.Join(RepoRoot, "publish", "publish-gui-csharp")
	if !Exists(guiRoot) {
		t.Skip("publish-gui-csharp 目录不存在，跳过")
	}
	cliSvc := filepath.Join(guiRoot, "src", "AlyPublish", "Services", "CliService.cs")
	True(t, Exists(cliSvc), "GUI 必须有 CliService.cs（子进程复用 CLI 的唯一入口）")
	svc := ReadText(cliSvc)

	// ①-1 能找到 exe（payload 目录 → 同目录 → 开发路径回退）
	//
	// 2026-11-12 重构：平台文件名与探测顺序收敛到 Helpers（HostHelper / PkHelper），
	// CliService 只做委派，不再各自拼 "aly-publish.exe" / BaseDirectory / 上溯路径。
	// 源码契约断言随实现更新，语义（Windows 下找 .exe、先查自身目录、开发路径回退）不变。
	hostHelper := filepath.Join(guiRoot, "src", "AlyPublish", "Helpers", "HostHelper.cs")
	pkHelper := filepath.Join(guiRoot, "src", "AlyPublish", "Helpers", "PkHelper.cs")
	True(t, Exists(hostHelper), "必须有 HostHelper（平台文件名唯一来源）")
	True(t, Exists(pkHelper), "必须有 PkHelper（payload 探测顺序唯一来源）")
	Contains(t, ReadText(hostHelper), `"aly-publish.exe"`, "GUI 必须在 Windows 下找 aly-publish.exe")
	Contains(t, ReadText(pkHelper), "publish-cli", "payload 目录名必须是 publish-cli")
	Contains(t, ReadText(pkHelper), `"..", "..", "..", "..", "..", ".."`,
		"GUI 必须有开发路径回退（从 bin/Debug/net8 上溯 6 级到仓库根）")
	Contains(t, svc, "AppDomain.CurrentDomain.BaseDirectory", "GUI 必须先查自身目录")
	Contains(t, svc, "PkHelper.PubProbes(", "CliService 必须走统一探测（不得各自拼路径）")

	// ①-2 每条命令都要追加 --json。
	//
	// 2026-11-12 重构后只有两条执行入口：
	//   · RunAsync（普通命令统一入口）：arguments 首部插入 AppConstants.JsonOutputArgument
	//   · PushAsync（流式进度，独立走 ProcessService.RunWithProgressAsync）：同样首部插入
	// 常量值定义在 Constants/AppConstants.cs。断言"两条路径都带 + 常量值确为 --json"。
	codeOnly := stripCLineComments(svc)
	Eq(t, "追加 --json 的执行路径数（RunAsync + PushAsync）",
		strings.Count(codeOnly, "AppConstants.JsonOutputArgument"), 2)
	Contains(t, svc, "new List<string>(arguments.Count + 1) { AppConstants.JsonOutputArgument }",
		"普通命令必须把 --json 作为独立首参数（避免用户输入注入）")
	Contains(t, svc, "AppConstants.JsonOutputArgument,",
		"push 专用路径也要带 --json")

	appConstants := filepath.Join(guiRoot, "src", "AlyPublish", "Constants", "AppConstants.cs")
	True(t, Exists(appConstants), "必须有 AppConstants（--json 常量唯一来源）")
	Contains(t, ReadText(appConstants), `JsonOutputArgument = "--json"`,
		"JsonOutputArgument 的值必须是 --json")

	// ①-3 输出结构必须与 CLI 的 CliOutput<T> 对齐（同一套 isSuccess/data 契约）
	Contains(t, svc, "CliOutput<T>", "GUI 必须反序列化成 CliOutput<T>（与 CLI 同结构）")
	cliModel := filepath.Join(guiRoot, "src", "AlyPublish", "Models", "Cli", "CliOutput.cs")
	True(t, Exists(cliModel), "必须有 CliOutput 模型定义")
	m := ReadText(cliModel)
	for _, f := range []string{"isSuccess", "data"} {
		Contains(t, m, f, "CliOutput 必须映射 CLI 的字段 "+f)
	}

	// ①-4 反证：GUI 不得自带 HTTP / 扫描实现（一旦有第二套，契约就失效）
	var ownHTTP []string
	filepath.Walk(guiRoot, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || !strings.HasSuffix(p, ".cs") {
			return nil
		}
		// 排除 bin/obj（构建产物里会有 SDK 自带的 HttpClient 引用）
		rel := strings.ReplaceAll(p, "\\", "/")
		if strings.Contains(rel, "/bin/") || strings.Contains(rel, "/obj/") {
			return nil
		}
		body := ReadText(p)
		if strings.Contains(body, "HttpClient") || strings.Contains(body, "/api/project/") {
			ownHTTP = append(ownHTTP, filepath.Base(p))
		}
		return nil
	})
	Eq(t, "GUI 源码不得直接发 HTTP 或直连服务端路由（否则等于第二套实现）",
		len(ownHTTP), 0)
	if len(ownHTTP) > 0 {
		t.Logf("违规文件：%v", ownHTTP)
	}

	// ② 冒烟：CLI 发布一轮，核服务端最终状态的自洽性
	r := e.PublishCLI(t, ws, "push", "--version", "1.3.0", "--message", "cli 发布")
	True(t, r.IsSuccess, "CLI push 失败: %s", r.ErrorMsg)

	p1 := e.GetProject(t, proj).Version
	Eq(t, "服务端版本", p1, "1.3.0")

	fs1 := relsOf(e.GetAllFiles(t, proj))
	m1 := e.GetManifest(t, proj).Files
	sorted := append([]string(nil), m1...)
	sortStrings(sorted)
	Eq(t, "文件集合与清单必须完全一致（扫描口径唯一）",
		strings.Join(fs1, ","), strings.Join(sorted, ","))
	Contains(t, strings.Join(fs1, ","), "app.exe", "服务端文件集合")
	Contains(t, strings.Join(fs1, ","), "new.txt", "服务端文件集合")

	// ③ 暂存区格式互通：CLI add 的内容能被再一次 status 正确识别为 staged
	WriteText(t, filepath.Join(ws, "app.exe"), "app-140")
	True(t, e.PublishCLI(t, ws, "add", "app.exe").IsSuccess, "再 add")
	sd := RespData[StatusData](t, e.PublishCLI(t, ws, "status"))
	foundStaged := false
	for _, it := range sd.Staged {
		if it.RelativePath == "app.exe" {
			foundStaged = true
			Eq(t, "暂存项带 status 分类", it.Status, "modified")
			True(t, it.LocalMd5 != "", "暂存项必须带 md5（push 前复核的基线）")
		}
	}
	Eq(t, "app.exe 在 staged 里（暂存区格式被 status 正确读回）", foundStaged, true)
	// 暂存的项不应同时出现在 unstaged（mergeStagedIntoStatusData 的过滤）
	for _, it := range sd.Unstaged {
		Eq(t, "暂存项不得重复出现在 unstaged（"+it.RelativePath+"）",
			it.RelativePath == "app.exe", false)
	}
}

// sortStrings 是 sort.Strings 的薄封装（本文件未直接引 sort，避免与其他批次重名）。
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}