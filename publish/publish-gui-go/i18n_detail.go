package main

// Strings only the project detail page uses. Shared keys stay in i18n.go;
// these add the count-bearing publish captions and the C# hints that have no
// existing translation.
func init() {
	mergeTranslations(map[string]map[string]string{
		"zh": {
			"detail.refreshTip":       "刷新（F5）",
			"detail.settingsTip":      "项目设置（Ctrl+E）",
			"changes.staging":         "正在暂存…",
			"changes.unstaging":       "正在取消暂存…",
			"history.latest":          "最新",
			"history.emptyHint":       "发布后的版本会显示在这里。",
			"publish.publishCountOne": "发布 {count} 个文件",
			"publish.publishCount":    "发布 {count} 个文件",
			"publish.preparingCount":  "正在准备上传 {count} 个文件…",
			"publish.uploadingCount":  "正在上传 {file}（{count}/{total}）",
			"publish.doneCount":       "完成 {file}（{count}/{total}）",
			"publish.failedCount":     "失败 {file}（{count}/{total}）：{error}",
		},
		"en": {
			"detail.refreshTip":       "Refresh (F5)",
			"detail.settingsTip":      "Project settings (Ctrl+E)",
			"changes.staging":         "Staging…",
			"changes.unstaging":       "Unstaging…",
			"history.latest":          "Latest",
			"history.emptyHint":       "Published versions appear here.",
			"publish.publishCountOne": "Publish {count} file",
			"publish.publishCount":    "Publish {count} files",
			"publish.preparingCount":  "Preparing to upload {count} files…",
			"publish.uploadingCount":  "Uploading {file} ({count}/{total})",
			"publish.doneCount":       "Done {file} ({count}/{total})",
			"publish.failedCount":     "Failed {file} ({count}/{total}): {error}",
		},
	})
}
