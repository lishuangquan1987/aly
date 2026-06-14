package diff

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"zap/publish-cli/pkg/models"
)

// fileEntry 遍历阶段收集的文件信息（不含哈希）
type fileEntry struct {
	absPath string
	relPath string
	size    int64
	modTime string
}

// ScanDirectory 递归扫描目录，返回文件列表（应用忽略规则）
// 哈希计算使用 worker goroutine 并行执行以加速大规模目录扫描。
func ScanDirectory(root string, ignoreFolders, ignoreFiles []string) ([]models.LocalFile, error) {
	// 阶段 1：遍历目录收集文件条目（不计算哈希）
	var entries []fileEntry
	err := filepath.Walk(root, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".publish-cli" {
				return filepath.SkipDir
			}
			// .updator/ 目录不整体跳过，因为 shared.json 是 client 端配置需要上传
			// 但跳过 staging/（本地暂存元数据）和 publish.json（CLI 专有）
			if info.Name() == "staging" {
				relPath, _ := filepath.Rel(root, absPath)
				if strings.HasPrefix(filepath.ToSlash(relPath), ".updator/") {
					return filepath.SkipDir
				}
			}
			relPath, _ := filepath.Rel(root, absPath)
			relPath = filepath.ToSlash(relPath)
			for _, ignoreFolder := range ignoreFolders {
				if isSubPath(relPath, ignoreFolder) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		relPath, _ := filepath.Rel(root, absPath)
		relPath = filepath.ToSlash(relPath)

		// .updator/publish.json 是 CLI 专有配置，不上传到服务端
		if relPath == ".updator/publish.json" {
			return nil
		}

		for _, ignoreFile := range ignoreFiles {
			if matchFile(relPath, ignoreFile) {
				return nil
			}
		}
		entries = append(entries, fileEntry{
			absPath: absPath,
			relPath: relPath,
			size:    info.Size(),
			modTime: info.ModTime().Format("2006-01-02 15:04:05"),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 阶段 2：并行计算哈希
	numWorkers := runtime.NumCPU() * 2
	sem := make(chan struct{}, numWorkers)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type hashResult struct {
		idx    int
		md5    string
		sha256 string
		err    error
	}
	results := make(chan hashResult, len(entries))

	for i, e := range entries {
		sem <- struct{}{}
		go func(idx int, e fileEntry) {
			defer func() { <-sem }()
			select {
			case <-ctx.Done():
				results <- hashResult{idx: idx, err: ctx.Err()}
				return
			default:
			}
			md5Str, sha256Str, err := HashFile(e.absPath)
			results <- hashResult{idx: idx, md5: md5Str, sha256: sha256Str, err: err}
		}(i, e)
	}

	// 收集结果
	files := make([]models.LocalFile, len(entries))
	for i := 0; i < len(entries); i++ {
		r := <-results
		if r.err != nil {
			cancel()
			// 继续消费完剩余结果避免 goroutine 泄漏
			for j := i + 1; j < len(entries); j++ {
				<-results
			}
			return nil, fmt.Errorf("hash %s: %w", entries[r.idx].absPath, r.err)
		}
		files[r.idx] = models.LocalFile{
			AbsolutePath: entries[r.idx].absPath,
			RelativePath: entries[r.idx].relPath,
			Size:         entries[r.idx].size,
			ModTime:      entries[r.idx].modTime,
			MD5:          r.md5,
			SHA256:       r.sha256,
		}
	}
	return files, nil
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
	allPathsSet := make(map[string]bool)
	for p := range localMap {
		allPathsSet[p] = true
	}
	for p := range serverMap {
		allPathsSet[p] = true
	}

	// 收集所有路径并排序，确保每次输出顺序一致
	allPaths := make([]string, 0, len(allPathsSet))
	for p := range allPathsSet {
		allPaths = append(allPaths, p)
	}
	sort.Strings(allPaths)

	for _, path := range allPaths {
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

// RunStatus 执行完整 status 流程：扫描 → 查询 → 比对
func RunStatus(projectPath string, ignoreFolders []string, ignoreFiles []string, apiClient interface {
	GetAllFiles(projectName string) ([]models.FileInfo, error)
}, projectName string) (*models.StatusData, error) {
	localFiles, err := ScanDirectory(projectPath, ignoreFolders, ignoreFiles)
	if err != nil {
		return nil, fmt.Errorf("scan local: %w", err)
	}
	serverFiles, err := apiClient.GetAllFiles(projectName)
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
// 支持精确匹配、*.ext 后缀匹配、标准 filepath.Match glob 匹配
func matchFile(relPath, pattern string) bool {
	// 精确匹配
	if relPath == pattern {
		return true
	}
	// 尝试标准 glob 匹配（支持 *, ?, [abc]）
	if matched, _ := filepath.Match(pattern, relPath); matched {
		return true
	}
	// 用文件名尝试 glob 匹配（方便只写文件名的情况）
	base := filepath.Base(relPath)
	if matched, _ := filepath.Match(pattern, base); matched {
		return true
	}
	// 如果 pattern 以 * 开头，匹配后缀（兼容旧行为）
	if strings.HasPrefix(pattern, "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(relPath, suffix)
	}
	return false
}
