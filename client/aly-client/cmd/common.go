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

// formatPidNames 将进程 PID 格式化为去重、排序的进程名列表（未知名回退 PID），用于日志
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
// 用户决定：谁占用杀谁——RM 探测的句柄持有者与 CWD 持有者（cmd.exe 等站在目录里的进程）
// 一律强杀，不弹窗询问。强杀名单写入 update.log 便于排查。
func closeProcessesHoldingFolder(folder string, timeout time.Duration) {
	selfPid := uint32(os.Getpid())
	pids := findHoldersOf(folder)
	var toKill []uint32
	for _, pid := range pids {
		if pid != selfPid {
			toKill = append(toKill, pid)
		}
	}
	if len(toKill) == 0 {
		return
	}
	// 先尝试优雅关闭（explorer 等会自行释放句柄），随后直接强杀
	for _, pid := range toKill {
		util.SendCloseMessageToProcess(pid)
	}
	wait := timeout
	if wait > forceKillWait {
		wait = forceKillWait
	}
	util.ForceKillPIDs(toKill, wait)
	// 日志记录杀了谁，便于排查误杀
	names := util.FindProcessNamesByPIDs(toKill)
	util.AppendToLog(logDir(), "update.log",
		fmt.Sprintf("killed folder holders: %v", formatPidNames(toKill, names)))
}

// forceKillWait 强制结束进程后等待退出的上限，避免更新长时间卡在等待上
const forceKillWait = 5 * time.Second

// renameDirWithKill 重命名文件夹，约定三条规则：
//  1. 源文件夹必须存在，否则直接失败；
//  2. 目标文件夹必须不存在：若目标已存在（例如上一次更新留下的旧版本备份），
//     先把它重命名为另一个不冲突的文件夹（to.old / to.old.1 / to.old.2 …）再执行正式重命名。
//     Windows 的 MoveFileEx 无法覆盖非空目录，即使无任何进程占用也会报 Access denied；
//  3. 重命名失败若因进程占用，探测占用 from/to 的进程并直接结束（排除更新器自身）后重试。
//     用户决定：谁占用杀谁，不弹窗询问。
func renameDirWithKill(from, to string, timeout time.Duration) error {
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
			if asideErr := renameWithKillRetry(to, aside, timeout); asideErr != nil {
				// 旁移失败（被占用且杀不掉）：直接返回，重试无意义
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

		// 4) 直接结束占用待重命名文件夹的进程（排除更新器自身）
		pids := findHoldersOf(from, to)
		if len(pids) > 0 {
			wait := timeout
			if wait > forceKillWait {
				wait = forceKillWait
			}
			util.ForceKillPIDs(pids, wait)
		}
		time.Sleep(retrySleep)
	}
	return lastErr
}

// renameWithKillRetry 执行重命名；失败时探测占用 from/to 的进程并直接强杀后重试。
// 调用方需保证目标 to 不存在（由 renameDirWithKill 负责挪开）。
func renameWithKillRetry(from, to string, timeout time.Duration) error {
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

		// 直接结束占用待重命名文件夹的进程（排除更新器自身）
		pids := findHoldersOf(from, to)
		if len(pids) > 0 {
			wait := timeout
			if wait > forceKillWait {
				wait = forceKillWait
			}
			util.ForceKillPIDs(pids, wait)
		} else {
			// RM 与 CWD 探测都找不到占用者时，通常是资源管理器窗口
			// 打开了该文件夹（Explorer 持目录句柄，RM 检测不到），或 64 位原生
			// cmd.exe 等 CWD 持有者（32 位客户端无法读取其 PEB，见 util/process.go）。
			// 关闭/结束 Explorer（系统会自动重启它）。
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("rename %s -> %s: no holders found by RM/CWD scan, closing explorer as fallback", from, to))
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

// findHoldersOf 收集占用指定路径集合的进程 PID（去重并排除更新器自身）。
// 来源：Restart Manager 句柄持有者 + CWD 持有者（cmd.exe 等站在目录里的进程）。
// 用户决定：谁占用杀谁，返回的全部 PID 都会被强杀。
// CWD 探测较昂贵（枚举全进程表 + 逐进程 PEB 读取，慢速 XP 上可达秒级），
// 仅在 RM 未发现任何持有者时执行：CWD-only 占用者会在重试轮中（RM 为空）被扫到。
func findHoldersOf(paths ...string) []uint32 {
	selfPid := uint32(os.Getpid())
	seen := make(map[uint32]bool)
	var holders []uint32
	rmFoundAny := false
	for _, p := range paths {
		found, err := util.FindProcessesHoldingPath(p)
		if err != nil {
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("find holders of %s failed: %v", p, err))
			continue
		}
		if len(found) > 0 {
			rmFoundAny = true
		}
		for _, pid := range found {
			if pid != selfPid && !seen[pid] {
				seen[pid] = true
				holders = append(holders, pid)
			}
		}
	}
	if !rmFoundAny {
		// CWD 持有者（如 32 位 cmd.exe 站在目录里，RM 不一定报得出）
		for _, p := range paths {
			cwdHolders := util.FindProcessesWithCWDUnder(p)
			for _, pid := range cwdHolders {
				if pid != selfPid && !seen[pid] {
					seen[pid] = true
					holders = append(holders, pid)
				}
			}
		}
	}
	return holders
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
