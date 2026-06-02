package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// Config 瀵瑰簲 client.yaml 鐨勯厤缃粨鏋?type Config struct {
	ProjectName          string   `yaml:"project_name"`
	URL                  string   `yaml:"url"`
	MainExeRelativePath  string   `yaml:"main_exe_relative_path"`
	UnCopyFiles          []string `yaml:"un_copy_files"`
	UnCopyFolders        []string `yaml:"un_copy_folders"`
	MustCloseProcessName []string `yaml:"must_close_process_name"`
	PostUpdateScript     string   `yaml:"post_update_script"`
}

// ExeDir 杩斿洖 client_updator.exe 鎵€鍦ㄧ洰褰?(UpdateFolder/)
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

// PackageDir 杩斿洖鍖呮牴鐩綍 (PackageFolder/)锛屽嵆 ExeDir 鐨勭埗鐩綍
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

// LoadConfig 浠?client.yaml 鍔犺浇閰嶇疆
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

// MergeFlags 灏嗗懡浠よ鍙傛暟鍚堝苟鍒伴厤缃腑锛堝懡浠よ鍙傛暟浼樺厛锛?func (c *Config) MergeFlags(url string, projectName string, mainExePath string, mustCloseProcessName string) {
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
		newNames := splitProcessNames(mustCloseProcessName)
		existing := make(map[string]bool)
		for _, n := range c.MustCloseProcessName {
			existing[n] = true
		}
		for _, n := range newNames {
			if !existing[n] {
				c.MustCloseProcessName = append(c.MustCloseProcessName, n)
			}
		}
	}
}

// MainExeFolderPath 杩斿洖涓荤▼搴忔牴鐩綍鐨勭粷瀵硅矾寰?(PackageFolder/ApplicationFolder/)
// main_exe_relative_path 鐩稿浜?ExeDir锛堝嵆 UpdateFolder/锛夛紝濡?"../ApplicationFolder/main_application.exe"
func (c *Config) MainExeFolderPath() (string, error) {
	exeDir, err := ExeDir()
	if err != nil {
		return "", err
	}
	mainExeAbsPath := filepath.Join(exeDir, c.MainExeRelativePath)
	return filepath.Dir(mainExeAbsPath), nil
}

// MainExeFolderName 杩斿洖涓荤▼搴忔枃浠跺す鍚嶇О锛堝 "ApplicationFolder"锛?func (c *Config) MainExeFolderName() (string, error) {
	folderPath, err := c.MainExeFolderPath()
	if err != nil {
		return "", err
	}
	return filepath.Base(folderPath), nil
}

// AppVersionDir 杩斿洖鎸囧畾鐗堟湰鐨勫簲鐢ㄧ洰褰曡矾寰?(PackageFolder/ApplicationFolder_{version}/)
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

// CheckUpdaterPath 杩斿洖 ApplicationFolder/check-updator.exe 鐨勭粷瀵硅矾寰?func (c *Config) CheckUpdaterPath() (string, error) {
	mainFolder, err := c.MainExeFolderPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(mainFolder, "check-updator.exe"), nil
}

// ShouldSkipFile 鍒ゆ柇鏂囦欢鏄惁鍦ㄦ帓闄ゅ垪琛ㄤ腑
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

// ShouldSkipFolder 鍒ゆ柇鏂囦欢澶规槸鍚﹀湪鎺掗櫎鍒楄〃涓?func (c *Config) ShouldSkipFolder(relPath string) bool {
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
	return ioutil.ReadAll(f)
}

