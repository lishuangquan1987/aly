package tc

// 用例 130–134：self_update 自更新执行命令 + UI 防循环 + SDK/UI 委派
//
// 背景（2026-11-09 自更新统一）：
//
//   - 此前自更新"检测"（check_self_update）与"执行"（C# SDK UpdateSelf /
//     UI UpdateUpdater）分离，且执行方各自实现一套复制/替换——裸控制台部署
//     （宿主直接 spawn aly-client）时 UpdateFolder 副本会永久漂移在旧版本；
//   - 新增 self_update 命令把执行能力下沉到更新器本体，并作为**正常路径**
//     供 UI UpdateUpdater / SDK 构造函数调用；仅在命令不可用（旧版更新器/
//     更新器缺失）时才直接复制兜底；
//   - UI 启动自更新新增 journal 防循环（30min 窗口内连续未收敛 ≥3 跳过，
//     延迟脚本的 move 失败也计入）与启动残留清理（ui/winui/app/selfupdate.go）。
//
// 覆盖矩阵（与既有用例 92 的"检测层"互补，本文件钉"执行/委派层"）：
//
//	130  self_update 一致跳过 + 残留清理（幂等）
//	131  self_update 差异替换闭环（PE 头造同长度合法新版 + SHA256 收敛 + .old 备份）
//	132  self_update 源缺失跳过（source_missing；self 缺失端到端不可构造，
//	     由 cmd 单测 TestRunSelfUpdateSelfMissing 覆盖）
//	133  UI journal 防循环 + UI 委派 self_update 更新 aly-client.exe
//	134  SDK 构造函数委派 self_update 更新 aly-client.exe（sd16）
//
// 执行环境口径与既有用例一致：aly-client / fake_main / UI 用 go1.10.8 +
// GOARCH=386（BuildAll 已固定），在 Windows 上执行；本文件同时被
// `go vet` / `go test -c` 在非 Windows 环境编译验证。

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// patchPETimeDateStamp 翻转 PE COFF 头的 TimeDateStamp（偏移 e_lfanew+8）。
// 该字段不参与 Windows 加载校验，翻转后仍是**同长度、合法可执行**的文件，
// 但 SHA256 不同——用于构造"新版更新器/UI"。
//
// 不能用纯文本伪造：self_update 替换后第二次调用会真的启动该文件，纯文本
// 会报"不是有效的 Win32 程序"（本轮修复前 131 即因此失败）。
func patchPETimeDateStamp(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	if len(b) < 0x40 {
		t.Fatalf("%s 太小，不像 PE 文件: %d 字节", path, len(b))
	}
	peOff := int(binary.LittleEndian.Uint32(b[0x3C:]))
	if peOff <= 0 || peOff+12 > len(b) || string(b[peOff:peOff+4]) != "PE\x00\x00" {
		t.Fatalf("%s PE 头不合法（e_lfanew=%d）", path, peOff)
	}
	b[peOff+8] ^= 0xFF // IMAGE_FILE_HEADER.TimeDateStamp
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatalf("写回 %s 失败: %v", path, err)
	}
}

// ────────────────────────────────────────────────────────────────
// 用例 130 — self_update：内容一致时跳过并清理残留（幂等性）
//
// 期望结果
//   - isSuccess=true 且 data.updated=false、reason=already_up_to_date；
//   - 预置的 .new / .old 残留被顺带清理（一致路径说明无在途替换，清理无竞态）；
//   - UpdateFolder 副本内容不变（跳过 ≠ 重写）。
//
// 失败判定
//   - updated=true → FAIL（每次启动都重写副本，徒增杀软扫描与 IO）；
//   - 残留未清理 → FAIL（残留会随历史堆积且污染下一次替换的现场）。
//
// ────────────────────────────────────────────────────────────────
func TestCase130(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-selfexec"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	// 前置：主目录副本与 UpdateFolder 副本一致
	selfPath := filepath.Join(c.Up(), "aly-client.exe")
	srcPath := filepath.Join(c.Main(), "aly-client.exe")
	CopyFile(t, selfPath, srcPath)
	selfBefore := ReadText(selfPath)

	// 预置历史残留（模拟上次替换中断 / 旧版舞步留下的 .old）
	WriteText(t, selfPath+".new", "half-written")
	WriteText(t, selfPath+".old", "previous-version")

	d := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
	True(t, d.Updated == false, "一致时不得触发替换, data=%+v", d)
	Eq(t, "跳过原因口径", d.Reason, "already_up_to_date")

	// 残留清理
	True(t, !Exists(selfPath+".new"), "一致路径应清理 .new 残留")
	True(t, !Exists(selfPath+".old"), "一致路径应清理 .old 残留")
	Eq(t, "跳过路径不得改写副本", ReadText(selfPath), selfBefore)
}

// ────────────────────────────────────────────────────────────────
// 用例 131 — self_update：内容不同时替换成功（执行层闭环）
//
// 前置：主目录副本 = 真实 exe + 翻转 PE TimeDateStamp（同长度、可执行、
// SHA256 不同），保证替换后第二次调用仍能启动该文件。
//
// 期望结果
//   - isSuccess=true 且 data.updated=true、reason=replaced、old_path 非空；
//   - 替换后 UpdateFolder 副本 SHA256 与主目录副本一致；
//   - old_path（.old）内容 == 替换前的旧副本（可回滚的备份）；
//   - 幂等：紧接着再次调用回到"一致跳过"（updated=false），并把 .old 清掉。
//
// 失败判定
//   - 替换后哈希不一致 → FAIL（严重：副本损坏会破坏后续所有更新）；
//   - old_path 为空或内容不是旧版 → FAIL（回滚保险不存在）；
//   - 二次调用仍 updated=true 或无法启动新 exe → FAIL（幂等性/合法性破坏）。
//
// ────────────────────────────────────────────────────────────────
func TestCase131(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-selfexec2"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	selfPath := filepath.Join(c.Up(), "aly-client.exe")
	srcPath := filepath.Join(c.Main(), "aly-client.exe")

	oldContent := ReadText(selfPath)
	// 主目录放"新版"：拷真实 exe + 翻转 PE 头时间戳 → 同长度、仍是合法可执行
	CopyFile(t, selfPath, srcPath)
	patchPETimeDateStamp(t, srcPath)
	newContent := ReadText(srcPath)
	Eq(t, "新版长度必须与旧版一致（排除 size 假替换）", len(newContent), len(oldContent))
	True(t, SHA256Of(srcPath) != SHA256Of(selfPath), "构造的新版必须与旧版内容不同")

	d := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
	True(t, d.Updated, "内容不同时应执行替换, data=%+v", d)
	Eq(t, "替换原因口径", d.Reason, "replaced")
	True(t, d.OldPath != "", "replaced 必须携带 old_path（.old 备份）")

	// 替换结果：副本内容 == 主目录新版
	Eq(t, "替换后副本与源一致（SHA256）",
		SHA256Of(selfPath), SHA256Of(srcPath))
	// .old 备份内容 == 替换前旧版
	Eq(t, ".old 备份必须保留旧版内容", ReadText(d.OldPath), oldContent)

	// 幂等回归：再次调用（此时运行的是可执行的新版）→ 一致跳过 + .old 清理
	d2 := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
	True(t, !d2.Updated, "替换后再次调用必须幂等跳过, data=%+v", d2)
	Eq(t, "幂等跳过原因", d2.Reason, "already_up_to_date")
	True(t, !Exists(d.OldPath), "幂等路径应清理 .old 备份")
}

// ────────────────────────────────────────────────────────────────
// 用例 132 — self_update：源缺失时跳过（source_missing，不算错误）
//
// 说明：self 缺失（异常现场）**无法在端到端构造**——Windows 不允许删除
// 正在运行的 exe，删掉后又无法 spawn 该命令。该分支由 cmd 单测
// TestRunSelfUpdateSelfMissing 覆盖；本用例只钉部署时必须正常处理的
// "主目录没有更新器"。
//
// 期望结果
//   - 主目录没有 aly-client.exe（首次部署）：isSuccess=true 且
//     updated=false、reason=source_missing，UpdateFolder 副本原样。
//
// 失败判定
//   - source_missing 被当错误 → FAIL（首次部署场景会刷错误日志）；
//   - 源缺失却改写了副本 → FAIL。
//
// ────────────────────────────────────────────────────────────────
func TestCase132(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-selfexec3"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	selfPath := filepath.Join(c.Up(), "aly-client.exe")
	srcPath := filepath.Join(c.Main(), "aly-client.exe")

	selfBefore := ReadText(selfPath)
	True(t, !Exists(srcPath), "前置：主目录无 aly-client.exe")
	d := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
	True(t, !d.Updated, "源缺失不得触发替换, data=%+v", d)
	Eq(t, "源缺失原因口径", d.Reason, "source_missing")
	Eq(t, "源缺失不得改写副本", ReadText(selfPath), selfBefore)
}

// ────────────────────────────────────────────────────────────────
// 用例 133 — UI 启动：journal 防循环 + 委派 self_update 更新 aly-client.exe
//
// 现场：
//   - UpdateFolder/aly-client-ui.exe = 真实 UI，主目录/aly-client-ui.exe = 同
//     长度、可执行、内容不同的"新版"（证明若未跳过就会替换）；
//   - 主目录/aly-client.exe = 同长度、可执行、内容不同的新版更新器；
//   - 预置 aly-client-ui.exe.selfupdate.json：连续未收敛 3 次、时间戳为当前。
//
// 期望结果
//   - UI 正常启动、检查、退出（退出码 0；无更新模态框被自动点掉）；
//   - journal 触发跳过：UI 自身副本不变、journal 保留、无 .new；
//   - **但** aly-client.exe 仍由 UpdateUpdater 委派 self_update 更新：主目录
//     与 UpdateFolder 副本 SHA256 一致，且存在 .old（命令路径的标志；直接
//     复制兜底不会产生 .old）。
//
// 失败判定
//   - UI 把 journal 当垃圾清掉并照常替换自身 → FAIL；
//   - UI 因 journal 解析失败崩溃 / 退出码非 0 → FAIL；
//   - aly-client.exe 未被更新，或未走 self_update（无 .old）→ FAIL。
//
// ────────────────────────────────────────────────────────────────
func TestCase133(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-ui-journal"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	uiExe := e.BuildUI(t)
	uiSelf := filepath.Join(c.Up(), "aly-client-ui.exe")
	CopyFile(t, uiExe, uiSelf)
	uiSrc := filepath.Join(c.Main(), "aly-client-ui.exe")
	CopyFile(t, uiExe, uiSrc)
	patchPETimeDateStamp(t, uiSrc)
	True(t, Exists(uiSrc), "前置：主目录必须有 UI 副本（否则走 source_missing，证明不了 journal）")
	True(t, SHA256Of(uiSrc) != SHA256Of(uiSelf), "前置：主目录 UI 必须与副本内容不同")

	// 主目录再放一份"新版更新器"，让 UpdateUpdater 真的走委派路径。
	updaterSelf := filepath.Join(c.Up(), "aly-client.exe")
	updaterSrc := filepath.Join(c.Main(), "aly-client.exe")
	CopyFile(t, updaterSelf, updaterSrc)
	patchPETimeDateStamp(t, updaterSrc)
	True(t, SHA256Of(updaterSrc) != SHA256Of(updaterSelf), "前置：主目录更新器必须与副本内容不同")

	journal := uiSelf + ".selfupdate.json"
	journalJSON := fmt.Sprintf(`{"count":3,"last_ts":%d,"last_error":"simulated disk full"}`,
		time.Now().Unix())
	WriteText(t, journal, journalJSON)
	uiBefore := ReadText(uiSelf)

	// 标题是 shared.json 的项目名；无更新时会弹模态框，必须自动点掉。
	res := e.RunUIModeAuto(t, uiSelf, c, "check", proj, 30*time.Second, 40*time.Second)
	Eq(t, "UI 应正常退出", res.ExitCode, 0)
	True(t, res.Dismissed, "无更新模态框应被点掉（标题=%s）", proj)

	// 防循环核心断言：UI 自身未被替换、journal 保留、无 .new
	Eq(t, "journal 触发跳过：UI 副本不得被替换", ReadText(uiSelf), uiBefore)
	True(t, Exists(journal), "journal 必须保留（窗口判定依据）")
	Contains(t, ReadText(journal), `"count":3`, "journal 仍应记录 3 次未收敛")
	True(t, !Exists(uiSelf+".new"), "跳过路径不得产生 .new 残留")

	// 委派断言：aly-client.exe 必须由 self_update 命令更新（.old 为证）。
	Eq(t, "UI 应委派 self_update 更新 aly-client.exe",
		SHA256Of(updaterSelf), SHA256Of(updaterSrc))
	True(t, Exists(updaterSelf+".old"),
		"应走 self_update 命令（.old 备份存在）；直接复制兜底不会产生 .old")
}

// ────────────────────────────────────────────────────────────────
// 用例 134 — SDK 构造函数委派 self_update 更新 aly-client.exe（sd16）
//
// 现场：客户端环境 + 主目录 aly-client.exe = 同长度、可执行、内容不同的新版。
// 步骤：BuildSDK 后用 sd16 构造 AlyUpdateClient（构造函数在后台任务里调
// self_update），smoke 通过 ALY_EXPECT_UPDATER_SHA 等待收敛。
//
// 期望结果
//   - smoke 无 FAIL、输出含 SD16；
//   - UpdateFolder 的 aly-client.exe 与主目录副本 SHA256 一致；
//   - 存在 .old（证明走的是命令，而不是 SDK 的直接替换兜底）。
//
// 失败判定
//   - SHA 不一致 → FAIL（SDK 自更新委派没有生效）；
//   - 无 .old 且 SHA 一致 → FAIL（说明走的是兜底，正常路径未委派）。
//
// ────────────────────────────────────────────────────────────────
func TestCase134(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-selfexec"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	updaterSelf := filepath.Join(c.Up(), "aly-client.exe")
	updaterSrc := filepath.Join(c.Main(), "aly-client.exe")
	CopyFile(t, updaterSelf, updaterSrc)
	patchPETimeDateStamp(t, updaterSrc)
	expected := SHA256Of(updaterSrc)
	True(t, expected != SHA256Of(updaterSelf), "前置：主目录更新器必须与副本内容不同")

	smoke := e.BuildSDK(t)
	out := e.RunSmokeEnv(t, smoke, "sd16", c,
		map[string]string{"ALY_EXPECT_UPDATER_SHA": expected})
	t.Logf("sd16 输出:\n%s", truncate(out, 3000))
	Eq(t, "sd16 不得有 FAIL", OutputHasFail(out), false)
	Contains(t, out, "SD16", "输出里必须有 SD16 标记（说明用例被找到并执行）")

	Eq(t, "SDK 构造函数应委派 self_update 同步更新器",
		SHA256Of(updaterSelf), expected)
	True(t, Exists(updaterSelf+".old"),
		"应走 self_update 命令（.old 备份存在）；直接替换兜底不会产生 .old")
}
