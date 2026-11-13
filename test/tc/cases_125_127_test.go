package tc

// 用例 125–127：publish-gui 初始化（布局 / 三件套 / 原子写）
//
// 背景：真机实测（2026-11-04）——在 publish-gui 里对一个真实项目"初始化项目"之后，
// 磁盘上的更新器目录三处都不对：
//
//	① 位置：UpdateFolder 落在主程序目录**内部**（…\bin\Debug\net8.0-windows\UpdateFolder），
//	   而 aly-client 的口径是"更新器目录与主程序目录同级"
//	   （README / DEVELOMENT_PLAN §3.0：PackageFolder/{ApplicationFolder, UpdateFolder}）；
//	② 内容：只复制了 aly-client.exe，缺 aly-client-ui.exe 与 client.json —— 写 client.json
//	   那一步抛 FileNotFoundException（System.IO 的 ReplaceFile 要求目标已存在），
//	   磁盘上只剩 client.json.tmp（用户看到的"client.json 被命名成 client.json.temp"）；
//	③ main_exe_relative_path = "..\WinFormsApp1.exe"：基准目录错了，相对路径跟着少了
//	   主程序目录名，正确值 = "..\net8.0-windows\WinFormsApp1.exe"。
//
// GUI 是 C#/Avalonia 的 UI 程序，框架跑不了它的窗口，所以每条都分两层钉：
//   - **源码契约**：读 publish-gui 的 C# 源码断言关键调用形态（与用例 104 的 ① 段同法）；
//   - **消费侧功能**：用真实 aly-client 消费 GUI 写出的那套布局与 client.json。
//     只在 Go 里把 C# 的路径算法重算一遍是自证自话——C# 侧改回旧写法时重算照样通过。
//
// 红线：更新器目录被放进主程序目录 → apply/rollback 给主目录改名时会把更新器与 .updator
// 一起卷走；且主目录的差量删除会盯上更新器自己的文件（更新器自毁）。

import (
	"os"
	"path/filepath"
	"testing"
)

// guiConfigServicePath 返回 publish-gui 里产出 UpdateFolder 那个源文件。
// 目录不在时 Skip：本用例只对 C# 源码，缺了没有可断言的对象。
//
// 说明（2026-11 复查）：这段逻辑原先写在 Services/ConfigService.cs，后来移到
// Services/UpdateFolderService.cs；安装包化之后又改成“从包内 client/<rid>/ 取更新器”。
// 本用例跟着指向真正的归属文件，并钉当前契约（意图不变：三件套齐全、与主程序目录同级）。
func guiConfigServicePath(t *testing.T) string {
	path := filepath.Join(RepoRoot, "publish", "publish-gui",
		"src", "AlyPublish", "Services", "UpdateFolderService.cs")
	if _, err := os.Stat(path); err != nil {
		t.Skip("publish-gui 源码不存在，跳过")
	}
	return path
}

// guiFileWriteHelperPath 返回 publish-gui 里原子写的归属文件（用例 127 用）。
func guiFileWriteHelperPath(t *testing.T) string {
	path := filepath.Join(RepoRoot, "publish", "publish-gui",
		"src", "AlyPublish", "Helpers", "FileWriteHelper.cs")
	if _, err := os.Stat(path); err != nil {
		t.Skip("publish-gui 源码不存在，跳过")
	}
	return path
}

// ────────────────────────────────────────────────────────────────
// 用例 125 — publish-gui 初始化：UpdateFolder 与主程序目录**同级**
//
// 缺陷根因：`Path.GetDirectoryName(Path.GetFullPath(projectPath))`。
// 浏览对话框返回的路径带**结尾分隔符**（实测 "…\bin\Debug\net8.0-windows\"），
// 此时 GetDirectoryName 回退成"目录自身" → "上级目录"算成了项目目录本身 →
// UpdateFolder 被创建在主程序目录**内部**。
//
// 本用例只读 C# 源码钉契约（不需要服务端与构建产物，故不建 Env）：
//  ① 先归一化（去掉结尾分隔符）；
//  ② 上级目录必须取自归一化后的值；
//  ③ UpdateFolder 挂在上级目录下（同级）；
//  ④ 反证：历史 bug 的两种形态都不得存在。
// ────────────────────────────────────────────────────────────────
func TestCase125(t *testing.T) {
	svc := ReadText(guiConfigServicePath(t))

	Contains(t, svc, "public static string? Create(", "初始化后产出 UpdateFolder 的入口")

	// ① 归一化：去掉结尾分隔符
	Contains(t, svc, "Path.TrimEndingDirectorySeparator(Path.GetFullPath(projectPath))",
		"必须先把 projectPath 归一化（浏览对话框返回的路径带结尾 '\\'）")

	// ② 上级目录必须取自归一化后的值
	Contains(t, svc, "Path.GetDirectoryName(projectDirectory)",
		"上级目录必须取自归一化后的项目目录")
	NotContains(t, svc, "Path.GetDirectoryName(Path.GetFullPath(projectPath))",
		"历史 bug 形态：带尾分隔符的路径会让 GetDirectoryName 回退成目录自身")

	// ③ 同级：UpdateFolder 挂在项目目录的上级目录下
	Contains(t, svc, `Path.Combine(parentDirectory, "UpdateFolder")`,
		"UpdateFolder 必须挂在项目目录的上级目录下（与主程序目录同级）")
	NotContains(t, svc, `Path.Combine(projectDirectory, "UpdateFolder")`,
		"❌ 放进主程序目录内：apply/rollback 给主目录改名会把更新器与 .updator 一起卷走")
}

// ────────────────────────────────────────────────────────────────
// 用例 126 — publish-gui 初始化：三件套与 client.json 的相对路径可被真实 client 消费
//
// ① 源码契约：UpdateFolder 内必须齐 aly-client.exe / aly-client-ui.exe / client.json 三件套
//    （UI 与更新器同目录：DEVELOPMENT_PLAN §3.0 明令 UI 必须住在 UpdateFolder）；
//    client.json 的目标名不得带临时后缀；相对路径以纠正后的 UpdateFolder 为基准。
// ② 消费侧：造"GUI 形状"的现场（主程序目录名 = 真实发布产物的名字 net8.0-windows，
//    UpdateFolder 与它同级，client.json 写 GUI 会写的那种相对路径），用真实 aly-client
//    跑 check → download → apply，验证更新真落在同级主程序目录、更新器目录没被卷走。
// ────────────────────────────────────────────────────────────────
func TestCase126(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-gui-init"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "WinFormsApp1.exe", "app-100")
	True(t, e.Publish(t, proj, "1.0.1", []string{"初始"}, "", "").IsSuccess, "发布 1.0.1 失败")

	// ── ① 源码契约（GUI 侧三件套 + 相对路径基准 + 更新器来自安装包 payload）──
	svc := ReadText(guiConfigServicePath(t))
	Contains(t, svc, "PkHelper.FindUp(",
		"更新器必须从安装包 payload（client/<rid>/）取，调用方不再自带二进制")
	Contains(t, svc, "HostHelper.UpBaseNames",
		"两个更新器走同一循环：aly-client 与 aly-client-ui 必须同进同退")
	Contains(t, svc, `Path.Combine(updateFolder, fileName)`,
		"必须复制到 UpdateFolder 下的同名文件（两件套同名不同名都不许错位）")
	Contains(t, svc, `Path.Combine(updateFolder, "client.json")`,
		"client.json 的目标必须是 UpdateFolder/client.json（不得带临时后缀）")
	Contains(t, svc, "GetRelativePath(updateFolder, mainExeFullPath)",
		"相对路径必须以纠正后的 UpdateFolder 为基准、以绝对主程序路径为目标")
	Contains(t, svc, "MustCloseProcessName", "必须写 must_close_process_name")
	Contains(t, svc, "GetFileNameWithoutExtension",
		"关进程名缺省时必须从主程序名推导（否则更新前不会关掉主程序）")

	// ── ①b 强制更新三态：GUI 复选框必须翻译成 --set / --clear 的显式一侧 ──
	// 审查 B7：CLI 改成三态后，"不传"=保持服务端原值；GUI 若在取消勾选时仍不传，
	// 用户看到的"关闭强制更新"不会生效。这里把两侧源码契约钉死。
	guiRoot := filepath.Join(RepoRoot, "publish", "publish-gui")
	cliSvc := ReadText(filepath.Join(guiRoot, "src", "AlyPublish", "Services", "CliService.cs"))
	Contains(t, cliSvc, `AddFlag(arguments, "--set-force-update", setForceUpdate)`,
		"GUI 推送必须能把强制更新显式置 true")
	Contains(t, cliSvc, `AddFlag(arguments, "--clear-force-update", clearForceUpdate)`,
		"GUI 推送必须能把强制更新显式置 false（不能靠不传参数）")
	vmSrc := ReadText(filepath.Join(guiRoot, "src", "AlyPublish", "ViewModels", "ProjectTabViewModel.cs"))
	Contains(t, vmSrc, "!SetForceUpdate && _forceUpdateKnown",
		"取消勾选时只在已知服务端现值时才发 --clear（未知时保持原值）")

	// ── ② 消费侧：GUI 形状的布局必须真能被 aly-client 消费 ──
	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100", DirName: "gui-site",
	})
	// 主程序目录改用真实发布产物的名字：GUI 面对的就是 "…\bin\Debug\net8.0-windows"
	guiDir := filepath.Join(c.Pkg, "net8.0-windows")
	if err := os.Rename(c.Main(), guiDir); err != nil {
		t.Fatalf("改名主程序目录失败: %v", err)
	}
	if err := os.Rename(filepath.Join(guiDir, "app.exe"),
		filepath.Join(guiDir, "WinFormsApp1.exe")); err != nil {
		t.Fatalf("改名主程序失败: %v", err)
	}
	// GUI 会写的那种 client.json：相对 UpdateFolder 指回同级主程序目录
	WriteJSON(t, filepath.Join(c.Up(), "client.json"), map[string]interface{}{
		"main_exe_relative_path":  "../net8.0-windows/WinFormsApp1.exe",
		"must_close_process_name": []string{"WinFormsApp1"},
	})

	// 服务端演进一版，看更新到底落在哪个目录
	e.PutFile(t, proj, "WinFormsApp1.exe", "app-110")
	True(t, e.Publish(t, proj, "1.1.0", []string{"演进"}, "", "").IsSuccess, "发布 1.1.0 失败")

	True(t, e.RunLast(t, c, "check_update").IsSuccess,
		"check_update 失败：GUI 产出的布局/相对路径不被更新器认可")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download_update 失败")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply_update 失败")

	Eq(t, "新内容落到同级主程序目录",
		ReadText(filepath.Join(guiDir, "WinFormsApp1.exe")), "app-110")
	if _, err := os.Stat(filepath.Join(c.Pkg, "net8.0-windows_1.0.0")); err != nil {
		t.Fatalf("备份目录应按主程序目录名命名（net8.0-windows_1.0.0）: %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.Up(), "client.json")); err != nil {
		t.Fatalf("apply 之后 client.json 必须仍在 UpdateFolder（更新器目录没被卷走）: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────
// 用例 127 — publish-gui 原子写：目标不存在时必须 File.Move，首次写不得只留 .tmp
//
// ① 源码契约：WriteAllTextAtomic 的形态必须是"先写 .tmp → 判目标是否存在 → Move/Replace"，
//    含非 Windows 平台的回退。System.IO 的 ReplaceFile **要求目标已存在**，
//    无条件 Replace 会让**首次写必然失败**，磁盘上只留下 .tmp —— 这就是
//    "client.json 被命名成 client.json.tmp"的真正根因（不是文件名拼错）。
// ② 消费侧：把 client.json 改名成 client.json.tmp（复现缺陷现场），跑 check_update 必须
//    明确失败并把原因指向 client.json —— 这种现场对用户来说就是"更新器不可用"。
// ────────────────────────────────────────────────────────────────
func TestCase127(t *testing.T) {
	// ── ① 源码契约：先写 .tmp、再一步落位，失败不留 .tmp ──
	// 实现已从 ConfigService.cs 移到 Helpers/FileWriteHelper.cs；落位用 Move(overwrite) 同时
	// 覆盖“目标不存在/已存在”，比“先判存在再 Move/Replace”少一个竞态窗口。
	svc := ReadText(guiFileWriteHelperPath(t))
	Contains(t, svc, `var temporaryPath = path + ".tmp"`, "原子写先写 .tmp")
	Contains(t, svc, "File.WriteAllText(temporaryPath, content)", "临时文件先落内容")
	Contains(t, svc, "File.Move(temporaryPath, path, overwrite: true)",
		"落位必须用 Move(overwrite)：目标不存在与已存在都要能成，首次写不得只留 .tmp")
	Contains(t, svc, "TryDeleteTemporaryFile(temporaryPath)",
		"落位失败必须清掉 .tmp（用户可见后果：目录里只剩 client.json.tmp）")
	NotContains(t, svc, "File.Replace(",
		"❌ 历史形态：无条件 Replace 要求目标已存在，首次写必抛 → 磁盘只剩 .tmp")

	// ── ② 消费侧：只有 .tmp 的现场不可用（用户可见后果）──
	e := NewEnv(t, true, 0)
	const proj = "tc-gui-atomic"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	True(t, e.Publish(t, proj, "1.0.1", []string{"初始"}, "", "").IsSuccess, "发布失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100", DirName: "gui-atomic",
	})
	// 复现缺陷现场：client.json 从未生成，只剩临时文件
	if err := os.Rename(filepath.Join(c.Up(), "client.json"),
		filepath.Join(c.Up(), "client.json.tmp")); err != nil {
		t.Fatalf("复现 client.json.tmp 现场失败: %v", err)
	}

	out := e.RunLast(t, c, "check_update")
	Eq(t, "只有 client.json.tmp 时命令必须失败（不得静默当没配置）", out.IsSuccess, false)
	Contains(t, out.ErrorMsg, "client.json", "失败原因必须指向 client.json")
}
