package staging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"publish-cli/internal/diff"
	"publish-cli/pkg/models"
)

// StagedFile 暂存文件条目
type StagedFile struct {
	RelativePath string `json:"relativePath"`
	Status       string `json:"status"`
	LocalMd5     string `json:"localMd5"`
	LocalSize    int64  `json:"localSize"`
}

// Load 读取暂存文件列表
func Load(projectPath string) ([]StagedFile, error) {
	path := stagingPath(projectPath)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []StagedFile
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, fmt.Errorf("parse staged-files.json: %w", err)
	}
	return files, nil
}

// Save 保存暂存文件列表
func Save(projectPath string, files []StagedFile) error {
	path := stagingPath(projectPath)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Add 将指定文件加入暂存区
func Add(projectPath string, relativePaths []string) error {
	current, _ := Load(projectPath)
	existingMap := make(map[string]bool)
	for _, f := range current {
		existingMap[f.RelativePath] = true
	}
	for _, rp := range relativePaths {
		if existingMap[rp] {
			continue
		}
		absPath := filepath.Join(projectPath, filepath.FromSlash(rp))
		md5Str, _, err := diff.HashFile(absPath)
		if err != nil {
			return fmt.Errorf("hash %s: %w", rp, err)
		}
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", rp, err)
		}
		current = append(current, StagedFile{
			RelativePath: rp,
			Status:       "modified",
			LocalMd5:     md5Str,
			LocalSize:    info.Size(),
		})
	}
	return Save(projectPath, current)
}

// Remove 从暂存区移除文件
func Remove(projectPath string, relativePaths []string) error {
	current, err := Load(projectPath)
	if err != nil {
		return err
	}
	removeSet := make(map[string]bool)
	for _, rp := range relativePaths {
		removeSet[rp] = true
	}
	var remaining []StagedFile
	for _, f := range current {
		if !removeSet[f.RelativePath] {
			remaining = append(remaining, f)
		}
	}
	return Save(projectPath, remaining)
}

// Clear 清空暂存区
func Clear(projectPath string) error {
	return Save(projectPath, nil)
}

// Verify 校验暂存文件：重新计算 MD5，若变化则返回冲突列表
func Verify(projectPath string) (conflicts []string, err error) {
	current, err := Load(projectPath)
	if err != nil || current == nil {
		return nil, err
	}
	for _, f := range current {
		absPath := filepath.Join(projectPath, filepath.FromSlash(f.RelativePath))
		md5Str, _, err := diff.HashFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", f.RelativePath, err)
		}
		if md5Str != f.LocalMd5 {
			conflicts = append(conflicts, f.RelativePath)
		}
	}
	return
}

// LoadAsStatusItems 加载暂存区并转为 FileStatusItem 列表（供 status 命令使用）
func LoadAsStatusItems(projectPath string) []models.FileStatusItem {
	files, _ := Load(projectPath)
	var items []models.FileStatusItem
	for _, f := range files {
		items = append(items, models.FileStatusItem{
			RelativePath: f.RelativePath,
			Status:       f.Status,
			LocalMd5:     f.LocalMd5,
			LocalSize:    f.LocalSize,
		})
	}
	return items
}

func stagingPath(projectPath string) string {
	return filepath.Join(projectPath, ".publish-cli", "staging", "staged-files.json")
}
