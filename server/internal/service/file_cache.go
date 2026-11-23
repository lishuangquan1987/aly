package service

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"aly/server/internal/utils"
	"aly/server/models"

	"golang.org/x/sync/singleflight"
	"github.com/utils-go/ngo/io/directory"
)

// 按项目缓存 get_all_files 的文件列表（含每个文件的 md5/sha256），
// 避免每次请求都对项目目录全量重算哈希（#13 性能问题）。
//
// 失效策略（用户要求）：
//   - 每次 upload（upload_file / upload_chunk / upload_chunk_complete）成功后
//     调用 InvalidateProjectFileList 使该项目的缓存失效；
//   - 下次 get_all_files 时重新扫描并缓存，确保缓存始终最新。
//
// 额外安全网：get_all_files 时做一次"仅 stat"的指纹比对（相对路径 + 大小 + mtime），
// 若磁盘文件在未走 upload 接口的情况下被外部改动（如直接拷贝进 data 目录），
// 指纹不匹配会触发重建，不会返回陈旧缓存。

// fileStamp 文件指纹：相对路径 + 大小 + mtime（纳秒），无需读取文件内容。
type fileStamp struct {
	relPath   string
	size      int64
	mtimeNano int64
}

// projectFileListCache 单个项目的文件列表缓存。
type projectFileListCache struct {
	mu          sync.RWMutex
	fileInfos   []models.FileInfo
	fingerprint []fileStamp
	built       bool
}

// fileListCaches 项目名 -> 缓存
var fileListCaches = struct {
	sync.RWMutex
	m map[string]*projectFileListCache
}{m: make(map[string]*projectFileListCache)}

// rebuildGroup 合并同一项目的并发缓存重建（code-review-2026-11-23 #7）：
// 缓存失效后多个 get_all_files 同时到达时，旧实现各自全量重算哈希；
// singleflight 保证同一项目同一时刻只有一个 goroutine 在重建，其余复用结果。
var rebuildGroup singleflight.Group

// InvalidateProjectFileList 使指定项目的文件列表缓存失效。
// upload 等文件变更接口成功后调用，保证下次 get_all_files 重新缓存。
func InvalidateProjectFileList(projectName string) {
	fileListCaches.Lock()
	_, existed := fileListCaches.m[projectName]
	delete(fileListCaches.m, projectName)
	fileListCaches.Unlock()
	if existed {
		log.Printf("file list cache INVALIDATED: project=%s", projectName)
	}
}

// GetProjectFileList 返回项目文件列表（含 md5/sha256），优先返回缓存。
// 参数 ignoreFolders / ignoreFiles 为项目的忽略规则（与 get_all_files 过滤逻辑一致）。
// workDir 不存在（项目刚创建、尚未上传文件）时返回空列表。
func GetProjectFileList(projectName, workDir string, ignoreFolders, ignoreFiles []string) ([]models.FileInfo, error) {
	if !directory.Exists(workDir) {
		return []models.FileInfo{}, nil
	}

	// 一次 walk：收集指纹（相对路径/大小/mtime），应用与 get_all_files 一致的过滤规则。
	// 只做 stat，不读文件内容，比全量哈希便宜一个数量级。
	stamps, err := collectFileStamps(workDir, ignoreFolders, ignoreFiles)
	if err != nil {
		return nil, err
	}

	fileListCaches.RLock()
	entry := fileListCaches.m[projectName]
	fileListCaches.RUnlock()

	if entry != nil {
		entry.mu.RLock()
		if entry.built && sameFingerprint(entry.fingerprint, stamps) {
			infos := entry.fileInfos
			entry.mu.RUnlock()
			log.Printf("file list cache HIT: project=%s files=%d (md5/sha256 来自缓存，未重算)", projectName, len(infos))
			return infos, nil
		}
		entry.mu.RUnlock()
	}

	// 缓存缺失或指纹变化（upload 失效后 / 外部直接改动文件）：重新计算哈希并缓存。
	// 审查修复（code-review-2026-11-23 #7）：singleflight 合并同一项目的并发重建，
	// 避免多个 get_all_files 同时到达时各自全量重算（大项目下 IO/CPU 翻倍）。
	// 缓存键包含指纹摘要：指纹不同（重建期间又发生变更）时不复用旧结果。
	v, err, _ := rebuildGroup.Do(projectName+"\x00"+fingerprintKey(stamps), func() (interface{}, error) {
		infos, bErr := buildFileInfos(workDir, stamps)
		if bErr != nil {
			return nil, bErr
		}
		log.Printf("file list cache REBUILD: project=%s files=%d (全量重算 md5/sha256)", projectName, len(infos))

		fileListCaches.Lock()
		entry := fileListCaches.m[projectName]
		if entry == nil {
			entry = &projectFileListCache{}
			fileListCaches.m[projectName] = entry
		}
		fileListCaches.Unlock()

		entry.mu.Lock()
		entry.fileInfos = infos
		entry.fingerprint = stamps
		entry.built = true
		entry.mu.Unlock()
		return infos, nil
	})
	if err != nil {
		return nil, err
	}
	infos, _ := v.([]models.FileInfo)
	return infos, nil
}

// collectFileStamps 遍历项目目录，收集所有文件的指纹（相对路径/大小/mtime）。
// 过滤规则与 get_all_files 完全一致：跳过上传中间态、应用忽略文件夹/文件规则。
// 审查修复（轻微③）：单个文件/目录不可读（Windows 上被占用很常见）不再让整个
// get_all_files 失败，改为跳过 + 告警日志——与 SweepStaleUploadTemps 的容错口径统一。
// 仅根目录不可读（info==nil 且在根上）仍返回错误。
func collectFileStamps(workDir string, ignoreFolders, ignoreFiles []string) ([]fileStamp, error) {
	var stamps []fileStamp
	var walkErrs int
	err := filepath.Walk(workDir, func(absPath string, info os.FileInfo, err error) error {
		if err != nil {
			walkErrs++
			log.Printf("collectFileStamps: skip unreadable %s: %v", absPath, err)
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, relErr := filepath.Rel(workDir, absPath)
		if relErr != nil {
			return relErr
		}
		relPath = strings.ReplaceAll(relPath, "\\", "/")

		// 跳过上传中间态文件（分片暂存 xxx.chunks/、合并临时 xxx.merging），
		// 避免上传进行中时被当作正式文件下发给客户端（#7）。
		if IsUploadTempPath(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// 跳过发布清单自身及其中间态（#4 + 审查修复轻微⑩）：
		// .updator_manifest.json 是更新器协议文件，不得下发；其 .tmp 中间态
		//（writeReleaseManifest 崩溃残留）同样不得下发，否则会被客户端
		// 当作项目文件接收后再被删除同步清掉（单轮自愈但会脏数据一轮）。
		if relPath == ManifestFileName || strings.HasPrefix(relPath, ManifestFileName+".") {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// 应用忽略文件夹规则
		for _, ignoreFolder := range ignoreFolders {
			if strings.HasPrefix(relPath, ignoreFolder+"/") || relPath == ignoreFolder {
				return nil
			}
		}
		// 应用忽略文件规则（支持 glob 匹配）
		for _, ignoreFile := range ignoreFiles {
			if MatchIgnoreFile(relPath, ignoreFile) {
				return nil
			}
		}

		stamps = append(stamps, fileStamp{
			relPath:   relPath,
			size:      info.Size(),
			mtimeNano: info.ModTime().UnixNano(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if walkErrs > 0 {
		log.Printf("collectFileStamps: %d entries skipped due to read errors in %s", walkErrs, workDir)
	}

	// 按相对路径排序，确保指纹比对与输出顺序稳定
	sort.Slice(stamps, func(i, j int) bool {
		return stamps[i].relPath < stamps[j].relPath
	})
	return stamps, nil
}

// buildFileInfos 计算每个文件的 md5/sha256，构建 FileInfo 列表（已按相对路径排序）。
// 审查修复（code-review-2026-11-23 #7）：单遍双哈希（MultiWriter），IO 减半。
func buildFileInfos(workDir string, stamps []fileStamp) ([]models.FileInfo, error) {
	fileInfos := make([]models.FileInfo, 0, len(stamps))
	for _, s := range stamps {
		absPath := filepath.Join(workDir, s.relPath)
		md5Str, sha256Str, err := utils.GetFileMD5AndSHA256(absPath)
		if err != nil {
			return nil, err
		}
		fileInfos = append(fileInfos, models.FileInfo{
			FileAbsolutePath: absPath,
			FileRelativePath: s.relPath,
			LastUpdateTime:   time.Unix(0, s.mtimeNano),
			FileSize:         s.size,
			MD5:              md5Str,
			SHA256:           sha256Str,
		})
	}
	return fileInfos, nil
}

// fingerprintKey 把指纹列表拼成 short key（数量+首个/末个样例 + 长度哈希近似），
// 仅用于 singleflight 去重键——相同 key 不会错误共享不同结果（结果仍由传入的
// stamps 决定，key 冲突只会导致多算一次，不会少算）。
func fingerprintKey(stamps []fileStamp) string {
	var sb strings.Builder
	sb.WriteString(strconv.Itoa(len(stamps)))
	if len(stamps) > 0 {
		sb.WriteByte('|')
		sb.WriteString(stamps[0].relPath)
		sb.WriteByte('|')
		last := stamps[len(stamps)-1]
		sb.WriteString(last.relPath)
		sb.WriteByte(':')
		sb.WriteString(strconv.FormatInt(last.size, 10))
		sb.WriteByte(':')
		sb.WriteString(strconv.FormatInt(last.mtimeNano, 10))
	}
	return sb.String()
}

// sameFingerprint 比较两份指纹是否一致（文件集合、大小、mtime 均相同）。
func sameFingerprint(a, b []fileStamp) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].relPath != b[i].relPath || a[i].size != b[i].size || a[i].mtimeNano != b[i].mtimeNano {
			return false
		}
	}
	return true
}

// IsUploadTempPath 判断路径是否含服务端上传中间态元素。
// 服务端实际创建的中间态只有两种固定形态（file_upload_controller.go）：
//   - {文件}.chunks/   分片暂存目录
//   - {文件}.merging   分片合并临时文件
//
// 不做 .tmp/.part 等宽泛后缀匹配，避免误伤项目内合法的同名文件/目录（#7 审查发现）。
func IsUploadTempPath(relPath string) bool {
	for _, seg := range strings.Split(relPath, "/") {
		if strings.HasSuffix(seg, ".chunks") || strings.HasSuffix(seg, ".merging") {
			return true
		}
	}
	return false
}

// MatchIgnoreFile 判断文件路径是否匹配忽略规则。
// 与 publish-cli scanner.go 的 matchFile 保持一致的逻辑：
// 精确匹配 → glob 匹配（全路径）→ glob 匹配（文件名）→ *.ext 后缀匹配
// 注意：pattern 为 "*" 时，suffix 为空，strings.HasSuffix 恒返回 true，
// 即忽略所有文件（类似 .gitignore 的 * 规则），这是有意设计。
func MatchIgnoreFile(relPath, pattern string) bool {
	if relPath == pattern {
		return true
	}
	if matched, err := filepath.Match(pattern, relPath); matched {
		return true
	} else if err != nil {
		log.Printf("WARN: invalid ignore pattern %q: %v", pattern, err)
	}
	base := filepath.Base(relPath)
	if matched, err := filepath.Match(pattern, base); matched {
		return true
	} else if err != nil {
		log.Printf("WARN: invalid ignore pattern %q: %v", pattern, err)
	}
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(relPath, pattern[1:])
	}
	return false
}
