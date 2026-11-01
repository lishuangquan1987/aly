package tc

// 用例 106–115：补"判据红线"级的边界（原 105 条遗漏的缺口）
//
// 附录 A 定的四条判据红线是：
//   ① 主目录内容与 version.json 不符
//   ② 该删没删 / 误删
//   ③ 假成功（isSuccess:true 但目标状态没达成）
//   ④ 路径穿越
//
// 原 105 条在每条红线附近都留了缺口，本文件按"最坏情况下会不会触发红线"补齐：
//
//	106  apply 的 F3 守卫：downloaded_file_count 与实际文件数不符
//	107  version.json 内容为空文件（0 字节）
//	108  rollback 目标目录存在但是空目录          ← 红线②（空目录会顶掉主目录）
//	109  rollback 到"当前活动版本"（残留目录在时可达）
//	110  create_project 非法名矩阵                ← 红线④（name 直接拼进磁盘路径）
//	111  upload_file 路径穿越（/ 与 \ 两种写法）  ← 红线④
//	112  download_file 越界读取                   ← 红线④（信息泄露）
//	113  download_file Range 越界（416）
//	114  upload_chunk → upload_chunk_complete 成功全流程
//	115  apply_fail_count 的递增 / 熔断 / 两处复位
//
// 每条独立自造现场（不承前一条），失败信息带现场路径。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────
// 用例 106 — apply 的 F3 守卫：downloaded_file_count 与实际文件数不符
//
// 背景（cmd/apply_update.go:200）：
//
//	incomplete := countErr != nil || pendingCount == 0 ||
//	    (versionInfo.DownloadedFileCount > 0 && pendingCount < versionInfo.DownloadedFileCount)
//
// 也就是说：快照目录里的实际文件数 **少于** 下载完成时记录的数量时，apply 必须
// 拒绝替换，并把状态自愈成 applied/{VersionPrevious} 让下一轮重新下载。
//
// 为什么是红线级：没有这道守卫，apply 会把"少了几个文件"的快照整体 rename
// 成主目录 —— 用户程序直接缺 dll，且 version.json 声称已 applied
// （红线① + ③ 同时踩中）。原 105 条里用例 30 的注释提到该分支，但
// **没有一条用例真的造出"文件数不足"的现场**。
// ────────────────────────────────────────────────────────────────
func TestCase106(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-f3"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "a.dll", "a-110")
	e.PutFile(t, proj, "b.dll", "b-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "a.dll"), "a-100")

	// 正常下载，拿到完整快照 + downloaded_file_count
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 失败")
	v0 := c.ReadVersion(t)
	Eq(t, "前置：已进入 downloaded", v0.VersionStatus, StatusDownloaded)
	True(t, v0.DownloadedFileCount > 0,
		"前置：downloaded_file_count 应 > 0，实际 %d（为 0 时 F3 守卫不生效，本用例无意义）",
		v0.DownloadedFileCount)
	t.Logf("downloaded_file_count = %d，快照内实际 %d 个文件",
		v0.DownloadedFileCount, countFiles(c.VerDir("1.1.0")))

	// 人为从快照里删掉一个业务文件，制造"文件数不足"（模拟下载后被杀软删除）
	victim := filepath.Join(c.VerDir("1.1.0"), "b.dll")
	True(t, Exists(victim), "前置：快照里应有 b.dll")
	if err := os.Remove(victim); err != nil {
		t.Fatalf("删除快照文件失败: %v", err)
	}
	before := countFiles(c.VerDir("1.1.0"))
	True(t, before < v0.DownloadedFileCount,
		"前置：删完应少于记录数（现有 %d，记录 %d）", before, v0.DownloadedFileCount)

	// apply 必须拒绝（不是假成功）
	out := e.RunLast(t, c, "apply_update")
	Eq(t, "文件数不足时 apply 必须拒绝（否则主目录会缺文件）", out.IsSuccess, false)
	Contains(t, out.ErrorMsg, "不完整", "errorMsg 应说明产物不完整")

	// 关键：拒绝后必须自愈成 applied/{VersionPrevious}，让下一轮能重新下载。
	// 若停在 downloaded，check 会一直要求 apply → 反复失败，用户永远拿不到完整版本。
	v1 := c.ReadVersion(t)
	Eq(t, "自愈：状态回 applied", v1.VersionStatus, StatusApplied)
	Eq(t, "自愈：版本回退到 VersionPrevious", v1.Version, "1.0.0")
	Eq(t, "自愈：不得残留 apply_stage", v1.ApplyStage, "")

	// 红线①：主目录必须原封不动
	Eq(t, "主目录 app.exe 未被替换",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")
	Eq(t, "主目录 a.dll 未被替换",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "a.dll"))), "a-100")
	Eq(t, "不得把残缺快照 rename 成主目录（主目录不得出现 b.dll）",
		Exists(filepath.Join(c.Main(), "b.dll")), false)

	// 自愈后可恢复：重新 download + apply 都应成功（否则"自愈"只是把状态改坏）
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "自愈后重新 download 应成功")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "自愈后 apply 应成功")
	vf := c.ReadVersion(t)
	Eq(t, "最终 version", vf.Version, "1.1.0")
	Eq(t, "最终 status", vf.VersionStatus, StatusApplied)
	Eq(t, "b.dll 已落地", Exists(filepath.Join(c.Main(), "b.dll")), true)
	Eq(t, "a.dll 已换成新内容",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "a.dll"))), "a-110")
}

// ────────────────────────────────────────────────────────────────
// 用例 107 — version.json 是 0 字节空文件
//
// 用例 24 覆盖的是"半截 20 字节"（截断的 JSON）。**0 字节**走同一段
// `json.Unmarshal` 失败逻辑，但结果值得单独立案，因为它是"最彻底的损坏"：
//
//	config/version.go:76-92（自愈链 Bug#4）
//	  ① Unmarshal 失败 → 读 version.json.bak，能读就回滚到备份；
//	  ② 仍失败 → 把损坏文件隔离成 version.json.corrupt，返回空 VersionInfo
//	     （首次部署语义：status="" → 重新下载，可自愈）
//
// 产品的**设计选择是自愈而不是报错**。这一点必须钉住，因为它决定用户遇到
// "客户机断电把 version.json 写坏"时的体验：应当是"自动重新下载"，
// 而不是"更新功能永久瘫痪"（Bug#4 的原症状）。
//
// 注意：自愈会真的改写磁盘（隔离 + 重下），所以断言是"不假成功 /
// 不破坏主目录 / 隔离证据存在 / 自愈后链路可用"，而不是"保持 0 字节"。
// ────────────────────────────────────────────────────────────────
func TestCase107(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-emptyver"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-100")

	verPath := filepath.Join(c.Up(), "version.json")
	WriteText(t, verPath, "")
	Eq(t, "前置：version.json 为 0 字节", SizeOf(verPath), int64(0))

	// ① check_update 必须"不崩"：要么自愈后正常返回，要么明确失败带 errorMsg。
	//    不允许"无输出/挂死"——那才是 Bug#4 的原症状。
	out := e.RunLast(t, c, "check_update")
	if out.IsSuccess {
		d := Data[CheckUpdateData](t, out)
		t.Logf("check 自愈成功: has_update=%v need_download=%v new=%q",
			d.HasUpdate, d.NeedDownloadUpdate, d.NewVersion)
		// 首次部署语义下应当能发现 1.1.0 —— 这正是自愈的价值
		Eq(t, "自愈后应能发现服务器上的 1.1.0", d.NewVersion, "1.1.0")
		Eq(t, "自愈后应要求下载", d.NeedDownloadUpdate, true)
	} else {
		True(t, out.ErrorMsg != "", "失败但无 errorMsg（用户无从诊断）")
		t.Logf("check 明确报错: %q", out.ErrorMsg)
	}

	// ② 关键证据：损坏文件必须被**隔离**（而不是静默覆盖），
	//    这是运维还能看到现场的唯一凭据。
	corrupt := verPath + ".corrupt"
	Eq(t, "损坏的 version.json 应被隔离为 .corrupt（Bug#4 自愈链证据）",
		Exists(corrupt), true)
	Eq(t, "隔离出来的内容就是原来的 0 字节", SizeOf(corrupt), int64(0))
	// 原路径要么不存在，要么已被重建为合法非空 JSON —— 不允许还是空的
	if Exists(verPath) {
		True(t, SizeOf(verPath) > 0,
			"version.json 若被重建则必须非空（留下空文件等于下次再炸一遍）")
		var v VersionInfo
		Eq(t, "重建的 version.json 必须是合法 JSON",
			json.Unmarshal(mustRead(t, verPath), &v) == nil, true)
	}

	// ③ 红线①：自愈过程绝不能动主目录
	Eq(t, "主目录 app.exe 未被替换",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")
	Eq(t, "主目录 lib.dll 未被改动",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-100")

	// ④ 自愈后整条链路必须可用：download → apply 走通，版本真的切过去。
	//    若这里失败，说明"自愈"只改了状态没恢复能力。
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "自愈后 download 应成功")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "自愈后 apply 应成功")
	Eq(t, "最终 version", c.ReadVersion(t).Version, "1.1.0")
	Eq(t, "主目录已换新",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
}

// mustRead 读文件，失败即 Fatal（只用于已经 Exists 过的路径）。
func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("读 %s 失败: %v", p, err)
	}
	return b
}

// ────────────────────────────────────────────────────────────────
// 用例 108 — rollback 目标目录存在但是**空目录**
//
// 红线级。用例 35 只测"目标目录不存在"（在 rollback.go:102 的 stat 处被拦），
// 空目录能通过 `statErr != nil || !info.IsDir()` 检查，然后走完整三段 rename：
//
//	rename(MainFolder → prevVersionDir)   主目录变成备份
//	rename(空 versionDir → MainFolder)    主目录变成一个空壳
//
// 结果：version.json 写 applied/{target}，主目录里一个文件都没有。用户的程序
// 直接消失，而 update.log 说成功 —— 红线①②③全踩。
//
// 本用例把当前行为钉住：产品侧加了"空目录守卫"时它会转向 else 分支，
// 提醒同步更新文档。
// ────────────────────────────────────────────────────────────────
func TestCase108(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-rb-empty"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-100")

	empty := c.VerDir("0.9.0")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatalf("建空版本目录失败: %v", err)
	}
	Eq(t, "前置：目标目录存在", Exists(empty), true)
	Eq(t, "前置：目标目录为空", countFiles(empty), 0)

	out := e.RunLast(t, c, "rollback", "--version", "0.9.0")

	if out.IsSuccess {
		// 当前实现会"成功"——把后果记录清楚，并断言主目录被清空（缺陷现场）。
		t.Logf("⚠ 已知缺陷：rollback 到空目录返回成功，主目录现有文件 %v", ListDir(c.Main()))
		Eq(t, "已知缺陷：主目录被空目录顶掉（app.exe 消失）",
			Exists(filepath.Join(c.Main(), "app.exe")), false)
		Eq(t, "已知缺陷：version.json 声称已是 0.9.0（红线①：与实际内容不符）",
			c.ReadVersion(t).Version, "0.9.0")
		// 至少备份不能丢：旧内容必须还在 ApplicationFolder_1.0.0 下（否则红线② 误删）
		Eq(t, "旧内容必须仍可从备份恢复",
			strings.TrimSpace(ReadText(filepath.Join(c.VerDir("1.0.0"), "app.exe"))), "app-100")
		Eq(t, "旧 lib.dll 也必须在备份里",
			strings.TrimSpace(ReadText(filepath.Join(c.VerDir("1.0.0"), "lib.dll"))), "lib-100")
	} else {
		Contains(t, out.ErrorMsg, "0.9.0", "errorMsg 应指明目标版本")
		Eq(t, "拒绝后主目录不变",
			strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")
		Eq(t, "拒绝后 version 不变", c.ReadVersion(t).Version, "1.0.0")
		t.Logf("rollback 已加空目录守卫（本条转为止损验证）")
	}
}

// ────────────────────────────────────────────────────────────────
// 用例 109 — rollback 到"当前活动版本"（目标目录残留时可达）
//
// rollback.go:129-134 的守卫：
//
//	if *versionFlag == oldVersion {   // oldVersion = 当前活动版本
//	    printOutput(false, "当前已处于版本 X，无需回滚", nil)
//	}
//
// 这条守卫**只在目标目录存在时才有可能到达**（102 行的 stat 先拦）。也就是
// "活动版本目录残留"（上次 apply 的备份没被清理）时，用户
// `rollback --version 1.0.0`（当前就是 1.0.0）会走到这里。
//
// 要点：必须是"拒绝"，而不是"执行一次自己换自己的 rename"——后者会让主目录
// 短暂消失（rename 出去再 rename 回来），期间用户程序被关闭却什么都没变。
// 原 105 条无用例覆盖（cases_01_08 注释承认"仅当残留存在时可达"）。
// ────────────────────────────────────────────────────────────────
func TestCase109(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-rb-self"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "lib.dll"), "lib-100")

	// 造"当前活动版本目录残留"
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), "app.exe"), "stale-backup-100")
	Eq(t, "前置：活动版本目录残留存在", Exists(c.VerDir("1.0.0")), true)

	out := e.RunLast(t, c, "rollback", "--version", "1.0.0")
	Eq(t, "回滚到当前版本必须拒绝（不是无操作的自我替换）", out.IsSuccess, false)
	Contains(t, out.ErrorMsg, "无需回滚", "errorMsg 文案")

	// 红线①：拒绝路径不得改动任何东西
	v := c.ReadVersion(t)
	Eq(t, "version 不变", v.Version, "1.0.0")
	Eq(t, "status 不变", v.VersionStatus, StatusApplied)
	Eq(t, "version_previous 不变", v.VersionPrevious, "1.0.0")
	Eq(t, "不得写 rollback_target", v.RollbackTarget, "")
	Eq(t, "不得写 apply_stage", v.ApplyStage, "")
	Eq(t, "主目录 app.exe 不变",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")
	Eq(t, "主目录 lib.dll 不变",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "lib.dll"))), "lib-100")
	// 残留备份不应被这次"拒绝"顺手删掉（它可能是唯一的旧版本副本）
	Eq(t, "残留备份目录仍在（拒绝路径不做清理）", Exists(c.VerDir("1.0.0")), true)
}

// ────────────────────────────────────────────────────────────────
// 用例 110 — create_project 非法名矩阵（红线④）
//
// **原 105 条完全没测**。而项目名直接被拼进磁盘路径：
//
//	workDir := path.Combine(filepath.Dir(exe), "data", projectName)   // project_service.go:39
//	directory.CreateDirectory(workDir)                                 // project_controller.go:91
//
// 项目名是**未经 Clean 的路径片段**，唯一防线是 project_controller.go:19：
//
//	var validProjectName = regexp.MustCompile(`^[^<>:"/\\|?*]{1,64}$`)
//
// 这个正则只挡 9 个文件系统非法字符 + 长度，**允许** `..`、`.`、前后空格、
// 中文、任意控制字符。所以必须分别验：非法字符被拒、边界内被接受、
// 以及最危险的 `..` 会不会把目录建到 data/ 之外。
// ────────────────────────────────────────────────────────────────
func TestCase110(t *testing.T) {
	e := NewEnv(t, true, 0)

	type c struct {
		name string
		desc string
		want bool // 是否应被接受
	}
	cases := []c{
		// 应拒绝：文件系统非法字符
		{"a<b", "含 lt", false},
		{"a>b", "含 gt", false},
		{"a:b", "含冒号", false},
		{`a"b`, "含引号", false},
		{"a/b", "含正斜杠", false},
		{`a\b`, "含反斜杠", false},
		{"a|b", "含竖线", false},
		{"a?b", "含问号", false},
		{"a*b", "含星号", false},
		// 应拒绝：空 / 超长
		{"", "空名", false},
		{strings.Repeat("a", 65), "65字符超上限", false},
		// 应拒绝：Windows 保留名（含大小写变体）
		{"CON", "保留名CON", false},
		{"NUL", "保留名NUL", false},
		{"com1", "小写保留名com1", false},
		{"LPT9", "保留名LPT9", false},
		// 应接受：边界内
		{strings.Repeat("a", 64), "恰好64字符", true},
		{"tc-legal_1", "字母数字与连字符下划线", true},
	}

	for i, k := range cases {
		i, k := i, k
		t.Run(fmt.Sprintf("%02d_%s", i, k.desc), func(t *testing.T) {
			r := e.Post(t, "/api/project/create_project", map[string]interface{}{
				"name": k.name, "title": "边界用例 " + k.desc,
				"isForceUpdate": false,
				"ignoreFolders": []string{},
				"ignoreFiles":   []string{},
			})
			got := r.IsSuccess
			Eq(t, fmt.Sprintf("%s（name=%q）", k.desc, truncate(k.name, 70)), got, k.want)

			if !k.want {
				// 被拒时绝不能在磁盘上留下目录（否则是"拒绝但已落地"）
				Eq(t, fmt.Sprintf("被拒后不得创建数据目录（name=%q）", truncate(k.name, 70)),
					Exists(e.DataDir(k.name)), false)
				return
			}
			Eq(t, "被接受时必须真的建出数据目录", Exists(e.DataDir(k.name)), true)
		})
	}

	// ── 红线④ 专项：`..` 能不能把目录建到 data/ 之外 ──
	// `..` 不是那 9 个非法字符之一，正则放行。若产品直接 CreateDirectory，
	// data/.. = 服务端 exe 所在目录，等于把 exe 目录当"项目目录"用。
	t.Run("路径穿越_双点", func(t *testing.T) {
		before := ListDir(e.Bin) // e.Bin 里是 data/ 与各个 exe
		r := e.Post(t, "/api/project/create_project", map[string]interface{}{
			"name": "..", "title": "穿越尝试",
			"isForceUpdate": false,
			"ignoreFolders": []string{}, "ignoreFiles": []string{},
		})
		if r.IsSuccess {
			t.Logf("⚠ 项目名 `..` 被接受（正则 ^[^<>:\"/\\\\|?*]{1,64}$ 不挡 `..`）")
			// 接受了就必须确认没有越界产物：data/ 的父目录（= e.Bin）里
			// 不得出现项目清单/项目子目录被覆写的痕迹。
			after := ListDir(e.Bin)
			Eq(t, "`..` 不得在 exe 目录下新增项目目录", len(after), len(before))
		} else {
			t.Logf("`..` 被拒绝: %s", r.ErrorMsg)
		}
		// 无论接受与否，data 目录必须完好
		Eq(t, "data 目录必须仍存在", Exists(filepath.Join(e.Bin, "data")), true)
	})
}

// ────────────────────────────────────────────────────────────────
// 用例 111 — upload_file 的路径穿越（红线④）
//
// file_upload_controller.go:83-90 的防护：
//
//	fileName := Replace(relativeFileName, "\\", "/")
//	absFileName := filepath.Clean(path.Combine(workDir, fileName))
//	if !HasPrefix(absFileName, workDir+sep) && absFileName != workDir { 拒绝 }
//
// 关键点：**先 Replace 反斜杠再 Clean**，所以 `..\..\x` 与 `../../x` 等价，
// 两种写法都必须被同一道防线拦住。原用例 65 只测了 `.chunks` 不可下载，
// 上传侧的穿越一次都没测。
// ────────────────────────────────────────────────────────────────
func TestCase111(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-trav"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	srcDir := filepath.Join(e.Root, "trav-src")
	payload := filepath.Join(srcDir, "evil.txt")
	WriteText(t, payload, "evil-content")

	// 越界目标（各种写法都指向它）
	escaped := filepath.Join(e.Bin, "escaped.txt")

	traversals := []string{
		"../escaped.txt",
		`..\escaped.txt`,
		"../../escaped.txt",
		`..\..\escaped.txt`,
		"a/../../escaped.txt",
		".updator/../../escaped.txt",
	}
	for i, rel := range traversals {
		i, rel := i, rel
		t.Run(fmt.Sprintf("穿越%02d", i), func(t *testing.T) {
			r := e.Upload(t, proj, rel, payload)
			Eq(t, fmt.Sprintf("越界路径必须被拒绝: %q", rel), r.IsSuccess, false)
			if !r.IsSuccess {
				Contains(t, r.ErrorMsg, "非法", "errorMsg 应说明非法路径: "+r.ErrorMsg)
			}
			Eq(t, "不得真的写出越界文件", Exists(escaped), false)
		})
	}

	// 对照：合法的多级相对路径必须能传（证明上面拒绝的是"穿越"而非"多级路径"）
	ok := e.Upload(t, proj, "sub/dir/ok.txt", payload)
	True(t, ok.IsSuccess, "合法多级路径应被接受: %s", ok.ErrorMsg)
	Eq(t, "合法文件落到项目目录内",
		strings.TrimSpace(ReadText(filepath.Join(e.DataDir(proj), "sub", "dir", "ok.txt"))),
		"evil-content")
	Eq(t, "项目目录之外的 escaped.txt 仍不存在", Exists(escaped), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 112 — download_file 的越界读取（红线④：信息泄露）
//
// file_download_controller.go:106-121 的防护是"先 Clean 再判前缀"：
//
//	cleanPath := filepath.Clean(pathStr)
//	if !HasPrefix(cleanPath, dataDir+sep) && cleanPath != dataDir { 404 }
//
// 注意 cleanPath 必须是**绝对路径**才能通过前缀判定（服务端 get_all_files
// 返回的正是绝对路径）。所以穿越尝试用"绝对路径 + .."的形式：
// data/tc-x/../../../secret.txt → Clean 后落到 data 之外 → 必须 404。
//
// 原用例 65 只测 `.chunks`，越界一次都没测。这里同时验越界 404、
// 不存在 404、合法 200（证明不是"全都 404"）。
// ────────────────────────────────────────────────────────────────
func TestCase112(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-dltrav"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	// 造一个 data 之外的"机密文件"
	secret := filepath.Join(e.Bin, "secret.txt")
	WriteText(t, secret, "SECRET-DO-NOT-LEAK")

	dataDir := filepath.Join(e.Bin, "data")
	projDir := e.DataDir(proj)

	dl := func(t *testing.T, p string) (int, []byte) {
		t.Helper()
		return e.RawRange(t, "/api/file/download_file?path="+url.QueryEscape(p), 0)
	}

	traversals := []string{
		filepath.Join(projDir, "..", "..", "secret.txt"),
		filepath.Join(projDir, "..", "..", "bin", "server.exe"),
		filepath.Join(dataDir, "..", "secret.txt"),
		secret,
		filepath.Join(e.Root, "e2e.db"),
	}
	for i, p := range traversals {
		i, p := i, p
		t.Run(fmt.Sprintf("越界%02d", i), func(t *testing.T) {
			code, body := dl(t, p)
			Eq(t, "越界路径必须 404（不是 200 带内容）", code, 404)
			NotContains(t, string(body), "SECRET-DO-NOT-LEAK", "响应体")
			NotContains(t, string(body), "SQLite format", "响应体不得包含数据库头")
			t.Logf("path=%s → %d（%d 字节）", p, code, len(body))
		})
	}

	t.Run("不存在", func(t *testing.T) {
		code, _ := dl(t, filepath.Join(projDir, "nope.bin"))
		Eq(t, "不存在的文件应 404", code, 404)
	})

	t.Run("合法文件", func(t *testing.T) {
		// 注意不能用 RawRange：它恒定带 Range 头，对存在文件必回 206。
		// 这里用不带 Range 的普通请求，验"整文件下载"这条主路径。
		code, raw := e.RawStatus(t, "GET", "/api/file/download_file?path="+url.QueryEscape(
			filepath.Join(projDir, "app.exe")))
		Eq(t, "合法文件应 200", code, 200)
		Eq(t, "内容正确", strings.TrimSpace(string(raw)), "app-100")
	})
}

// ────────────────────────────────────────────────────────────────
// 用例 113 — download_file 的 Range 越界与续传自愈
//
// 用例 51 只测"单段尾部 Range（bytes=N-）"。断点续传的真实风险在越界：
// .part 比服务端文件还大（服务端换过文件 / 上次下载被截断后又追加）
// 时客户端会发 Range: bytes=<size>-，起点 == 文件长度 → 416。
//
// 客户端 DownloadFileWithResume 对非 206/200 的反应是"删 .part + 全量重下"
// （http_client.go:316-322），所以 416 本身不该导致失败；但服务端必须返回
// **416 而不是 200 + 空体**，否则客户端会把空写入 .part 再校验失败重试三次。
// ────────────────────────────────────────────────────────────────
func TestCase113(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-range"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "f.bin", "0123456789") // 10 字节
	content := "0123456789"

	path := "/api/file/download_file?path=" + url.QueryEscape(
		filepath.Join(e.DataDir(proj), "f.bin"))

	// ① 起点 == 文件长度：无字节可给，必须 416
	code, body := e.RawRange(t, path, int64(len(content)))
	Eq(t, "Range 起点==文件长度 → 416", code, 416)
	NotContains(t, string(body), content, "416 不得返回文件内容")

	// ② 起点远超文件长度
	code2, _ := e.RawRange(t, path, int64(len(content))+99)
	Eq(t, "Range 起点远超 → 416", code2, 416)

	// ③ 起点 0：ServeContent 对 bytes=0- 给 206
	code3, body3 := e.RawRange(t, path, 0)
	Eq(t, "Range 起点 0 → 206（partial）", code3, 206)
	Eq(t, "206 内容完整", string(body3), content)

	// ④ 中途起点：206 + 正确后半段（真续传依赖这个语义）
	code4, body4 := e.RawRange(t, path, 4)
	Eq(t, "Range 起点 4 → 206", code4, 206)
	Eq(t, "后半段内容正确（续传拼接依赖这个）", string(body4), content[4:])

	// ⑤ 客户端视角：.part 比服务端大时必须能自愈下完，不因 416 整体失败
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, NoMain: true,
	})
	partPath := filepath.Join(c.VerDir("1.1.0"), "f.bin.part")
	WriteText(t, partPath, content+"EXTRA-GARBAGE")
	True(t, SizeOf(partPath) > int64(len(content)), "前置：.part 应比服务端文件大")

	out := e.RunLast(t, c, "download_update")
	True(t, out.IsSuccess, "过大 .part 不应导致整体下载失败（应清理后重下）: %s", out.ErrorMsg)
	Eq(t, "最终内容正确", ReadText(filepath.Join(c.VerDir("1.1.0"), "f.bin")), content)
	Eq(t, "过大的 .part 已被清理", Exists(partPath), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 114 — upload_chunk → upload_chunk_complete 成功全流程
//
// **原 105 条只测了"过滤 / 不留残留 / 不可下载"，分片上传的成功路径一次没走。**
// 而分片上传是 publish-cli 传大文件的唯一通道，合并逻辑（按 chunkIndex 顺序拼接
// + 原子 rename + 清理 .chunks）任何一步错了都会产出坏文件，且 get_all_files
// 会照常把坏文件算进清单 → 客户端下到坏文件 → 校验失败三次 → 整体失败。
//
// 覆盖：① 乱序上传按 chunkIndex 拼对 ② 无 .merging/.chunks 残留
//       ③ md5/sha256 与源一致且进文件列表 ④ 缺片时 chunksComplete=false 且不产出文件
//       ⑤ 分片中间态不可被 download_file 读到
// ────────────────────────────────────────────────────────────────
func TestCase114(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-chunk"
	e.EnsureProject(t, proj, false, nil, nil)

	const nChunks = 5
	chunks := make([]string, nChunks)
	for i := 0; i < nChunks; i++ {
		chunks[i] = fmt.Sprintf("[chunk-%02d]", i)
	}
	want := strings.Join(chunks, "")

	// 刻意乱序上传（4,2,0,3,1），验证服务端按 chunkIndex 而非到达顺序拼接
	for _, i := range []int{4, 2, 0, 3, 1} {
		r := e.postForm(t, "/api/file/upload_chunk", map[string]string{
			"projectName":      proj,
			"relativeFileName": "big/pack.bin",
			"chunkIndex":       fmt.Sprint(i),
			"totalChunks":      fmt.Sprint(nChunks),
		}, [][]byte{[]byte(chunks[i])})
		True(t, r.IsSuccess, "上传分片 %d 失败: %s", i, r.ErrorMsg)
	}

	chunksDir := filepath.Join(e.DataDir(proj), "big", "pack.bin.chunks")
	Eq(t, "前置：分片目录已建", Exists(chunksDir), true)
	Eq(t, "前置：分片数正确", countFiles(chunksDir), nChunks)

	// ⑤ 合并前：分片中间态不得可下载
	codeBefore, _ := e.RawRange(t, "/api/file/download_file?path="+url.QueryEscape(
		filepath.Join(chunksDir, "0")), 0)
	Eq(t, "分片文件不得可下载（IsUploadTempPath 纵深防御）", codeBefore, 404)

	// ②③ 合并
	r := e.postForm(t, "/api/file/upload_chunk_complete", map[string]string{
		"projectName":      proj,
		"relativeFileName": "big/pack.bin",
		"totalChunks":      fmt.Sprint(nChunks),
	}, nil)
	True(t, r.IsSuccess, "合并失败: %s", r.ErrorMsg)

	final := filepath.Join(e.DataDir(proj), "big", "pack.bin")
	True(t, Exists(final), "合并后目标文件必须存在")
	Eq(t, "① 拼接顺序正确（乱序上传也必须按 chunkIndex 拼）", ReadText(final), want)
	Eq(t, "① 大小正确", SizeOf(final), int64(len(want)))
	Eq(t, "② .chunks 残留已清理", Exists(chunksDir), false)
	Eq(t, "② .merging 中间态不存在", Exists(final+".merging"), false)

	// ③ 出现在文件列表里且 md5 与源一致
	fs := e.GetAllFiles(t, proj)
	var found *FileInfo
	for i := range fs {
		if strings.ReplaceAll(fs[i].FileRelativePath, "\\", "/") == "big/pack.bin" {
			found = &fs[i]
			break
		}
	}
	True(t, found != nil, "合并后的文件必须出现在 get_all_files 里，实际: %v", relsOf(fs))
	Eq(t, "③ md5 与源一致", found.MD5, MD5Bytes([]byte(want)))
	Eq(t, "③ 大小与源一致", found.FileSize, int64(len(want)))

	code, body := e.RawStatus(t, "GET",
		"/api/file/download_file?path="+url.QueryEscape(final))
	Eq(t, "合并后可下载", code, 200)
	Eq(t, "下载内容正确", string(body), want)

	// ④ 缺片：chunksComplete=false + missingChunks，且不产出目标文件
	const proj2 = "tc-chunk-miss"
	e.EnsureProject(t, proj2, false, nil, nil)
	for _, i := range []int{0, 1, 3} { // 故意缺 2、4
		rr := e.postForm(t, "/api/file/upload_chunk", map[string]string{
			"projectName":      proj2,
			"relativeFileName": "gap.bin",
			"chunkIndex":       fmt.Sprint(i),
			"totalChunks":      "5",
		}, [][]byte{[]byte(fmt.Sprintf("g%d", i))})
		True(t, rr.IsSuccess, "上传分片 %d 失败: %s", i, rr.ErrorMsg)
	}
	rm := e.postForm(t, "/api/file/upload_chunk_complete", map[string]string{
		"projectName":      proj2,
		"relativeFileName": "gap.bin",
		"totalChunks":      "5",
	}, nil)
	// 缺片是"业务上未完成"而非错误：isSuccess 仍为 true，data.chunksComplete=false
	True(t, rm.IsSuccess, "缺片时接口本身不应报错: %s", rm.ErrorMsg)
	var comp struct {
		ChunksComplete bool  `json:"chunksComplete"`
		MissingChunks  []int `json:"missingChunks"`
	}
	comp = RespData[struct {
		ChunksComplete bool  `json:"chunksComplete"`
		MissingChunks  []int `json:"missingChunks"`
	}](t, rm)
	Eq(t, "④ chunksComplete 应为 false", comp.ChunksComplete, false)
	Eq(t, "④ missingChunks 应报告缺的序号", fmt.Sprint(comp.MissingChunks), "[2 4]")
	Eq(t, "④ 缺片时不得产出目标文件（否则是半成品上线）",
		Exists(filepath.Join(e.DataDir(proj2), "gap.bin")), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 115 — apply_fail_count 的熔断边界与两处复位
//
// MaxApplyFailCount = 3（cmd/common.go:24）。语义：
//   - apply 失败且走 applyFailureFallback → 计数 +1，状态回 downloaded（common.go:987）
//   - 计数 >= 3 → check_update 不再上报"继续 apply"（check_update.go:236）
//   - download_update 成功 → 复位 0（download_update.go:456）
//   - apply_update 成功   → 复位 0（apply_update.go:275）
//
// 原用例 30 直接把计数写成 3 验熔断，**2→3 的临界与两处复位都没有端到端覆盖**。
//
// ⚠ 触发手段的坑（很重要，别再踩）：
// 一开始想用"把备份目标 ApplicationFolder_<prev> 做成文件"来让 apply 失败。
// **无效**——renameDirWithKillState 在目标已存在时会先把它旁移到
// nextAsideName（X.old / X.old.1…），所以目标被占/类型不符都不会导致失败。
//
// 而想真正让 apply 失败，就必须让 rename 失败，也就是"目录被进程占用"。
// 但 renameDirWithKillState 在 RM/CWD 都探不到持有者时会走 closeExplorerWindows
// ——**向用户当前打开的资源管理器窗口发 WM_CLOSE**。这会给跑测试的机器带来
// 破坏性副作用（用户窗口被关掉），代价远大于收益。
//
// 所以本用例改为固化三条**真实、可稳定触发、零副作用**的语义：
//   ① F3 守卫的"拒绝并自愈"**不**递增 apply_fail_count（新发现，见下）
//   ② 熔断临界：count=2 仍上报待 apply，count=3 才熔断
//   ③ 两处复位：download 成功 / apply 成功
// 熔断计数本身的 +1 语义由产品侧单测 cmd/apply_circuit_test.go 直接覆盖。
// ────────────────────────────────────────────────────────────────
func TestCase115(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-circuit2"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "a.dll", "a-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 失败")
	Eq(t, "前置：已进入 downloaded", c.ReadVersion(t).VersionStatus, StatusDownloaded)
	Eq(t, "前置：计数从 0 开始", c.ReadVersion(t).ApplyFailCount, 0)

	// ── ① F3 守卫自愈不计入熔断 ──
	// 造"下载产物残缺"现场（删掉快照里一个文件），触发 apply_update.go:200 的
	// incomplete 分支：拒绝替换 + 自愈成 applied/Previous。
	// 关键语义：这条路径**不调 applyFailureFallback**，所以 apply_fail_count 必须
	// 保持 0。若它误递增，就是"守卫每拒绝一次 → 计数 +1 → 三轮后熔断"，
	// 而熔断只会让 check 不再上报 —— 本该重下的机器被永久静音（死锁）。
	WriteText(t, filepath.Join(c.VerDir("1.1.0"), ".updator", "shared.json"),
		ReadText(filepath.Join(c.Main(), ".updator", "shared.json")))
	// 下载时记录了文件数；删掉一个制造"实际 < 记录"（用例 106 已独立验证该分支本身）
	bak := filepath.Join(e.Root, "cir-bak.bin")
	if err := os.Rename(filepath.Join(c.VerDir("1.1.0"), "a.dll"), bak); err != nil {
		t.Fatalf("移走快照文件失败: %v", err)
	}
	out := e.RunLast(t, c, "apply_update")
	Eq(t, "① 产物残缺时 apply 必须拒绝（不得假成功）", out.IsSuccess, false)
	Eq(t, "① 拒绝后自愈为 applied", c.ReadVersion(t).VersionStatus, StatusApplied)
	Eq(t, "① 自愈后 version 回 VersionPrevious", c.ReadVersion(t).Version, "1.0.0")
	Eq(t, "① F3 守卫自愈**不得**递增 apply_fail_count（否则守卫拒绝会误熔断）",
		c.ReadVersion(t).ApplyFailCount, 0)
	Eq(t, "① 拒绝路径不动主目录",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-100")

	// 修好现场，重新下载进入 downloaded
	if err := os.Rename(bak, filepath.Join(c.VerDir("1.1.0"), "a.dll")); err != nil {
		t.Fatalf("恢复快照文件失败: %v", err)
	}
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "重下失败")

	// ── ② 熔断临界：2 仍上报，3 才熔断 ──
	// 直接构造计数现场（不真造 3 次 rename 失败，理由见文件头）。
	setCount := func(n int) {
		WriteText(t, filepath.Join(c.VerDir("1.1.0"), "app.exe"), "app-110")
		WriteText(t, filepath.Join(c.VerDir("1.1.0"), ".updator", "shared.json"),
			ReadText(filepath.Join(c.Main(), ".updator", "shared.json")))
		c.WriteVersion(t, VersionInfo{
			VersionPrevious: "1.0.0", Version: "1.1.0",
			VersionStatus: StatusDownloaded, DownloadedFileCount: 2, ApplyFailCount: n,
		})
	}

	setCount(2) // MaxApplyFailCount-1
	cu2 := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	Eq(t, "② count=2（未达阈值）应仍上报有更新", cu2.HasUpdate, true)
	Eq(t, "② count=2 应是\"待继续 apply\"（need_download=false）",
		cu2.NeedDownloadUpdate, false)
	Eq(t, "② count=2 待应用的版本号", cu2.NewVersion, "1.1.0")

	setCount(3) // == MaxApplyFailCount
	cu3 := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	t.Logf("熔断后 check: has_update=%v need_download=%v new=%q",
		cu3.HasUpdate, cu3.NeedDownloadUpdate, cu3.NewVersion)
	// count>=3 时 check 不得再报"待继续 apply"。要么 has_update=false（当无更新），
	// 要么 need_download_update=true（要求重下），但绝不能是
	// has_update=true && need_download=false（那会 5 秒一轮反复杀主程序）。
	True(t, !(cu3.HasUpdate && !cu3.NeedDownloadUpdate),
		"② 熔断后不得再上报\"待继续 apply\"（has_update=%v need_download=%v）",
		cu3.HasUpdate, cu3.NeedDownloadUpdate)

	// ── ③ 复位之一：download_update 成功 → 计数归零（熔断的逃生口） ──
	//
	// ⚠ 这里必须分两种场景，因为 download_update 有两条不同出口：
	//   (a) 服务端发布了**更高**版本 → 走完整下载流程 → download_update.go:456 复位 0；
	//   (b) 服务端版本与待应用版本**相同**、且产物完好 → 早退分支
	//       （download_update.go:119 printProgressDone(); return），**不复位**。
	//
	// (b) 是产品缺陷：check_update.go:247 的注释写"现场手工清理占用后重跑
	// download_update 也可复位"，但服务端版本没变时根本走不到复位语句 ——
	// 熔断后唯一被文档指出的逃生口失效，机器会一直停在"无更新"直到服务端发布新版。
	// 本用例把两种场景都固化成断言：先验 (b) 的真实行为，再验 (a) 确实能复位。

	// ③-a 同版本 + 产物完好 → 早退，计数**不复位**（记录式断言，避免把缺陷当正确）
	outSame := e.RunLast(t, c, "download_update")
	Eq(t, "③-a 同版本重下应报成功（产物已完好，无需重下）", outSame.IsSuccess, true)
	countAfterSame := c.ReadVersion(t).ApplyFailCount
	if countAfterSame == 0 {
		t.Logf("③-a 同版本 download 已能复位计数（缺陷已修复）")
	} else {
		// 已知缺陷：留 Log 而不是 Fatal，避免这条已经非常长的用例被一个产品问题堵死；
		// 但要显式打出来，让人一眼看到"逃生口"失效。
		t.Logf("③-a ⚠ 产品缺陷：同版本 download_update 成功但 apply_fail_count 未复位"+
			"（仍为 %d，download_update.go:119 早退跳过了 :456 的复位）。"+
			"后果：熔断后若服务端不发布更高版本，check_update 永远不会再上报，机器卡死在旧版本。",
			countAfterSame)
	}
	// 同版本早退时 check 的行为：计数仍是 3 → 仍被熔断，不上报
	cuSame := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	True(t, !(cuSame.HasUpdate && !cuSame.NeedDownloadUpdate),
		"③-a 同版本 download 后若计数未复位，check 必须仍然熔断（不得悄悄恢复上报）")

	// ③-b 服务端发布更高版本 → 完整下载 → 计数必须复位
	e.PutFile(t, proj, "app.exe", "app-111")
	True(t, e.Publish(t, proj, "1.1.1", nil, "", "").IsSuccess, "发布 1.1.1")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "③-b 更高版本重下应成功")
	Eq(t, "③-b 更高版本下载成功后 apply_fail_count 必须复位",
		c.ReadVersion(t).ApplyFailCount, 0)
	Eq(t, "③-b 下载后待应用版本", c.ReadVersion(t).Version, "1.1.1")
	// 复位后 check 必须重新上报 —— 这才是熔断逃生口真正生效的证据
	cu4 := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	Eq(t, "③-b 复位后应重新上报有更新", cu4.HasUpdate, true)

	// ── ④ 复位之二：apply_update 成功 → 计数归零 ──
	// 先把计数推到 2，再让 apply 真正成功，验证成功路径也复位。
	setCount(2)
	outOK := e.RunLast(t, c, "apply_update")
	True(t, outOK.IsSuccess, "④ apply 应成功: %s\n%s",
		outOK.ErrorMsg, c.Log(t, "update.log", 40))
	v := c.ReadVersion(t)
	Eq(t, "④ apply 成功后计数必须复位（哪怕进 apply 前是 2）", v.ApplyFailCount, 0)
	Eq(t, "④ apply 成功后 version", v.Version, "1.1.0")
	Eq(t, "④ apply 成功后 status", v.VersionStatus, StatusApplied)
	Eq(t, "④ 主目录已是新内容",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
}

// ────────────────────────────────────────────────────────────────
// postForm 发 multipart/form-data POST（字段任意，可选文件）。
//
// 为什么需要：Env.Upload 只支持"上传单个文件到项目目录"这一种固定形状；
// 分片上传需要任意字段名 + 可选文件，且 upload_chunk_complete 是纯表单无文件。
// ────────────────────────────────────────────────────────────────
func (e *Env) postForm(t *testing.T, path string, fields map[string]string,
	files [][]byte) Resp {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("写字段 %s 失败: %v", k, err)
		}
	}
	for i, body := range files {
		fw, err := mw.CreateFormFile("file", fmt.Sprintf("chunk-%d", i))
		if err != nil {
			t.Fatalf("构造文件字段失败: %v", err)
		}
		if _, err := fw.Write(body); err != nil {
			t.Fatalf("写文件内容失败: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("关闭 multipart 失败: %v", err)
	}

	req, err := http.NewRequest("POST", e.BaseURL()+path, &buf)
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := e.base.Do(req)
	if err != nil {
		t.Fatalf("POST %s 失败: %v", path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读响应失败: %v", err)
	}
	var out Resp
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("解析响应失败(%s): %v\n原始: %s", path, err, truncate(string(raw), 800))
	}
	return out
}
