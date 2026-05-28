package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// Config 对应 client.yaml 的配置结构
type Config struct {
	ProjectName          string   `yaml:"project_name"`
	URL                  string   `yaml:"url"`
	MainExeRelativePath  string   `yaml:"main_exe_relative_path"`
	UnCopyFiles          []string `yaml:"un_copy_files"`
	UnCopyFolders        []string `yaml:"un_copy_folders"`
	MustCloseProcessName []string `yaml:"must_close_process_name"`
	PostUpdateScript     string   `yaml:"post_update_script"`
}

// ExeDir 返回 client_updator.exe 所在目录 (UpdateFolder/)
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

func configPath() (string, error) {
	dir, err := ExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "client.yaml"), nil
}

// LoadConfig 从 client.yaml 加载配置
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
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file failed: %v", err)
	}

	return &cfg, nil
}

// MergeFlags 将命令行参数合并到配置中（命令行参数优先）
func (c *Config) MergeFlags(url string, projectName string, mainExePath string, mustCloseProcessName string) {
	if url != "" {
		c.URL = url
	}
	if projectName != "" {
		c.ProjectName = projectName
	}
	if mainExePath != "" {
		c.MainExeRelativePath = mainExePath
	}
	if mustCloseProcessName != "" {
		c.MustCloseProcessName = splitProcessNames(mustCloseProcessName)
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

// CheckUpdaterPath 返回 ApplicationFolder/check-updator.exe 的绝对路径
func (c *Config) CheckUpdaterPath() (string, error) {
	mainFolder, err := c.MainExeFolderPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(mainFolder, "check-updator.exe"), nil
}

// ShouldSkipFile 判断文件是否在排除列表中
func (c *Config) ShouldSkipFile(relPath string) bool {
	base := filepath.Base(relPath)
	for _, pattern := range c.UnCopyFiles {
		if match, _ := filepath.Match(pattern, base); match {
			return true
		}
		if strings.EqualFold(pattern, base) {
			return true
		}
	}
	return false
}

// ShouldSkipFolder 判断文件夹是否在排除列表中
func (c *Config) ShouldSkipFolder(relPath string) bool {
	for _, pattern := range c.UnCopyFolders {
		if match, _ := filepath.Match(pattern, relPath); match {
			return true
		}
		if strings.EqualFold(pattern, relPath) {
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
	stat, err := f.Stat()
	if err != nil {
		return nil, err
	}
	data := make([]byte, stat.Size())
	_, err = f.Read(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}
