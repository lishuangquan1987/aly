// +build windows

package i18n

// zhCN 是简体中文文案表。键必须与 en_US.go 的 enUS 完全一致。
var zhCN = map[string]string{
	KeyChecking:        "正在检查更新…",
	KeyNoUpdate:        "已是最新版本",
	KeyFoundVersion:    "发现新版本 v%s",
	KeyDownloading:     "正在下载 %s（%d/%d）",
	KeyDownloaded:      "下载完成",
	KeyApplying:        "正在应用更新…",
	KeyApplyDone:       "更新完成",
	KeyCheckNow:        "检查更新",
	KeyDownload:        "下载",
	KeyCancel:          "取消",
	KeyApply:           "立即应用",
	KeyLater:           "稍后",
	KeyRetry:           "重试",
	KeyClose:           "关闭",
	KeyCurrentVersion:  "当前版本 v%s",
	KeyAskDownload:     "是否下载新版本 v%s？",
	KeyAskApply:        "是否立即重启以应用 v%s？",
	KeyOffline:         "暂时无法检查更新",
	KeyErrorGeneric:    "更新过程中出现问题",
	KeyDetail:          "详细信息",
	KeyUpdateAvailable: "有可用更新",
	KeyRollback:        "回滚",
	KeyVersionList:     "可回滚版本",
}
