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
	MustCloseProcessName []string `yaml:"must_close_process_name"`
}

// ExeDir 返回可执行文件所在目录
func ExeDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %v", err)
	}
	resolved, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		resolved = exePath
	}
	return filepath.Dir(resolved), nil
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
		return nil, fmt.Errorf("读取配置文件 %s 失败: %v", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
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

// MainExeFolderPath 返回主程序所在的文件夹路径
func (c *Config) MainExeFolderPath() (string, error) {
	exeDir, err := ExeDir()
	if err != nil {
		return "", err
	}
	mainExePath := filepath.Join(exeDir, c.MainExeRelativePath)
	return filepath.Dir(mainExePath), nil
}

// UpdateDir 返回 update 目录的路径
func (c *Config) UpdateDir() (string, error) {
	folder, err := c.MainExeFolderPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(folder, "update"), nil
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
