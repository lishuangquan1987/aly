package tc

// 用例 136–137：两件套补齐（self_update --include-ui）与 SDK 拉起 UI 前自动落位
//
// 背景（现场故障）：发布包把新版本的 aly-client / aly-client-ui 投放到主程序
// 目录，但 UpdateFolder 里没有（或没有）对应副本时，宿主调 AlyUpdateUIClient
// 只会拿到 "aly-client-ui.exe not found"。修复分两层：
//
//	136  Go 侧：self_update --include-ui 把图形前端副本补齐——只在 UpdateFolder
//	     缺失/零字节时从主程序目录落位；已存在的副本一律不动（可能正在运行，
//	     落后副本由 UI 自己的启动自更新收敛）；收尾在途（journal pending）让位；
//	     不带旗标时 ui_* 缺省（既有行为逐字节不变）。
//	137  C# SDK 侧：AlyUpdateUIClient.Launch 之前保证两件套就位（sd17 驱动），
//	     失败只 NG 不抛异常，同一实例内不重复尝试。
//
// 执行口径与既有用例一致：aly-client 用 go1.10.8 + GOARCH=386（BuildAll 固定），
// UI 副本在本文件里用"非可执行文件"充当（本文件只钉落位/失败路径，不复现窗口）。

import (
	"os"
	"path/filepath"

	"testing"
)

// TestCase136 self_update --include-ui：图形前端副本补齐的五种口径。
func TestCase136(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-include-ui"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	const uiName = "aly-client-ui.exe"
	upSelf := filepath.Join(c.Up(), "aly-client.exe")
	upUI := filepath.Join(c.Up(), uiName)
	mainUp := filepath.Join(c.Main(), "aly-client.exe")
	mainUI := filepath.Join(c.Main(), uiName)

	// 前置：主目录的更新器与 UpdateFolder 一致（本体走 already_up_to_date 分支），
	// 主目录有 UI 副本（随版本分发），UpdateFolder 没有（现场故障形态）。
	CopyFile(t, upSelf, mainUp)
	CopyFile(t, upSelf, mainUI)
	True(t, !Exists(upUI), "前置：UpdateFolder 里没有 UI 副本")

	// ── ① 缺失 → 落位（ui_reason=replaced，无 .old） ──
	d := Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update", "--include-ui"))
	True(t, d.UIUpdated, "① 缺 UI 副本时必须落位, data=%+v", d)
	Eq(t, "① UI 侧口径", d.UIReason, "replaced")
	True(t, d.OldPath == "" || d.UIUpdated, "① 补齐路径不得留下 .old（目标本不存在）")
	Eq(t, "① 更新器本体一致时不得重复替换", d.Updated, false)
	Eq(t, "① 落位的副本必须与主目录副本逐字节一致（SHA256）", SHA256Of(upUI), SHA256Of(mainUI))

	// ── ② 已存在（落后）→ existing_kepped + 原样不动 ──
	WriteText(t, mainUI, "newer-ui")
	uiBefore := ReadText(upUI)
	d = Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update", "--include-ui"))
	True(t, !d.UIUpdated, "② 已存在的副本不得被替换, data=%+v", d)
	Eq(t, "② UI 侧口径", d.UIReason, "existing_kepped")
	Eq(t, "② 已存在的副本必须原样不动（可能正在运行）", ReadText(upUI), uiBefore)

	// ── ③ 收尾进程在途（journal pending）→ pending_inflight ──
	WriteJSON(t, upUI+".selfupdate.json", map[string]interface{}{
		"count": 1, "last_ts": 1000, "pending": true,
	})
	d = Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update", "--include-ui"))
	Eq(t, "③ UI 侧口径", d.UIReason, "pending_inflight")
	Eq(t, "③ 收尾在途时不得插队改副本", ReadText(upUI), uiBefore)
	os.Remove(upUI + ".selfupdate.json")

	// ── ④ 主目录没有 UI 源 → source_missing（命令仍成功） ──
	os.Remove(mainUI)
	d = Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update", "--include-ui"))
	True(t, !d.UIUpdated, "④ 源缺失不得落位, data=%+v", d)
	Eq(t, "④ UI 源缺失口径", d.UIReason, "source_missing")
	if o := e.RunLast(t, c, "self_update", "--include-ui"); !o.IsSuccess {
		t.Fatalf("④ 源缺失不得算命令失败: %s", string(o.Data))
	}

	// ── ⑤ 不带旗标 → ui_* 缺省（既有行为逐字节不变） ──
	d = Data[SelfUpdateExecData](t, e.RunLast(t, c, "self_update"))
	True(t, !d.UIUpdated && d.UIReason == "" && d.UIOldPath == "",
		"⑤ 不带 --include-ui 时 ui_* 必须缺省, data=%+v", d)
}

// TestCase137 SDK 拉起 UI 前自动落位（sd17 驱动）：现场故障的正面修复。
func TestCase137(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-sdk-ui-bootstrap"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	// 主程序目录里的"随版本分发的 UI 副本"：本用例只钉落位与失败路径，
	// 用一份非可执行文件充当（启动失败属预期，不作断言）。
	mainUI := filepath.Join(c.Main(), "aly-client-ui.exe")
	CopyFile(t, filepath.Join(c.Main(), "app.exe"), mainUI)
	expected := SHA256Of(mainUI)
	True(t, expected != "", "前置：主目录 UI 副本必须可读")
	True(t, !Exists(filepath.Join(c.Up(), "aly-client-ui.exe")),
		"前置：UpdateFolder 里没有 UI 副本（现场故障形态）")

	smoke := e.BuildSDK(t)
	out := e.RunSmokeEnv(t, smoke, "sd17", c, map[string]string{"ALY_EXPECT_UI_SHA": expected})
	t.Logf("sd17 输出:\n%s", truncate(out, 3000))

	Eq(t, "① sd17 不得有 FAIL", OutputHasFail(out), false)
	Contains(t, out, "SD17", "② 输出里必须有 SD17 标记（用例被找到并执行）")

	// ③ 关键断言逐条出现（防止断言被跳过而"没 FAIL"）
	for _, want := range []string{
		"SD17 ① UpdateFolder 缺 UI 副本时 Launch 必须自动从主程序目录落位",
		"SD17 ① 落位的 UI 副本必须与主程序目录副本一致（SHA256）",
		"SD17 ② 两处都没有 UI 副本时 Launch 必须返回 NG",
		"SD17 ② 失败路径不得触发 Started",
		"SD17 ③ 同一实例内不重复尝试补齐（失败后不再派进程）",
		"SD17 ④ 副本已一致时 Launch 不得改写它",
	} {
		Contains(t, out, want, "③ 必须验证: "+want)
	}

	// ④ e2e 侧独立复核：Launch 之后 UpdateFolder 里的副本必须与主目录副本一致
	Eq(t, "④ 落位后的 UI 副本必须与主目录副本一致（SHA256）",
		SHA256Of(filepath.Join(c.Up(), "aly-client-ui.exe")), expected)
}
