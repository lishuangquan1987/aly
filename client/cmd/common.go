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
	"clientupdator/client/util"
)

// printJSON outputs value as JSON to stdout
func printJSON(v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

// normalizePath converts backslashes to forward slashes
func normalizePath(p string) string {
	return strings.Replace(p, "\\", "/", -1)
}

// stripVPrefix removes leading V/v from version string
func stripVPrefix(v string) string {
	if len(v) > 0 && (v[0] == 'V' || v[0] == 'v') {
		return v[1:]
	}
	return v
}

// compareVersion returns 1 if v1>v2, -1 if v1<v2, 0 if equal
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

// closeProcessesGracefully sends WM_CLOSE then waits, then force kills
func closeProcessesGracefully(names []string, timeout time.Duration) {
	for _, name := range names {
		pids, _ := util.FindProcessesByName(name)
		for _, pid := range pids {
			util.SendCloseMessageToProcess(pid)
		}
	}
	util.KillProcessesAndWait(names, timeout)
}

// atomicReplace atomically replaces mainFolder with its update/{version} directory
func atomicReplace(mainFolder string, version string) error {
	oldDir := mainFolder + "_old"
	mainParentDir := filepath.Dir(mainFolder)
	tempUpdateDir := filepath.Join(mainParentDir, ".temp_update_"+version)
	
	// 1. 清理之前的 _old 和临时目录
	os.RemoveAll(oldDir)
	os.RemoveAll(tempUpdateDir)
	
	updateDir := filepath.Join(mainFolder, "update")
	updateVersionDir := filepath.Join(updateDir, version)
	
	// 2. 先将整个 update 目录移到临时位置（保留所有备份版本）
	tempWholeUpdateDir := filepath.Join(mainParentDir, ".temp_whole_update")
	os.RemoveAll(tempWholeUpdateDir)
	if err := os.Rename(updateDir, tempWholeUpdateDir); err != nil {
		return fmt.Errorf("move whole update dir to temp failed: %v", err)
	}
	
	// 3. 将目标版本目录移出来
	tempVersionDir := filepath.Join(mainParentDir, ".temp_version_"+version)
	if err := os.Rename(filepath.Join(tempWholeUpdateDir, version), tempVersionDir); err != nil {
		// 失败时将 update 目录移回去
		os.Rename(tempWholeUpdateDir, updateDir)
		return fmt.Errorf("move version dir failed: %v", err)
	}
	
	// 4. 将当前目录重命名为 _old
	if err := os.Rename(mainFolder, oldDir); err != nil {
		// 失败时回滚
		os.Rename(tempVersionDir, filepath.Join(tempWholeUpdateDir, version))
		os.Rename(tempWholeUpdateDir, updateDir)
		return fmt.Errorf("rename current to _old failed: %v", err)
	}
	
	// 5. 将临时版本目录重命名为主目录
	if err := os.Rename(tempVersionDir, mainFolder); err != nil {
		// 失败时回滚
		os.Rename(oldDir, mainFolder)
		os.Rename(tempVersionDir, filepath.Join(tempWholeUpdateDir, version))
		os.Rename(tempWholeUpdateDir, updateDir)
		return fmt.Errorf("rename temp version dir to main failed: %v", err)
	}
	
	// 6. 将整个 update 目录（包含所有备份）移回新的 mainFolder
	newUpdateDir := filepath.Join(mainFolder, "update")
	if err := os.Rename(tempWholeUpdateDir, newUpdateDir); err != nil {
		// 即使这步失败也没关系，只是备份版本丢失，但更新已成功
		fmt.Fprintf(os.Stderr, "warning: failed to move backup versions: %v\n", err)
	}
	
	// 7. 清理 _old
	os.RemoveAll(oldDir)
	return nil
}

// launchMainExe starts the main executable
func launchMainExe(cfg *config.Config) {
	exeDir, err := config.ExeDir()
	if err != nil {
		return
	}
	exePath := filepath.Join(exeDir, cfg.MainExeRelativePath)
	cmd := exec.Command(exePath)
	cmd.Start()
}

// runScript executes a post-update script, logs to update.log
func runScript(scriptPath, logPath string) {
	if _, err := os.Stat(scriptPath); err != nil {
		logToFile(logPath, fmt.Sprintf("script not found: %s", scriptPath))
		return
	}

	logToFile(logPath, fmt.Sprintf("executing script: %s", scriptPath))

	cmd := exec.Command("cmd", "/c", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logToFile(logPath, fmt.Sprintf("script failed: %v, output: %s", err, string(output)))
	} else {
		logToFile(logPath, fmt.Sprintf("script succeeded, output: %s", string(output)))
	}
}

func logToFile(logPath, message string) {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, message))
}
