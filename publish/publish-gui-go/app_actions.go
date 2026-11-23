package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

func (a *app) openCLIFolder() {
	if !a.cli.available() {
		a.toastMsg = a.t("toolbar.cliMissing")
		return
	}
	dir := filepath.Dir(a.cli.exe)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		a.toastMsg = a.t("folder.missing")
		return
	}
	go func() { _ = mygo.Shell.OpenPath(dir) }()
}

func (a *app) openLogFolder() {
	dir := logDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.toastMsg = err.Error()
		return
	}
	go func() { _ = mygo.Shell.OpenPath(dir) }()
}

func (a *app) openProjectFolder() {
	d := a.detail
	if d == nil {
		return
	}
	path := d.project.ProjectPath
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		a.toastMsg = a.t("folder.missing")
		return
	}
	go func() { _ = mygo.Shell.OpenPath(path) }()
}

func (a *app) launchMain() {
	d := a.detail
	if d == nil {
		return
	}
	exe := resolveProjectPath(d.project.ProjectPath, d.project.MainExePath)
	if exe == "" {
		a.toastMsg = a.t("edit.mainExeHint")
		return
	}
	if !fileExists(exe) {
		a.toastMsg = a.tf("launch.missing", "path", exe)
		return
	}
	switch strings.ToLower(filepath.Ext(exe)) {
	case ".bat", ".cmd":
		go func() { _ = mygo.Shell.OpenPath(exe) }()
		return
	}
	go func() {
		cmd := exec.Command(exe)
		cmd.Dir = filepath.Dir(exe)
		if err := cmd.Start(); err != nil {
			a.post(func() { a.toastMsg = err.Error() })
		}
	}()
}

func (a *app) openFileLocation(f fileItem) {
	d := a.detail
	if d == nil {
		return
	}
	full := resolveProjectPath(d.project.ProjectPath, f.RelativePath)
	dir := filepath.Dir(full)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		a.toastMsg = a.t("folder.missing")
		return
	}
	go func() { _ = mygo.Shell.OpenPath(dir) }()
}

func (a *app) pickFolder(multiple bool, title string, cb func([]string)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    a.win,
			Title:     title,
			Directory: true,
			Multiple:  multiple,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.post(func() { cb(paths) })
	}()
}

func (a *app) pickFiles(title string, cb func([]string)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.win,
			Title:    title,
			Multiple: true,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.post(func() { cb(paths) })
	}()
}

func (a *app) pickExecutable(title string, cb func(string)) {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:   a.win,
			Title:    title,
			Multiple: false,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.post(func() { cb(paths[0]) })
	}()
}

func removeValue(list []string, value string) []string {
	for i, item := range list {
		if item == value {
			return append(list[:i], list[i+1:]...)
		}
	}
	return list
}

func (a *app) addProjectPath(list *[]string, projectPath, path string) {
	rel, ok := projectRelativePath(projectPath, path)
	if !ok {
		a.toastMsg = a.t("edit.outside")
		return
	}
	*list = appendUnique(*list, rel)
}

func projectRelativePath(projectPath, path string) (string, bool) {
	rel := relativeToProject(projectPath, path)
	if rel == "" || filepath.IsAbs(rel) {
		return "", false
	}
	return rel, true
}

func appendUnique(list []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return list
	}
	for _, item := range list {
		if item == value {
			return list
		}
	}
	return append(list, value)
}
