// +build windows

package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

// 通过 cscript + VBS（WScript.Shell COM）解析/修改 .lnk 快捷方式。
// 选择 VBS 的原因：Windows XP 自带 Windows Script Host（cscript.exe + WSH），
// WScript.Shell.CreateShortcut 是解析/创建快捷方式的标准方式，兼容性好；
// 纯 Go 手写解析 .lnk 二进制（MS-SHLLINK）非常脆弱。
//
// 脚本约定：
//   - 参数0 = 主程序 exe 绝对路径（匹配基准/更新目标）
//   - 参数1 = 应用包根目录（过滤同目录树下的快捷方式，避免误改其他同名 exe 的快捷方式）
//   - 参数2 = 模式：find（只查找）/ update（查找并改指向）
//   - 参数3 = 要扫描的根目录列表（分号分隔）；为空时使用系统标准快捷方式目录
//   - 参数4 = 结果输出文件（UTF-8，每行 "SHORTCUT:<完整路径>"）
// 结果写入文件而非 stdout，避免依赖管道/控制台编码（cscript 的 stdout 在不同环境下
// 可能是 ANSI/UTF-16，文件方式最稳，也兼容中文路径）。

const shortcutFinderVBS = `Option Explicit
Dim fso, shell, targetExe, appRoot, mode, rootsCsv, roots, root, outFile, out

Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("WScript.Shell")

targetExe = LCase(WScript.Arguments(0))
appRoot = LCase(WScript.Arguments(1))
mode = LCase(WScript.Arguments(2))
rootsCsv = WScript.Arguments(3)
outFile = WScript.Arguments(4)

' 未指定根目录时，使用系统标准快捷方式目录（自动适配 XP/Win7+ 与本地化名称）
If rootsCsv = "" Then
  rootsCsv = shell.SpecialFolders("Desktop") & ";" & _
             shell.SpecialFolders("AllUsersDesktop") & ";" & _
             shell.SpecialFolders("Programs") & ";" & _
             shell.SpecialFolders("AllUsersPrograms") & ";" & _
             shell.SpecialFolders("StartMenu") & ";" & _
             shell.ExpandEnvironmentStrings("%APPDATA%\Microsoft\Internet Explorer\Quick Launch") & ";" & _
             shell.ExpandEnvironmentStrings("%APPDATA%\Microsoft\Internet Explorer\Quick Launch\User Pinned\TaskBar")
End If

Set out = CreateObject("ADODB.Stream")
out.Type = 2
out.Charset = "utf-8"
out.Open

roots = Split(rootsCsv, ";")
For Each root In roots
  If fso.FolderExists(root) Then Call ScanFolder(root)
Next

out.SaveToFile outFile, 2
out.Close

Sub ScanFolder(folderPath)
  Dim folder, f, subFolder, sc, t, targetName, exeName
  On Error Resume Next
  Set folder = fso.GetFolder(folderPath)
  If Err.Number <> 0 Then Err.Clear : Exit Sub
  On Error GoTo 0
  For Each f In folder.Files
    If LCase(fso.GetExtensionName(f.Name)) = "lnk" Then
      On Error Resume Next
      Set sc = shell.CreateShortcut(f.Path)
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
          out.WriteText "SHORTCUT:" & f.Path & vbCrLf
        End If
      End If
      On Error GoTo 0
    End If
  Next
  For Each subFolder In folder.SubFolders
    Call ScanFolder(subFolder.Path)
  Next
End Sub
`

// FindShortcutsForExe 查找系统上所有指向指定主程序 exe 的快捷方式（.lnk）。
// exePath：主程序 exe 的绝对路径；appRoot：应用包根目录（用于过滤，只匹配该目录树下的快捷方式）。
func FindShortcutsForExe(exePath, appRoot string) ([]string, error) {
	return runShortcutVBS(exePath, appRoot, "find", "")
}

// UpdateShortcutsForExe 将系统上所有指向该应用主程序 exe 的快捷方式改指向 exePath
// （即更新到最新版本目录），返回被更新的快捷方式列表。
func UpdateShortcutsForExe(exePath, appRoot string) ([]string, error) {
	return runShortcutVBS(exePath, appRoot, "update", "")
}

// runShortcutVBS 执行快捷方式查找/更新脚本。rootsCsv 为空时使用系统标准目录（测试可传入自定义目录）。
func runShortcutVBS(exePath, appRoot, mode, rootsCsv string) ([]string, error) {
	if exePath == "" {
		return nil, fmt.Errorf("shortcut: exe path is empty")
	}
	f, err := ioutil.TempFile("", "aly_shortcut_*.vbs")
	if err != nil {
		return nil, fmt.Errorf("shortcut: create temp vbs failed: %v", err)
	}
	vbsPath := f.Name()
	f.Close()
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

	cmd := exec.Command("cscript.exe", "//Nologo", vbsPath, exePath, appRoot, mode, rootsCsv, outFile)
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
