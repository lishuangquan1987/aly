package cmd

import (
	"flag"
	"fmt"
	"os"

	apiclient "aly/client/aly-client/client"
	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
)

// CheckUpdate checks for new version
func CheckUpdate() {
	fs := flag.NewFlagSet("check_update", flag.ExitOnError)
	urlFlag := fs.String("url", "", "server url")
	projectNameFlag := fs.String("project-name", "", "project name")
	fs.Parse(os.Args[2:])

	fc, err := loadFullConfig(*urlFlag, *projectNameFlag, "")
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

	versionInfo, err := config.ReadVersion()
	if err != nil {
		printOutput(false, fmt.Sprintf("read version: %v", err), nil)
		return
	}

	localVersion := stripVPrefix(versionInfo.Version)
	status := versionInfo.VersionStatus

	switch {
	case status == "" || status == config.VersionStatusApplied:
		checkUpdateApplied(fc, localVersion)

	case status == config.VersionStatusDownloaded || status == config.VersionStatusApplying:
		checkUpdatePending(fc, versionInfo, localVersion)

	default:
		// 未知状态一律按 applied 处理（与 apply/download/rollback 的口径统一，F6）：
		// check 只比较版本、不做替换；download_update 会把它重写成 downloaded 从而修复；
		// apply_update 对未知状态直接拒绝替换。这里不再"静默当成 applied"，而是明确告警。
		// 注意：check_update 可能被宿主每秒调用，故只写 stderr，不写 update.log（避免刷爆日志）。
		fmt.Fprintf(os.Stderr,
			"check_update: unknown version_status %q, treating as applied (only download_update can rewrite it)\n", status)
		checkUpdateApplied(fc, localVersion)
	}
}

// checkUpdateApplied 处理 applied / 空状态：必须联系服务器判断是否有更新
func checkUpdateApplied(fc *FullConfig, localVersion string) {
	project, projectErr := apiclient.FindProjectByName(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if projectErr != nil {
		fmt.Fprintf(os.Stderr, "check_update: FindProjectByName failed: %v\n", projectErr)
	}
	logs, logsErr := apiclient.GetProjectChangeLogs(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if logsErr != nil {
		fmt.Fprintf(os.Stderr, "check_update: GetProjectChangeLogs failed: %v\n", logsErr)
	}

	serverReachable := projectErr == nil && logsErr == nil && len(logs) > 0
	if !serverReachable {
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:      false,
			CurrentVersion: localVersion,
		})
		return
	}

	latestLog := findLatestLog(logs)
	serverVersion := stripVPrefix(latestLog.Version)

	if !needUpdate(serverVersion, localVersion) {
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:      false,
			CurrentVersion: localVersion,
		})
	} else {
		forceUpdate := project.ForceUpdate
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:          true,
			NeedDownloadUpdate: true,
			CurrentVersion:     localVersion,
			NewVersion:         serverVersion,
			ForceUpdate:        &forceUpdate,
		})
	}
}

// checkUpdatePending 处理 downloaded / applying 状态：优先继续 apply，只有服务器版本不一致才重新下载。
//
// 断网（服务器不可达 / 无可用变更日志）：downloaded 状态不报告待应用更新（has_update=false）——
// 无法与服务器核对就不上报，避免断网时上位机显示"可更新"、联网时却是另一种状态；已下载内容
// 与 version.json 保持不动，服务器恢复后自动继续 apply。
// applying（崩溃恢复）与回滚续跑必须在断网时照样上报（见函数内提前返回的两条分支）。
func checkUpdatePending(fc *FullConfig, versionInfo *config.VersionInfo, localVersion string) {
	// 回滚中断（status=applying && rollback_previous != ""）：优先完成回滚，
	// 绝不因服务器发布新版本而转向下载/升级（修复 Bug#1/#5）。
	// SDK 据此得到 NeedDownloadUpdate=false → 调用 apply_update → resumeRollback 续跑回滚。
	if versionInfo.RollbackPrevious != "" {
		target := versionInfo.RollbackTarget
		if target == "" {
			target = versionInfo.Version // 老数据兜底：至少不下载、不升级
		}
		prev := versionInfo.RollbackPrevious
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:          true,
			NeedDownloadUpdate: false,
			CurrentVersion:     stripVPrefix(prev),
			NewVersion:         stripVPrefix(target),
		})
		return
	}

	// applying（更新中断，非回滚）：先完成/重试在途 apply，不因服务器新版本转向重下（修复 Bug#3）。
	// 若 apply 失败且主目录/版本目录都不存在，applyFailureFallback 会降级为 downloaded，
	// 下一轮 check 走下方 downloaded 分支即可下载新版本（逃生口保留）。
	if versionInfo.VersionStatus == config.VersionStatusApplying {
		currentVer := versionInfo.VersionPrevious
		if currentVer == "" {
			currentVer = versionInfo.Version
		}
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:          true,
			NeedDownloadUpdate: false,
			CurrentVersion:     stripVPrefix(currentVer),
			NewVersion:         stripVPrefix(versionInfo.Version),
		})
		return
	}

	// 只请求变更日志（1 次 HTTP），减少断网时的等待
	logs, logsErr := apiclient.GetProjectChangeLogs(fc.Shared.ServerURL, fc.Shared.ProjectName)
	if logsErr != nil {
		fmt.Fprintf(os.Stderr, "check_update: GetProjectChangeLogs failed: %v\n", logsErr)
	}

	serverReachable := logsErr == nil && len(logs) > 0

	if !serverReachable {
		// 断网（或服务器无可用变更日志）：本机虽有已下载的待应用版本，但无法与服务器核对，
		// 不报告"待应用更新"（否则断网时上位机显示 ready to apply，与联网时状态不一致）。
		// 已下载内容与 version.json 原样保留（check_update 只读），服务器可达后走下方联网分支继续 apply。
		// 注意：applying（崩溃恢复）与回滚续跑分支已在上方提前返回，不受此判断影响 —— 那两条路径
		// 必须在断网时照样报 has_update=true，否则主目录缺失/回滚中断的机器永远无法自愈。
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate: false,
			// 当前实际内容版本（downloaded 状态下 MainFolder 仍是 VersionPrevious 的内容；
			// 首次部署时为"未知"，输出空串，不再回退成待应用的 Version 造成 current == new 自相矛盾）
			CurrentVersion: stripVPrefix(versionInfo.VersionPrevious),
		})
		return
	}

	// F3：待应用版本目录缺失/为空/文件数少于下载时记录的数量（下载产物丢失或残缺）。
	// 注意此处只做本地判断，决策融合进下面的版本比较：服务器同版本 → 要求重下同版本补齐；
	// 服务器更高 → 按常规"重新下载更高版本"（download 会连同状态一起修复）；
	// 服务器更低 → 落到"继续 apply"，由 apply_update 的 F3 守卫拒绝并自愈为 applied + Previous。
	// apply_update 侧另有兜底：拒绝替换并自愈（防直接调用时假成功）。
	// A2：目录存在但读不到（ACL/杀软锁）时不当成"丢失"，避免无谓重下。
	pendingMissing := false
	if pendingDir, dirErr := fc.ExeCfg.AppVersionDir(versionInfo.Version); dirErr == nil {
		pendingCount, countErr := countRegularFiles(pendingDir)
		switch {
		case countErr != nil && !os.IsNotExist(countErr):
			fmt.Fprintf(os.Stderr,
				"check_update: pending version dir unreadable (%s: %v), skip re-download decision\n", pendingDir, countErr)
		case pendingCount == 0:
			pendingMissing = true
			fmt.Fprintf(os.Stderr,
				"check_update: pending version dir missing or empty (%s), need re-download\n", pendingDir)
		case versionInfo.DownloadedFileCount > 0 && pendingCount < versionInfo.DownloadedFileCount:
			pendingMissing = true
			fmt.Fprintf(os.Stderr,
				"check_update: pending version dir incomplete (%s: %d files < recorded %d), need re-download\n",
				pendingDir, pendingCount, versionInfo.DownloadedFileCount)
		}
	}

	latestLog := findLatestLog(logs)
	serverVersion := stripVPrefix(latestLog.Version)

	if !needUpdate(serverVersion, localVersion) {
		// F3：服务器"正好是该待应用版本"但产物丢失/残缺 → 重下同版本补齐。
		// 仅在 serverVersion == localVersion 时成立：服务器更低时 download 会因
		// "already at latest version"拒绝重下（永远补不回来），必须落到下面的 apply 自愈。
		if pendingMissing && serverVersion == localVersion {
			forceUpdate := false
			project, projectErr := apiclient.FindProjectByName(fc.Shared.ServerURL, fc.Shared.ProjectName)
			if projectErr != nil {
				fmt.Fprintf(os.Stderr, "check_update: FindProjectByName failed: %v\n", projectErr)
			} else {
				forceUpdate = project.ForceUpdate
			}
			printOutput(true, "", &model.CheckUpdateData{
				HasUpdate:          true,
				NeedDownloadUpdate: true,
				CurrentVersion:     stripVPrefix(versionInfo.VersionPrevious),
				NewVersion:         stripVPrefix(versionInfo.Version),
				ForceUpdate:        &forceUpdate,
			})
			return
		}
		// 异常/遗留状态：VersionPrevious == Version 且非空 —— 备份目录与版本目录同路径，
		// apply_update 会拒绝替换。此时不再上报"待应用更新"，否则 SDK 会反复
		// apply → 拒绝 → 兜底回 downloaded，每轮都要关掉并拉起主程序（F4）。
		if versionInfo.VersionPrevious != "" && versionInfo.VersionPrevious == versionInfo.Version {
			fmt.Fprintf(os.Stderr,
				"check_update: version_status=downloaded but version_previous == version (%s): invalid state, not reporting pending update\n",
				versionInfo.Version)
			printOutput(true, "", &model.CheckUpdateData{
				HasUpdate:      false,
				CurrentVersion: stripVPrefix(versionInfo.Version),
			})
			return
		}
		// 服务器版本不高于本地待应用版本：继续 apply 已下载版本。
		// current_version 用"MainFolder 真实内容版本"（VersionPrevious）：为空即未知（首次部署），
		// 不再回退成待应用的 Version 造成 current == new 自相矛盾（与断网分支口径统一，F10）。
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:          true,
			NeedDownloadUpdate: false,
			CurrentVersion:     stripVPrefix(versionInfo.VersionPrevious),
			NewVersion:         stripVPrefix(versionInfo.Version),
		})
	} else {
		// 服务器有新版本：需要重新下载，补拿 ForceUpdate
		forceUpdate := false
		project, projectErr := apiclient.FindProjectByName(fc.Shared.ServerURL, fc.Shared.ProjectName)
		if projectErr != nil {
			fmt.Fprintf(os.Stderr, "check_update: FindProjectByName failed: %v\n", projectErr)
		} else {
			forceUpdate = project.ForceUpdate
		}
		printOutput(true, "", &model.CheckUpdateData{
			HasUpdate:          true,
			NeedDownloadUpdate: true,
			CurrentVersion:     localVersion,
			NewVersion:         serverVersion,
			ForceUpdate:        &forceUpdate,
		})
	}
}

// needUpdate 统一版本判断口径：仅当服务器版本严格高于本地版本时才需要更新。
// 与 download_update 的 compareVersion(newVersion, currentVersion) > 0 语义保持一致，
// 避免服务端版本低于本地（回滚发布/连错环境）时 check 报有更新而 download 报已是最新，
// 导致开启强制更新的客户端永久卡死。
func needUpdate(serverVersion, localVersion string) bool {
	return compareVersion(serverVersion, localVersion) > 0
}

// findLatestLog 返回版本号最大的变更日志；版本相同则取 ID 最大（最新记录优先）。
// 按版本号而非 ID 取最新，避免"回滚发布"（重发旧版本号生成新记录）时取到旧版本（#19）。
func findLatestLog(logs []model.ProjectChangeLog) model.ProjectChangeLog {
	latest := logs[0]
	for i := 1; i < len(logs); i++ {
		c := compareVersion(stripVPrefix(logs[i].Version), stripVPrefix(latest.Version))
		if c > 0 || (c == 0 && logs[i].ID > latest.ID) {
			latest = logs[i]
		}
	}
	return latest
}
