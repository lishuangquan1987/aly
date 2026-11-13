package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// ProjectConfig is a GUI bookmark. The real project settings live in
// <project>/.updator/shared.json.
type ProjectConfig struct {
	DisplayName string `json:"DisplayName"`
	ProjectPath string `json:"ProjectPath"`
	MainExePath string `json:"MainExePath"`
}

// SharedConfig is the shape of .updator/shared.json.
type SharedConfig struct {
	ServerURL     string   `json:"server_url"`
	ProjectName   string   `json:"project_name"`
	IgnoreFolders []string `json:"ignore_folders"`
	IgnoreFiles   []string `json:"ignore_files"`
	UnCopyFolders []string `json:"un_copy_folders"`
	UnCopyFiles   []string `json:"un_copy_files"`
}

type appSettings struct {
	Language string `json:"language"`
	Theme    string `json:"theme,omitempty"`
	Accent   string `json:"accent,omitempty"`
}

func configDir() string {
	if d := os.Getenv("ALYPUBLISH_CONFIG_DIR"); d != "" {
		return d
	}
	switch runtime.GOOS {
	case "windows":
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "AlyPublish")
		}
	case "darwin":
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Library", "Application Support", "AlyPublish")
		}
	default:
		if d := os.Getenv("XDG_DATA_HOME"); d != "" {
			return filepath.Join(d, "AlyPublish")
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".local", "share", "AlyPublish")
		}
	}
	return filepath.Join(os.TempDir(), "AlyPublish")
}

func configPath() string   { return filepath.Join(configDir(), "config.json") }
func settingsPath() string { return filepath.Join(configDir(), "settings.json") }

func appDir() string {
	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		return filepath.Dir(exe)
	}
	wd, _ := os.Getwd()
	return wd
}

func logDir() string {
	candidates := []string{
		filepath.Join(appDir(), "logs"),
		filepath.Join(configDir(), "logs"),
		filepath.Join(os.TempDir(), "AlyPublish", "logs"),
	}
	for _, dir := range candidates {
		if err := os.MkdirAll(dir, 0o755); err == nil {
			return dir
		}
	}
	return appDir()
}

func loadProjects() []ProjectConfig {
	raw, err := os.ReadFile(configPath())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("读取项目配置失败: %v", err)
		}
		return nil
	}
	var projects []ProjectConfig
	if err := json.Unmarshal(raw, &projects); err != nil {
		log.Printf("解析项目配置失败: %v", err)
		return nil
	}
	for i := range projects {
		projects[i].DisplayName = strings.TrimSpace(projects[i].DisplayName)
		if path := strings.TrimSpace(projects[i].ProjectPath); path != "" {
			projects[i].ProjectPath = filepath.Clean(path)
		}
		projects[i].MainExePath = strings.TrimSpace(projects[i].MainExePath)
		if projects[i].DisplayName == "" {
			projects[i].DisplayName = filepath.Base(projects[i].ProjectPath)
		}
	}
	return projects
}

func saveProjects(projects []ProjectConfig) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(configPath(), append(raw, '\n'))
}

func loadSettings() appSettings {
	raw, err := os.ReadFile(settingsPath())
	if err != nil {
		return appSettings{}
	}
	var s appSettings
	_ = json.Unmarshal(raw, &s)
	return s
}

func saveSettings(s appSettings) error {
	if err := os.MkdirAll(configDir(), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return writeFileAtomic(settingsPath(), raw)
}

func writeFileAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	// os.Rename replaces the target on every supported platform (MoveFileEx
	// on Windows); removing it first would lose the file on a crash.
	return os.Rename(name, path)
}

func sharedConfigPath(projectPath string) string {
	return filepath.Join(projectPath, ".updator", "shared.json")
}

func loadShared(projectPath string) (SharedConfig, error) {
	var shared SharedConfig
	raw, err := os.ReadFile(sharedConfigPath(projectPath))
	if err != nil {
		return shared, fmt.Errorf("读取 .updator/shared.json 失败: %w", err)
	}
	if err := json.Unmarshal(raw, &shared); err != nil {
		return shared, fmt.Errorf("解析 .updator/shared.json 失败: %w", err)
	}
	return shared, nil
}

func isInitialized(projectPath string) bool {
	_, err := os.Stat(sharedConfigPath(projectPath))
	return err == nil
}

func hostRIDs() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{"windows-amd64"}
	case "darwin":
		return []string{"darwin-arm64", "darwin-amd64"}
	default:
		if runtime.GOARCH == "arm64" {
			return []string{"linux-arm64", "linux-amd64"}
		}
		return []string{"linux-amd64", "linux-arm64"}
	}
}

func executableName(rid string) string {
	if strings.HasPrefix(rid, "windows-") {
		return "aly-publish.exe"
	}
	return "aly-publish"
}

// findCLI looks for aly-publish next to the app, in the packaged payload, in
// the repository's development layout, and on PATH.
func findCLI() string {
	if v := strings.TrimSpace(os.Getenv("ALY_PUBLISH")); v != "" {
		if fileExists(v) {
			return v
		}
	}
	app := appDir()
	rids := hostRIDs()
	var candidates []string
	for _, rid := range rids {
		name := executableName(rid)
		candidates = append(candidates,
			filepath.Join(app, name),
			filepath.Join(app, "publish-cli", rid, name),
		)
	}
	dir := app
	for i := 0; i < 8; i++ {
		for _, rid := range rids {
			name := executableName(rid)
			candidates = append(candidates,
				filepath.Join(dir, "publish-cli", rid, name),
				filepath.Join(dir, "publish-cli", "aly-publish-"+rid),
				filepath.Join(dir, "publish-cli", "aly-publish-"+rid+".exe"),
				filepath.Join(dir, "publish", "publish-cli", "aly-publish.exe"),
				filepath.Join(dir, "publish", "publish-cli", "aly-publish"),
				filepath.Join(dir, "publish", "publish-cli", "aly-publish-"+rid),
				filepath.Join(dir, "publish", "publish-cli", "aly-publish-"+rid+".exe"),
			)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for _, candidate := range candidates {
		if fileExists(candidate) {
			return candidate
		}
	}
	for _, name := range []string{"aly-publish.exe", "aly-publish"} {
		if path, err := exec.LookPath(name); err == nil {
			return path
		}
	}
	return ""
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func normalizePath(path string) string {
	return filepath.Clean(path)
}

func relativeToProject(projectPath, path string) string {
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) {
		return filepath.ToSlash(filepath.Clean(path))
	}
	rel, err := filepath.Rel(projectPath, path)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return path
	}
	return filepath.ToSlash(rel)
}

func resolveProjectPath(projectPath, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(projectPath, filepath.FromSlash(path))
}

func formatSize(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d B", bytes)
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", size), "0"), ".") + " " + units[unit]
}

var versionPattern = regexp.MustCompile(`^[vV]?\d+(?:\.\d+)*$`)

func suggestVersion(current string) string {
	current = strings.TrimSpace(current)
	if !versionPattern.MatchString(current) {
		return ""
	}
	prefix := ""
	if current[0] == 'v' || current[0] == 'V' {
		prefix = current[:1]
		current = current[1:]
	}
	parts := strings.Split(current, ".")
	last := parts[len(parts)-1]
	n, err := strconv.Atoi(last)
	if err != nil || n >= int(^uint(0)>>1) {
		return ""
	}
	parts[len(parts)-1] = strconv.Itoa(n + 1)
	return prefix + strings.Join(parts, ".")
}

func diffList(current, next []string) (added, removed []string) {
	cur := make(map[string]bool, len(current))
	nxt := make(map[string]bool, len(next))
	for _, v := range current {
		cur[v] = true
	}
	for _, v := range next {
		nxt[v] = true
	}
	for _, v := range next {
		if !cur[v] {
			added = append(added, v)
		}
	}
	for _, v := range current {
		if !nxt[v] {
			removed = append(removed, v)
		}
	}
	return added, removed
}

var errCLIMissing = errors.New("未找到 aly-publish")

func initLogging() {
	f, err := os.OpenFile(filepath.Join(logDir(), "log.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	// Put the file first: a windowsgui build has no valid stderr, and
	// io.MultiWriter stops at the first writer that fails.
	log.SetOutput(io.MultiWriter(f, os.Stderr))
}
