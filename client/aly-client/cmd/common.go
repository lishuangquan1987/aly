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
	"syscall"
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

// forceKillWait 强制结束进程后等待退出的上限，避免更新长时间卡在等待上
const forceKillWait = 5 * time.Second

// renameProbeState 一次 apply/rollback 内共享的探测状态，避免重复全量扫描（#23）：
//   - killedPids：本次已击杀的 PID，不再重复击杀/重复计入；
//   - deepPaths：本次已深扫（全系统句柄枚举）过的路径，不再重复深扫。
//
// 注意：浅扫（RM + CWD）结果不缓存——占用者会随时间变化，每次失败重试都必须重新探测，
// 才能满足"再失败，再查杀"；只有 10s-60s 级的深扫才按路径去重。
type renameProbeState struct {
	killedPids map[uint32]bool
	deepPaths  map[string]bool
}

func newRenameProbeState() *renameProbeState {
	return &renameProbeState{
		killedPids: make(map[uint32]bool),
		deepPaths:  make(map[string]bool),
	}
}

// isRenameRetryableErr 判断重命名失败错误是否属于"占用类"错误（值得探测+击杀+重试）。
// 占用类（可重试）：ERROR_SHARING_VIOLATION(32) / ERROR_LOCK_VIOLATION(33) /
// ERROR_USER_MAPPED_FILE(1224) / ERROR_ACCESS_DENIED(5)。
// 非占用类（重试无意义，立即失败）：ERROR_NOT_SAME_DEVICE(17 跨卷) /
// ERROR_DIR_NOT_EMPTY(145) / ERROR_ALREADY_EXISTS(183 目标已存在，走旁移) /
// ERROR_PATH_NOT_FOUND(3 源不存在)。
// 无法分类时保守视为占用类（保持旧行为：探测→击杀→重试）。
func isRenameRetryableErr(err error) bool {
	le, ok := err.(*os.LinkError)
	if !ok {
		return true
	}
	errno, ok := le.Err.(syscall.Errno)
	if !ok {
		return true
	}
	switch errno {
	case 32, 33, 1224, 5:
		return true
	default:
		return false
	}
}

// renameDirWithKill 重命名文件夹（乐观模式），单次调用使用独立探测状态。
// 约定：
//  1. 源文件夹必须存在，否则直接失败；
//  2. 目标文件夹必须不存在：若目标已存在（例如上一次更新留下的旧版本备份），
//     先把它重命名为另一个不冲突的文件夹（to.old / to.old.1 / to.old.2 …）再执行正式重命名。
//     Windows 的 MoveFileEx 无法覆盖非空目录，即使无任何进程占用也会报 Access denied；
//  3. 先直接重命名；失败时按 Windows 错误码分类——只有"占用类"错误才探测占用者并击杀后重试，
//     非占用类错误立即失败（重试只会白杀进程）；
//  4. 探测占用 from/to 的进程（排除更新器自身）并直接结束；RM 与 CWD 探测都找不到占用者时，
//     兜底关闭浏览该目录的 explorer 窗口（精准 → 全部）。
func renameDirWithKill(from, to string, timeout time.Duration) error {
	return renameDirWithKillState(from, to, timeout, newRenameProbeState())
}

// renameDirWithKillState 重命名文件夹，st 为本次 apply/rollback 内共享的探测状态
// （已击杀 PID / 已浅扫路径 / 已深扫路径在多次 rename 间复用，避免重复全量扫描，#23）。
func renameDirWithKillState(from, to string, timeout time.Duration, st *renameProbeState) error {
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
			if asideErr := renameWithKillRetryState(to, aside, timeout, st); asideErr != nil {
				// 旁移失败（被占用且杀不掉）：直接返回，重试无意义
				return fmt.Errorf("move aside %s -> %s: %v", to, aside, asideErr)
			}
		}
		// 3) 正式重命名（乐观模式：先直接试，失败才探测击杀）
		err := os.Rename(from, to)
		if err == nil {
			return nil
		}
		lastErr = err
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("rename %s -> %s attempt %d/%d failed: %v", from, to, attempt, maxAttempts, err))

		// 4) 错误码分类：非占用类错误立即失败（重试只会白杀进程）
		if !isRenameRetryableErr(err) {
			return fmt.Errorf("rename %s -> %s: %v", from, to, err)
		}

		// 5) 探测占用者（浅扫 → 第 3 轮起深扫 → explorer 兜底），击杀后重试
		pids := probeHoldersState(from, to, attempt, st)
		if len(pids) > 0 {
			wait := timeout
			if wait > forceKillWait {
				wait = forceKillWait
			}
			util.ForceKillPIDs(pids, wait)
			for _, pid := range pids {
				st.killedPids[pid] = true
			}
			names := util.FindProcessNamesByPIDs(pids)
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("killed folder holders: %v", formatPidNames(pids, names)))
		} else {
			// RM 与 CWD 探测都找不到占用者：通常是资源管理器窗口
			// （Explorer 持目录句柄，RM 检测不到），或 64 位原生 cmd.exe 等
			// CWD 持有者（32 位客户端无法读取其 PEB，见 util/process.go）。
			// 先精准关闭浏览 from/to 的 Explorer 窗口，未命中再杀全部 explorer 兜底（#2）。
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("rename %s -> %s: no holders found by RM/CWD scan, closing explorer windows", from, to))
			closeExplorerWindows(timeout, from, to)
		}
		time.Sleep(retrySleep)
	}
	return lastErr
}

// renameWithKillRetryState 执行重命名（乐观模式），st 为本次 apply/rollback 内共享的探测状态。
func renameWithKillRetryState(from, to string, timeout time.Duration, st *renameProbeState) error {
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

		// 错误码分类：非占用类错误立即失败（重试只会白杀进程）
		if !isRenameRetryableErr(err) {
			return fmt.Errorf("rename %s -> %s: %v", from, to, err)
		}

		pids := probeHoldersState(from, to, attempt, st)
		if len(pids) > 0 {
			wait := timeout
			if wait > forceKillWait {
				wait = forceKillWait
			}
			util.ForceKillPIDs(pids, wait)
			for _, pid := range pids {
				st.killedPids[pid] = true
			}
			names := util.FindProcessNamesByPIDs(pids)
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("killed folder holders: %v", formatPidNames(pids, names)))
		} else {
			// 无占用者：explorer 兜底（#2）
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("rename %s -> %s: no holders found by RM/CWD scan, closing explorer windows", from, to))
			closeExplorerWindows(timeout, from, to)
		}
		time.Sleep(retrySleep)
	}
	return lastErr
}

// probeHoldersState 探测占用 from/to 的进程（排除更新器自身与本次已击杀 PID）。
// 探测策略：
//   - 浅扫（RM + CWD）每次失败重试都重新执行——占用者会随时间变化，
//     必须"再失败，再查杀"，结果不缓存；
//   - 深扫（全系统句柄枚举）从第 3 轮起启用，但同一路径在本次 apply/rollback 内
//     最多深扫一次（10s-60s 级成本，#23）。
//
// 只探测真实存在的路径（to 通常是尚不存在的目标目录，注册不存在的资源纯属浪费）。
func probeHoldersState(from, to string, attempt int, st *renameProbeState) []uint32 {
	selfPid := uint32(os.Getpid())
	seen := make(map[uint32]bool)
	var holders []uint32
	add := func(pids []uint32) {
		for _, pid := range pids {
			if pid != selfPid && !st.killedPids[pid] && !seen[pid] {
				seen[pid] = true
				holders = append(holders, pid)
			}
		}
	}

	// 只探测存在的路径
	var paths []string
	for _, p := range []string{from, to} {
		if _, statErr := os.Stat(p); statErr == nil {
			paths = append(paths, p)
		}
	}

	// 浅扫（RM 句柄持有者 + CWD 持有者）：每次重试都重新探测
	if len(paths) > 0 {
		add(findHoldersOf(paths...))
	}

	// 深扫：第 3 轮起，仅未深扫过的路径（全系统句柄枚举，覆盖 64 位进程/RM 盲区）
	if attempt >= 3 {
		deepUnprobed := deepScanCandidates(paths, st)
		if len(deepUnprobed) > 0 {
			add(findDeepHolders(deepUnprobed...))
			for _, p := range deepUnprobed {
				st.deepPaths[p] = true
			}
		}
	}
	return holders
}

// deepScanCandidates 返回 paths 中本次 apply/rollback 内尚未深扫过的路径。
// 深扫（全系统句柄枚举）是 10s-60s 级操作，同一路径在本次 apply 内最多做一次（#23）。
func deepScanCandidates(paths []string, st *renameProbeState) []string {
	var out []string
	for _, p := range paths {
		if !st.deepPaths[p] {
			out = append(out, p)
		}
	}
	return out
}

// closeExplorerWindows 解除 Explorer 对目录的占用：
//  1) 精准方案：只关闭"当前文件夹 == 目标"的 Explorer 窗口（Shell.Application COM via
//     cscript，业界推荐做法），避免误关用户其他资源管理器窗口、不重启 shell；
//  2) 兜底：精准关闭未命中任何窗口（或 cscript 不可用）时，按"谁占用杀谁"结束全部
//     explorer（系统会自动重启它）。
func closeExplorerWindows(timeout time.Duration, folders ...string) {
	for _, folder := range folders {
		closed, err := util.CloseExplorerWindowsBrowsing(folder)
		if err != nil {
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("close explorer windows (targeted) failed for %s: %v", folder, err))
			continue
		}
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("closed %d explorer window(s) browsing %s", closed, folder))
		if closed > 0 {
			return // 已精准关闭目标窗口，无需杀全部 explorer
		}
	}

	// 兜底：杀全部 explorer（系统自动重启）
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

// nextAsideName 生成一个不冲突的"挪开目标"名称：X.old、X.old.1、X.old.2 …
// 若 to 本身已是 X.old（旁移目标/残留备份），先剥掉 .old 后缀再命名（base = X），
// 保证旁移命名始终收敛在 X.old / X.old.N 家族内，不会产生 X.old.old 链式残留（#18）。
func nextAsideName(to string) string {
	base := to
	if strings.HasSuffix(base, ".old") {
		base = strings.TrimSuffix(base, ".old")
	}
	aside := base + ".old"
	for i := 1; ; i++ {
		if _, err := os.Stat(aside); os.IsNotExist(err) {
			return aside
		}
		aside = fmt.Sprintf("%s.old.%d", base, i)
	}
}

// removeAsideVariants 清理 base 的旁移残留：base.old、base.old.1、base.old.2 …
// 用于 apply/rollback 入口（清历史残留）与成功路径（清本次旁移），
// 防止 X.old / X.old.N 泄漏占用磁盘（#18）。不存在时 os.RemoveAll 返回 nil，无害。
func removeAsideVariants(base string) {
	// 64 个旁移变体已是极端场景上限（正常最多 1-2 个）
	candidates := make([]string, 0, 65)
	candidates = append(candidates, base+".old")
	for i := 1; i <= 64; i++ {
		candidates = append(candidates, fmt.Sprintf("%s.old.%d", base, i))
	}
	for _, c := range candidates {
		if err := os.RemoveAll(c); err != nil {
			util.AppendToLog(logDir(), "update.log", fmt.Sprintf("remove aside variant %s: %v", c, err))
		}
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

// findDeepHolders 深扫：在浅扫（RM + 32 位 CWD）基础上追加全系统句柄枚举，
// 覆盖 64 位原生进程的 CWD/句柄、RM 探测不到的持有者。耗时（秒级），仅重试后期调用。
func findDeepHolders(paths ...string) []uint32 {
	holders := findHoldersOf(paths...)
	seen := make(map[uint32]bool)
	for _, p := range holders {
		seen[p] = true
	}
	for _, p := range paths {
		for _, pid := range util.FindProcessesWithHandlesUnder(p) {
			if pid != uint32(os.Getpid()) && !seen[pid] {
				seen[pid] = true
				holders = append(holders, pid)
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
// 正常情况下将 version.json 状态回退 downloaded、启动旧版本主程序（保证应用可用）、记录错误日志，
// 返回给调用方输出的错误信息字符串。
//
// 特殊场景（#7）：若 MainFolder 已丢失（备份改名成功、应用改名与回滚改名都失败）
// 但对应版本目录仍存在，则**保持 applying 状态不降级**——降级成 downloaded 会让
// 崩溃恢复分支（只认 applying）失效，应用目录将永久缺失。保持 applying 后，
// 下次 check_update/apply_update 会走崩溃恢复分支完成 versionDir → MainFolder 重命名。
func applyFailureFallback(fc *FullConfig, versionInfo *config.VersionInfo, failErr error) string {
	if versionInfo != nil {
		if mainFolderLostButVersionDirExists(fc, versionInfo) {
			// 主目录缺失 + 版本目录存在：保持 applying，交给崩溃恢复分支修复。
			// 注意：主目录已丢失，无法从这里启动旧 exe（exe 位于缺失目录下，
			// launchMainExe 会失败），故不调用启动；崩溃恢复完成后会启动主程序。
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("apply failed with main folder lost, keep applying for crash recovery: %v", failErr))
			return failErr.Error()
		}
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

// mainFolderLostButVersionDirExists 判断"主目录已丢失但对应版本目录仍存在"（#7 恢复盲区）。
func mainFolderLostButVersionDirExists(fc *FullConfig, versionInfo *config.VersionInfo) bool {
	if _, statErr := os.Stat(fc.MainFolder); !os.IsNotExist(statErr) {
		return false // 主目录存在：普通失败，正常降级
	}
	versionDir, err := fc.ExeCfg.AppVersionDir(versionInfo.Version)
	if err != nil {
		return false
	}
	if _, statErr := os.Stat(versionDir); statErr != nil {
		return false // 版本目录也不存在，无从恢复
	}
	return true
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
