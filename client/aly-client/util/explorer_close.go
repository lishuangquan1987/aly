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

// 精准关闭"正在浏览指定文件夹"的 Explorer 窗口。
//
// 与 shortcut.go 相同的实现策略：cscript + VBS（Windows XP 自带 Windows Script Host）。
// 用 Shell.Application COM 枚举所有资源管理器窗口，比较 LocationURL（file:///... 形式）
// 与目标文件夹，命中才调用 w.Quit() 关闭该窗口——不会误关用户其他资源管理器窗口，
// 也不会重启整个 shell（旧实现是杀全部 explorer.exe，会关掉桌面壳与所有窗口）。
//
// 结果写 UTF-8 文件而非 stdout（沿用 shortcut.go 的约定，避免管道/控制台编码问题）。

// closeExplorerVBS 参数：
//   - 参数0 = 目标文件夹绝对路径（比较基准，大小写不敏感）
//   - 参数1 = 结果输出文件（UTF-8，行格式 "CLOSED:<数量>"）
const closeExplorerVBS = `Option Explicit
Dim shell, wnd, target, closed, url, path
Set shell = CreateObject("Shell.Application")
target = LCase(Trim(WScript.Arguments(0)))
closed = 0
For Each wnd In shell.Windows()
  On Error Resume Next
  url = wnd.LocationURL
  If Err.Number = 0 And url <> "" Then
    If LCase(Left(url, 7)) = "file://" Then
      path = Mid(url, 8)
      path = URLDecode(path)
      path = Replace(path, "/", "\")
      Do While Len(path) > 0 And (Right(path, 1) = "\")
        path = Left(path, Len(path) - 1)
      Loop
      Do While Len(path) > 0 And (Left(path, 1) = "\")
        path = Mid(path, 2)
      Loop
      If LCase(path) = target Then
        wnd.Quit()
        closed = closed + 1
      End If
    End If
  End If
  On Error GoTo 0
Next

Dim fso, outFile, out
Set fso = CreateObject("Scripting.FileSystemObject")
outFile = WScript.Arguments(1)
Set out = CreateObject("ADODB.Stream")
out.Type = 2
out.Charset = "utf-8"
out.Open
out.WriteText "CLOSED:" & closed & vbCrLf
out.SaveToFile outFile, 2
out.Close

Function URLDecode(s)
  Dim i, ch, code, out
  out = ""
  i = 1
  Do While i <= Len(s)
    ch = Mid(s, i, 1)
    If ch = "%" And i + 2 <= Len(s) Then
      code = "&H" & Mid(s, i + 1, 2)
      out = out & Chr(code)
      i = i + 3
    Else
      out = out & ch
      i = i + 1
    End If
  Loop
  URLDecode = out
End Function
`

// CloseExplorerWindowsBrowsing 关闭所有正在浏览指定文件夹的 Explorer 窗口，
// 返回关闭的窗口数量。cscript 不可用或 VBS 执行失败时返回错误（调用方决定是否降级）。
func CloseExplorerWindowsBrowsing(folder string) (int, error) {
	if folder == "" {
		return 0, fmt.Errorf("explorer close: folder is empty")
	}
	folder = strings.TrimRight(folder, `\`)

	// 注意：Go 1.10 的 ioutil.TempFile 不支持 "*" 占位符（Go 1.11+ 特性），
	// 若 pattern 含 "*"，随机串会拼在末尾导致扩展名变成 ".vbs87321" 而非 ".vbs"，
	// cscript 会拒绝执行（#4）。因此先按无扩展名前缀创建，再手动补 ".vbs" 后缀。
	f, err := ioutil.TempFile("", "aly_closeexplorer_")
	if err != nil {
		return 0, fmt.Errorf("explorer close: create temp vbs failed: %v", err)
	}
	vbsPath := f.Name() + ".vbs"
	f.Close()
	os.Remove(f.Name()) // 移除占位文件，使用带 .vbs 后缀的路径
	// VBScript 多行语句要求 CRLF 换行，LF 会导致编译错误（与 shortcut.go 一致）
	content := strings.Replace(closeExplorerVBS, "\n", "\r\n", -1)
	if err := ioutil.WriteFile(vbsPath, []byte(content), 0644); err != nil {
		os.Remove(vbsPath)
		return 0, fmt.Errorf("explorer close: write temp vbs failed: %v", err)
	}
	defer os.Remove(vbsPath)

	outFile := vbsPath + ".out"
	os.Remove(outFile)
	defer os.Remove(outFile)

	cmd := exec.Command("cscript.exe", "//Nologo", "//B", vbsPath, folder, outFile)
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("explorer close: run cscript failed: %v", err)
	}

	data, err := ioutil.ReadFile(outFile)
	if err != nil {
		return 0, fmt.Errorf("explorer close: read result failed: %v", err)
	}
	// ADODB.Stream 以 utf-8 写入时会带 BOM，剥掉
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))

	closed := 0
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if strings.HasPrefix(line, "CLOSED:") {
			fmt.Sscanf(strings.TrimPrefix(line, "CLOSED:"), "%d", &closed)
		}
	}
	return closed, nil
}
