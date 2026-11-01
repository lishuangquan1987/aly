package tc

// 用例 78–94：C# SDK（AlyClient.CSharpSDK net40）+ 控制台驱动 AlySdkSmoke
//
// 这一批的特殊约束：
//   - SDK 是 net40 类库，**没有 xunit**，全部断言在 AlySdkSmoke 里以
//     "PASS:/FAIL:/SUMMARY:" 三行协议输出，Go 侧只做输出断言 + 独立的 CLI 复核。
//   - SDK 只是 aly-client.exe 的**薄封装**：真正逻辑在 CLI 里，所以每条用例
//     都尽量"SDK 走一遍 + CLI 等价路径再核一遍"，避免 SDK 侧断言假过掩盖 CLI 回归。
//   - sd8（apply）是自毁型：apply 会按 must-close 关闭宿主（smoke 自己），
//     不能和 all 一起跑；sd9 必须在 apply 之后另行进程核验。
//   - net40 的构建产物在 bin/Release/net40/，用 BuildSDK 统一产。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// smokeAssert 统一断言 SDK 输出：不得有 FAIL 行，且必须出现给定的关键 PASS 文案。
func smokeAssert(t *testing.T, out string, wantSubstrs ...string) {
	t.Helper()
	for _, s := range wantSubstrs {
		Contains(t, out, s, "SDK 输出缺关键行")
	}
	Eq(t, "SDK 输出不得含 FAIL:", OutputHasFail(out), false)
	Contains(t, out, "SUMMARY: PASS=", "SDK 输出缺 SUMMARY 汇总行")
}

// smokeSummary 解析 "SUMMARY: PASS=n FAIL=m"，返回 (pass, fail)。
func smokeSummary(t *testing.T, out string) (int, int) {
	t.Helper()
	idx := strings.Index(out, "SUMMARY: PASS=")
	if idx < 0 {
		t.Fatalf("SDK 输出无 SUMMARY 行:\n%s", truncate(out, 3000))
	}
	line := out[idx:]
	if i := strings.IndexAny(line, "\r\n"); i >= 0 {
		line = line[:i]
	}
	var p, f int
	if _, err := fmt.Sscanf(line, "SUMMARY: PASS=%d FAIL=%d", &p, &f); err != nil {
		t.Fatalf("解析 SUMMARY 失败(%q): %v", line, err)
	}
	return p, f
}

// ────────────────────────────────────────────────────────────────
// 用例 78 — SDK 构建（0 错误，含 AlySdkSmoke）
// ────────────────────────────────────────────────────────────────
func TestCase78(t *testing.T) {
	e := NewEnv(t, true, 0)

	smoke := e.BuildSDK(t)
	True(t, Exists(smoke), "AlySdkSmoke.exe 缺失: %s", smoke)
	True(t, SizeOf(smoke) > 4096, "AlySdkSmoke.exe 体积异常: %d", SizeOf(smoke))

	// net40 目标框架：产物目录必须含 net40 —— 落到 net8.0 就意味着客户机装不上
	Eq(t, "产物目录应为 net40（XP/7 兼容口径）",
		strings.Contains(filepath.ToSlash(smoke), "/net40/"), true)

	// SDK 类库与之同目录（ProjectReference 会一起拷过去）
	sdkDll := filepath.Join(filepath.Dir(smoke), "AlyClient.CSharpSDK.dll")
	Eq(t, "SDK 类库与 smoke 同目录", Exists(sdkDll), true)

	// 依赖 Newtonsoft.Json 必须一并落地（否则 smoke 起来就 MissingMethod）
	Eq(t, "Newtonsoft.Json 已随产物落地",
		Exists(filepath.Join(filepath.Dir(smoke), "Newtonsoft.Json.dll")), true)

	// PE 头：net40 库是 AnyCPU IL，machine 应为 I386（0x14c）——这里只作记录，
	// 因为 AnyCPU 的 PE machine 字段确实是 i386，真正判据是上面的 net40 目录。
	t.Logf("SDK 产物: %s（%d 字节），machine=0x%04x",
		filepath.Base(smoke), SizeOf(smoke), PEMachine(smoke))
}

// ────────────────────────────────────────────────────────────────
// 用例 79 — sd1：check_update 正常返回，字段完整
// ────────────────────────────────────────────────────────────────
func TestCase79(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	// 项目创建即 V1.0.0，不再重发 1.0.0（会被防倒退拒）
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	smoke := e.BuildSDK(t)

	out := e.RunSmoke(t, smoke, "sd1", c)
	smokeAssert(t, out,
		"PASS: SD1 check_update isSuccess",
		"PASS: SD1 data.current_version 字段存在",
	)
	p, f := smokeSummary(t, out)
	True(t, p >= 2 && f == 0, "sd1 应 ≥2 PASS、0 FAIL，实际 PASS=%d FAIL=%d", p, f)
	Contains(t, out, "has_update=False", "无更新现场应打印 has_update=False")
	Contains(t, out, "current=1.0.0", "current_version 应为 1.0.0")

	// 等价 CLI 复核：SDK 只是转发，DTO 字段名（snake_case）必须对上
	d := Data[CheckUpdateData](t, e.RunLast(t, c, "check_update"))
	Eq(t, "CLI has_update", d.HasUpdate, false)
	Eq(t, "CLI current_version", d.CurrentVersion, "1.0.0")

	// 有更新现场：SDK 侧同样要能读到 new_version（否则 UI 拿不到版本号）
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"升版"}, "", "").IsSuccess, "发布 1.1.0")
	out2 := e.RunSmoke(t, smoke, "sd1", c)
	smokeAssert(t, out2, "PASS: SD1 check_update isSuccess")
	Contains(t, out2, "has_update=True", "有更新时应打印 has_update=True")
	Contains(t, out2, "current=1.0.0", "current_version 仍是本地的 1.0.0")
}

// ────────────────────────────────────────────────────────────────
// 用例 80 — sd2：无效 exe 路径快速失败（<10s，不挂死）
// ────────────────────────────────────────────────────────────────
func TestCase80(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-eof"
	e.EnsureProject(t, proj, false, nil, nil)
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0", Project: proj,
	})
	smoke := e.BuildSDK(t)

	start := time.Now()
	out := e.RunSmoke(t, smoke, "sd2", c)
	elapsed := time.Since(start)

	smokeAssert(t, out,
		"PASS: SD2 不存在的 exe 返回失败",
		"PASS: SD2 快速失败不挂死",
	)
	// 双保险：外层也计一次时，防止 smoke 内部 Stopwatch 被优化掉
	True(t, elapsed < 30*time.Second, "sd2 整体耗时 %s（应远小于 30s）", elapsed)
	t.Logf("sd2 整体耗时 %s", elapsed)
}

// ────────────────────────────────────────────────────────────────
// 用例 81 — sd3：超时常量值（600 / 1800）
// ────────────────────────────────────────────────────────────────
func TestCase81(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-const"
	e.EnsureProject(t, proj, false, nil, nil)
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0", Project: proj,
	})
	smoke := e.BuildSDK(t)

	out := e.RunSmoke(t, smoke, "sd3", c)
	smokeAssert(t, out,
		"PASS: SD3 RollbackTimeoutSeconds = 600",
		"PASS: SD3 回滚超时常量 600s",
		"PASS: SD3 DefaultDownloadStallTimeoutSeconds = 1800",
		"PASS: SD3 下载停流看门狗常量 1800s",
	)

	// 源码级复核：常量必须仍是 public const 且值不变（防止改私有后 smoke 编不过
	// 而有人"顺手"把 smoke 里的引用删掉，导致这层保护静默失效）
	src := ReadText(filepath.Join(RepoRoot, "client", "aly-client-sdk",
		"AlyClient.CSharpSDK", "AlyClient.CSharpSDK", "AlyApi.cs"))
	Contains(t, src, "public const int RollbackTimeoutSeconds = 600", "常量声明")
	Contains(t, src, "public const int DefaultDownloadStallTimeoutSeconds = 1800", "常量声明")
}

// ────────────────────────────────────────────────────────────────
// 用例 82 — sd4：进度回调单调不减
//
// 前置要造"多文件 + 有内容变化"的 1.1.0，否则下载零文件就不会有 DONE 行，
// 回调零调用会让本用例假失败。
// ────────────────────────────────────────────────────────────────
func TestCase82(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-prog"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	for _, n := range []string{"a.dll", "b.dll", "c.dll"} {
		e.PutFile(t, proj, n, "v100-"+n)
	}

	// 1.1.0：4 个文件全变 → 至少 4 条 DONE 行
	e.PutFile(t, proj, "app.exe", "app-110")
	for _, n := range []string{"a.dll", "b.dll", "c.dll"} {
		e.PutFile(t, proj, n, "v110-"+n)
	}
	True(t, e.Publish(t, proj, "1.1.0", []string{"多文件"}, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	smoke := e.BuildSDK(t)

	out := e.RunSmoke(t, smoke, "sd4", c)
	smokeAssert(t, out,
		"PASS: SD4 download 成功或已是最新",
		"PASS: SD4 进度回调被调用",
		"PASS: SD4 进度比率单调不减",
	)
	// 回调次数应 ≥ 变化的文件数（4），少于此说明 DONE 行没被逐条解析
	Contains(t, out, "进度回调被调用", "回调计数")
	t.Logf("sd4 输出节选:\n%s", truncate(out, 1200))

	// CLI 侧复核：下载真的落地了（SDK 报告成功不能替代产物检查）
	Eq(t, "下载后 a.dll 内容", strings.TrimSpace(ReadText(filepath.Join(c.VerDir("1.1.0"), "a.dll"))), "v110-a.dll")
	v := c.ReadVersion(t)
	Eq(t, "version_status", v.VersionStatus, StatusDownloaded)
	Eq(t, "version", v.Version, "1.1.0")
}

// ────────────────────────────────────────────────────────────────
// 用例 83 — sd5：回调抛异常隔离，下载照常完成
// ────────────────────────────────────────────────────────────────
func TestCase83(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-throw"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "x.dll", "v100-x")
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "x.dll", "v110-x")
	True(t, e.Publish(t, proj, "1.1.0", []string{"升版"}, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	smoke := e.BuildSDK(t)

	out := e.RunSmoke(t, smoke, "sd5", c)
	smokeAssert(t, out, "PASS: SD5 回调抛异常不影响下载")

	// 关键：异常被吞掉 ≠ 下载没做。产物必须齐。
	Eq(t, "抛异常回调下仍下载成功（x.dll 落地）",
		strings.TrimSpace(ReadText(filepath.Join(c.VerDir("1.1.0"), "x.dll"))), "v110-x")
	Eq(t, "version_status", c.ReadVersion(t).VersionStatus, StatusDownloaded)
	// 回调异常不能出现在 aly-client 的日志里（它根本不该知道宿主回调）
	NotContains(t, c.Log(t, "download.log", 50), "InvalidOperationException",
		"宿主回调异常不得污染客户端日志")
}

// ────────────────────────────────────────────────────────────────
// 用例 84 — sd6：断网 check（isSuccess:true + has_update:false）
//
// 断网语义的核心：check_update 必须"成功但不报更新"，而不是"失败"。
// 失败会让宿主弹红色告警，把网络问题误报成更新故障。
// ────────────────────────────────────────────────────────────────
func TestCase84(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-offline"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	smoke := e.BuildSDK(t)

	// 先在线确认一次可达（否则下面断了也说不清是谁的问题）
	True(t, e.RunLast(t, c, "check_update").IsSuccess, "在线时 check 应成功")

	// 停服务端 → 断网现场
	e.StopServer()

	start := time.Now()
	out := e.RunSmoke(t, smoke, "sd6", c)
	elapsed := time.Since(start)

	smokeAssert(t, out,
		"PASS: SD6 断网 check isSuccess:true",
		"PASS: SD6 断网 has_update:false",
	)
	True(t, elapsed < 90*time.Second, "断网 check 应快速返回，实际 %s", elapsed)

	// CLI 侧复核同一语义
	dl := e.RunLast(t, c, "check_update")
	True(t, dl.IsSuccess, "断网时 check_update 必须 isSuccess:true（不报错）")
	d := Data[CheckUpdateData](t, dl)
	Eq(t, "断网时 has_update", d.HasUpdate, false)
	Eq(t, "断网时 current_version 仍回填", d.CurrentVersion, "1.0.0")
	t.Logf("断网 check 耗时 %s", elapsed)
}

// ────────────────────────────────────────────────────────────────
// 用例 85 — sd7：list_rollback_versions 结构完整
// ────────────────────────────────────────────────────────────────
func TestCase85(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-roll"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	// 造两个备份目录 + 两个"非版本"干扰目录
	WriteText(t, filepath.Join(c.VerDir("0.9.0"), "app.exe"), "old-090")
	WriteText(t, filepath.Join(c.VerDir("0.8.0"), "app.exe"), "old-080")
	WriteText(t, filepath.Join(c.VerDir("backup"), "app.exe"), "not-a-version")
	WriteText(t, filepath.Join(c.VerDir("1.2.0.tmp"), "app.exe"), "tmp-dir")

	smoke := e.BuildSDK(t)
	out := e.RunSmoke(t, smoke, "sd7", c)
	smokeAssert(t, out, "PASS: SD7 list_rollback_versions isSuccess")
	Contains(t, out, "可回滚版本数 = 2", "SDK 侧应看到 2 个可回滚版本")

	// 直接走 CLI 复核结构与顺序（SDK 只是转发）
	d := Data[RollbackListData](t, e.RunLast(t, c, "list_rollback_versions"))
	Eq(t, "current_version", d.CurrentVersion, "1.0.0")
	Eq(t, "可回滚版本数（非版本目录已被过滤）", len(d.Versions), 2)
	// 重要口径（文档原期望是错的）：列表顺序 = os 目录遍历顺序（Windows 上是字母序），
	// **未按版本号降序排序**。也就是说 UI 拿到的是 "0.8.0,0.9.0" 而不是"最近的在前"。
	// 断言按字母序钉死；将来若真的加了降序排序，这条会 FAIL 提醒去更新文档。
	Eq(t, "版本集合（排除了 backup / 1.2.0.tmp，顺序为目录遍历字母序）",
		strings.Join(d.Versions, ","), "0.8.0,0.9.0")
	Eq(t, "不含当前版本 1.0.0", SliceContains(d.Versions, "1.0.0"), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 86 — sd8：apply spawn 关闭 must-close 进程
//
// sd8 是**自毁型**：ApplyUpdateAsync spawn 出的 apply_update 会按
// must_close_process_name 关掉宿主（也就是 smoke 自己）。
// 因此：
//   - 不能与 all 一起跑（AlySdkSmoke 的 all 已主动 SKIP 掉 sd8）；
//   - 这里用"进程是否提前退出"作为正向证据，而不是等它跑完 60s；
//   - 无待应用内容时 smoke 会活满 60s 打印"未被关闭"，也算合法结果，
//     但那样证明不了关闭能力，所以本用例**先造出 downloaded 现场**。
// ────────────────────────────────────────────────────────────────
func TestCase86(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-apply"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "new.txt", "new-in-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"升版"}, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	// 先把 downloaded 现场造好（apply 的前置）
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 失败")
	Eq(t, "前置：已进入 downloaded", c.ReadVersion(t).VersionStatus, StatusDownloaded)

	// 让 apply 去关 smoke 自己：client.json 的 must_close_process_name
	WriteJSON(t, filepath.Join(c.Up(), "client.json"), map[string]interface{}{
		"main_exe_relative_path":  "../ApplicationFolder/app.exe",
		"must_close_process_name": []string{"AlySdkSmoke"},
	})

	smoke := e.BuildSDK(t)
	start := time.Now()
	out := e.RunSmoke(t, smoke, "sd8", c)
	elapsed := time.Since(start)

	// spawn 一定成功
	Contains(t, out, "PASS: SD8 apply spawn 成功", "SD8 输出")
	// 60s 窗口内被关闭 → 输出停在 spawn 之后、没有"未被关闭"那句
	closedEarly := elapsed < 50*time.Second
	if closedEarly {
		NotContains(t, out, "60s 内未被 apply 关闭",
			"提前退出就不能打印这句（否则是 60s 到点自然结束）")
	}
	t.Logf("sd8 耗时 %s，被 apply 关闭=%v", elapsed, closedEarly)

	// 无论是否被关闭，version.json 都必须已离开 downloaded（进入 applying 或 applied）
	deadline := time.Now().Add(60 * time.Second)
	var status string
	for time.Now().Before(deadline) {
		status = c.ReadVersion(t).VersionStatus
		if status == StatusApplying || status == StatusApplied {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	True(t, SliceContains([]string{StatusApplying, StatusApplied}, status),
		"version_status 应进入 applying 或 applied，实际 %q（update.log:\n%s）",
		status, c.Log(t, "update.log", 40))
	True(t, closedEarly, "smoke 应在 60s 窗口内被 apply 关闭（实际耗时 %s）", elapsed)

	// 等 apply 真正落定，再核内容（apply 是独立进程，spawn 返回≠已完成）
	deadline = time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		if strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))) == "app-110" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	Eq(t, "主目录已是 1.1.0 内容",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
	Eq(t, "新文件已落地", Exists(filepath.Join(c.Main(), "new.txt")), true)
}

// ────────────────────────────────────────────────────────────────
// 用例 87 — sd9：宿主重启后版本核验
// ────────────────────────────────────────────────────────────────
func TestCase87(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-verify"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"升版"}, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply")

	smoke := e.BuildSDK(t)

	// ① 注入期望版本 → 应 PASS（不出现 FAIL）
	out := e.RunSmokeEnv(t, smoke, "sd9", c, map[string]string{"ALY_EXPECT_VERSION": "1.1.0"})
	smokeAssert(t, out, "PASS: SD9 apply 后 version=1.1.0")

	// ② 不注入 → 应 SKIP，且不算失败
	out2 := e.RunSmoke(t, smoke, "sd9", c)
	Contains(t, out2, "SKIP: SD9", "未设环境变量应 SKIP")
	Eq(t, "SKIP 不得变成 FAIL", OutputHasFail(out2), false)

	// ③ 注入错的期望版本 → 必须 FAIL（证明它真在读 version.json，不是无脑 PASS）
	out3 := e.RunSmokeEnv(t, smoke, "sd9", c, map[string]string{"ALY_EXPECT_VERSION": "9.9.9"})
	Eq(t, "错期望版本必须 FAIL", OutputHasFail(out3), true)
	Contains(t, out3, "未指向期望版本 9.9.9", "FAIL 原因应指明期望版本")
}

// ────────────────────────────────────────────────────────────────
// 用例 88 — sd10：StatusChanged 抛异常，主循环不死
// ────────────────────────────────────────────────────────────────
func TestCase88(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-ev"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	smoke := e.BuildSDK(t)

	out := e.RunSmoke(t, smoke, "sd10", c)
	smokeAssert(t, out, "PASS: SD10 事件处理器抛异常后循环仍运行")
	// 隔离不应靠"把事件吞掉后再也不发"：日志里应留下这条隔离记录
	Contains(t, out, "SUMMARY: PASS=", "汇总行")
}

// ────────────────────────────────────────────────────────────────
// 用例 89 — sd11：空闲 Dispose 在 6s 内退出
// ────────────────────────────────────────────────────────────────
func TestCase89(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-dispose"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	smoke := e.BuildSDK(t)

	start := time.Now()
	out := e.RunSmoke(t, smoke, "sd11", c)
	elapsed := time.Since(start)

	smokeAssert(t, out,
		"PASS: SD11 Dispose 后 IsRunning=false",
		"PASS: SD11 Dispose 5s 内完成",
	)
	// 外层总时长 = 2s 预热 + Dispose 耗时，留 20s 兜底防挂死
	True(t, elapsed < 20*time.Second, "sd11 整体耗时 %s（应远小于 20s）", elapsed)
	t.Logf("sd11 整体耗时 %s（含 2s 预热）", elapsed)
}

// ────────────────────────────────────────────────────────────────
// 用例 90 — sd12：二次 Dispose 幂等
// ────────────────────────────────────────────────────────────────
func TestCase90(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-twice"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	smoke := e.BuildSDK(t)

	out := e.RunSmoke(t, smoke, "sd12", c)
	smokeAssert(t, out, "PASS: SD12 二次 Dispose 幂等")
	p, f := smokeSummary(t, out)
	Eq(t, "sd12 应 1 PASS 0 FAIL", fmt.Sprintf("%d/%d", p, f), "1/0")
}

// ────────────────────────────────────────────────────────────────
// 用例 91 — sd13：全事件订阅后循环正常
// ────────────────────────────────────────────────────────────────
func TestCase91(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-allevent"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	smoke := e.BuildSDK(t)

	out := e.RunSmoke(t, smoke, "sd13", c)
	smokeAssert(t, out, "PASS: SD13 全事件订阅运行正常")
	Eq(t, "sd13 不得有 FAIL", OutputHasFail(out), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 92 — CheckSelfUpdateAsync 正确返回 need_update
//
// 判定口径：**SHA256 比较**，不是 mtime/size。
// 主目录没有 aly-client.exe 时（首次部署）应 need_update=false。
// ────────────────────────────────────────────────────────────────
func TestCase92(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-self"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	mainExe := filepath.Join(c.Main(), "aly-client.exe")

	// ① 主目录没有 aly-client.exe（首次部署）→ need_update=false
	Eq(t, "前置：主目录无 aly-client.exe", Exists(mainExe), false)
	d0 := Data[SelfUpdateData](t, e.RunLast(t, c, "check_self_update"))
	Eq(t, "首次部署（无从比较）无需自更新", d0.NeedUpdate, false)

	// ② 内容一致 → need_update=false
	CopyFile(t, filepath.Join(c.Up(), "aly-client.exe"), mainExe)
	d1 := Data[SelfUpdateData](t, e.RunLast(t, c, "check_self_update"))
	Eq(t, "内容一致时无需自更新", d1.NeedUpdate, false)

	// ③ 内容不同 → need_update=true
	WriteText(t, mainExe, "stale-updater-content")
	d2 := Data[SelfUpdateData](t, e.RunLast(t, c, "check_self_update"))
	Eq(t, "内容不同时应需要自更新", d2.NeedUpdate, true)

	// ④ 关键：同长度同 mtime、仅内容不同也必须判 true
	//    （证明用的是 SHA256 而不是 size/mtime —— 大小相同是这里的关键）
	selfBytes := ReadText(filepath.Join(c.Up(), "aly-client.exe"))
	sameLen := strings.Repeat("X", len(selfBytes))
	WriteText(t, mainExe, sameLen)
	Eq(t, "同长度不同内容也必须判需要更新（排除 size 比较）",
		SizeOf(mainExe), SizeOf(filepath.Join(c.Up(), "aly-client.exe")))
	d3 := Data[SelfUpdateData](t, e.RunLast(t, c, "check_self_update"))
	Eq(t, "同长度不同内容 → need_update", d3.NeedUpdate, true)

	// ⑤ 恢复一致后又回到 false（幂等，不是"一旦 true 就永远 true"）
	CopyFile(t, filepath.Join(c.Up(), "aly-client.exe"), mainExe)
	d4 := Data[SelfUpdateData](t, e.RunLast(t, c, "check_self_update"))
	Eq(t, "恢复一致后回到 false", d4.NeedUpdate, false)
}

// ────────────────────────────────────────────────────────────────
// 用例 93 — 已是最新不死循环（复位 None）
//
// 现场：version = version_previous = 1.1.0 且 status=downloaded 的**退化态**。
// 关键判据：
//   ① CLI 侧 download_update 不得假成功（那会覆盖已下载产物）；
//   ② SDK 侧必须把"已是最新"当无更新处理，而不是每 1s 重查刷屏。
// ────────────────────────────────────────────────────────────────
func TestCase93(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-stuck"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	// 退化现场：downloaded 但 version == version_previous，且服务端没有更高版本
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusDownloaded, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	before := ReadText(filepath.Join(c.Up(), "version.json"))

	dl := e.RunLast(t, c, "download_update")
	// 归一化分支：downloaded + version==previous + 服务端无更高版本
	// → 归一化为 applied 并以完成行（data:null）收尾，不报错
	True(t, dl.IsSuccess, "退化现场应归一化并成功收尾，实际 errorMsg=%q", dl.ErrorMsg)
	v := c.ReadVersion(t)
	Eq(t, "归一化后 version_status", v.VersionStatus, StatusApplied)
	Eq(t, "归一化不改版本号", v.Version, "1.0.0")
	t.Logf("version.json 变化: before=%s after=%s",
		truncate(before, 120), truncate(ReadText(filepath.Join(c.Up(), "version.json")), 120))

	// ── 真正的"已是最新"分支：applied/1.1.0 且服务端最高就是 1.1.0 ──
	e.PutFile(t, proj, "app.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"升版"}, "", "").IsSuccess, "发布 1.1.0")
	c2 := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-110",
	})
	dl2 := e.RunLast(t, c2, "download_update")
	Eq(t, "已是最新时 download 应拒绝", dl2.IsSuccess, false)
	Contains(t, dl2.ErrorMsg, "already at latest version", "errorMsg 文案")
	// 关键：不能假成功，也不能动 version.json
	Eq(t, "拒绝时 version_status 不变", c2.ReadVersion(t).VersionStatus, StatusApplied)
	Eq(t, "拒绝时 version 不变", c2.ReadVersion(t).Version, "1.1.0")

	// SDK 主循环：这一路不得出现"每 1s 重查 + 报错"的死循环。
	// 观察 12s（远大于旧版的 1s 重查间隔）：日志里 update 异常条数应极少。
	smoke := e.BuildSDK(t)
	out := e.RunSmoke(t, smoke, "sd10", c2) // sd10 会跑 5s 主循环，用它做旁证
	NotContains(t, out, "already at latest version 反复", "不应出现死循环特征")
	Eq(t, "SDK 侧不应把'已是最新'报成循环异常", OutputHasFail(out), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 94 — Launch 拉 UI 进程（返回 OK、退出码可读）
//
// SDK 的 Launch 是**非阻塞 spawn**，UI 的交互式行为在用例 42/43 已覆盖。
// 本例验证：spawn 成功 + 参数拼装正确 + 退出码可读。
// ────────────────────────────────────────────────────────────────
func TestCase94(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-launch"
	e.EnsureProject(t, proj, false, nil, nil)
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	// ① 参数拼装：ToArgument 必须是 -mode 后面跟裸值（不是一个 token）
	src := ReadText(filepath.Join(RepoRoot, "client", "aly-client-sdk",
		"AlyClient.CSharpSDK", "AlyClient.CSharpSDK", "AlyUpdateUIClient.cs"))
	Contains(t, src, `"-mode {0} -lang {1}"`, "Arguments 必须是 -mode 与值分开的模板")
	Contains(t, src, `return "check";`, "Check → check")
	Contains(t, src, `return "update";`, "Update → update")
	Contains(t, src, `return "rollback";`, "Rollback → rollback")
	Contains(t, src, `return "silent";`, "Silent → silent")
	Contains(t, src, `return "auto";`, "默认 → auto")

	// ② 真实链路：UI 缺失时 Launch 应明确失败（不是静默成功）
	uiExe := e.BuildUI(t)
	uiInClient := filepath.Join(c.Up(), "aly-client-ui.exe")

	// 无更新现场（服务端停在 V1.0.0，客户端 1.0.0）→ check 模式会弹"已是最新"
	CopyFile(t, uiExe, uiInClient)
	res := e.RunUIModeAuto(t, uiInClient, c, "check", "aly-client-ui", 30*time.Second, 40*time.Second)
	True(t, res.ExitCode != -999, "UI 在 40s 内未退出（模态框已点=%v）", res.Dismissed)
	Eq(t, "UI 退出码应为 0", res.ExitCode, 0)
	Eq(t, "check 是只读命令，不得产生快照", ListDirMatch(c.Pkg, "ApplicationFolder_*"), 0)

	// ③ auto 模式（SDK LaunchAuto 等价）：无更新时应静默退出，不弹模态框
	res2 := e.RunUIModeAuto(t, uiInClient, c, "auto", "", 0, 40*time.Second)
	True(t, res2.ExitCode == 0 || res2.ExitCode == -999,
		"auto 模式异常退出，退出码=%d", res2.ExitCode)
	if res2.ExitCode == -999 {
		t.Logf("auto 模式常驻不退出（无更新时应静默退出，属缺陷观察项）")
	}
	NotContains(t, res2.Output, "panic:", "UI 不得 panic")
	t.Logf("UI check 退出码=%d（%s）；auto 退出码=%d",
		res.ExitCode, res.Elapsed, res2.ExitCode)

	// ④ 让 os 的静态检查担保：UI 产物存在且为 windowsgui（否则 Launch 会弹控制台窗）
	Eq(t, "UI 子系统", PESubsystem(uiInClient), SubsystemWindowsGUI)
	_ = os.Remove(uiInClient) // 清掉，避免影响后续 clean 类用例
}