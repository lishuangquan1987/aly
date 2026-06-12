package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	VersionStatusDownloaded = "downloaded"
	VersionStatusApplying   = "applying"
	VersionStatusApplied    = "applied"
)

// VersionInfo represents the version.json structure.
// Supports both "version_previous" (new) and "version_previouse" (old, for backward compatibility).
type VersionInfo struct {
	VersionPrevious        string `json:"version_previous"`
	Version                string `json:"version"`
	VersionStatus          string `json:"version_status"`
	AfterApplyUpdateScript string `json:"after_apply_update_script,omitempty"`
}

// UnmarshalJSON supports both "version_previous" and "version_previouse" keys.
func (v *VersionInfo) UnmarshalJSON(data []byte) error {
	type alias VersionInfo // avoid recursion
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*v = VersionInfo(a)

	// Also try the legacy key
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil // non-fatal
	}
	if legacyRaw, ok := raw["version_previouse"]; ok && v.VersionPrevious == "" {
		var legacyVal string
		if err := json.Unmarshal(legacyRaw, &legacyVal); err == nil {
			v.VersionPrevious = legacyVal
		}
	}
	return nil
}

func versionPath() (string, error) {
	dir, err := ExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "version.json"), nil
}

// ReadVersion reads version.json; returns an empty struct if the file does not exist (first deploy).
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

// WriteVersion writes version.json.
func WriteVersion(info *VersionInfo) error {
	path, err := versionPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 version.json 失败: %v", err)
	}

	if err := ioutil.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入 version.json 失败: %v", err)
	}

	return nil
}
