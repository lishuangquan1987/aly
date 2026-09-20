// +build windows

package util

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeAndRunVBS 写 VBS 并运行，输出到指定文件
func writeAndRunVBS(t *testing.T, vbsContent, outFile string) {
	t.Helper()
	vbsPath := outFile + ".vbs"
	if err := ioutil.WriteFile(vbsPath, []byte(vbsContent), 0644); err != nil {
		t.Fatalf("写 VBS 失败: %v", err)
	}
	defer os.Remove(vbsPath)
	cmd := exec.Command("cscript.exe", "//Nologo", vbsPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("运行 VBS 失败: %v (%s)", err, string(out))
	}
}

// createShortcutViaVBS 用 cscript 创建一个 .lnk（测试辅助）
func createShortcutViaVBS(t *testing.T, lnkPath, targetPath, workDir string) {
	t.Helper()
	vbs := "Set sh = CreateObject(\"WScript.Shell\")\r\n" +
		"Set sc = sh.CreateShortcut(\"" + lnkPath + "\")\r\n" +
		"sc.TargetPath = \"" + targetPath + "\"\r\n" +
		"sc.WorkingDirectory = \"" + workDir + "\"\r\n" +
		"sc.Save()\r\n"
	writeAndRunVBS(t, vbs, lnkPath+".mk")
}

// resolveShortcutTarget 解析 .lnk 目标（测试辅助，结果写入 UTF-8 文件后读取）
func resolveShortcutTarget(t *testing.T, lnkPath string) string {
	t.Helper()
	outFile := lnkPath + ".target"
	os.Remove(outFile)
	vbs := "Set sh = CreateObject(\"WScript.Shell\")\r\n" +
		"Set sc = sh.CreateShortcut(\"" + lnkPath + "\")\r\n" +
		"Set st = CreateObject(\"ADODB.Stream\")\r\n" +
		"st.Type = 2\r\n" +
		"st.Charset = \"utf-8\"\r\n" +
		"st.Open\r\n" +
		"st.WriteText sc.TargetPath\r\n" +
		"st.SaveToFile \"" + outFile + "\", 2\r\n" +
		"st.Close\r\n"
	writeAndRunVBS(t, vbs, lnkPath+".rs")
	defer os.Remove(outFile)
	data, err := ioutil.ReadFile(outFile)
	if err != nil {
		t.Fatalf("读取目标失败: %v", err)
	}
	// 去掉 ADODB.Stream utf-8 写入的 BOM
	s := string(data)
	s = strings.TrimPrefix(s, "\uFEFF")
	return strings.TrimSpace(s)
}

// TestShortcutFindAndUpdate 验证：
//  1. 根据 exe 能在一组根目录中找到所有指向它的快捷方式（含嵌套子目录）；
//  2. 文件名相同的其他 exe、应用根目录外的同名 exe 的快捷方式不会被误匹配；
//  3. update 模式会把所有匹配的快捷方式改指向新 exe。
func TestShortcutFindAndUpdate(t *testing.T) {
	root, err := ioutil.TempDir("", "shortcut-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)
	outRoot := root + "-outside"
	defer os.RemoveAll(outRoot)

	// 目录结构：
	//   root/app/App.exe           <- 主程序（旧版本目录）
	//   root/appNew/App.exe        <- 更新后的新版本目录
	//   root/other/Other.exe       <- 文件名不同的 exe
	//   root-outside/App.exe       <- 应用根目录外的同名 exe
	//   root/desktop/MyApp.lnk     -> app/App.exe
	//   root/startmenu/sub/Other.lnk -> app/App.exe
	//   root/desktop/Decoy.lnk     -> other/Other.exe（文件名不同，不应匹配）
	//   root/desktop/Outside.lnk   -> root-outside/App.exe（不在应用根目录内，不应匹配）
	appRoot := root
	oldExe := filepath.Join(root, "app", "App.exe")
	newExe := filepath.Join(root, "appNew", "App.exe")
	otherExe := filepath.Join(root, "other", "Other.exe")
	outExe := filepath.Join(outRoot, "App.exe")

	for _, p := range []string{oldExe, newExe, otherExe, outExe} {
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if err := ioutil.WriteFile(p, []byte("fake exe"), 0644); err != nil {
			t.Fatalf("写入文件失败: %v", err)
		}
	}

	desktop := filepath.Join(root, "desktop")
	startMenu := filepath.Join(root, "startmenu", "sub")
	if err := os.MkdirAll(desktop, 0755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.MkdirAll(startMenu, 0755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}

	lnk1 := filepath.Join(desktop, "MyApp.lnk")
	lnk2 := filepath.Join(startMenu, "Another.lnk")
	lnkDecoy := filepath.Join(desktop, "Decoy.lnk")
	lnkOutside := filepath.Join(desktop, "Outside.lnk")
	createShortcutViaVBS(t, lnk1, oldExe, filepath.Dir(oldExe))
	createShortcutViaVBS(t, lnk2, oldExe, filepath.Dir(oldExe))
	createShortcutViaVBS(t, lnkDecoy, otherExe, filepath.Dir(otherExe))
	createShortcutViaVBS(t, lnkOutside, outExe, filepath.Dir(outExe))

	rootsCsv := desktop + ";" + startMenu

	// 1) find：找到 2 个匹配的快捷方式，排除 Decoy 与 Outside
	found, err := runShortcutVBS(oldExe, appRoot, "find", rootsCsv)
	if err != nil {
		t.Fatalf("find 失败: %v", err)
	}
	norm := func(p string) string { return strings.ToLower(filepath.Clean(p)) }
	foundSet := make(map[string]bool)
	for _, p := range found {
		foundSet[norm(p)] = true
	}
	if !foundSet[norm(lnk1)] {
		t.Errorf("应找到 %s，实际 %v", lnk1, found)
	}
	if !foundSet[norm(lnk2)] {
		t.Errorf("应找到 %s（嵌套子目录），实际 %v", lnk2, found)
	}
	if foundSet[norm(lnkDecoy)] {
		t.Errorf("不应匹配不同文件名的 exe: %s", lnkDecoy)
	}
	if foundSet[norm(lnkOutside)] {
		t.Errorf("不应匹配应用根目录外的同名 exe: %s", lnkOutside)
	}

	// 2) update：改指向新版本 exe
	updated, err := runShortcutVBS(newExe, appRoot, "update", rootsCsv)
	if err != nil {
		t.Fatalf("update 失败: %v", err)
	}
	if len(updated) != 2 {
		t.Fatalf("应更新 2 个快捷方式，实际 %v", updated)
	}
	if got := resolveShortcutTarget(t, lnk1); !strings.EqualFold(got, newExe) {
		t.Errorf("lnk1 应指向 %s，实际 %s", newExe, got)
	}
	if got := resolveShortcutTarget(t, lnk2); !strings.EqualFold(got, newExe) {
		t.Errorf("lnk2 应指向 %s，实际 %s", newExe, got)
	}
	// Decoy / Outside 不应被动过
	if got := resolveShortcutTarget(t, lnkDecoy); !strings.EqualFold(got, otherExe) {
		t.Errorf("Decoy 不应被修改，实际指向 %s", got)
	}
	if got := resolveShortcutTarget(t, lnkOutside); !strings.EqualFold(got, outExe) {
		t.Errorf("Outside 不应被修改，实际指向 %s", got)
	}
}

// TestShortcutEmptyExe 验证空 exe 路径直接报错
func TestShortcutEmptyExe(t *testing.T) {
	if _, err := FindShortcutsForExe("", "C:\\"); err == nil {
		t.Fatal("空 exe 路径应返回错误")
	}
}
