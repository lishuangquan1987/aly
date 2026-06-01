package diff

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"publish-cli/internal/config"
	"publish-cli/pkg/models"
)

// ScanDirectory 递归扫描目录，返回文件列表（应用忽略规则）
func ScanDirectory(root string, ignoreFolders, ignoreFiles []string) ([]models.LocalFile, error) {
	var files []models.LocalFile
	err := filepath.Walk(root, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// 跳过 .publish-cli 元数据目录
			if info.Name() == ".publish-cli" {
				return filepath.SkipDir
			}
			// 应用忽略文件夹规则
			relPath, _ := filepath.Rel(root, absPath)
			relPath = filepath.ToSlash(relPath)
			for _, ignoreFolder := range ignoreFolders {
				if isSubPath(relPath, ignoreFolder) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		// 获取相对路径
		relPath, _ := filepath.Rel(root, absPath)
		relPath = filepath.ToSlash(relPath)

		// 应用忽略文件规则
		for _, ignoreFile := range ignoreFiles {
			if matchFile(relPath, ignoreFile) {
				return nil
			}
		}

		// 计算 MD5 和 SHA256
		md5Str, sha256Str, err := hashFile(absPath)
		if err != nil {
			return fmt.Errorf("hash %s: %w", absPath, err)
		}

		files = append(files, models.LocalFile{
			AbsolutePath: absPath,
			RelativePath: relPath,
			Size:         info.Size(),
			ModTime:      info.ModTime().Format("2006-01-02 15:04:05"),
			MD5:          md5Str,
			SHA256:       sha256Str,
		})
		return nil
	})
	return files, err
}

// Diff 比对本地与服务端文件列表
func Diff(localFiles []models.LocalFile, serverFiles []models.FileInfo) []models.FileDiff {
	localMap := make(map[string]models.LocalFile)
	for _, f := range localFiles {
		localMap[f.RelativePath] = f
	}
	serverMap := make(map[string]models.FileInfo)
	for _, f := range serverFiles {
		serverMap[f.FileRelativePath] = f
	}

	var diffs []models.FileDiff
	allPaths := make(map[string]bool)
	for p := range localMap {
		allPaths[p] = true
	}
	for p := range serverMap {
		allPaths[p] = true
	}

	for path := range allPaths {
		local, hasLocal := localMap[path]
		server, hasServer := serverMap[path]

		d := models.FileDiff{RelativePath: path}
		if hasLocal {
			d.LocalMd5 = local.MD5
			d.LocalSize = local.Size
		}
		if hasServer {
			d.ServerMd5 = server.MD5
			d.ServerSize = server.FileSize
		}

		switch {
		case hasLocal && !hasServer:
			d.Status = "new"
		case !hasLocal && hasServer:
			d.Status = "deleted"
		case hasLocal && hasServer && local.MD5 == server.MD5:
			d.Status = "unchanged"
		case hasLocal && hasServer && local.MD5 != server.MD5:
			d.Status = "modified"
		}
		diffs = append(diffs, d)
	}
	return diffs
}

// HashFile 计算单个文件的 MD5 和 SHA256
func HashFile(path string) (md5Str, sha256Str string, err error) {
	return hashFile(path)
}

// RunStatus 执行完整 status 流程：扫描 → 查询 → 比对
func RunStatus(cfg config.Config, apiClient interface {
	GetAllFiles(projectID int) ([]models.FileInfo, error)
}, projectID int) (*models.StatusData, error) {
	localFiles, err := ScanDirectory(cfg.Project.Path, cfg.Ignore.Folders, cfg.Ignore.Files)
	if err != nil {
		return nil, fmt.Errorf("scan local: %w", err)
	}
	serverFiles, err := apiClient.GetAllFiles(projectID)
	if err != nil {
		return nil, fmt.Errorf("get server files: %w", err)
	}
	diffs := Diff(localFiles, serverFiles)

	sd := &models.StatusData{}
	for _, d := range diffs {
		item := models.FileStatusItem{
			RelativePath: d.RelativePath,
			Status:       d.Status,
			LocalMd5:     d.LocalMd5,
			LocalSize:    d.LocalSize,
			ServerMd5:    d.ServerMd5,
			ServerSize:   d.ServerSize,
		}
		switch d.Status {
		case "new", "modified", "deleted":
			sd.Unstaged = append(sd.Unstaged, item)
		case "unchanged":
			sd.Unchanged = append(sd.Unchanged, item)
		}
	}
	return sd, nil
}

// ─── 内部工具函数 ──────────────────────────────────────────────────────

func hashFile(path string) (string, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	md5h := md5.New()
	sha256h := sha256.New()
	w := io.MultiWriter(md5h, sha256h)
	if _, err := io.Copy(w, f); err != nil {
		return "", "", err
	}
	return fmt.Sprintf("%x", md5h.Sum(nil)), fmt.Sprintf("%x", sha256h.Sum(nil)), nil
}

// isSubPath 判断 child 是否是 parent 的子路径（路径前缀匹配）
func isSubPath(child, parent string) bool {
	child = strings.TrimPrefix(child, "./")
	parent = strings.TrimPrefix(parent, "./")
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+"/")
}

// matchFile 判断文件路径是否匹配忽略规则
func matchFile(relPath, pattern string) bool {
	// 精确匹配
	if relPath == pattern {
		return true
	}
	// 简单通配：如果 pattern 以 * 开头，匹配后缀
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(relPath, suffix)
	}
	// 如果 pattern 包含目录分隔符，精确路径匹配
	return relPath == strings.TrimPrefix(pattern, "./")
}
