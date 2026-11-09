package tc

// 用例 130–133：self_update 自更新执行命令 + UI 自更新防循环保护
//
// 背景（2026-11-09 自更新优化）：
//
//   - 此前自更新"检测"（check_self_update）与"执行"（C# SDK UpdateSelf /
//     UI UpdateUpdater）分离，且执行方只有 SDK/UI——裸控制台部署（宿主
//     直接 spawn aly-client）时 UpdateFolder 副本会永久漂移在旧版本；
//   - 新增 self_update 命令把执行能力下沉到更新器本体：无差异跳过（顺带
//     清理 .new/.old 残留）、源缺失跳过、差异时走"运行中 exe 改名 .old →
//     .new 中转 + SHA256 校验 + os.Rename 原子落位 → 失败回滚"舞步；
//   - UI 启动自更新新增 journal 防循环（观察窗口内连续失败 ≥3 跳过）与
//     启动时残留清理（ui/winui/app/selfupdate.go）。
//
// 覆盖矩阵（与既有用例 92 的"检测层"互补，本文件钉"执行层"）：
//
//	130  self_update 一致跳过 + 残留清理（幂等）
//	131  self_update 差异替换闭环（updated=true + SHA256 收敛 + .old 备份）
//	132  self_update 源缺失跳过（source_missing，不算错误）+ self 缺失报错
//	133  UI journal 防循环：预置连续失败 → 启动 UI 必须跳过自更新
//
// 执行环境口径与既有用例一致：aly-client / fake_main / UI 用 go1.10.8 +
// GOARCH=386（BuildAll 已固定），在 Windows 上执行；本文件同时被
// `go vet` / `go test -c` 在非 Windows 环境编译验证。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// removePath 删除文件（缺失不算错误）。
func removePath(t *testing.T, p string) {
	t.Helper()
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		t.Fatalf("删除 %s 失败: %v", p, err)
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
// 期望结果
//   - isSuccess=true 且 data.updated=true、reason=replaced、old_path 非空；
//   - 替换后 UpdateFolder 副本 SHA256 与主目录副本一致（SHA256Of 口径）；
//   - old_path（.old）内容 == 替换前的旧副本（可回滚的备份）；
//   - 幂等：紧接着再次调用回到"一致跳过"（updated=false），并把 .old 清掉。
//
// 失败判定
//   - 替换后哈希不一致 → FAIL（严重：副本损坏会破坏后续所有更新）；
//   - old_path 为空或内容不是旧版 → FAIL（回滚保险不存在）；
//   - 二次调用仍 updated=true → FAIL（幂等性破坏 → 无限自更新）。
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
	// 主目录放"新版"（模拟随版本包下发的新更新器；长度与旧版一致，
	// 排除任何依赖 size 的假替换）
	newContent := strings.Repeat("N", len(oldContent))
	WriteText(t, srcPath, newContent)

	d := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
	True(t, d.Updated, "内容不同时应执行替换, data=%+v", d)
	Eq(t, "替换原因口径", d.Reason, "replaced")
	True(t, d.OldPath != "", "replaced 必须携带 old_path（.old 备份）")

	// 替换结果：副本内容 == 主目录新版
	Eq(t, "替换后副本与源一致（SHA256）",
		SHA256Of(selfPath), SHA256Of(srcPath))
	// .old 备份内容 == 替换前旧版
	Eq(t, ".old 备份必须保留旧版内容", ReadText(d.OldPath), oldContent)

	// 幂等回归：再次调用 → 一致跳过 + .old 清理
	d2 := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
	True(t, !d2.Updated, "替换后再次调用必须幂等跳过, data=%+v", d2)
	Eq(t, "幂等跳过原因", d2.Reason, "already_up_to_date")
	True(t, !Exists(d.OldPath), "幂等路径应清理 .old 备份")
}

// ────────────────────────────────────────────────────────────────
// 用例 132 — self_update：源缺失跳过 + self 缺失报错
//
// 期望结果
//   - 主目录没有 aly-client.exe（首次部署）：isSuccess=true 且
//     updated=false、reason=source_missing，UpdateFolder 副本原样；
//   - 主目录副本存在但 UpdateFolder 副本缺失（异常现场）：isSuccess=false，
//     错误信息可定位到比较阶段（不静默、不假装成功）。
//
// 失败判定
//   - source_missing 被当错误 → FAIL（首次部署场景会刷错误日志）；
//   - self 缺失被静默跳过 → FAIL（更新器缺失是必须暴露的现场异常）。
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

	// ① 源缺失：跳过不算错误
	selfBefore := ReadText(selfPath)
	True(t, !Exists(srcPath), "前置：主目录无 aly-client.exe")
	d := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
	True(t, !d.Updated, "源缺失不得触发替换, data=%+v", d)
	Eq(t, "源缺失原因口径", d.Reason, "source_missing")
	Eq(t, "源缺失不得改写副本", ReadText(selfPath), selfBefore)

	// ② self 缺失（异常现场）：报错并指明阶段
	removePath(t, selfPath)
	out := e.RunLast(t, c, "self_update")
	True(t, !out.IsSuccess, "self 缺失必须报错而不是静默成功, out=%+v", out)
	Contains(t, out.ErrorMsg, "compare self vs source", "self 缺失错误信息")
}

// ────────────────────────────────────────────────────────────────
// 用例 133 — UI 启动自更新防循环：journal 达到上限后跳过
//
// 现场：预置 aly-client-ui.exe.selfupdate.json（连续失败 3 次、时间戳为
// 当前）→ 启动 UI（-mode check）。
//
// 期望结果
//   - UI 正常启动并按 check 模式自动退出（退出码 0，不因跳过逻辑异常退出）；
//   - UpdateFolder/aly-client-ui.exe 内容**不变**（跳过替换）；
//   - journal 文件仍在（下次启动继续按窗口判定）。
//
// 失败判定
//   - UI 把预置 journal 当垃圾清掉并照常替换 → FAIL（防循环保护失效，
//     "替换失败 → 重启 → 再替换失败"会无限循环）；
//   - UI 因 journal 解析失败崩溃 / 退出码非 0 → FAIL。
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
	CopyFile(t, uiExe, filepath.Join(c.Up(), "aly-client-ui.exe"))

	uiSelf := filepath.Join(c.Up(), "aly-client-ui.exe")
	journal := uiSelf + ".selfupdate.json"

	// 预置 journal：连续失败 3 次（=上限）、最近失败在窗口内
	journalJSON := fmt.Sprintf(`{"count":3,"last_ts":%d,"last_error":"simulated disk full"}`,
		time.Now().Unix())
	WriteText(t, journal, journalJSON)
	uiBefore := ReadText(uiSelf)

	code, out := e.RunUIMode(t, filepath.Join(c.Up(), "aly-client-ui.exe"), c, "check", 40*time.Second)
	Eq(t, "UI 应正常退出", code, 0)
	_ = out

	// 防循环核心断言：副本未被替换
	Eq(t, "journal 触发跳过：UI 副本不得被替换", ReadText(uiSelf), uiBefore)
	True(t, Exists(journal), "journal 必须保留（窗口判定依据）")
	// UI 启动自更新还会处理 aly-client.exe：与主目录一致 → noop，不误伤
	True(t, !Exists(filepath.Join(c.Up(), "aly-client-ui.exe.new")),
		"跳过路径不得产生 .new 残留")
}
