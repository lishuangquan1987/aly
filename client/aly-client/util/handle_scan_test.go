// +build windows

package util

import (
	"strings"
	"testing"
)

// TestHandleNameMatches 验证句柄对象名匹配逻辑（前缀 == 目标目录 或 目标目录下任意路径）
func TestHandleNameMatches(t *testing.T) {
	prefixes := []string{`\??\c:\otdr3001\win-x64`}
	cases := []struct {
		name string
		want bool
	}{
		{`\??\c:\otdr3001\win-x64`, true},                    // 目录本身（如进程 CWD）
		{`\??\c:\otdr3001\win-x64\app.exe`, true},            // 目录内文件
		{`\??\c:\otdr3001\win-x64\config\x.ini`, true},       // 子目录内文件
		{`\??\c:\otdr3001\win-x642`, false},                  // 前缀相似但不同目录
		{`\??\c:\otdr3001\win-x64_1.0\app.exe`, false},       // 其他版本目录
		{`\device\harddiskvolume2\otdr3001\win-x64`, false},  // 未在 prefixes 中（由 buildTargetNTForms 补充）
		{`\??\e:\yofc\code\...\win-x64`, false},              // 其他盘的同名目录
	}
	for _, c := range cases {
		if got := handleNameMatches(c.name, prefixes); got != c.want {
			t.Errorf("handleNameMatches(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestBuildTargetNTForms 验证 NT 路径形态生成：至少包含 \??\ 形式，且盘符映射出 \device\ 形式
func TestBuildTargetNTForms(t *testing.T) {
	forms := buildTargetNTForms(`C:\OTDR3001\win-x64`)
	if len(forms) == 0 {
		t.Fatal("buildTargetNTForms 不应返回空")
	}
	if forms[0] != `\??\c:\otdr3001\win-x64` {
		t.Errorf("首个形态应为 \\??\\c:\\otdr3001\\win-x64，实际 %q", forms[0])
	}
	hasDevice := false
	for _, f := range forms[1:] {
		if strings.HasPrefix(f, `\device\`) && strings.HasSuffix(f, `\otdr3001\win-x64`) {
			hasDevice = true
		}
	}
	if !hasDevice {
		t.Errorf("应包含 \\device\\ 形态的盘符映射，实际 %v", forms)
	}
}
