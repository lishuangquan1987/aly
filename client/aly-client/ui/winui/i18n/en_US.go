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
	KeyAskApply:        "Restart now to apply v%s?",
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
	KeyWillRestart: "Click \"Restart now\" to skip the wait",
	KeyRestartNow:  "Restart now",

	// Streamlined plan item 4 (byte-level progress).
	KeyDownloadingSimple: "Downloading…",
}
