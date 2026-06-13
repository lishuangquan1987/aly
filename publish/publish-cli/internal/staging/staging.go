package staging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"zap/publish-cli/internal/diff"
	"zap/publish-cli/pkg/models"
)

// StagedFile represents a staged file entry.
// 与 models.FileStatusItem 结构一致，用类型别名保持语义清晰
type StagedFile = models.FileStatusItem

// Load reads the staged file list.
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

// Save writes the staged file list.
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
		status := "modified"
		if statusMap != nil {
			if s, ok := statusMap[rp]; ok {
				status = s
			}
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

func stagingPath(projectPath string) string {
	return filepath.Join(projectPath, ".updator", "staging", "staged-files.json")
}
