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

// TestShortcutFindAndUpdate 验证对候选列表的精确解析：
//  1. 目标 exe 文件名一致且位于应用根目录树内 → 命中；
//  2. 文件名不同的 exe、应用根目录外的同名 exe → 不命中；
//  3. update 模式把命中项改指向新 exe。
func TestShortcutFindAndUpdate(t *testing.T) {
	root, err := ioutil.TempDir("", "shortcut-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)
	outRoot := root + "-outside"
	defer os.RemoveAll(outRoot)

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

	dir1 := filepath.Join(root, "dir1")
	dir2 := filepath.Join(root, "dir2", "sub")
	if err := os.MkdirAll(dir1, 0755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.MkdirAll(dir2, 0755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}

	lnk1 := filepath.Join(dir1, "MyApp.lnk")
	lnk2 := filepath.Join(dir2, "Another.lnk")
	lnkDecoy := filepath.Join(dir1, "Decoy.lnk")
	lnkOutside := filepath.Join(dir1, "Outside.lnk")
	createShortcutViaVBS(t, lnk1, oldExe, filepath.Dir(oldExe))
	createShortcutViaVBS(t, lnk2, oldExe, filepath.Dir(oldExe))
	createShortcutViaVBS(t, lnkDecoy, otherExe, filepath.Dir(otherExe))
	createShortcutViaVBS(t, lnkOutside, outExe, filepath.Dir(outExe))

	candidates := []string{lnk1, lnk2, lnkDecoy, lnkOutside}
	norm := func(p string) string { return strings.ToLower(filepath.Clean(p)) }

	// 1) find
	found, err := runShortcutVBSOnCandidates(oldExe, appRoot, "find", candidates)
	if err != nil {
		t.Fatalf("find 失败: %v", err)
	}
	foundSet := make(map[string]bool)
	for _, p := range found {
		foundSet[norm(p)] = true
	}
	if !foundSet[norm(lnk1)] {
		t.Errorf("应找到 %s，实际 %v", lnk1, found)
	}
	if !foundSet[norm(lnk2)] {
		t.Errorf("应找到 %s，实际 %v", lnk2, found)
	}
	if foundSet[norm(lnkDecoy)] {
		t.Errorf("不应匹配不同文件名的 exe: %s", lnkDecoy)
	}
	if foundSet[norm(lnkOutside)] {
		t.Errorf("不应匹配应用根目录外的同名 exe: %s", lnkOutside)
	}

	// 2) update：改指向新版本 exe
	updated, err := runShortcutVBSOnCandidates(newExe, appRoot, "update", candidates)
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
	if got := resolveShortcutTarget(t, lnkDecoy); !strings.EqualFold(got, otherExe) {
		t.Errorf("Decoy 不应被修改，实际指向 %s", got)
	}
	if got := resolveShortcutTarget(t, lnkOutside); !strings.EqualFold(got, outExe) {
		t.Errorf("Outside 不应被修改，实际指向 %s", got)
	}
}

// TestCollectShortcutCandidates 验证字节预过滤：能命中目标 exe 的 .lnk，
// 排除不相关的 .lnk（含同名但字节里没有 exe 名的场景）。
func TestCollectShortcutCandidates(t *testing.T) {
	root, err := ioutil.TempDir("", "shortcut-prefilter")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	appDir := filepath.Join(root, "app")
	dir := filepath.Join(root, "desktop")
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	exe := filepath.Join(appDir, "App.exe")
	if err := ioutil.WriteFile(exe, []byte("x"), 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	lnk1 := filepath.Join(dir, "A.lnk")
	lnk2 := filepath.Join(dir, "B.lnk")
	lnkOther := filepath.Join(dir, "C.lnk")
	createShortcutViaVBS(t, lnk1, exe, appDir)
	createShortcutViaVBS(t, lnk2, exe, appDir)
	createShortcutViaVBS(t, lnkOther, filepath.Join(appDir, "Other.exe"), appDir)

	candidates := collectShortcutCandidates("App.exe", []string{dir})
	if len(candidates) != 2 {
		t.Fatalf("预过滤应命中 2 个 .lnk，实际 %v", candidates)
	}
	norm := func(p string) string { return strings.ToLower(filepath.Clean(p)) }
	if !(norm(candidates[0]) == norm(lnk1) || norm(candidates[0]) == norm(lnk2)) {
		t.Errorf("候选列表异常: %v", candidates)
	}
	if norm(candidates[0]) == norm(candidates[1]) {
		t.Fatalf("候选列表重复: %v", candidates)
	}
	for _, c := range candidates {
		if norm(c) == norm(lnkOther) {
			t.Errorf("不应命中其他 exe 的快捷方式: %s", c)
		}
	}
}

// TestShortcutEmptyExe 验证空 exe 路径直接报错
func TestShortcutEmptyExe(t *testing.T) {
	if _, err := FindShortcutsForExe("", "C:\\"); err == nil {
		t.Fatal("空 exe 路径应返回错误")
	}
}
