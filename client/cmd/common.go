package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"clientupdator/client/config"
	"clientupdator/client/model"
	"clientupdator/client/util"
)

// printOutput 按 isSuccess/errorMsg/data 格式输出 JSON 到 stdout
func printOutput(success bool, errMsg string, data interface{}) {
	out := model.Output{
		IsSuccess: success,
		ErrMsg:    errMsg,
		Data:      data,
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(bytes))
}

// normalizePath 将反斜杠转为正斜杠
func normalizePath(p string) string {
	return strings.Replace(p, "\\", "/", -1)
}

// stripVPrefix 去除版本号前导的 V/v
func stripVPrefix(v string) string {
	if len(v) > 0 && (v[0] == 'V' || v[0] == 'v') {
		return v[1:]
	}
	return v
}

// compareVersion 按 . 分割逐段数值比较：v1 > v2 返回 1，v1 < v2 返回 -1，相等返回 0
func compareVersion(v1, v2 string) int {
	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")
	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}
	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(parts1) {
			n1, _ = strconv.Atoi(parts1[i])
		}
		if i < len(parts2) {
			n2, _ = strconv.Atoi(parts2[i])
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	return 0
}

// closeProcessesGracefully 优雅关闭进程：先 WM_CLOSE，超时后强杀
func closeProcessesGracefully(names []string, timeout time.Duration) {
	for _, name := range names {
		pids, _ := util.FindProcessesByName(name)
		for _, pid := range pids {
			util.SendCloseMessageToProcess(pid)
		}
	}
	util.KillProcessesAndWait(names, timeout)
}

// launchMainExe 启动主程序
func launchMainExe(cfg *config.Config) {
	exeDir, err := config.ExeDir()
	if err != nil {
		util.AppendToLog(".", "update.log", fmt.Sprintf("launch main exe: get exe dir failed: %v", err))
		return
	}
	exePath := filepath.Join(exeDir, cfg.MainExeRelativePath)
	cmd := exec.Command(exePath)
	if err := cmd.Start(); err != nil {
		util.AppendToLog(exeDir, "update.log", fmt.Sprintf("launch main exe failed: %s %v", exePath, err))
	}
}

// filepathFromSlash converts forward-slash paths to OS-specific separators.
// Replaces Go 1.17+ filepath.FromSlash for Go 1.10 compatibility.
func filepathFromSlash(path string) string {
	if os.PathSeparator == '/' {
		return path
	}
	result := make([]byte, len(path))
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			result[i] = os.PathSeparator
		} else {
			result[i] = path[i]
		}
	}
	return string(result)
}

// runScript 执行 post-update 脚本，异步等待完成并记录结果到 update.log
func runScript(scriptPath string) {
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return
	}
	var cmd *exec.Cmd
	if os.PathSeparator == '\\' {
		cmd = exec.Command("cmd", "/c", scriptPath)
	} else {
		cmd = exec.Command("sh", "-c", scriptPath)
	}
	if err := cmd.Start(); err != nil {
		exeDir, dirErr := config.ExeDir()
		if dirErr == nil {
			util.AppendToLog(exeDir, "update.log",
				fmt.Sprintf("script start failed: %s %v", scriptPath, err))
		}
		return
	}
	// Wait asynchronously so we don't block main exe launch
	go func() {
		err := cmd.Wait()
		exeDir, dirErr := config.ExeDir()
		if dirErr != nil {
			return
		}
		if err != nil {
			util.AppendToLog(exeDir, "update.log",
				fmt.Sprintf("script failed: %s %v", scriptPath, err))
		} else {
			util.AppendToLog(exeDir, "update.log",
				fmt.Sprintf("script completed: %s", scriptPath))
		}
	}()
}
