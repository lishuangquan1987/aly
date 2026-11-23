package tc

// 用例 125–127：发布 GUI 初始化（布局 / 三件套 / 原子写），断言对象 publish-gui-go
//
// 背景：真机实测（2026-11-04）——在历史 C# 版（publish-gui-csharp）里对一个真实项目
// "初始化项目"之后，磁盘上的更新器目录三处都不对：
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
// 2026-11-23 起发布 GUI 主实现切换为 publish-gui-go（历史 C# 版保留但不再默认构建），
// 三条用例随之改写为对 Go 源码的等价断言；缺陷背景与红线不变。
//
// GUI 是 UI 程序，框架跑不了它的窗口，所以每条都分两层钉：
//   - **源码契约**：读 publish-gui-go 的 Go 源码断言关键调用形态（与用例 104 的 ① 段同法）；
//   - **消费侧功能**：用真实 aly-client 消费 GUI 写出的那套布局与 client.json。
//     只在 Go 里把路径算法重算一遍是自证自话——实现侧改回旧写法时重算照样通过。
//
// 红线：更新器目录被放进主程序目录 → apply/rollback 给主目录改名时会把更新器与 .updator
// 一起卷走；且主目录的差量删除会盯上更新器自己的文件（更新器自毁）。

import (
	"os"
	"path/filepath"
	"testing"
)

// guiUpdateFolderSource 返回 publish-gui-go 里产出 UpdateFolder 的源文件。
// 目录不在时 Skip：本用例只对 Go 源码，缺了没有可断言的对象。
func guiUpdateFolderSource(t *testing.T) string {
	path := filepath.Join(RepoRoot, "publish", "publish-gui-go", "updatefolder.go")
	if _, err := os.Stat(path); err != nil {
		t.Skip("publish-gui-go 源码不存在，跳过")
	}
	return path
}

// guiConfigSource 返回 publish-gui-go 里原子写的归属文件（用例 127 用）。
func guiConfigSource(t *testing.T) string {
	path := filepath.Join(RepoRoot, "publish", "publish-gui-go", "config.go")
	if _, err := os.Stat(path); err != nil {
		t.Skip("publish-gui-go 源码不存在，跳过")
	}
	return path
}

// ────────────────────────────────────────────────────────────────
// 用例 125 — 发布 GUI 初始化：UpdateFolder 与主程序目录**同级**
//
// 历史 C# 缺陷根因：对带结尾分隔符的路径取"上级目录"会回退成目录自身，
// UpdateFolder 被创建在主程序目录内部。
//
// Go 版契约（updatefolder.go）：先 Clean 归一化，再取 Dir 作上级，
// UpdateFolder 挂在上级目录下（同级）。
// ────────────────────────────────────────────────────────────────
func TestCase125(t *testing.T) {
	svc := ReadText(guiUpdateFolderSource(t))

	Contains(t, svc, "func createUpdateFolder(projectPath, mainExePath, closeProcessName string) (string, error)",
		"初始化后产出 UpdateFolder 的入口")

	// ① 归一化：filepath.Clean 去掉结尾分隔符
	Contains(t, svc, "projectDir := filepath.Clean(projectPath)",
		"必须先把 projectPath 归一化（浏览对话框返回的路径可能带结尾 '\\'）")

	// ② 上级目录必须取自归一化后的值
	Contains(t, svc, "parent := filepath.Dir(projectDir)",
		"上级目录必须取自归一化后的项目目录")

	// ③ 同级：UpdateFolder 挂在项目目录的上级目录下
	Contains(t, svc, `updateFolder := filepath.Join(parent, "UpdateFolder")`,
		"UpdateFolder 必须挂在项目目录的上级目录下（与主程序目录同级）")
	NotContains(t, svc, `filepath.Join(projectDir, "UpdateFolder")`,
		"❌ 放进主程序目录内：apply/rollback 给主目录改名会把更新器与 .updator 一起卷走")
}

// ────────────────────────────────────────────────────────────────
// 用例 126 — 发布 GUI 初始化：三件套与 client.json 的相对路径可被真实 client 消费
//
// ① 源码契约：UpdateFolder 内必须齐 aly-client.exe / aly-client-ui.exe / client.json 三件套
//    （UI 与更新器同目录：DEVELOPMENT_PLAN §3.0 明令 UI 必须住在 UpdateFolder）；
//    client.json 的目标名不得带临时后缀；相对路径以同级 UpdateFolder 为基准；
//    更新器从安装包 payload（client/<rid>/）取。
// ①b 强制更新三态：GUI 复选框必须翻译成 --set / --clear 的显式一侧，且只在
//    读到过服务端现值时才发（未知时保持原值）。
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
	svc := ReadText(guiUpdateFolderSource(t))
	Contains(t, svc, "updaterRIDsFor(app)",
		"更新器必须按包内 payload 的 client_rid 取（client/<rid>/），与 findCLI 同一套口径")
	Contains(t, svc, `filepath.Join(dir, "client", rid, name)`,
		"更新器来源必须是安装包 payload 的 client/<rid>/（缺省回退开发树）")
	Contains(t, svc, "func updaterNames() []string",
		"两个更新器走同一循环：aly-client 与 aly-client-ui 必须同进同退")
	Contains(t, svc, "filepath.Join(updateFolder, name)",
		"必须复制到 UpdateFolder 下的同名文件（两件套同名不同名都不许错位）")
	Contains(t, svc, `filepath.Join(updateFolder, "client.json")`,
		"client.json 的目标必须是 UpdateFolder/client.json（不得带临时后缀）")
	Contains(t, svc, "filepath.Rel(updateFolder, mainExeFull)",
		"相对路径必须以同级 UpdateFolder 为基准、以绝对主程序路径为目标")
	Contains(t, svc, "MustCloseProcessName", "必须写 must_close_process_name")
	Contains(t, svc, "strings.TrimSuffix(filepath.Base(mainExeFull), filepath.Ext(mainExeFull))",
		"关进程名缺省时必须从主程序名推导（否则更新前不会关掉主程序）")

	// ── ①b 强制更新三态：GUI 复选框必须翻译成 --set / --clear 的显式一侧 ──
	// 审查 B7：CLI 改成三态后，"不传"=保持服务端原值；GUI 若在取消勾选时仍不传，
	// 用户看到的"关闭强制更新"不会生效。这里把两侧源码契约钉死。
	guiRoot := filepath.Join(RepoRoot, "publish", "publish-gui-go")
	cliSrc := ReadText(filepath.Join(guiRoot, "cli.go"))
	Contains(t, cliSrc, `args = append(args, "--set-force-update")`,
		"GUI 推送必须能把强制更新显式置 true")
	Contains(t, cliSrc, `args = append(args, "--clear-force-update")`,
		"GUI 推送必须能把强制更新显式置 false（不能靠不传参数）")
	detailSrc := ReadText(filepath.Join(guiRoot, "app_detail.go"))
	Contains(t, detailSrc, "if d.forceKnown {",
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
// 用例 127 — 发布 GUI 原子写：先写 .tmp 再一步落位，首次写不得只留 .tmp
//
// ① 源码契约：writeFileAtomic 的形态必须是"CreateTemp 写 .tmp → Sync → Rename 落位"，
//    且失败时清掉 .tmp。历史 C# 缺陷的根因是 System.IO 的 ReplaceFile **要求目标已存在**，
//    无条件 Replace 让**首次写必然失败**，磁盘上只留下 .tmp —— 即
//    "client.json 被命名成 client.json.tmp"的现场（不是文件名拼错）。
//    Go 的 os.Rename 在所有支持平台上都按"替换目标"语义执行（Windows 走 MoveFileEx），
//    目标不存在与已存在都能成，天然规避该缺陷。
// ② 消费侧：把 client.json 改名成 client.json.tmp（复现缺陷现场），跑 check_update 必须
//    明确失败并把原因指向 client.json —— 这种现场对用户来说就是"更新器不可用"。
// ────────────────────────────────────────────────────────────────
func TestCase127(t *testing.T) {
	// ── ① 源码契约：先写 .tmp、再一步落位，失败不留 .tmp ──
	svc := ReadText(guiConfigSource(t))
	Contains(t, svc, "func writeFileAtomic(path string, data []byte) error",
		"配置落盘必须走统一的原子写入口")
	Contains(t, svc, `os.CreateTemp(dir, filepath.Base(path)+".*.tmp")`,
		"原子写先写 .tmp（同目录临时文件，保证 rename 同盘原子）")
	Contains(t, svc, "tmp.Sync()", "临时文件必须 Sync 落盘后再 rename（崩溃安全）")
	Contains(t, svc, "os.Rename(name, path)",
		"落位必须用 Rename：目标不存在与已存在都要能成，首次写不得只留 .tmp")
	Contains(t, svc, "defer os.Remove(name)",
		"落位失败必须清掉 .tmp（用户可见后果：目录里只剩 client.json.tmp）")

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
