package controllers

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"aly/server/ent"
	"aly/server/ent/project"
	"aly/server/internal/db"
	"aly/server/internal/pathcfg"
	"aly/server/internal/service"
	"aly/server/models"

	"github.com/gin-gonic/gin"
	"github.com/shirou/gopsutil/disk"
	"github.com/utils-go/ngo/io/path"
	"github.com/utils-go/ngo/stringUtils"
)

// minDiskFreeReserve 上传前要求预留的空闲空间（审查 M5）。
// 除文件本身外再留 512MB，避免把磁盘写满导致 SQLite/日志/系统异常。
const minDiskFreeReserve int64 = 512 << 20

// maxChunkTotalChunks 分片总数上限（审查 N1，与 B3 同入口）。
// publish-cli 分片 128KiB、单文件上限 1GB ⇒ 最多 8192 片，65536 留足裕量。
// 不设上限时，单个请求传 totalChunks=2e9 会让 complete 的校验循环跑 20 亿次
// os.Stat（小时级 CPU 自旋），且 missing 切片无界增长直接 OOM。
const maxChunkTotalChunks = 65536

// perFileLock 按目标文件串行化"合并/落盘"，避免两个发布者并发 complete 同一文件
// 时互相清掉对方分片、静默产出损坏文件（审查 M3）。
// 审查修复（轻微①）：旧实现 map 只增不减（慢内存泄漏），改为引用计数式：
// 最后一个使用者释放时删除表项。
type refLock struct {
	mu   sync.Mutex
	refs int
}

var perFileLock = struct {
	sync.Mutex
	m map[string]*refLock
}{m: make(map[string]*refLock)}

// releaseFileLock 释放 lockForFile 返回的锁；引用归零时从表中删除。
func releaseFileLock(p string) {
	key := strings.ToLower(filepath.Clean(p))
	perFileLock.Lock()
	defer perFileLock.Unlock()
	if l, ok := perFileLock.m[key]; ok {
		l.refs--
		if l.refs <= 0 {
			delete(perFileLock.m, key)
		}
	}
}

// acquireFileLock 返回指定落盘路径对应的互斥锁（按路径懒创建），并递增引用计数。
// 使用方必须 defer releaseFileLock(p)。
func acquireFileLock(p string) *sync.Mutex {
	key := strings.ToLower(filepath.Clean(p))
	perFileLock.Lock()
	defer perFileLock.Unlock()
	l, ok := perFileLock.m[key]
	if !ok {
		l = &refLock{}
		perFileLock.m[key] = l
	}
	l.refs++
	return &l.mu
}

// checkUploadProject 校验上传目标任务存在且未软删除（审查 M5）。
//
// 旧实现允许往"库里不存在"的项目上传（连目录都会被自动创建），
// 等于开放任意目录写入；现在要求项目先通过 create_project 建立。
func checkUploadProject(ctx *gin.Context, projectName string) bool {
	if err := service.ValidateProjectName(projectName); err != nil {
		ctx.JSON(200, models.NG(err.Error()))
		return false
	}
	_, err := db.Client.Project.Query().
		Where(project.NameEQ(projectName), project.IsDeletedEQ(false)).
		First(ctx.Request.Context())
	if err != nil {
		if ent.IsNotFound(err) {
			ctx.JSON(200, models.NG(fmt.Sprintf("项目不存在: %s（请先创建项目）", projectName)))
		} else {
			ctx.JSON(200, models.NGWithError(err))
		}
		return false
	}
	return true
}

// checkDiskSpace 上传前检查目标磁盘剩余空间（审查 M5）。
func checkDiskSpace(ctx *gin.Context, workDir string, needBytes int64) bool {
	usage, err := disk.Usage(workDir)
	if err != nil {
		// 拿不到磁盘信息时放行（不能因统计接口故障阻断正常发布），但必须留痕——
		// 审查修复（轻微②）：注释声称"仅记录"却没有任何 log 调用，故障完全无痕。
		log.Printf("checkDiskSpace: disk.Usage(%s) failed (allowing upload): %v", workDir, err)
		return true
	}
	if usage.Free < uint64(needBytes+minDiskFreeReserve) {
		ctx.JSON(200, models.NG(fmt.Sprintf(
			"磁盘空间不足: 需要 %d 字节（含预留 %d 字节），当前可用 %d 字节",
			needBytes+minDiskFreeReserve, minDiskFreeReserve, usage.Free)))
		return false
	}
	return true
}

// resolveUploadTarget 解析上传目标：工作目录 + 规范化的绝对文件路径。
//
// 统一承载三处上传入口的公共校验（审查 B1）：
//  1. 项目名校验（拒绝 "."/".." 等逃逸语义）；
//  2. 相对文件名规范化（反斜杠转正斜杠）并断言落点位于 workDir 内；
//  3. workDir 由 service.GetProjectWorkPath 解析（内含 data 目录落位断言）。
//
// 审查修复（轻微⑥/⑦）：额外拒绝尾分隔符（"sub/" 经 Clean 会落到目录路径，
// 造成后续同名目录/文件冲突与报错混乱）与路径段内的 Windows 保留设备名/尾点尾空格
//（"CON"、"a/aux" 会在 Windows 上生成异常路径）。
//
// 返回 ok=false 时已向客户端写出 NG 响应。
func resolveUploadTarget(ctx *gin.Context, projectName, relativeFileName string) (workDir, absFileName string, ok bool) {
	workDir, err := service.GetProjectWorkPath(projectName)
	if err != nil {
		ctx.JSON(200, models.NG(err.Error()))
		return "", "", false
	}

	fileName := stringUtils.Replace(relativeFileName, "\\", "/")
	// 尾分隔符：Clean 后语义丢失，先拒（避免造出与目录同名的文件 / 语义混乱）
	if strings.HasSuffix(fileName, "/") {
		ctx.JSON(200, models.NG("非法的文件路径: 不能以 / 结尾"))
		return "", "", false
	}
	// 每段校验：保留设备名（大小写不敏感）、尾点/尾空格（Windows 静默剥离）
	for _, seg := range strings.Split(fileName, "/") {
		if err := service.ValidateFileNameSegment(seg); err != nil {
			ctx.JSON(200, models.NG(fmt.Sprintf("非法的文件路径: %v", err)))
			return "", "", false
		}
	}
	absFileName = filepath.Clean(path.Combine(workDir, fileName))
	// 落点必须严格位于 workDir 之下（workDir 本身是目录，不允许作为文件落点）
	if !strings.HasPrefix(absFileName, workDir+string(filepath.Separator)) {
		ctx.JSON(200, models.NG("非法的文件路径"))
		return "", "", false
	}
	return workDir, absFileName, true
}

// SweepStaleUploadTemps 清理超龄的上传中间态（审查 M3）。
//
// 进程在合并/上传中途崩溃会留下 *.chunks 目录与 *.merging 文件，旧实现
// 只在成功路径 RemoveAll，孤儿会永久残留。启动时按 mtime 清扫一次。
// 返回清理的条目数与错误（错误不致命，由调用方记录日志）。
func SweepStaleUploadTemps(maxAge time.Duration) (int, error) {
	dataDir, err := pathcfg.DataDir()
	if err != nil {
		return 0, err
	}
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		return 0, nil
	}
	cutoff := time.Now().Add(-maxAge)
	cleaned := 0
	err = filepath.Walk(dataDir, func(p string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil // 单个条目不可读不影响整体清扫
		}
		name := info.Name()
		isChunksDir := info.IsDir() && strings.HasSuffix(name, ".chunks")
		isMergingFile := !info.IsDir() && strings.HasSuffix(name, ".merging")
		// 审查修复（轻微⑩）：writeReleaseManifest 的 .updator_manifest.json.tmp
		// 崩溃残留同样清扫（它不在任何过滤白名单内，残留会被 get_all_files 下发一轮）。
		isManifestTmp := !info.IsDir() && strings.HasPrefix(name, service.ManifestFileName+".")
		if !isChunksDir && !isMergingFile && !isManifestTmp {
			return nil
		}
		if info.ModTime().After(cutoff) {
			return nil
		}
		if err := os.RemoveAll(p); err != nil {
			return nil
		}
		cleaned++
		if isChunksDir {
			return filepath.SkipDir
		}
		return nil
	})
	return cleaned, err
}
