package main

// init registers the strings of the project-settings and add-local-project
// dialogs. Keys are namespaced edit.* / addLocal.*; existing keys are reused
// where they already say the right thing (edit.cancel, edit.add, edit.browse,
// addLocal.confirm, addProject.path, addProject.displayNameHint, …).
func init() {
	mergeTranslations(map[string]map[string]string{
		"zh": {
			"edit.localDisplay":       "本机显示",
			"edit.configSection":      "项目配置",
			"edit.displayNameHint":    "显示在左侧项目列表中。",
			"edit.projectNameHint":    "服务端上的项目名，用于同步忽略规则。",
			"edit.subtitle":           "正在编辑 {project}",
			"edit.knownServers":       "常用服务器",
			"edit.loadFailed":         "加载 .updator/shared.json 失败：{error}",
			"edit.requireServer":      "请填写服务器地址。",
			"edit.requireProject":     "请填写服务端项目名。",
			"edit.requireDisplayName": "请填写显示名称。",
			"edit.mainExeStored":      "将保存为 {path}",
			"edit.mainExeOutside":     "不在项目目录内，将保存绝对路径。",
			"edit.syncNoServer":       "未设置服务器地址或项目名，忽略规则未同步到服务端。",

			"addLocal.folderPlaceholder": "包含 .updator/shared.json 的项目目录",
			"addLocal.dropHint":          "也可以把文件夹拖到上面。",
			"addLocal.folderHint":        "选择已初始化的项目目录（含 .updator/shared.json）。",
			"addLocal.dirMissing":        "目录不存在。",
			"addLocal.configFound":       "已找到项目配置，确认预览后添加。",
			"addLocal.adding":            "正在更新 UpdateFolder…",
		},
		"en": {
			"edit.localDisplay":       "Local display",
			"edit.configSection":      "Project config",
			"edit.displayNameHint":    "Shown in the project list on the left.",
			"edit.projectNameHint":    "The project name on the server; used to sync ignore rules.",
			"edit.subtitle":           "Editing {project}",
			"edit.knownServers":       "Known servers",
			"edit.loadFailed":         "Could not load .updator/shared.json: {error}",
			"edit.requireServer":      "Server URL is required.",
			"edit.requireProject":     "Server project is required.",
			"edit.requireDisplayName": "Display name is required.",
			"edit.mainExeStored":      "Will be saved as {path}",
			"edit.mainExeOutside":     "Outside the project; an absolute path will be saved.",
			"edit.syncNoServer":       "No server URL or project name; ignore rules were not synced to the server.",

			"addLocal.folderPlaceholder": "Project folder with .updator/shared.json",
			"addLocal.dropHint":          "You can also drop a folder here.",
			"addLocal.folderHint":        "Choose an initialized project folder (with .updator/shared.json).",
			"addLocal.dirMissing":        "That folder does not exist.",
			"addLocal.configFound":       "Configuration found; review the preview and add the project.",
			"addLocal.adding":            "Updating UpdateFolder…",
		},
	})
}
