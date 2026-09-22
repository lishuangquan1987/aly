// +build windows

package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// 快捷方式（.lnk）查找/重指向。
//
// 性能设计：先由 Go 侧做"字节预过滤"——.lnk 文件内嵌目标路径（UTF-16LE），
// 读取文件字节、大小写不敏感地搜索主程序 exe 文件名，只有命中的少数候选
// 才交给 cscript（WScript.Shell COM）做精确解析。实测 400+ 个快捷方式
// 预过滤约 200ms，全程 <300ms，避免了对每个快捷方式都启动 COM 解析。
//
// cscript + VBS 的原因：Windows XP 自带 Windows Script Host（cscript.exe + WSH），
// WScript.Shell.CreateShortcut 是解析/修改快捷方式的标准方式；纯 Go 手写解析
// .lnk 二进制（MS-SHLLINK）非常脆弱。结果写 UTF-8 文件而非 stdout，避免
// 管道/控制台编码问题（含中文路径）。

// shortcutFinderVBS 处理 Go 侧预过滤出的候选 .lnk 列表。
//   - 参数0 = 主程序 exe 绝对路径（匹配基准/更新目标）
//   - 参数1 = 应用包根目录（过滤同目录树下的快捷方式）
//   - 参数2 = 模式：find / update
//   - 参数3 = 候选 .lnk 完整路径列表（| 分隔）
//   - 参数4 = 结果输出文件（UTF-8，每行 "SHORTCUT:<完整路径>"）
const shortcutFinderVBS = `Option Explicit
Dim fso, shell, targetExe, appRoot, mode, lnkList, lnks, lnk, outFile, out

Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("WScript.Shell")

targetExe = LCase(WScript.Arguments(0))
appRoot = LCase(WScript.Arguments(1))
mode = LCase(WScript.Arguments(2))
lnkList = WScript.Arguments(3)
outFile = WScript.Arguments(4)

Set out = CreateObject("ADODB.Stream")
out.Type = 2
out.Charset = "utf-8"
out.Open

lnks = Split(lnkList, "|")
For Each lnk In lnks
  If lnk <> "" Then Call ProcessLnk(lnk)
Next

out.SaveToFile outFile, 2
out.Close

Sub ProcessLnk(lnkPath)
  Dim sc, t, targetName, exeName
  On Error Resume Next
  Set sc = shell.CreateShortcut(lnkPath)
  If Err.Number = 0 Then
    t = LCase(sc.TargetPath)
    targetName = LCase(fso.GetFileName(sc.TargetPath))
    exeName = LCase(fso.GetFileName(targetExe))
    ' 目标 exe 文件名一致且位于应用包目录树内（精确前缀匹配，防止 appRoot 是其他路径的前缀）
    If targetName = exeName And (t = appRoot Or InStr(t, appRoot & "\") = 1) Then
      If mode = "update" Then
        sc.TargetPath = WScript.Arguments(0)
        sc.WorkingDirectory = fso.GetParentFolderName(WScript.Arguments(0))
        sc.Save()
      End If
      out.WriteText "SHORTCUT:" & lnkPath & vbCrLf
    End If
  End If
  On Error GoTo 0
End Sub
`

// FindShortcutsForExe 查找系统上所有指向指定主程序 exe 的快捷方式（.lnk）。
// exePath：主程序 exe 的绝对路径；appRoot：应用包根目录（用于过滤，只匹配该目录树下的快捷方式）。
func FindShortcutsForExe(exePath, appRoot string) ([]string, error) {
	return runShortcutVBS(exePath, appRoot, "find")
}

// UpdateShortcutsForExe 将系统上所有指向该应用主程序 exe 的快捷方式改指向 exePath
// （即更新到最新版本目录），返回被更新的快捷方式列表。
func UpdateShortcutsForExe(exePath, appRoot string) ([]string, error) {
	return runShortcutVBS(exePath, appRoot, "update")
}

// runShortcutVBS 执行快捷方式查找/更新：Go 侧字节预过滤 + cscript 精确解析。
func runShortcutVBS(exePath, appRoot, mode string) ([]string, error) {
	if exePath == "" {
		return nil, fmt.Errorf("shortcut: exe path is empty")
	}
	exeName := filepath.Base(exePath)
	candidates := collectShortcutCandidates(exeName, defaultShortcutRoots())
	if len(candidates) == 0 {
		return nil, nil // 没有候选，无需启动 cscript
	}
	return runShortcutVBSOnCandidates(exePath, appRoot, mode, candidates)
}

// runShortcutVBSOnCandidates 对给定的候选 .lnk 列表执行精确解析（find/update）。
// 候选由调用方提供（生产走 collectShortcutCandidates，测试可注入临时目录）。
func runShortcutVBSOnCandidates(exePath, appRoot, mode string, candidates []string) ([]string, error) {
	if len(candidates) == 0 {
		return nil, nil
	}

	// 注意：Go 1.10 的 ioutil.TempFile 不支持 "*" 占位符（Go 1.11+ 特性），
	// 若 pattern 含 "*"，随机串会拼在末尾导致扩展名变成 ".vbs87321" 而非 ".vbs"，
	// cscript 会拒绝执行（#4）。因此先按无扩展名前缀创建，再手动补 ".vbs" 后缀。
	f, err := ioutil.TempFile("", "aly_shortcut_")
	if err != nil {
		return nil, fmt.Errorf("shortcut: create temp vbs failed: %v", err)
	}
	vbsPath := f.Name() + ".vbs"
	f.Close()
	os.Remove(f.Name()) // 移除占位文件，使用带 .vbs 后缀的路径
	// VBScript 的多行 If...Then 与 _ 续行要求 CRLF 换行，LF 会导致编译错误
	content := strings.Replace(shortcutFinderVBS, "\n", "\r\n", -1)
	if err := ioutil.WriteFile(vbsPath, []byte(content), 0644); err != nil {
		os.Remove(vbsPath)
		return nil, fmt.Errorf("shortcut: write temp vbs failed: %v", err)
	}
	defer os.Remove(vbsPath)

	outFile := vbsPath + ".out"
	os.Remove(outFile)
	defer os.Remove(outFile)

	cmd := exec.Command("cscript.exe", "//Nologo", vbsPath,
		exePath, appRoot, mode, strings.Join(candidates, "|"), outFile)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("shortcut: run cscript failed: %v", err)
	}

	data, err := ioutil.ReadFile(outFile)
	if err != nil {
		return nil, fmt.Errorf("shortcut: read result failed: %v", err)
	}
	// ADODB.Stream 以 utf-8 写入时会带 BOM，剥掉
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))

	var matches []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(line, "SHORTCUT:") {
			matches = append(matches, strings.TrimPrefix(line, "SHORTCUT:"))
		}
	}
	return matches, nil
}

// defaultShortcutRoots 返回系统标准快捷方式目录（兼容 XP 与 Vista+，不存在的跳过）。
func defaultShortcutRoots() []string {
	userProfile := os.Getenv("USERPROFILE")
	appData := os.Getenv("APPDATA")
	public := os.Getenv("PUBLIC")
	programData := os.Getenv("ProgramData")

	roots := []string{
		filepath.Join(userProfile, "Desktop"),
		filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs"),
		filepath.Join(appData, "Microsoft", "Internet Explorer", "Quick Launch"),
		filepath.Join(appData, "Microsoft", "Internet Explorer", "Quick Launch", "User Pinned", "TaskBar"),
	}
	if public != "" {
		roots = append(roots, filepath.Join(public, "Desktop"))
	} else {
		roots = append(roots, `C:\Documents and Settings\All Users\Desktop`)
	}
	if programData != "" {
		roots = append(roots, filepath.Join(programData, "Microsoft", "Windows", "Start Menu", "Programs"))
	} else {
		roots = append(roots, `C:\Documents and Settings\All Users\Start Menu\Programs`)
	}
	// XP 用户开始菜单物理路径
	roots = append(roots, filepath.Join(userProfile, "Start Menu", "Programs"))

	var existing []string
	for _, r := range roots {
		if info, err := os.Stat(r); err == nil && info.IsDir() {
			existing = append(existing, r)
		}
	}
	return existing
}

// collectShortcutCandidates 在给定根目录下枚举 *.lnk，做字节预过滤：
// .lnk 内嵌目标路径（UTF-16LE），大小写不敏感地搜索 exe 文件名，
// 命中才作为候选返回（只有候选才需要 COM 解析，大幅减少耗时）。
func collectShortcutCandidates(exeName string, roots []string) []string {
	needle := utf16LEBytesOf(strings.ToLower(exeName))
	var candidates []string
	for _, root := range roots {
		_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(p), ".lnk") {
				return nil
			}
			b, err := ioutil.ReadFile(p)
			if err != nil || len(b) < len(needle) {
				return nil
			}
			if bytesContainsFold(b, needle) {
				candidates = append(candidates, p)
			}
			return nil
		})
	}
	return candidates
}

// utf16LEBytesOf 将字符串编码为 UTF-16LE 字节
func utf16LEBytesOf(s string) []byte {
	out := make([]byte, 0, len(s)*2)
	for i := 0; i < len(s); i++ {
		out = append(out, s[i], 0)
	}
	return out
}

// bytesContainsFold 大小写不敏感搜索（仅 ASCII 字母大小写折叠）
func bytesContainsFold(haystack, needle []byte) bool {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		ok := true
		for j := 0; j < len(needle); j++ {
			h := haystack[i+j]
			if h >= 'A' && h <= 'Z' {
				h += 'a' - 'A'
			}
			if h != needle[j] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
