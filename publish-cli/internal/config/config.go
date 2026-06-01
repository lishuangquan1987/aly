package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config 发布项目配置
type Config struct {
	Server struct {
		URL string `json:"url"`
	} `json:"server"`
	Project struct {
		Name string `json:"name"`
		Path string `json:"path"`
		ID   int    `json:"id"`
	} `json:"project"`
	Ignore struct {
		Folders []string `json:"folders"`
		Files   []string `json:"files"`
	} `json:"ignore"`
	Output struct {
		Format string `json:"format"` // human / json
	} `json:"output"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	c := Config{}
	c.Output.Format = "human"
	c.Ignore.Folders = []string{}
	c.Ignore.Files = []string{}
	return c
}

// GlobalPath 全局配置文件路径
func GlobalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".publish-cli", "config.json"), nil
}

// ProjectPath 项目级配置文件路径（<project-path>/.publish-cli/config.json）
func ProjectPath(projectPath string) string {
	return filepath.Join(projectPath, ".publish-cli", "config.json")
}

// LoadGlobal 加载全局配置
func LoadGlobal() (Config, error) {
	cfg := DefaultConfig()
	path, err := GlobalPath()
	if err != nil {
		return cfg, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse global config: %w", err)
	}
	return cfg, nil
}

// LoadProject 加载项目级配置（返回合并后的完整配置）
func LoadProject(projectPath string) (Config, error) {
	global, _ := LoadGlobal()
	path := ProjectPath(projectPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return global, nil
		}
		return global, err
	}
	var projectCfg Config
	if err := json.Unmarshal(data, &projectCfg); err != nil {
		return global, fmt.Errorf("parse project config: %w", err)
	}
	// 合并：项目级覆盖全局
	merged := merge(global, projectCfg)
	return merged, nil
}

// SaveGlobal 保存全局配置
func SaveGlobal(cfg Config) error {
	path, err := GlobalPath()
	if err != nil {
		return err
	}
	return writeJSON(path, cfg)
}

// SaveProject 保存项目级配置（仅写入项目特有字段）
func SaveProject(projectPath string, cfg Config) error {
	path := ProjectPath(projectPath)
	return writeJSON(path, cfg)
}

// merge 项目级覆盖全局级（非零值覆盖）
func merge(global, project Config) Config {
	if project.Server.URL != "" {
		global.Server.URL = project.Server.URL
	}
	if project.Project.Name != "" {
		global.Project.Name = project.Project.Name
	}
	if project.Project.Path != "" {
		global.Project.Path = project.Project.Path
	}
	if project.Project.ID != 0 {
		global.Project.ID = project.Project.ID
	}
	if len(project.Ignore.Folders) > 0 {
		global.Ignore.Folders = project.Ignore.Folders
	}
	if len(project.Ignore.Files) > 0 {
		global.Ignore.Files = project.Ignore.Files
	}
	if project.Output.Format != "" {
		global.Output.Format = project.Output.Format
	}
	return global
}

func writeJSON(path string, v interface{}) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
