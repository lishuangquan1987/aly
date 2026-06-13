package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SharedConfig publish-cli + client 共用配置（.updator/shared.json）
type SharedConfig struct {
	ServerURL     string   `json:"server_url"`
	ProjectName   string   `json:"project_name"`
	IgnoreFolders []string `json:"ignore_folders"`
	IgnoreFiles   []string `json:"ignore_files"`
}

// PublishConfig publish-cli 专有配置（.updator/publish.json，本地不上传）
type PublishConfig struct {
}

// DefaultShared 返回默认共用配置
func DefaultShared() SharedConfig {
	return SharedConfig{
		IgnoreFolders: []string{},
		IgnoreFiles:   []string{},
	}
}

// DefaultPublish 返回默认 publish-cli 配置
func DefaultPublish() PublishConfig {
	return PublishConfig{}
}

// UpdatorDir 返回 .updator/ 目录路径
func UpdatorDir(projectPath string) string {
	return filepath.Join(projectPath, ".updator")
}

// SharedPath 返回 .updator/shared.json 路径
func SharedPath(projectPath string) string {
	return filepath.Join(UpdatorDir(projectPath), "shared.json")
}

// PublishPath 返回 .updator/publish.json 路径
func PublishPath(projectPath string) string {
	return filepath.Join(UpdatorDir(projectPath), "publish.json")
}

// LoadShared 加载共用配置
func LoadShared(projectPath string) (SharedConfig, error) {
	cfg := DefaultShared()
	path := SharedPath(projectPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse shared.json: %w", err)
	}
	return cfg, nil
}

// SaveShared 保存共用配置
func SaveShared(projectPath string, cfg SharedConfig) error {
	return writeJSON(SharedPath(projectPath), cfg)
}

// LoadPublish 加载 publish-cli 专有配置
func LoadPublish(projectPath string) (PublishConfig, error) {
	cfg := DefaultPublish()
	path := PublishPath(projectPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse publish.json: %w", err)
	}
	return cfg, nil
}

// SavePublish 保存 publish-cli 专有配置
func SavePublish(projectPath string, cfg PublishConfig) error {
	return writeJSON(PublishPath(projectPath), cfg)
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
	return os.WriteFile(path, data, 0600)
}
