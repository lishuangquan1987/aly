package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	VersionStatusDownloaded = "downloaded"
	VersionStatusApplying   = "applying"
	VersionStatusApplied    = "applied"
)

// VersionInfo 对应 version.json 的结构
// Note: JSON key "version_previouse" preserved for backward compatibility with older version.json files
type VersionInfo struct {
	VersionPrevious string `json:"version_previouse"`
	Version          string `json:"version"`
	VersionStatus    string `json:"version_status"`
}

func versionPath() (string, error) {
	dir, err := ExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "version.json"), nil
}

// ReadVersion 读取 version.json，文件不存在时返回空结构体（首次部署）
func ReadVersion() (*VersionInfo, error) {
	path, err := versionPath()
	if err != nil {
		return nil, err
	}

	data, err := ioutilReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &VersionInfo{}, nil
		}
		return nil, fmt.Errorf("读取 version.json 失败: %v", err)
	}

	var info VersionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("解析 version.json 失败: %v", err)
	}

	return &info, nil
}

// WriteVersion 写入 version.json（先写临时文件再 rename，保证原子性）
func WriteVersion(info *VersionInfo) error {
	path, err := versionPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 version.json 失败: %v", err)
	}

	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %v", err)
	}

	_, err = f.Write(data)
	if err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("写入临时文件失败: %v", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("关闭临时文件失败: %v", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("重命名临时文件失败: %v", err)
	}

	return nil
}
