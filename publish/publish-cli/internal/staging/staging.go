package staging

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"aly/publish-cli/internal/diff"
	"aly/publish-cli/pkg/models"
)

// StagedFile represents a staged file entry.
// 与 models.FileStatusItem 结构一致，用类型别名保持语义清晰
type StagedFile = models.FileStatusItem

// Load reads the staged file list (按路径排序).
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
	sort.Slice(files, func(i, j int) bool {
		return files[i].RelativePath < files[j].RelativePath
	})
	return files, nil
}

// Save writes the staged file list (按路径排序保存).
func Save(projectPath string, files []StagedFile) error {
	path := stagingPath(projectPath)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].RelativePath < files[j].RelativePath
	})
	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}
	// 原子写：先写临时文件再 rename，避免崩溃留下半截 staged-files.json
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

// Add adds files to the staging area with default status "modified".
func Add(projectPath string, relativePaths []string) error {
	return AddWithStatus(projectPath, relativePaths, nil)
}

// AddWithStatus adds files to the staging area with explicit status mapping.
// statusMap maps relativePath to status ("new" or "modified").
// If statusMap is nil or a path is not in the map, defaults to "modified".
func AddWithStatus(projectPath string, relativePaths []string, statusMap map[string]string) error {
	current, err := Load(projectPath)
	if err != nil {
		// Staging file corrupted: backup and continue
		savePath := stagingPath(projectPath)
		backupPath := savePath + ".corrupted"
		if renameErr := os.Rename(savePath, backupPath); renameErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to backup corrupted staging: %v\n", renameErr)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: staged file corrupted, backed up to %s\n", backupPath)
		}
		current = nil
	}
	existingMap := make(map[string]bool)
	for _, f := range current {
		existingMap[f.RelativePath] = true
	}
	for _, rp := range relativePaths {
		// 审查 M21：用户手工输入的路径可能带反斜杠、"./"、"a//b" 等形态，
		// 而扫描产出恒为 ToSlash 形式。不归一会导致"同一文件既 staged 又
		// unstaged"（status 里双行显示）、服务端落盘脏数据。
		rp = normalizeRelPath(rp)
		if rp == "" {
			continue
		}
		if existingMap[rp] {
			continue
		}
		status := "modified"
		if statusMap != nil {
			if s, ok := statusMap[rp]; ok {
				status = s
			}
		}
		// 审查 B6：deleted 文件本地已不存在，不能去 Hash/Stat——
		// 旧实现会让"删除同步"在 add 阶段就报错，链路永远走不通。
		if status == "deleted" {
			current = append(current, StagedFile{
				RelativePath: rp,
				Status:       status,
			})
			existingMap[rp] = true
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
			Status:       status,
			LocalMd5:     md5Str,
			LocalSize:    info.Size(),
		})
	}
	return Save(projectPath, current)
}

// Remove removes files from the staging area.
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

// Clear clears the staging area.
func Clear(projectPath string) error {
	return Save(projectPath, nil)
}

// Verify re-computes MD5 for staged files and returns conflicting paths.
// Returns (nil, nil) if staging area is empty or doesn't exist.
// Returns (conflicts, nil) if there are conflicting files.
// Returns (nil, err) on read or computation errors.
func Verify(projectPath string) (conflicts []string, err error) {
	current, err := Load(projectPath)
	if err != nil {
		return nil, fmt.Errorf("load staging: %w", err)
	}
	if current == nil {
		return nil, nil
	}
	for _, f := range current {
		// 审查 B6：deleted 文件本地已不存在，无从 Verify
		if f.Status == "deleted" {
			continue
		}
		absPath := filepath.Join(projectPath, filepath.FromSlash(f.RelativePath))
		md5Str, _, err := diff.HashFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("hash %s: %w", f.RelativePath, err)
		}
		if !strings.EqualFold(md5Str, f.LocalMd5) {
			conflicts = append(conflicts, f.RelativePath)
		}
	}
	return
}

// Sync 移除暂存区中"已确认发布成功"的文件。
//
// 审查 B7：旧实现只看"服务端 MD5 与本地一致"就移除——但上传成功、
// PublishVersion 失败时服务端同样已有该文件，暂存区被清空后版本号再也补不上，
// 与 push 的"可重试"承诺直接矛盾。
//
// 现在加第二判据：manifestFiles 非 nil 时，要求文件在当前发布清单里
// （清单只在建版本成功时写入）；manifestFiles == nil 表示老服务器无该接口，
// 降级为旧行为。
func Sync(projectPath string, serverMD5s map[string]string, manifestFiles map[string]bool) (removed []string, err error) {
	current, err := Load(projectPath)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, nil
	}

	var remaining []StagedFile
	for _, f := range current {
		serverMd5, ok := serverMD5s[f.RelativePath]
		if !ok || !strings.EqualFold(f.LocalMd5, serverMd5) {
			remaining = append(remaining, f)
			continue
		}
		// 文件在服务端且内容一致；再看是否已进入发布清单
		if manifestFiles != nil && !manifestFiles[f.RelativePath] {
			// 已上传但版本未建立：保留，供用户重试 push 同版本号
			remaining = append(remaining, f)
			continue
		}
		removed = append(removed, f.RelativePath)
	}

	if len(removed) > 0 {
		if err := Save(projectPath, remaining); err != nil {
			return removed, err
		}
	}
	return removed, nil
}

// normalizeRelPath 归一化用户输入的文件相对路径（审查 M21）：
// 反斜杠转正斜杠、Clean 掉 "./" 与重复分隔符、去掉前导 "./"。
// 注意不做大小写归一（会改变实际上传路径、影响服务端目录形态）。
func normalizeRelPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	p = filepath.ToSlash(p)
	p = strings.TrimPrefix(p, "./")
	p = path.Clean(p)
	if p == "." || p == ".." || strings.HasPrefix(p, "../") {
		return ""
	}
	return p
}

func stagingPath(projectPath string) string {
	return filepath.Join(projectPath, ".updator", "staging", "staged-files.json")
}
