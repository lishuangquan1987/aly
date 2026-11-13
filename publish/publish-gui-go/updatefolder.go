package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// updateClientConfig is the client.json written next to UpdateFolder.
type updateClientConfig struct {
	MainExeRelativePath  string   `json:"main_exe_relative_path"`
	MustCloseProcessName []string `json:"must_close_process_name"`
}

func updaterNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"aly-client.exe", "aly-client-ui.exe"}
	}
	return []string{"aly-client", "aly-client-ui"}
}

func updaterRIDs() []string {
	if runtime.GOOS == "windows" {
		return []string{"windows-x86", "windows-amd64"}
	}
	if runtime.GOOS == "darwin" {
		return []string{"darwin-arm64", "darwin-amd64"}
	}
	return []string{"linux-amd64", "linux-arm64"}
}

func findUpdaterFile(name string) string {
	app := appDir()
	dir := app
	for i := 0; i < 10; i++ {
		for _, rid := range updaterRIDs() {
			candidates := []string{
				filepath.Join(dir, "client", rid, name),
				filepath.Join(dir, "client", name),
				filepath.Join(dir, "publish", "publish-gui-csharp", "src", "AlyPublish", "bin", "Debug", "net8.0", "client", rid, name),
				filepath.Join(dir, "publish", "publish-gui-csharp", "src", "AlyPublish", "bin", "Release", "net8.0", "client", rid, name),
			}
			if runtime.GOOS == "windows" {
				switch name {
				case "aly-client.exe":
					candidates = append(candidates, filepath.Join(dir, "client", "aly-client", name))
				case "aly-client-ui.exe":
					candidates = append(candidates, filepath.Join(dir, "client", "aly-client-ui", "aly-client-winui", name))
				}
			}
			for _, candidate := range candidates {
				if fileExists(candidate) {
					return candidate
				}
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func copyUpdaters(updateFolder string) (warning string) {
	var missing []string
	for _, name := range updaterNames() {
		src := findUpdaterFile(name)
		if src == "" {
			missing = append(missing, name)
			continue
		}
		data, err := os.ReadFile(src)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s (%v)", name, err))
			continue
		}
		mode := os.FileMode(0o755)
		if info, err := os.Stat(src); err == nil {
			mode = info.Mode()
		}
		if err := os.WriteFile(filepath.Join(updateFolder, name), data, mode); err != nil {
			missing = append(missing, fmt.Sprintf("%s (%v)", name, err))
		}
	}
	if len(missing) > 0 {
		return "更新器文件缺失或复制失败: " + strings.Join(missing, "; ")
	}
	return ""
}

// createUpdateFolder implements the C# UpdateFolderService.Create contract:
// PackageFolder/UpdateFolder with updater executables and client.json.
func createUpdateFolder(projectPath, mainExePath, closeProcessName string) (string, error) {
	if strings.TrimSpace(mainExePath) == "" {
		return "", errors.New("main executable is required")
	}
	projectDir := filepath.Clean(projectPath)
	parent := filepath.Dir(projectDir)
	if parent == projectDir {
		return "", errors.New("project directory has no parent")
	}
	updateFolder := filepath.Join(parent, "UpdateFolder")
	if err := os.MkdirAll(updateFolder, 0o755); err != nil {
		return "", err
	}
	warning := copyUpdaters(updateFolder)

	mainExeFull := resolveProjectPath(projectDir, mainExePath)
	rel, err := filepath.Rel(updateFolder, mainExeFull)
	if err != nil || filepath.IsAbs(rel) {
		return warning, errors.New("main executable is not on the same volume as UpdateFolder")
	}
	processName := strings.TrimSpace(closeProcessName)
	if processName == "" {
		processName = strings.TrimSuffix(filepath.Base(mainExeFull), filepath.Ext(mainExeFull))
	}
	cfg := updateClientConfig{
		MainExeRelativePath:  rel,
		MustCloseProcessName: []string{processName},
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return warning, err
	}
	if err := writeFileAtomic(filepath.Join(updateFolder, "client.json"), raw); err != nil {
		return warning, err
	}
	return warning, nil
}

// ensureUpdatersFor creates the sibling UpdateFolder when the project has a
// main executable and no UpdateFolder yet, matching the C# EnsureUpdaters
// path. A missing main executable is skipped just like C# does.
func ensureUpdatersFor(projectPath, mainExePath string) string {
	if strings.TrimSpace(mainExePath) == "" {
		return ""
	}
	projectDir := filepath.Clean(projectPath)
	parent := filepath.Dir(projectDir)
	if parent == projectDir {
		return ""
	}
	updateFolder := filepath.Join(parent, "UpdateFolder")
	if info, err := os.Stat(updateFolder); err == nil && info.IsDir() {
		return ""
	}
	mainExeFull := resolveProjectPath(projectDir, mainExePath)
	if !fileExists(mainExeFull) {
		return ""
	}
	warning, err := createUpdateFolder(projectDir, mainExeFull, "")
	if err != nil {
		return err.Error()
	}
	return warning
}

// refreshUpdaters updates an existing sibling UpdateFolder without touching
// client.json, matching the C# RefreshUpdaters path.
func refreshUpdaters(projectPath string) (warning string, err error) {
	if strings.TrimSpace(projectPath) == "" {
		return "", nil
	}
	projectDir := filepath.Clean(projectPath)
	parent := filepath.Dir(projectDir)
	if parent == projectDir {
		return "", nil
	}
	updateFolder := filepath.Join(parent, "UpdateFolder")
	info, err := os.Stat(updateFolder)
	if err != nil || !info.IsDir() {
		return "", nil
	}
	return copyUpdaters(updateFolder), nil
}
