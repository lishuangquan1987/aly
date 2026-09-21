package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
	"aly/client/aly-client/util"
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

// FullConfig 运行时完整配置（client.json + .updator/shared.json + CLI 参数）
type FullConfig struct {
	ExeCfg     *config.Config
	Shared     *config.SharedConfig
	MainFolder string
}

// loadFullConfig 加载完整配置：client.json → MainExeRelativePath → .updator/shared.json
// url/projectName/mainExePath 为 CLI 参数覆盖
func loadFullConfig(url, projectName, mainExePath string) (*FullConfig, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %v", err)
	}
	cfg.MergeFlags(mainExePath)

	mainFolder, err := cfg.MainExeFolderPath()
	if err != nil {
		return nil, err
	}

	shared, err := config.LoadSharedConfig(mainFolder)
	if err != nil {
		return nil, fmt.Errorf("load shared.json: %v", err)
	}

	// CLI 参数覆盖
	if url != "" {
		shared.ServerURL = url
	}
	if projectName != "" {
		shared.ProjectName = projectName
	}

	return &FullConfig{
		ExeCfg:     cfg,
		Shared:     shared,
		MainFolder: mainFolder,
	}, nil
}

// normalizePath 将反斜杠转为正斜杠
func normalizePath(p string) string {
	return strings.Replace(p, "\\", "/", -1)
}

// printProgress 输出下载进度到 stdout，每行统一用 {isSuccess, errorMsg, data} 包裹的 JSON。
// data 中包含 index/total/file/status/file_size/error 字段。
func printProgress(index, total int, file, status string, fileSize int64, errMsg string) {
	out := model.Output{
		IsSuccess: true,
		ErrMsg:    "",
		Data: model.DownloadProgress{
			Index:    index,
			Total:    total,
			File:     file,
			Status:   status,
			FileSize: fileSize,
			Error:    errMsg,
		},
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		return
	}
	fmt.Println(string(bytes))
}

// printProgressFail 输出失败的进度行（isSuccess: false），后跟最终结果。
func printProgressFail(index, total int, file string, fileSize int64, errMsg string) {
	out := model.Output{
		IsSuccess: false,
		ErrMsg:    fmt.Sprintf("%s: %s", file, errMsg),
		Data: model.DownloadProgress{
			Index:    index,
			Total:    total,
			File:     file,
			Status:   "FAIL",
			FileSize: fileSize,
			Error:    errMsg,
		},
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		return
	}
	fmt.Println(string(bytes))
}

// printProgressDone 输出最终完成行（isSuccess: true, data: null）。
func printProgressDone() {
	out := model.Output{
		IsSuccess: true,
		ErrMsg:    "",
		Data:      nil,
	}
	bytes, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		return
	}
	fmt.Println(string(bytes))
}

// stripVPrefix 去除版本号前导的 V/v
func stripVPrefix(v string) string {
	if len(v) > 0 && (v[0] == 'V' || v[0] == 'v') {
		return v[1:]
	}
	return v
}

// compareVersion 按 . 分割逐段数值比较：v1 > v2 返回 1，v1 < v2 返回 -1，相等返回 0
// 非数字字段按 0 处理（如 "V1.0" 中 "V1" 的 "V" 前缀）
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
		pids, err := util.FindProcessesByName(name)
		if err != nil {
			util.AppendToLog(".", "update.log", fmt.Sprintf("closeProcessesGracefully: find process %s failed: %v", name, err))
			continue
		}
		for _, pid := range pids {
			util.SendCloseMessageToProcess(pid)
		}
	}
	if err := util.KillProcessesAndWait(names, timeout); err != nil {
		util.AppendToLog(".", "update.log", fmt.Sprintf("closeProcessesGracefully: kill failed: %v", err))
	}
}

// buildKillWhitelist 可强杀进程白名单：explorer + 主程序 exe 名 + must_close_process_name。
// 白名单之外的占用者只做优雅关闭尝试，不强杀，改为返回错误提示用户手动关闭（#13）。
func buildKillWhitelist(fc *FullConfig) []string {
	names := []string{"explorer"}
	if fc.ExeCfg.MainExeRelativePath != "" {
		base := filepath.Base(fc.ExeCfg.MainExeRelativePath)
		names = append(names, strings.TrimSuffix(base, filepath.Ext(base)))
	}
	names = append(names, fc.ExeCfg.MustCloseProcessName...)
	return names
}

// whitelistPIDs 返回白名单进程名对应的 PID 集合（按进程名查找，查找失败仅记日志）
func whitelistPIDs(names []string) map[uint32]bool {
	wl := make(map[uint32]bool)
	for _, name := range names {
		pids, err := util.FindProcessesByName(name)
		if err != nil {
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("whitelistPIDs: find %s failed: %v", name, err))
			continue
		}
		for _, pid := range pids {
			wl[pid] = true
		}
	}
	return wl
}

// partitionHolders 将持有者 PID 按白名单分区：killable（可强杀）与 blocked（非白名单）。
// 提取为纯函数便于单测（#13）。
func partitionHolders(holders []uint32, wl map[uint32]bool) (killable []uint32, blocked []uint32) {
	for _, pid := range holders {
		if wl[pid] {
			killable = append(killable, pid)
		} else {
			blocked = append(blocked, pid)
		}
	}
	return killable, blocked
}

// formatPidNames 将阻塞进程 PID 格式化为去重、排序的进程名列表（未知名回退 PID）
func formatPidNames(pids []uint32, names map[uint32]string) []string {
	seen := make(map[string]bool)
	var list []string
	for _, pid := range pids {
		name := names[pid]
		if name == "" {
			name = fmt.Sprintf("PID %d", pid)
		}
		if !seen[name] {
			seen[name] = true
			list = append(list, name)
		}
	}
	sort.Strings(list)
	return list
}

// closeProcessesHoldingFolder 探测并结束占用指定文件夹的进程（排除更新器自身），
// 先发 WM_CLOSE 优雅关闭，随后直接强杀（不长时间等待优雅退出）。
// 仅对白名单内进程（must_close_process_name / 主程序 / explorer）强杀；
// 非白名单占用者（记事本/IDE/杀毒等无关进程）不强杀，返回错误提示用户手动关闭（#13）。
func closeProcessesHoldingFolder(folder string, whitelist []string, timeout time.Duration) error {
	selfPid := uint32(os.Getpid())
	pids, err := util.FindProcessesHoldingPath(folder)
	if err != nil {
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("closeProcessesHoldingFolder: find processes holding %s failed: %v", folder, err))
		return nil
	}
	var holders []uint32
	for _, pid := range pids {
		if pid != selfPid {
			holders = append(holders, pid)
		}
	}
	if len(holders) == 0 {
		return nil
	}

	killable, blocked := partitionHolders(holders, whitelistPIDs(whitelist))
	// 先尝试优雅关闭（explorer 等会自行释放句柄），随后直接强杀白名单进程
	for _, pid := range killable {
		util.SendCloseMessageToProcess(pid)
	}
	wait := timeout
	if wait > forceKillWait {
		wait = forceKillWait
	}
	util.ForceKillPIDs(killable, wait)

	if len(blocked) > 0 {
		names := util.FindProcessNamesByPIDs(blocked)
		return fmt.Errorf("以下进程占用更新目录，请关闭后重试: %v", formatPidNames(blocked, names))
	}
	return nil
}

// forceKillWait 强制结束进程后等待退出的上限，避免更新长时间卡在等待上
const forceKillWait = 5 * time.Second

// renameDirWithKill 重命名文件夹，约定三条规则：
//  1. 源文件夹必须存在，否则直接失败；
//  2. 目标文件夹必须不存在：若目标已存在（例如上一次更新留下的旧版本备份），
//     先把它重命名为另一个不冲突的文件夹（to.old / to.old.1 / to.old.2 …）再执行正式重命名。
//     Windows 的 MoveFileEx 无法覆盖非空目录，即使无任何进程占用也会报 Access denied；
//  3. 重命名失败若因进程占用，探测占用 from/to 的进程：白名单内直接结束（排除更新器自身），
//     非白名单占用者返回错误提示用户手动关闭（#13）。
func renameDirWithKill(from, to string, whitelist []string, timeout time.Duration) error {
	const maxAttempts = 5
	const retrySleep = 300 * time.Millisecond
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// 1) 源必须存在（不存在时重试无意义）
		if _, statErr := os.Stat(from); statErr != nil {
			return fmt.Errorf("rename %s -> %s: source not found: %v", from, to, statErr)
		}
		// 2) 目标若存在：先挪到另一个不冲突的文件夹
		if _, statErr := os.Stat(to); statErr == nil {
			aside := nextAsideName(to)
			if asideErr := renameWithKillRetry(to, aside, whitelist, timeout); asideErr != nil {
				// 旁移失败（如非白名单占用）：重试无意义，直接返回
				return fmt.Errorf("move aside %s -> %s: %v", to, aside, asideErr)
			}
		}
		// 3) 正式重命名
		err := os.Rename(from, to)
		if err == nil {
			return nil
		}
		lastErr = err
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("rename %s -> %s attempt %d/%d failed: %v", from, to, attempt, maxAttempts, err))

		// 4) 只强杀白名单进程；非白名单占用者返回错误（重试无意义）
		killable, blocked := findHoldersOf(whitelist, from, to)
		if len(blocked) > 0 {
			names := util.FindProcessNamesByPIDs(blocked)
			return fmt.Errorf("rename %s -> %s: 以下进程占用，请关闭后重试: %v",
				from, to, formatPidNames(blocked, names))
		}
		if len(killable) > 0 {
			wait := timeout
			if wait > forceKillWait {
				wait = forceKillWait
			}
			util.ForceKillPIDs(killable, wait)
		}
		time.Sleep(retrySleep)
	}
	return lastErr
}

// renameWithKillRetry 执行重命名；失败时探测占用 from/to 的进程并直接强杀后重试。
// 调用方需保证目标 to 不存在（由 renameDirWithKill 负责挪开）。
func renameWithKillRetry(from, to string, whitelist []string, timeout time.Duration) error {
	const maxAttempts = 5
	const retrySleep = 300 * time.Millisecond
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := os.Rename(from, to)
		if err == nil {
			return nil
		}
		lastErr = err
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("rename %s -> %s attempt %d/%d failed: %v", from, to, attempt, maxAttempts, err))

		// 只强杀白名单进程；非白名单占用者返回错误（重试无意义）
		killable, blocked := findHoldersOf(whitelist, from, to)
		if len(blocked) > 0 {
			names := util.FindProcessNamesByPIDs(blocked)
			return fmt.Errorf("rename %s -> %s: 以下进程占用，请关闭后重试: %v",
				from, to, formatPidNames(blocked, names))
		}
		if len(killable) > 0 {
			wait := timeout
			if wait > forceKillWait {
				wait = forceKillWait
			}
			util.ForceKillPIDs(killable, wait)
		} else {
			// Restart Manager 探测不到占用者时，通常是资源管理器窗口
			// 打开了该文件夹（Explorer 持目录句柄，RM 检测不到），
			// 关闭/结束 Explorer（系统会自动重启它）。用户已确认：explorer
			// 占用导致重命名失败时杀掉它是预期补救手段（#13）。
			closeExplorerWindows(timeout)
		}
		time.Sleep(retrySleep)
	}
	return lastErr
}

// closeExplorerWindows 关闭资源管理器（先 WM_CLOSE 优雅关闭，随后强杀，Explorer 会自动重启）。
// 用于解除 Explorer 文件夹窗口对目录句柄的占用。
func closeExplorerWindows(timeout time.Duration) {
	pids, err := util.FindProcessesByName("explorer")
	if err != nil {
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("closeExplorerWindows: find explorer failed: %v", err))
		return
	}
	if len(pids) == 0 {
		return
	}
	for _, pid := range pids {
		util.SendCloseMessageToProcess(pid)
	}
	wait := timeout
	if wait > forceKillWait {
		wait = forceKillWait
	}
	util.ForceKillPIDs(pids, wait)
	util.AppendToLog(logDir(), "update.log", "closed explorer windows to release folder handle")
}

// nextAsideName 生成一个不冲突的"挪开目标"名称：to.old、to.old.1、to.old.2 …
// 依次检查，返回第一个不存在的名称，避免与上次残留的 .old 冲突。
func nextAsideName(to string) string {
	aside := to + ".old"
	for i := 1; ; i++ {
		if _, err := os.Stat(aside); os.IsNotExist(err) {
			return aside
		}
		aside = fmt.Sprintf("%s.old.%d", to, i)
	}
}

// findHoldersOf 收集占用指定路径集合的进程 PID（去重并排除更新器自身），
// 按白名单分区为"可强杀"与"需提示用户"两类（#13）。
func findHoldersOf(whitelist []string, paths ...string) (killable []uint32, blocked []uint32) {
	selfPid := uint32(os.Getpid())
	seen := make(map[uint32]bool)
	var holders []uint32
	for _, p := range paths {
		found, err := util.FindProcessesHoldingPath(p)
		if err != nil {
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("find holders of %s failed: %v", p, err))
			continue
		}
		for _, pid := range found {
			if pid != selfPid && !seen[pid] {
				seen[pid] = true
				holders = append(holders, pid)
			}
		}
	}
	return partitionHolders(holders, whitelistPIDs(whitelist))
}

// buildMainExeCmd 构造主程序启动命令（提取为可测试函数，断言工作目录）
func buildMainExeCmd(exePath, mainFolder string) *exec.Cmd {
	cmd := exec.Command(exePath)
	cmd.Dir = mainFolder
	return cmd
}

// launchMainExe 启动主程序，显式指定工作目录为主程序目录，
// 避免主程序 CWD 继承更新器目录（UpdateFolder）导致相对路径配置读错（#10）。
func launchMainExe(cfg *config.Config, mainFolder string) {
	exeDir, err := config.ExeDir()
	if err != nil {
		util.AppendToLog(".", "update.log", fmt.Sprintf("launch main exe: get exe dir failed: %v", err))
		return
	}
	exePath := filepath.Join(exeDir, cfg.MainExeRelativePath)
	cmd := buildMainExeCmd(exePath, mainFolder)
	if err := cmd.Start(); err != nil {
		util.AppendToLog(exeDir, "update.log", fmt.Sprintf("launch main exe failed: %s %v", exePath, err))
	}
}

// launchMainExeFn 启动主程序入口（测试可替换为桩，记录调用）
var launchMainExeFn = launchMainExe

// applyFailureFallback 更新失败兜底（用户要求：重命名失败时启动旧 exe 并附带错误信息）。
// 将 version.json 状态回退 downloaded、启动旧版本主程序（保证应用可用）、记录错误日志，
// 返回给调用方输出的错误信息字符串。
func applyFailureFallback(fc *FullConfig, versionInfo *config.VersionInfo, failErr error) string {
	if versionInfo != nil {
		versionInfo.VersionStatus = config.VersionStatusDownloaded
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			util.AppendToLog(logDir(), "update.log", fmt.Sprintf("rollback after apply fail: write version failed: %v", wErr))
		}
	}
	// 启动旧版本主程序：更新失败也保证应用可继续运行
	launchMainExeFn(fc.ExeCfg, fc.MainFolder)
	util.AppendToLog(logDir(), "update.log", fmt.Sprintf("apply failed, launched old exe: %v", failErr))
	return failErr.Error()
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

// scriptTimeout 后置脚本执行超时上限：脚本挂死时强制结束，避免阻塞主程序启动过久（#11）。
// 声明为 var 以便测试覆盖超时路径。
var scriptTimeout = 60 * time.Second

// runScript 同步执行 post-update 脚本并记录结果到 update.log。
// 同步执行保证更新器进程退出前脚本已完成、结果已落盘（原异步 goroutine 会随进程退出被杀，
// 导致脚本结果日志丢失，且脚本与主程序启动无先后保证，#11）。
// 调用方应保证顺序：脚本先于主程序启动（保持既有语义）。
func runScript(scriptPath, workDir string) {
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return
	}
	var cmd *exec.Cmd
	if os.PathSeparator == '\\' {
		cmd = exec.Command("cmd", "/c", scriptPath)
	} else {
		cmd = exec.Command("sh", "-c", scriptPath)
	}
	cmd.Dir = workDir
	if err := cmd.Start(); err != nil {
		exeDir, dirErr := config.ExeDir()
		if dirErr == nil {
			util.AppendToLog(exeDir, "update.log",
				fmt.Sprintf("script start failed: %s %v", scriptPath, err))
		}
		return
	}
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
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
	case <-time.After(scriptTimeout):
		cmd.Process.Kill()
		// 回收已结束的 Wait goroutine，避免子进程句柄残留
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
		exeDir, dirErr := config.ExeDir()
		if dirErr != nil {
			return
		}
		util.AppendToLog(exeDir, "update.log",
			fmt.Sprintf("script timed out and killed: %s", scriptPath))
	}
}
