package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	apiclient "aly/client/aly-client/client"
	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
	"aly/client/aly-client/util"
)

const largeFileThreshold = 100 * 1024 * 1024 // 100MB

// DownloadUpdate downloads only changed files from server
func DownloadUpdate() {
	fs := flag.NewFlagSet("download_update", flag.ContinueOnError)
	urlFlag := fs.String("url", "", "server url")
	projectNameFlag := fs.String("project-name", "", "project name")
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	// A6：ContinueOnError + JSON 信封，参数解析失败 exit 2；-h 打印 usage 后 exit 0
	parseCommandFlags(fs, os.Args[2:])

	// 全局更新锁：同一时刻只允许一个更新操作（下载/应用/回滚）
	releaseLock, lockErr := AcquireUpdateLock("download_update")
	if lockErr != nil {
		printOutput(false, lockErr.Error(), nil)
		return
	}
	defer releaseLock()

	fc, err := loadFullConfig(*urlFlag, *projectNameFlag, *mainExePathFlag)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	if fc.Shared.ServerURL == "" {
		printOutput(false, "no server url configured", nil)
		return
	}
	if fc.Shared.ProjectName == "" {
		printOutput(false, "no project name configured", nil)
		return
	}

	logs, err := apiclient.GetProjectChangeLogs(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}
	if len(logs) == 0 {
		printOutput(false, "no change logs on server", nil)
		return
	}

	latestLog := findLatestLog(logs)
	newVersion := stripVPrefix(latestLog.Version)

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printOutput(false, fmt.Sprintf("read version: %v", err), nil)
		return
	}
	currentVersion := stripVPrefix(versionInfo.Version)

	// applying 期间：不重复下载、不改写版本状态。由 apply_update 走崩溃恢复分支完成在途操作
	// （修复 Bug#3：download 不得把 applying 降级成 downloaded、不得覆盖 VersionPrevious/RollbackPrevious）。
	if versionInfo.VersionStatus == config.VersionStatusApplying {
		util.AppendToLog(logDir(), "download.log", "applying 进行中，跳过 download（由 apply_update 恢复）")
		// 用标准完成行（data:null）收尾：SDK 只认 data:null 为完成信号，
		// 输出 data 非 null 的成功行会让 SDK 无法识别完成、进程退出后 EOF 空转挂死（#1c）。
		printProgressDone()
		return
	}

	// F4：非法状态（downloaded 且 VersionPrevious == Version，非空）：备份目录与版本目录同路径，
	// apply/rollback 都会拒绝替换。按 downloaded 的不变式（MainFolder 内容 == VersionPrevious），
	// 此时主目录内容就是 Version 本身 → 状态等价于 applied，直接归一化。
	// 注意：必须限定"服务器没有更高版本"时才归一化 —— 否则服务器已发新版时会把本次
	// 下载请求变成空操作（check 已要求下载更高版本，归一化会让紧随其后的 apply 报"无待应用更新"）。
	if versionInfo.VersionStatus == config.VersionStatusDownloaded &&
		versionInfo.VersionPrevious != "" && versionInfo.VersionPrevious == versionInfo.Version &&
		!needUpdate(newVersion, currentVersion) {
		util.AppendToLog(logDir(), "download.log",
			fmt.Sprintf("非法状态 downloaded 且 Version == VersionPrevious(%s)：归一化为 applied", versionInfo.Version))
		versionInfo.VersionStatus = config.VersionStatusApplied
		clearUpdateTransientState(versionInfo)
		if wErr := config.WriteVersion(versionInfo); wErr != nil {
			printProgressFail(0, 0, "version.json", 0, fmt.Sprintf("write version: %v", wErr))
			return
		}
		// 用标准完成行（data:null）收尾：SDK 视其为"下载完成"，随后 apply 会返回"无待应用更新"
		printProgressDone()
		return
	}

	// Guard: if this exact version was already downloaded, skip re-download.
	// F3：必须确认待应用版本目录真的存在且含文件 —— 否则"下载产物丢失"会被当成"已下载"跳过，
	// 后续 apply 只能拿到空/残缺目录（apply 已改为拒绝并自愈）；这里直接重新下载补齐。
	reDownload := false
	if versionInfo.VersionStatus == config.VersionStatusDownloaded &&
		currentVersion == newVersion {
		targetDir, dErr := fc.ExeCfg.AppVersionDir(newVersion)
		if dErr != nil {
			reDownload = true // 连目标路径都算不出来：交给下面的正常流程报错
		} else {
			pendingCount, countErr := countRegularFiles(targetDir)
			switch {
			case countErr == nil && pendingCount > 0:
				// 产物完好：用标准完成行（data:null）收尾（#1c，SDK 只认 data:null 为完成）。
				printProgressDone()
				return
			case countErr != nil && !os.IsNotExist(countErr):
				// A2：目录存在但读不到（ACL/杀软锁/慢盘）：不当成"丢失"而触发全量重下
				util.AppendToLog(logDir(), "download.log",
					fmt.Sprintf("downloaded 状态但待应用版本目录不可读（%s: %v），跳过重下", targetDir, countErr))
				printProgressDone()
				return
			default:
				util.AppendToLog(logDir(), "download.log",
					fmt.Sprintf("downloaded 状态但待应用版本目录缺失或为空（%s），重新下载补齐", targetDir))
				// 允许"重下同版本"：否则下面的"已是最新"判断会直接返回错误，产物永远补不回来
				reDownload = true
			}
		}
	}
	if !reDownload && compareVersion(newVersion, currentVersion) <= 0 {
		printOutput(false, "already at latest version", nil)
		return
	}

	serverFiles, err := apiclient.GetAllFiles(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if err != nil {
		printOutput(false, fmt.Sprintf("get file list: %v", err), nil)
		return
	}

	localMD5Map, localMD5Err := util.LocalFileMD5Map(fc.MainFolder)
	if localMD5Err != nil {
		exeDir, _ := config.ExeDir()
		util.AppendToLog(exeDir, "download.log", fmt.Sprintf("local md5 scan warning: %v", localMD5Err))
	}

	targetDir, err := fc.ExeCfg.AppVersionDir(newVersion)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}
	if err := util.EnsureDir(targetDir); err != nil {
		printOutput(false, fmt.Sprintf("create target dir: %v", err), nil)
		return
	}

	// B2：同版本重下（reDownload）中途失败时，目标目录会留下"半成品"文件。此时 version.json
	// 仍是 downloaded/该版本（下载未完成不写状态是既定不变量），半成品目录会让后续 apply 误判
	// "产物还在"并混装旧/新文件 → 失败时把目标目录清掉，让下一轮重新识别为"产物丢失"再重下。
	// 注意：仅对 reDownload 生效（首次下载失败保留已校验文件，供断点续传复用）。
	downloadedOK := false
	defer func() {
		if !reDownload || downloadedOK {
			return
		}
		if err := os.RemoveAll(targetDir); err != nil {
			util.AppendToLog(logDir(), "download.log",
				fmt.Sprintf("clean partial re-download dir %s failed: %v", targetDir, err))
			return
		}
		util.AppendToLog(logDir(), "download.log",
			fmt.Sprintf("同版本重下失败，已清理半成品目录 %s", targetDir))
	}()

	// 只打印真正需要下载的差异文件：本地已匹配、或目标目录已有正确文件的
	// 都不输出（不打印 SKIP），total 取差异文件数，避免界面按行统计进度出错。
	// 逻辑：
	// - 本地当前版本 MD5 已匹配 → 无需下载（不打印）
	// - 目标版本目录已有正确文件（断点续传/已下载过）→ 无需下载（不打印）
	// - 否则 → START → 下载（3 次重试 + MD5/SHA256 校验）→ DONE
	type fileToDownload struct {
		idx        int
		serverFile model.FileInfo
	}
	var downloadList []fileToDownload
	// 单文件失败不整体失败（计划条目 6）：记录失败清单，全部跑完统一输出。
	// 下一轮 download 的断点续传语义天然支持"只重失败文件"（已成功的会被
	// "目标目录已有正确文件"分支跳过）。M12 的非法相对路径也计入该清单。
	var failedFiles []string
	for i := range serverFiles {
		relPath := normalizePath(serverFiles[i].FileRelativePath)

		// M12：服务端下发的相对路径必须先过白名单——恶意/异常服务端可用 `..` 把
		// 下载产物写出目标版本目录之外。不合法即记入失败清单，不下载。
		if !safeRelPath(serverFiles[i].FileRelativePath) {
			exeDir, _ := config.ExeDir()
			util.AppendToLog(exeDir, "download.log",
				fmt.Sprintf("skip unsafe relative path from server: %q", serverFiles[i].FileRelativePath))
			failedFiles = append(failedFiles, relPath+" (unsafe path)")
			continue
		}

		// 本地当前版本已匹配：非差异文件，无需下载
		localMD5, localExists := localMD5Map[relPath]
		if localExists && localMD5 == serverFiles[i].MD5 {
			continue
		}

		// 目标目录已有正确文件（MD5+SHA256）：无需重新下载
		localPath := filepath.Join(targetDir, filepathFromSlash(relPath))
		if info, statErr := os.Stat(localPath); statErr == nil && info.Size() == serverFiles[i].FileSize {
			targetMD5, md5Err := util.FileMD5(localPath)
			targetSHA256, shaErr := util.FileSHA256(localPath)
			if md5Err == nil && shaErr == nil && targetMD5 == serverFiles[i].MD5 && targetSHA256 == serverFiles[i].SHA256 {
				continue
			}
		}

		downloadList = append(downloadList, fileToDownload{idx: len(downloadList) + 1, serverFile: serverFiles[i]})
	}

	total := len(downloadList)

	// 字节级进度（计划条目 4）：BEGIN 预告行携带 total_bytes（差异文件大小之和），
	// 每文件 DONE 行累计 downloaded_bytes 并携带 3 秒滑动窗口均值速度。
	var totalBytes int64
	for _, dl := range downloadList {
		totalBytes += dl.serverFile.FileSize
	}
	printProgressBytes(0, total, "", "BEGIN", 0, 0, totalBytes, 0, "")

	type speedSample struct {
		at    time.Time
		bytes int64
	}
	var speedWindow []speedSample
	var downloadedBytes int64
	var speedMu sync.Mutex
	// speedBpsLocked 计算窗口速度，调用方必须已持 speedMu。
	// 窗口语义（与旧版一致）：3 秒滑动窗口内**文件完成时**的采样之和 / 3。
	// 注意：文件中途撤销（3 秒节流）不补 done 采样，属可接受的近似。
	speedBpsLocked := func() int64 {
		cutoff := time.Now().Add(-3 * time.Second)
		kept := speedWindow[:0]
		var sum int64
		for _, s := range speedWindow {
			if s.at.After(cutoff) {
				kept = append(kept, s)
				sum += s.bytes
			}
		}
		speedWindow = kept
		if len(speedWindow) == 0 {
			return 0
		}
		return sum / 3 // 窗口 3 秒
	}

	// sampleRate 计算两采样点间的瞬时速率（字节差/时间差），无效（首样本/时间差过小）
	// 时回退窗口速度 windowBps。供单文件 PROGRESS 行使用（3 秒节流采样）。
	sampleRate := func(prevAt time.Time, prevBytes int64, now time.Time, curBytes int64, windowBps int64) int64 {
		dt := now.Sub(prevAt)
		if prevAt.IsZero() || dt < 500*time.Millisecond {
			return windowBps
		}
		rate := (curBytes - prevBytes) * int64(time.Second) / int64(dt)
		if rate < 0 {
			return windowBps
		}
		return rate
	}

	// 失败清单 failedFiles 已在收集下载列表前声明（非法相对路径也要计入）。
	for _, dl := range downloadList {
		relPath := normalizePath(dl.serverFile.FileRelativePath)
		// 纵深防御（M12）：列表构建时已过滤，此处再校验一次，防未来改动绕过。
		if !safeRelPath(dl.serverFile.FileRelativePath) {
			lastErr := "unsafe relative path"
			exeDir, _ := config.ExeDir()
			util.AppendToLog(exeDir, fmt.Sprintf("update_%s_fail.log", newVersion), relPath+" "+lastErr)
			printProgressFail(dl.idx, total, relPath, dl.serverFile.FileSize, lastErr)
			failedFiles = append(failedFiles, relPath+" (unsafe path)")
			continue
		}
		localPath := filepath.Join(targetDir, filepathFromSlash(relPath))

		// START 行也带字节字段（downloaded/total_bytes/speed）：否则 UI 在
		// 字节百分比与文件计数两种进度模式间来回切换，进度条视觉抖动。
		speedMu.Lock()
		startSpeed := speedBpsLocked()
		speedMu.Unlock()
		printProgressBytes(dl.idx, total, relPath, "START", dl.serverFile.FileSize,
			downloadedBytes, totalBytes, startSpeed, "")

		// Download with retry up to 3 times
		fileOK := false
		var lastErr string
		for retry := 0; retry < 3; retry++ {
			// 单文件周期进度行（PROGRESS）：大文件下载期间也要有输出——
			// UI 的 90s idle 超时（bridge/commands.go）与 SDK 的看门狗都依赖
			// "stdout 有活动"判活；旧版只在 START/DONE 输出，单文件 >90s 无输出
			// 会被误判卡死杀进程（#5 修复后 300s 超时没了，90s 成了新瓶颈）。
			// 节流 3s 一行：读回调给出该文件累计字节数，直接换算速度（样本间差值/时间差）。
			startBytes := downloadedBytes
			var lastEmit time.Time
			var lastEmitBytes int64
			onBytes := func(written int64) {
				totalFileBytes := startBytes + written
				speedMu.Lock()
				curBps := speedBpsLocked()
				speedMu.Unlock()
				now := time.Now()
				if now.Sub(lastEmit) < 3*time.Second {
					return
				}
				curBps = sampleRate(lastEmit, lastEmitBytes, now, totalFileBytes, curBps)
				lastEmit = now
				lastEmitBytes = totalFileBytes
				printProgressBytes(dl.idx, total, relPath, "PROGRESS", dl.serverFile.FileSize,
					totalFileBytes, totalBytes, curBps, "")
			}
			if err := apiclient.DownloadFileWithResume(fc.Shared.ServerURL, dl.serverFile.FileAbsolutePath, localPath, dl.serverFile.FileSize, largeFileThreshold, onBytes); err != nil {
				lastErr = fmt.Sprintf("download error: %v", err)
				continue
			}

			// Verify MD5 + SHA256
			localMD5, md5Err := util.FileMD5(localPath)
			localSHA256, shaErr := util.FileSHA256(localPath)

			if md5Err != nil || shaErr != nil {
				lastErr = "hash compute error"
				os.Remove(localPath)
				continue
			}

			if localMD5 == dl.serverFile.MD5 && localSHA256 == dl.serverFile.SHA256 {
				fileOK = true
				break
			}

			lastErr = "checksum mismatch"
			os.Remove(localPath)
		}

		if !fileOK {
			// 重试 3 次仍失败：记录并继续下一个文件（不再整体 return）
			exeDir, _ := config.ExeDir()
			util.AppendToLog(exeDir, fmt.Sprintf("update_%s_fail.log", newVersion),
				fmt.Sprintf("%s %s (server_md5=%s local_md5见前文)", relPath, lastErr, dl.serverFile.MD5))
			printProgressFail(dl.idx, total, relPath, dl.serverFile.FileSize, lastErr)
			failedFiles = append(failedFiles, relPath)
			continue
		}

		speedMu.Lock()
		downloadedBytes += dl.serverFile.FileSize
		speedWindow = append(speedWindow, speedSample{at: time.Now(), bytes: dl.serverFile.FileSize})
		doneSpeed := speedBpsLocked()
		speedMu.Unlock()
		printProgressBytes(dl.idx, total, relPath, "DONE", dl.serverFile.FileSize,
			downloadedBytes, totalBytes, doneSpeed, "")
	}

	// 有失败文件：整体视为失败（不写 version.json 的待应用状态，避免半成品被
	// apply 激活——与既有不变量"下载未完成不写状态"一致），失败清单随结果
	// 下发，宿主重试时已成功文件全部跳过（断点续传），等效"只重失败文件"。
	if len(failedFiles) > 0 {
		printOutput(false,
			fmt.Sprintf("下载完成但有 %d 个文件失败: %s", len(failedFiles), strings.Join(failedFiles, ", ")),
			&model.DownloadUpdateData{Version: newVersion, FailedFiles: failedFiles})
		return
	}

	// 物化收尾（计划条目 5）：把 MainFolder 中未变化的文件拷进版本目录，
	// 让 apply 只剩纯 rename（关进程窗口从分钟级降到秒级）。
	// 时机在主程序仍在运行时——只读复制对运行中应用无害（读文件不锁目录改名）。
	// 用户决策（计划文档 §五.3）：apply 侧的复制逻辑已删除，物化是唯一的补齐
	// 途径，因此失败必须让整轮下载失败（重试时已下载文件断点续传跳过，
	// 已物化文件因 CopyFile 跳过语义同样空转，重试成本很低）。
	// 排除规则与原 apply 侧一致：un_copy_folders / un_copy_files 不随迁。
	//
	// M6：主目录缺失（applying 崩溃现场、等待重下补齐）时物化无源可拷：跳过复制并留痕，
	// 而不是让整轮下载失败——否则"重下 → materialize 失败 → defer 清空目标目录"永不收敛，
	// check_update 的"重下补齐"自愈出口被堵死。此时 LocalFileMD5Map 为空、下载列表天然
	// 是全量服务端文件，产物依然完整，apply 激活后即可重建主目录。
	mainMissing, mainStatErr := statMissingOrErr(fc.MainFolder)
	if mainStatErr != nil {
		util.AppendToLog(logDir(), "download.log",
			fmt.Sprintf("stat main folder before materialize %s failed: %v", fc.MainFolder, mainStatErr))
		printProgressFail(total, total, "materialize", 0,
			fmt.Sprintf("stat main folder before materialize failed: %v", mainStatErr))
		return
	}
	if mainMissing {
		util.AppendToLog(logDir(), "download.log",
			fmt.Sprintf("materialize skipped: main folder %s missing (crash recovery, downloaded full server file set)", fc.MainFolder))
	} else if err := util.CopyDirWithExclude(fc.MainFolder, targetDir,
		func(relPath string) bool {
			return config.ShouldSkipFile(relPath, fc.Shared.UnCopyFiles)
		},
		func(relPath string) bool {
			return config.ShouldSkipFolder(relPath, fc.Shared.UnCopyFolders)
		}); err != nil {
		util.AppendToLog(logDir(), "download.log",
			fmt.Sprintf("materialize unchanged files %s -> %s failed: %v", fc.MainFolder, targetDir, err))
		printProgressFail(total, total, "materialize", 0,
			fmt.Sprintf("materialize unchanged files failed: %v", err))
		return
	}
	util.AppendToLog(logDir(), "download.log",
		fmt.Sprintf("materialized unchanged files %s -> %s (apply will only rename)", fc.MainFolder, targetDir))

	// #4 删除同步（差量版）：删除集 = 上次清单 ∖ 本次清单 = 服务端确认删除的文件。
	// 时序在 materialize 之后——materialize 会把 MainFolder 的未变文件拷进版本目录，
	// 其中可能包含"上上版本有、本版已删"的文件；此刻按差量清理，apply 上线后即为干净内容。
	// 为什么必须差量而不是"版本目录内不在新清单即删"：materialize 拷入的 MainFolder
	// 运行时生成文件（应用自写配置/数据，从不在服务端清单里）会被全量差集误删，
	// apply 后应用运行时数据丢失。
	// 清单缺失（老服务器无接口 / 拉取失败 / 本地无缓存）→ 保守跳过删除
	// （与旧行为兼容，仅退化为不删）。
	if manifest, mErr := apiclient.GetReleaseManifest(fc.Shared.ServerURL, fc.Shared.ProjectName); mErr != nil {
		util.AppendToLog(logDir(), "download.log",
			fmt.Sprintf("get release manifest skipped (delete-sync disabled): %v", mErr))
	} else if len(manifest.Files) == 0 {
		// 服务器从未发布过（或清单为空）：无依据，跳过
		util.AppendToLog(logDir(), "download.log", "release manifest empty, skip delete-sync")
	} else {
		manifestSet := make(map[string]bool, len(manifest.Files))
		normalized := make([]string, 0, len(manifest.Files))
		for _, rel := range manifest.Files {
			n := normalizePath(rel)
			if !manifestSet[n] {
				manifestSet[n] = true
				normalized = append(normalized, n)
			}
		}
		// 差量依据 = 本地缓存的上次发布清单；首个正常发布周期里老客户端升级
		// 到本版时缓存尚不存在 → 本轮不删（下一轮起差量生效）。
		cached, cErr := util.ReadCachedManifest(logDir())
		if cErr != nil {
			// 缓存损坏：按"无依据"处理而非报错中断（删除同步是尽力而为的收敛手段）
			util.AppendToLog(logDir(), "download.log",
				fmt.Sprintf("read cached manifest failed (delete-sync disabled): %v", cErr))
			cached = nil
		} else if cached == nil {
			util.AppendToLog(logDir(), "download.log", "no cached manifest (first cycle), skip delete-sync")
		}
		if cached != nil {
			var deleteSet []string
			for rel := range cached {
				if !manifestSet[rel] {
					deleteSet = append(deleteSet, rel)
				}
			}
			if len(deleteSet) > 0 {
				deleteSetMap := make(map[string]bool, len(deleteSet))
				for _, rel := range deleteSet {
					deleteSetMap[rel] = true
				}
				if removed, dErr := util.RemoveFilesByRelPaths(targetDir, deleteSetMap); dErr != nil {
					util.AppendToLog(logDir(), "download.log",
						fmt.Sprintf("delete-sync walk failed: %v", dErr))
				} else if len(removed) > 0 {
					util.AppendToLog(logDir(), "download.log",
						fmt.Sprintf("delete-sync removed %d file(s) deleted by server: %s",
							len(removed), strings.Join(removed, ", ")))
				}
			}
		}
		// 固化本次清单为下轮差量依据（即便本轮没做删除也要写：标志"客户端已同步到该清单口径"）。
		if wErr := util.WriteCachedManifest(logDir(), normalized); wErr != nil {
			util.AppendToLog(logDir(), "download.log",
				fmt.Sprintf("cache release manifest failed (next delete-sync degraded): %v", wErr))
		}
	}

	// Update version.json
	// VersionPrevious 语义 = "MainFolder 当前真实内容版本"，仅在可确定时更新（修复 Bug#2）：
	//   - downloaded：MainFolder 仍是 VersionPrevious 的内容 → 保持不变（"待应用期间发布新版"
	//     不再把 VersionPrevious 错写成从未应用过的中间版本）
	//   - applied / 空：MainFolder = Version → 记录为 VersionPrevious
	//   （applying 已在函数开头 return，不会走到这里）
	if versionInfo.VersionStatus != config.VersionStatusDownloaded {
		versionInfo.VersionPrevious = versionInfo.Version
	}
	versionInfo.Version = newVersion
	versionInfo.VersionStatus = config.VersionStatusDownloaded
	versionInfo.ApplyFailCount = 0 // #2 熔断：新一轮下载成功，复位失败计数
	// 新的待应用现场：清掉上一轮回滚/apply 留下的过渡字段（RollbackPrevious/RollbackTarget/ApplyStage，F7/F5）
	clearUpdateTransientState(versionInfo)
	// M12：后置脚本路径来自服务端（after_apply_update_script），写入 version.json 前先过
	// 白名单——apply 阶段会用 cmd /c / sh 执行它，`..` 逃逸即任意命令执行（与 M11 组合）。
	// 不合法则丢弃该字段（不阻断下载），并留痕。
	if safeRelPath(latestLog.AfterApplyUpdateScript) {
		versionInfo.AfterApplyUpdateScript = latestLog.AfterApplyUpdateScript
	} else {
		versionInfo.AfterApplyUpdateScript = ""
		if latestLog.AfterApplyUpdateScript != "" {
			util.AppendToLog(logDir(), "download.log",
				fmt.Sprintf("discard unsafe after_apply_update_script from server: %q (M12 whitelist)", latestLog.AfterApplyUpdateScript))
		}
	}
	// 记录下载产物里的文件数（不含 .part/.tmp）：apply_update 据此识别"目录还在但文件被删/
	// 重下中途被杀"的残缺现场，避免混装半成品（B2）。算不出来（读目录失败）时置 0 = 未知，
	// apply 退化为"目录存在且非空"的弱校验。
	if count, cErr := countRegularFiles(targetDir); cErr == nil {
		versionInfo.DownloadedFileCount = count
	} else {
		versionInfo.DownloadedFileCount = 0
		util.AppendToLog(logDir(), "download.log",
			fmt.Sprintf("count downloaded files in %s failed: %v", targetDir, cErr))
	}
	if err := config.WriteVersion(versionInfo); err != nil {
		printProgressFail(0, 0, "version.json", 0, fmt.Sprintf("write version: %v", err))
		return
	}
	downloadedOK = true // 成功：取消"失败清理半成品目录"的兜底

	printProgressDone()
}
