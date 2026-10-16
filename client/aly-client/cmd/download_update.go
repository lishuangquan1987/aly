package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	apiclient "aly/client/aly-client/client"
	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
	"aly/client/aly-client/util"
)

const largeFileThreshold = 100 * 1024 * 1024 // 100MB

// DownloadUpdate downloads only changed files from server
func DownloadUpdate() {
	fs := flag.NewFlagSet("download_update", flag.ExitOnError)
	urlFlag := fs.String("url", "", "server url")
	projectNameFlag := fs.String("project-name", "", "project name")
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	fs.Parse(os.Args[2:])

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
		printOutput(true, "", &model.DownloadUpdateData{Version: currentVersion})
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
				printOutput(true, "", &model.DownloadUpdateData{Version: newVersion})
				return
			case countErr != nil && !os.IsNotExist(countErr):
				// A2：目录存在但读不到（ACL/杀软锁/慢盘）：不当成"丢失"而触发全量重下
				util.AppendToLog(logDir(), "download.log",
					fmt.Sprintf("downloaded 状态但待应用版本目录不可读（%s: %v），跳过重下", targetDir, countErr))
				printOutput(true, "", &model.DownloadUpdateData{Version: newVersion})
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
	for i := range serverFiles {
		relPath := normalizePath(serverFiles[i].FileRelativePath)

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
	for _, dl := range downloadList {
		relPath := normalizePath(dl.serverFile.FileRelativePath)
		localPath := filepath.Join(targetDir, filepathFromSlash(relPath))

		printProgress(dl.idx, total, relPath, "START", dl.serverFile.FileSize, "")

		// Download with retry up to 3 times
		var lastErr string
		for retry := 0; retry < 3; retry++ {
			if err := apiclient.DownloadFileWithResume(fc.Shared.ServerURL, dl.serverFile.FileAbsolutePath, localPath, dl.serverFile.FileSize, largeFileThreshold); err != nil {
				lastErr = fmt.Sprintf("download error: %v", err)
				if retry == 2 {
					exeDir, _ := config.ExeDir()
					util.AppendToLog(exeDir, fmt.Sprintf("update_%s_fail.log", newVersion),
						fmt.Sprintf("%s %s", dl.serverFile.FileRelativePath, lastErr))
					printProgressFail(dl.idx, total, relPath, dl.serverFile.FileSize, lastErr)
					return
				}
				continue
			}

			// Verify MD5 + SHA256
			localMD5, md5Err := util.FileMD5(localPath)
			localSHA256, shaErr := util.FileSHA256(localPath)

			if md5Err != nil || shaErr != nil {
				lastErr = "hash compute error"
				if retry == 2 {
					exeDir, _ := config.ExeDir()
					util.AppendToLog(exeDir, fmt.Sprintf("update_%s_fail.log", newVersion),
						fmt.Sprintf("%s %s", dl.serverFile.FileRelativePath, lastErr))
					printProgressFail(dl.idx, total, relPath, dl.serverFile.FileSize, lastErr)
					return
				}
				os.Remove(localPath)
				continue
			}

			if localMD5 == dl.serverFile.MD5 && localSHA256 == dl.serverFile.SHA256 {
				break
			}

			lastErr = "checksum mismatch"
			if retry == 2 {
				exeDir, _ := config.ExeDir()
				util.AppendToLog(exeDir, fmt.Sprintf("update_%s_fail.log", newVersion),
					fmt.Sprintf("%s %s (server_md5=%s local_md5=%s)", dl.serverFile.FileRelativePath, lastErr, dl.serverFile.MD5, localMD5))
				printProgressFail(dl.idx, total, relPath, dl.serverFile.FileSize, lastErr)
				return
			}
			os.Remove(localPath)
		}

		printProgress(dl.idx, total, relPath, "DONE", dl.serverFile.FileSize, "")
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
	// 新的待应用现场：清掉上一轮回滚/apply 留下的过渡字段（RollbackPrevious/RollbackTarget/ApplyStage，F7/F5）
	clearUpdateTransientState(versionInfo)
	versionInfo.AfterApplyUpdateScript = latestLog.AfterApplyUpdateScript
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
