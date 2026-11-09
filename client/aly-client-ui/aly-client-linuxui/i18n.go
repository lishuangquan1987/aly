package main

import (
	"fmt"
	"strings"
)

const (
	keyChecking          = "checking"
	keyNoUpdate          = "no_update"
	keyNoUpdateBox       = "no_update_box"
	keyFoundVersion      = "found_version"
	keyDownloading       = "downloading"
	keyDownloaded        = "downloaded"
	keyApplying          = "applying"
	keyApplyDone         = "apply_done"
	keyCheckNow          = "check_now"
	keyDownload          = "download"
	keyCancel            = "cancel"
	keyApply             = "apply"
	keyLater             = "later"
	keyRetry             = "retry"
	keyClose             = "close"
	keyCurrentVersion    = "current_version"
	keyAskDownload       = "ask_download"
	keyAskApply          = "ask_apply"
	keyOffline           = "offline"
	keyErrorGeneric      = "error_generic"
	keyDetail            = "detail"
	keyUpdateAvailable   = "update_available"
	keyRollback          = "rollback"
	keyVersionList       = "version_list"
	keyCountdown         = "countdown"
	keyPause             = "pause"
	keyResume            = "resume"
	keyPaused            = "paused"
	keyUpdateNow         = "update_now"
	keyRetryLater        = "retry_later"
	keySpeedFmt          = "speed_fmt"
	keyRetrying          = "retrying"
	keyWillRestart       = "will_restart"
	keyRestartNow        = "restart_now"
	keyDownloadingSimple = "downloading_simple"
)

var zhCN = map[string]string{
	keyChecking:          "正在检查更新…",
	keyNoUpdate:          "已是最新版本",
	keyNoUpdateBox:       "当前已是最新版本，没有可用的更新。",
	keyFoundVersion:      "发现新版本 v%s",
	keyDownloading:       "正在下载 %s（%d/%d）",
	keyDownloaded:        "下载完成",
	keyApplying:          "正在应用更新…",
	keyApplyDone:         "更新完成",
	keyCheckNow:          "检查更新",
	keyDownload:          "下载",
	keyCancel:            "取消",
	keyApply:             "立即应用",
	keyLater:             "稍后",
	keyRetry:             "重试",
	keyClose:             "关闭",
	keyCurrentVersion:    "当前版本 v%s",
	keyAskDownload:       "是否下载新版本 v%s？",
	keyAskApply:          "是否重启以完成更新（v%s）？",
	keyOffline:           "暂时无法检查更新",
	keyErrorGeneric:      "更新过程中出现问题",
	keyDetail:            "详细信息",
	keyUpdateAvailable:   "有可用更新",
	keyRollback:          "回滚",
	keyVersionList:       "可回滚版本",
	keyCountdown:         "将在 %d 秒后自动重启…",
	keyPause:             "暂停",
	keyResume:            "继续",
	keyPaused:            "已暂停",
	keyUpdateNow:         "立即更新",
	keyRetryLater:        "稍后再说",
	keySpeedFmt:          "已下载 %s / %s · %s/s",
	keyRetrying:          "正在重试…",
	keyWillRestart:       "点「重启完成更新」可跳过等待",
	keyRestartNow:        "重启完成更新",
	keyDownloadingSimple: "正在下载…",
}

var enUS = map[string]string{
	keyChecking:          "Checking for updates…",
	keyNoUpdate:          "You are up to date",
	keyNoUpdateBox:       "You are already on the latest version. No updates available.",
	keyFoundVersion:      "New version v%s available",
	keyDownloading:       "Downloading %s (%d/%d)",
	keyDownloaded:        "Download complete",
	keyApplying:          "Applying update…",
	keyApplyDone:         "Update complete",
	keyCheckNow:          "Check for updates",
	keyDownload:          "Download",
	keyCancel:            "Cancel",
	keyApply:             "Apply now",
	keyLater:             "Later",
	keyRetry:             "Retry",
	keyClose:             "Close",
	keyCurrentVersion:    "Current version v%s",
	keyAskDownload:       "Download new version v%s?",
	keyAskApply:          "Restart to apply the update (v%s)?",
	keyOffline:           "Unable to check for updates",
	keyErrorGeneric:      "Something went wrong during the update",
	keyDetail:            "Details",
	keyUpdateAvailable:   "Update available",
	keyRollback:          "Rollback",
	keyVersionList:       "Available versions",
	keyCountdown:         "Restarting in %d seconds…",
	keyPause:             "Pause",
	keyResume:            "Resume",
	keyPaused:            "Paused",
	keyUpdateNow:         "Update now",
	keyRetryLater:        "Later",
	keySpeedFmt:          "Downloaded %s / %s · %s/s",
	keyRetrying:          "Retrying…",
	keyWillRestart:       "Click \"Restart to apply\" to skip the wait",
	keyRestartNow:        "Restart to apply",
	keyDownloadingSimple: "Downloading…",
}

func normalizeLang(lang string) string {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "en", "en-us", "en_us":
		return "en"
	case "zh", "zh-cn", "zh_cn", "zh-hans", "zh-hans-cn":
		return "zh-Hans"
	default:
		return "zh-Hans"
	}
}

func tr(lang, key string) string {
	m := zhCN
	if normalizeLang(lang) == "en" {
		m = enUS
	}
	if v, ok := m[key]; ok {
		return v
	}
	return key
}

func trf(lang, key string, a ...interface{}) string {
	return fmt.Sprintf(tr(lang, key), a...)
}
