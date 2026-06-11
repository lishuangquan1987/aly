package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Config 对应精简版 client.json（main_exe_relative_path + must_close_process_name）
type Config struct {
	MainExeRelativePath  string   `json:"main_exe_relative_path"`
	MustCloseProcessName []string `json:"must_close_process_name"`
}

// SharedConfig 对应 .updator/shared.json（publish-cli + client 共用）
type SharedConfig struct {
	ServerURL     string   `json:"server_url"`
	ProjectName   string   `json:"project_name"`
	IgnoreFolders []string `json:"ignore_folders"`
	IgnoreFiles   []string `json:"ignore_files"`
}

// ExeDir 返回 zap-update.exe 所在目录 (UpdateFolder/)
func ExeDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("get executable path failed: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		resolved = exePath
	}
	return filepath.Dir(resolved), nil
}

// PackageDir 返回包根目录 (PackageFolder/)，即 ExeDir 的父目录
func PackageDir() (string, error) {
	exeDir, err := ExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exeDir), nil
}

// UpdatorDir 返回 .updator/ 目录路径（位于 mainFolder 下）
func UpdatorDir(mainFolder string) string {
	return filepath.Join(mainFolder, ".updator")
}

func configPath() (string, error) {
	dir, err := ExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "client.json"), nil
}

// LoadConfig 从 client.json 加载配置（main_exe_relative_path + must_close_process_name）
func LoadConfig() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := ioutilReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file %s failed: %v", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file failed: %v", err)
	}

	return &cfg, nil
}

// LoadSharedConfig 从 ApplicationFolder/.updator/shared.json 加载共用配置
func LoadSharedConfig(mainFolder string) (*SharedConfig, error) {
	path := filepath.Join(UpdatorDir(mainFolder), "shared.json")
	data, err := ioutilReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read shared.json failed: %v", err)
	}
	var cfg SharedConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse shared.json failed: %v", err)
	}
	return &cfg, nil
}

// MergeFlags 将命令行参数合并到 Config（命令行参数优先）
func (c *Config) MergeFlags(mainExePath string) {
	if mainExePath != "" {
		c.MainExeRelativePath = mainExePath
	}
}

// MainExeFolderPath 返回主程序根目录的绝对路径 (PackageFolder/ApplicationFolder/)
// main_exe_relative_path 相对于 ExeDir（即 UpdateFolder/），如 "../ApplicationFolder/main_application.exe"
func (c *Config) MainExeFolderPath() (string, error) {
	exeDir, err := ExeDir()
	if err != nil {
		return "", err
	}
	mainExeAbsPath := filepath.Join(exeDir, c.MainExeRelativePath)
	return filepath.Dir(mainExeAbsPath), nil
}

// MainExeFolderName 返回主程序文件夹名称（如 "ApplicationFolder"）
func (c *Config) MainExeFolderName() (string, error) {
	folderPath, err := c.MainExeFolderPath()
	if err != nil {
		return "", err
	}
	return filepath.Base(folderPath), nil
}

// AppVersionDir 返回指定版本的应用目录路径 (PackageFolder/ApplicationFolder_{version}/)
func (c *Config) AppVersionDir(version string) (string, error) {
	pkgDir, err := PackageDir()
	if err != nil {
		return "", err
	}
	mainExeFolderName, err := c.MainExeFolderName()
	if err != nil {
		return "", err
	}
	return filepath.Join(pkgDir, mainExeFolderName+"_"+version), nil
}

// CheckUpdaterPath 返回 ApplicationFolder/zap-update.exe 的绝对路径
func (c *Config) CheckUpdaterPath() (string, error) {
	mainFolder, err := c.MainExeFolderPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(mainFolder, "zap-update.exe"), nil
}

// ShouldSkipFolder 判断文件夹是否在忽略列表中（基于 SharedConfig.IgnoreFolders）
// relPath 为相对于源目录的路径（OS 原生分隔符）
func ShouldSkipFolder(relPath string, ignoreFolders []string) bool {
	relPath = filepath.ToSlash(relPath)
	for _, pattern := range ignoreFolders {
		if match, _ := filepath.Match(pattern, relPath); match {
			return true
		}
		if strings.EqualFold(pattern, relPath) {
			return true
		}
	}
	return false
}

// ShouldSkipFile 判断文件是否在忽略列表中（基于 SharedConfig.IgnoreFiles）
// relPath 为相对于源目录的路径
func ShouldSkipFile(relPath string, ignoreFiles []string) bool {
	base := filepath.Base(relPath)
	for _, pattern := range ignoreFiles {
		if match, _ := filepath.Match(pattern, base); match {
			return true
		}
		if strings.EqualFold(pattern, base) {
			return true
		}
		if match, _ := filepath.Match(pattern, relPath); match {
			return true
		}
	}
	return false
}

func splitProcessNames(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, name := range strings.Split(s, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			result = append(result, name)
		}
	}
	return result
}

func ioutilReadFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ioutil.ReadAll(f)
}
