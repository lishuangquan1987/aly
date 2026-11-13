package diff

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"aly/publish-cli/pkg/models"
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
				// 审查 M19：目录规则按"任意层级 + 大小写不敏感 + 去尾斜杠"匹配
				if matchDir(relPath, ignoreFolder) {
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

	// 按相对路径排序 entries，确保并行哈希结果也是有序的
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	// 阶段 2：并行计算哈希
	numWorkers := runtime.NumCPU() * 2
	if numWorkers < 1 {
		numWorkers = 1
	}
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
			md5Str, err := HashFileMD5(e.absPath)
			// 扫描路径不再计算 SHA256（审查 publish 轻微②：零消费者，纯白耗 CPU）；
			// LocalFile.SHA256 保持空串，需要时由调用方自行调 HashFile。
			results <- hashResult{idx: idx, md5: md5Str, err: err}
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

// HashFile 计算单个文件的 MD5 和 SHA256。
//
// 注意：扫描/比对路径请用 HashFileMD5（审查 publish 轻微②）——SHA256 目前
// **没有任何消费者**（服务端自己算、上传只发原始字节），status/diff 每个文件
// 都多算一遍 SHA256 属纯白耗 CPU（SHA256 约为 MD5 的 2-3 倍耗时）。
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

// HashFileMD5 只计算 MD5（扫描/暂存校验用；SHA256 无消费者，省一次全文件哈希）
func HashFileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	md5h := md5.New()
	if _, err := io.Copy(md5h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", md5h.Sum(nil)), nil
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

// isSubPath 判断 child 是否是 parent 的子路径（路径前缀匹配）。
//
// 审查 M19：入参统一归一化——ToSlash + Clean + 去尾部 "/" + Windows 下忽略大小写。
func isSubPath(child, parent string) bool {
	child = normalizeRulePath(child)
	parent = normalizeRulePath(parent)
	if child == parent {
		return true
	}
	return strings.HasPrefix(child, parent+"/")
}

// normalizeRulePath 归一化 ignore 规则/相对路径（审查 M19）。
//
// 旧实现的三处边界问题：
//  1. 规则 "build" 对 "a/build" 不生效（只做根级前缀匹配）——语义上"目录规则"
//     应命中任意层级，这里统一用 "/"+parent+"/" 的形式在 matchDir 里按段匹配；
//  2. Windows 下大小写敏感（HasPrefix 逐字节比较）；
//  3. 规则写成 "build/" 时 HasPrefix("build/x","build//") 恒 false → 静默完全失效。
func normalizeRulePath(p string) string {
	p = filepath.ToSlash(strings.TrimSpace(p))
	p = strings.TrimPrefix(p, "./")
	p = path.Clean(p)
	if p == "." {
		return ""
	}
	p = strings.TrimSuffix(p, "/")
	return p
}

// matchDir 判断目录相对路径是否命中 ignore 规则（审查 M19）。
// 规则按"任意层级目录名/前缀"匹配，Windows 下大小写不敏感。
func matchDir(relPath, rule string) bool {
	rel := strings.ToLower(normalizeRulePath(relPath))
	rule = strings.ToLower(normalizeRulePath(rule))
	if rule == "" {
		return false
	}
	if rel == rule {
		return true
	}
	// 任意层级命中：a/build、a/b/build
	if strings.HasPrefix(rel, rule+"/") {
		return true
	}
	return strings.Contains(rel, "/"+rule+"/") || strings.HasSuffix(rel, "/"+rule)
}

// matchFile 判断文件路径是否匹配忽略规则。
//
// 审查 M19：改用 path.Match（全平台按 "/" 切段，`*` 不跨目录）。
// 旧实现用 filepath.Match 配 ToSlash 路径，在 Windows 上 `/` 是普通字符，
// `a*` 会意外匹配 "a/b/c" 整条路径；Linux 上分隔符是 `/` 则不会——同一份
// 规则两端行为不一致。
func matchFile(relPath, pattern string) bool {
	relPath = filepath.ToSlash(relPath)
	pattern = filepath.ToSlash(pattern)
	// 精确匹配
	if strings.EqualFold(relPath, pattern) {
		return true
	}
	// 标准 glob 匹配（支持 *, ?, [abc]）；path.Match 按 / 切段
	matched, matchErr := path.Match(pattern, relPath)
	if matchErr != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid ignore pattern %q: %v\n", pattern, matchErr)
		return false
	}
	if matched {
		return true
	}
	// 用文件名尝试匹配（方便只写文件名的情况）
	base := path.Base(relPath)
	if matched, err := path.Match(pattern, base); err == nil && matched {
		return true
	}
	// 如果 pattern 以 * 开头，匹配后缀（兼容旧行为）
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(strings.ToLower(relPath), strings.ToLower(pattern[1:]))
	}
	return false
}
