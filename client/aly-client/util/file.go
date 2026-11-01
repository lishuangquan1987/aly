package util

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// FileMD5 calculates the MD5 hash of a file.
func FileMD5(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// FileSHA256 calculates the SHA-256 hash of a file.
func FileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// RemoveFilesNotInManifest 删除 dir 中不在 manifest（相对路径集合，正斜杠分隔）内的文件，
// 并清理因此变空的子目录（#4 删除同步：发布端删除的文件随更新同步删除）。
//
// ⚠️ 历史实现，已被 RemoveFilesByRelPaths 取代（差量删除）：
// 本函数的删除面是"版本目录内所有不在清单的文件"——materialize 会把 MainFolder
// 的运行时生成文件（应用自写配置/数据，从不在服务端清单里）拷进版本目录，
// 用本函数会把它们一并删掉，apply 后应用运行时数据丢失。保留仅供测试与历史参照。
//
// 安全边界：只在版本目录（下载产物目录）上调用，绝不用于 MainFolder。
// keep 集合为 nil 时不做任何删除。返回被删除的相对路径清单（供日志留痕），
// 遍历失败返回 error。
func RemoveFilesNotInManifest(dir string, manifest map[string]bool) ([]string, error) {
	if manifest == nil {
		return nil, nil
	}

	var removed []string
	// 收集空目录候选（删除文件后自底向上清理）
	var emptyCandidates []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path != dir {
				emptyCandidates = append(emptyCandidates, path)
			}
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		if manifest[relSlash] {
			return nil
		}
		if rmErr := os.Remove(path); rmErr != nil {
			// 文件被占用等：跳过该文件继续（删除同步是尽力而为，不阻断更新）
			return nil
		}
		removed = append(removed, relSlash)
		return nil
	})
	if err != nil {
		return removed, err
	}

	// 自底向上清理空目录（Walk 是先序遍历，候选顺序是父在前，倒序即子在前）
	for i := len(emptyCandidates) - 1; i >= 0; i-- {
		os.Remove(emptyCandidates[i]) // 非空时失败，忽略
	}
	return removed, nil
}

// cachedManifestFile 本地缓存的上次发布清单文件名（#4 删除同步·差量版）。
// 内容为上一次下载成功时固化的服务端清单；下一轮更新时
// diff = 旧清单 ∖ 新清单 = 服务端确认删除的文件，作为唯一删除依据。
const cachedManifestFile = "release_manifest.json"

// ReadCachedManifest 读取本地缓存的上次发布清单（UpdateFolder/release_manifest.json）。
// dir 为 UpdateFolder 目录。文件不存在（首次部署/被清理）返回 (nil, nil)：
// 调用方据此保守跳过差量删除。文件存在但损坏：返回 error——继续用损坏数据做
// 差量可能把"误读出的缺席"当成"服务端删除"从而误删文件，宁可中断删除同步
// （仅退化为不删，不阻断更新）。
func ReadCachedManifest(dir string) (map[string]bool, error) {
	data, err := ioutil.ReadFile(filepath.Join(dir, cachedManifestFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var files []string
	if err := json.Unmarshal(data, &files); err != nil {
		return nil, fmt.Errorf("解析缓存清单失败: %v", err)
	}
	if files == nil {
		return nil, nil
	}
	set := make(map[string]bool, len(files))
	for _, rel := range files {
		if rel != "" {
			set[rel] = true
		}
	}
	return set, nil
}

// WriteCachedManifest 原子写入本地缓存清单（UpdateFolder/release_manifest.json）。
// dir 为 UpdateFolder 目录。写失败返回 error 由调用方记日志，
// 不阻断更新主流程（下一轮退化为"无差量依据不删"）。
func WriteCachedManifest(dir string, manifest []string) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化缓存清单失败: %v", err)
	}
	// 原子写（与 config.WriteVersion 同范式）：tmp → rename，
	// 中断不留半截文件（半截 JSON 会被 ReadCachedManifest 判损坏进而禁用删除同步）。
	path := filepath.Join(dir, cachedManifestFile)
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("写入缓存清单失败: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("写入缓存清单失败: %v", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("关闭缓存清单失败: %v", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("原子替换缓存清单失败: %v", err)
	}
	return nil
}

// ClearCachedManifest 删除本地缓存清单（rollback 成功后调用：
// 回滚后 MainFolder 内容对应旧版本，"上次清单"不再可信，清除后
// 下一轮下载无差量依据即保守不删，等新清单重新固化）。
// dir 为 UpdateFolder 目录。
func ClearCachedManifest(dir string) {
	if err := os.Remove(filepath.Join(dir, cachedManifestFile)); err != nil && !os.IsNotExist(err) {
		AppendToLog(dir, "download.log", fmt.Sprintf("clear cached manifest failed: %v", err))
	}
}

// RemoveFilesByRelPaths 删除 dir 下相对路径（正斜杠分隔）在 relPaths 集合内的
// 文件，并清理因此变空的子目录（#4 删除同步·差量版）。
//
// 与 RemoveFilesNotInManifest（全量差集）的区别：删除面由调用方精确给出
// （上一版清单 ∖ 当前清单 = 服务端确认删除的文件），版本目录中的其他内容
// ——包括 materialize 从 MainFolder 拷入的运行时生成文件（应用自写配置/数据，
// 从不在任何服务端清单里）——一律不动，杜绝"删同步误删运行时数据"。
// relPaths 为 nil 时不做任何删除（无差量依据时保守跳过）。
// 返回实际删除的相对路径清单（供日志留痕），遍历失败返回 error。
func RemoveFilesByRelPaths(dir string, relPaths map[string]bool) ([]string, error) {
	if relPaths == nil || len(relPaths) == 0 {
		return nil, nil
	}

	var removed []string
	var emptyCandidates []string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		relSlash := filepath.ToSlash(rel)
		if info.IsDir() {
			if path != dir {
				emptyCandidates = append(emptyCandidates, path)
			}
			return nil
		}
		if !relPaths[relSlash] {
			return nil
		}
		if rmErr := os.Remove(path); rmErr != nil {
			// 文件被占用等：跳过该文件继续（删除同步是尽力而为，不阻断更新）
			return nil
		}
		removed = append(removed, relSlash)
		return nil
	})
	if err != nil {
		return removed, err
	}

	// 自底向上清理空目录（先序遍历候选是父在前，倒序即子在前）
	for i := len(emptyCandidates) - 1; i >= 0; i-- {
		os.Remove(emptyCandidates[i]) // 非空时失败，忽略
	}
	return removed, nil
}

// CopyFile copies a single file. overwrite controls whether to overwrite the target file.
func CopyFile(src, dst string, overwrite bool) error {
	if !overwrite {
		if _, err := os.Stat(dst); err == nil {
			return nil
		}
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败 %s: %v", src, err)
	}
	defer srcFile.Close()

	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败 %s: %v", dstDir, err)
	}

	// 原子写：先写临时文件再 rename，失败/中断不会留下半截目标文件
	dstTmp := dst + ".tmp"
	dstFile, err := os.Create(dstTmp)
	if err != nil {
		return fmt.Errorf("创建目标文件失败 %s: %v", dst, err)
	}

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		os.Remove(dstTmp)
		return fmt.Errorf("复制文件内容失败: %v", err)
	}

	srcInfo, statErr := srcFile.Stat()
	if statErr != nil {
		dstFile.Close()
		os.Remove(dstTmp)
		return fmt.Errorf("获取源文件信息失败 %s: %v", src, statErr)
	}
	// Chmod is not supported on Windows; ignore on unsupported platforms
	if chmodErr := dstFile.Chmod(srcInfo.Mode()); chmodErr != nil && runtime.GOOS != "windows" {
		dstFile.Close()
		os.Remove(dstTmp)
		return fmt.Errorf("chmod failed %s: %v", dst, chmodErr)
	}
	closeErr := dstFile.Close()
	if closeErr != nil {
		os.Remove(dstTmp)
		return fmt.Errorf("关闭目标文件失败 %s: %v", dst, closeErr)
	}
	if err := os.Rename(dstTmp, dst); err != nil {
		os.Remove(dstTmp)
		return fmt.Errorf("原子替换目标文件失败 %s: %v", dst, err)
	}

	return nil
}

// LocalFileMD5Map returns a map of relativePath -> md5 for all files under root.
// The "update" directory is excluded. Keys use forward slashes for cross-platform consistency.
// MD5 computation is parallelized across multiple goroutines for better performance.
func LocalFileMD5Map(root string) (map[string]string, error) {
	// Step 1: collect all file paths (serial walk, fast)
	type fileEntry struct {
		absPath string
		relPath string
	}
	var files []fileEntry

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		// Normalize to forward slashes for consistent map keys
		relPath = filepath.ToSlash(relPath)

		if info.IsDir() {
			// 注：UpdateFolder 位于 PackageFolder 下（MainFolder 的兄弟目录），
			// 不在本 Walk(root=MainFolder) 范围内，无需在此排除；
			// .updator/ 需参与扫描（shared.json 随版本下发），不可排除（#12）。
			return nil
		}

		files = append(files, fileEntry{absPath: path, relPath: relPath})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Step 2: compute MD5 in parallel using a bounded worker pool
	result := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2
	}
	workCh := make(chan fileEntry, len(files))
	errCh := make(chan error, len(files))

	// Start workers (use continue instead of return to drain work channel)
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fe := range workCh {
				md5Val, md5Err := FileMD5(fe.absPath)
				if md5Err != nil {
					errCh <- fmt.Errorf("计算文件 MD5 失败 %s: %v", fe.absPath, md5Err)
					continue
				}
				mu.Lock()
				result[fe.relPath] = md5Val
				mu.Unlock()
			}
		}()
	}

	// Send work
	for _, fe := range files {
		workCh <- fe
	}
	close(workCh)
	wg.Wait()
	close(errCh)

	// Check for errors (collect all, not just the last one)
	var md5Err error
	for e := range errCh {
		if md5Err == nil {
			md5Err = fmt.Errorf("MD5计算失败: %v", e)
		} else {
			md5Err = fmt.Errorf("%v; %v", md5Err, e)
		}
	}

	return result, md5Err
}

// EnsureDir ensures a directory exists.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// FileSize returns the size of a file.
func FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// CopyDirWithExclude copies all files from srcDir to dstDir,
// skipping files/folders for which shouldSkipFile or shouldSkipFolder returns true.
// The skip predicates receive paths relative to srcDir (OS-native separators).
func CopyDirWithExclude(srcDir, dstDir string, shouldSkipFile func(relPath string) bool, shouldSkipFolder func(relPath string) bool) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			if relPath != "." && shouldSkipFolder != nil && shouldSkipFolder(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		if shouldSkipFile != nil && shouldSkipFile(relPath) {
			return nil
		}

		dstPath := filepath.Join(dstDir, relPath)
		return CopyFile(path, dstPath, false)
	})
}

// AppendToLog appends a line to a log file (creates it if needed).
func AppendToLog(logDir, filename, line string) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return
	}
	logPath := filepath.Join(logDir, filename)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(line + "\n")
}
