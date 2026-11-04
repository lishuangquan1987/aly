// +build windows

package i18n

// enUS 是英文文案表。键必须与 zh_CN.go 的 zhCN 完全一致。
var enUS = map[string]string{
	KeyChecking:        "Checking for updates…",
	KeyNoUpdate:        "You are up to date",
	KeyNoUpdateBox:     "You are already on the latest version. No updates available.",
	KeyFoundVersion:    "New version v%s available",
	KeyDownloading:     "Downloading %s (%d/%d)",
	KeyDownloaded:      "Download complete",
	KeyApplying:        "Applying update…",
	KeyApplyDone:       "Update complete",
	KeyCheckNow:        "Check for updates",
	KeyDownload:        "Download",
	KeyCancel:          "Cancel",
	KeyApply:           "Apply now",
	KeyLater:           "Later",
	KeyRetry:           "Retry",
	KeyClose:           "Close",
	KeyCurrentVersion:  "Current version v%s",
	KeyAskDownload:     "Download new version v%s?",
	KeyAskApply:        "Restart to apply the update (v%s)?",
	KeyOffline:         "Unable to check for updates",
	KeyErrorGeneric:    "Something went wrong during the update",
	KeyDetail:          "Details",
	KeyUpdateAvailable: "Update available",
	KeyRollback:        "Rollback",
	KeyVersionList:     "Available versions",

	// Streamlined plan items 1/2/3 (buttons card / countdown / pause).
	KeyCountdown:   "Restarting in %d seconds…",
	KeyPause:       "Pause",
	KeyResume:      "Resume",
	KeyPaused:      "Paused",
	KeyUpdateNow:   "Update now",
	KeyRetryLater:  "Later",
	KeySpeedFmt:    "Downloaded %s / %s · %s/s",
	KeyRetrying:    "Retrying…",
	// 按钮文案受卡片主按钮宽度限制（btnPrimaryW = 104 逻辑像素，BodyPt = 9）：
	// 过长会被 walk 以省略号截断，故英文用 "Restart to apply"（≈88px），
	// 而不是 "Restart to finish update"（≈145px，会被截成 "Restart to fini…"）。
	KeyWillRestart: "Click \"Restart to apply\" to skip the wait",
	KeyRestartNow:  "Restart to apply",

	// Streamlined plan item 4 (byte-level progress).
	KeyDownloadingSimple: "Downloading…",
}
