// +build windows

// Package app 负责应用装配、生命周期与少量配置读取。
package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

// clientConfig 只声明 UI 需要的字段。
//
// 注意：这里刻意不 import aly/client/aly-client/config —— 按 AGENTS.md
// 「禁止共享 config/cmd/util」，那个包会拽入 shared.json 解析、崩溃恢复、
// 磁盘副作用等 UI 不需要的逻辑。UI 只需要一个字段。
type clientConfig struct {
	MainExeRelativePath string `json:"main_exe_relative_path"`
}

// ReadMainExeRelativePath 读取 UpdateFolder/client.json 的 main_exe_relative_path。
//
// 只读这一个字段；不解析 shared.json，不做任何写操作。
func ReadMainExeRelativePath(updateFolder string) (string, error) {
	path := filepath.Join(updateFolder, "client.json")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %v", path, err)
	}
	var c clientConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return "", fmt.Errorf("parse %s: %v", path, err)
	}
	if c.MainExeRelativePath == "" {
		return "", fmt.Errorf("%s: main_exe_relative_path is empty", path)
	}
	return c.MainExeRelativePath, nil
}

// MainFolder 由 UpdateFolder 与 main_exe_relative_path 推导主程序目录。
//
// main_exe_relative_path 是相对 UpdateFolder 的路径，例如
// "../win-x64/YOFC.OTDR3001.exe" → 主程序目录为 <PackageFolder>/win-x64。
// 该目录即 aly-client 的 MainFolder，会被 apply_update 原子改名，
// 也是 check_self_update 比对 aly-client.exe 的位置。
func MainFolder(updateFolder, mainExeRelativePath string) (string, error) {
	if mainExeRelativePath == "" {
		return "", fmt.Errorf("main_exe_relative_path is empty")
	}
	if updateFolder == "" {
		return "", fmt.Errorf("updateFolder is empty")
	}
	abs := filepath.Join(updateFolder, mainExeRelativePath)
	return filepath.Dir(abs), nil
}
