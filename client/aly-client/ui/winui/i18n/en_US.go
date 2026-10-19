// +build windows

package i18n

// enUS 是英文文案表。键必须与 zh_CN.go 的 zhCN 完全一致。
var enUS = map[string]string{
	KeyChecking:        "Checking for updates…",
	KeyNoUpdate:        "You are up to date",
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
}
