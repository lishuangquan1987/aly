package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aly/server/ent"
	"aly/server/ent/project"
	"aly/server/ent/projectchangelog"
	"aly/server/internal/db"
	"aly/server/internal/pathcfg"
	"aly/server/models"

	"github.com/utils-go/ngo/datetime"
)

const (
	// DefaultVersion 创建项目时的初始版本号
	DefaultVersion = "V1.0.0"
	// DataDirName 服务端数据存储目录名
	DataDirName = "data"
)

// validProjectName 项目名称合法性：只要能创建为文件夹名即可
// 禁止 Windows/Linux 文件系统非法字符：< > : " / \ | ? *
var validProjectName = regexp.MustCompile(`^[^<>:"/\\|?*]{1,64}$`)

// reservedProjectNames Windows 系统保留设备名
var reservedProjectNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true,
	"COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true,
	"LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// ValidateProjectName 校验项目名称（审查 B1）。
//
// 关键修复：旧实现的正则 ^[^<>:"/\\|?*]{1,64}$ **放行 "." 与 ".."**——
// 而 "."/".." 经 filepath.Join 会被 Clean 掉，导致落点逃逸到 data 目录之外
// （".." → 服务端 exe 目录，"../sub" → 任意上级子目录）。
//
// 注意：Go 的 regexp 是 RE2，**不支持负向前瞻**，无法用 (?!\.{1,2}$) 表达；
// 把 "." 加进排除字符类又会误杀 "a.b" 这类合法名。因此必须用精确等值判断。
//
// 另外拒绝以点或空格结尾的名字（Windows 上会被静默去掉尾点/尾空格，
// 造成 "abc." 与 "abc" 落同一目录、客户端看到的路径与预期不一致）。
func ValidateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("项目名称不能为空")
	}
	// B1 核心：拒绝当前目录/上级目录语义
	if name == "." || name == ".." {
		return fmt.Errorf("项目名称不能为 . 或 ..")
	}
	if !validProjectName.MatchString(name) {
		return fmt.Errorf("项目名称包含非法字符或长度不在1-64范围内")
	}
	// Windows 会静默剥掉尾部的点和空格
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return fmt.Errorf("项目名称不能以点或空格结尾")
	}
	// Windows 保留名检查
	upper := strings.ToUpper(name)
	if reservedProjectNames[upper] {
		return fmt.Errorf("项目名称不能使用系统保留名: %s", name)
	}
	return nil
}

// GetProjectWorkPath 返回项目的磁盘工作目录（{dataDir}/{projectName}）。
//
// 两重保护（审查 B1）：
//  1. 入口先跑 ValidateProjectName，拒绝 "."/".."/含分隔符等非法名；
//  2. 落位断言：Clean 后必须直接位于 dataDir 之下——兜住未来任何漏校验入口。
func GetProjectWorkPath(projectName string) (string, error) {
	if err := ValidateProjectName(projectName); err != nil {
		return "", err
	}
	dataDir, err := pathcfg.DataDir()
	if err != nil {
		return "", err
	}
	workDir := filepath.Clean(filepath.Join(dataDir, projectName))
	if filepath.Dir(workDir) != dataDir ||
		!strings.HasPrefix(workDir, dataDir+string(filepath.Separator)) {
		return "", fmt.Errorf("非法的项目路径: %s", projectName)
	}
	return workDir, nil
}

func CreateProjectWithFirstLog(ctx context.Context, name string, title string, isForceUpdate bool, ignoreFolders []string, ignoreFiles []string) models.CommonResponse {
	var project *ent.Project
	err := db.WithTx(ctx, func(tx *ent.Tx) error {
		//插入项目
		var err error
		project, err = tx.Project.Create().
			SetName(name).
			SetTitle(title).
			SetVersion(DefaultVersion).
			SetForceUpdate(isForceUpdate).
			SetIgnoreFolders(ignoreFolders).
			SetIgnoreFiles(ignoreFiles).
			Save(ctx)
		if err != nil {
			return err
		}
		//插入项目变更日志
		timeStr := datetime.Now().ToStringWithFormat("yyyy-MM-dd HH:mm:ss")
		_, err = tx.ProjectChangeLog.Create().
			SetProject(project).
			SetVersion(DefaultVersion).
			SetLogs([]string{
				"第一次创建",
			}).
			SetTime(timeStr).
			Save(ctx)
		return err
	})
	if err != nil {
		// 并发安全：UNIQUE 约束冲突时返回友好消息而非原始 DB 错误
		if ent.IsConstraintError(err) {
			return models.NG(fmt.Sprintf("项目名称:%s已存在", name))
		}
		return models.NGWithError(err)
	}

	return models.OKWithData(project)
}

func UpdateProject(ctx context.Context, name string, title string, isForceUpdate bool, ignoreFolders []string, ignoreFiles []string) models.CommonResponse {
	err := db.WithTx(ctx, func(tx *ent.Tx) error {
		//更新项目（仅未软删除的项目）
		var err error
		_, err = tx.Project.Update().
			Where(project.NameEQ(name), project.IsDeletedEQ(false)).
			SetTitle(title).
			SetForceUpdate(isForceUpdate).
			SetIgnoreFolders(ignoreFolders).
			SetIgnoreFiles(ignoreFiles).
			Save(ctx)
		if err != nil {
			return err
		}
		// 审查修正：ent 的 Update().Save(ctx) 实际返回 (affected int, err error)，
		// 可以直接用 affected 判断是否命中行（同文件 SetForceUpdate/DeleteProject
		// 即如此）。这里保留再查一次的实现，是为了同时校验"未被并发软删除"，
		// 语义上比单看 affected 更严格；但原注释"无法检查 affected 行数"是错的。
		exists, checkErr := tx.Project.Query().Where(project.NameEQ(name), project.IsDeletedEQ(false)).Exist(ctx)
		if checkErr != nil {
			return checkErr
		}
		if !exists {
			return fmt.Errorf("项目不存在: %s", name)
		}
		return nil
	})
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OK()
}

// SetForceUpdate 只更新项目的 force_update 字段，避免 read-modify-write 竞态
func SetForceUpdate(ctx context.Context, projectName string, forceUpdate bool) models.CommonResponse {
	err := db.WithTx(ctx, func(tx *ent.Tx) error {
		affected, err := tx.Project.Update().
			Where(project.NameEQ(projectName), project.IsDeletedEQ(false)).
			SetForceUpdate(forceUpdate).
			Save(ctx)
		if err != nil {
			return err
		}
		if affected == 0 {
			return fmt.Errorf("项目不存在: %s", projectName)
		}
		return nil
	})
	if err != nil {
		return models.NGWithError(err)
	}
	return models.OK()
}

func GetAllProjects(ctx context.Context) models.CommonResponse {
	projects, err := db.Client.Project.Query().Where(project.IsDeletedEQ(false)).All(ctx)
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OKWithData(projects)
}

func GetProjectChangeLogs(ctx context.Context, projectName string) models.CommonResponse {
	// 条数上限（审查 server 轻微④）：该接口返回项目有史以来全部变更日志，
	// 项目长期发布后会无界膨胀，且是无鉴权端点。取最近 200 条（倒序），
	// 客户端只需要最新版本信息，历史全量价值低。
	projectLogs, err := db.Client.ProjectChangeLog.
		Query().
		Where(projectchangelog.HasProjectWith(project.NameEQ(projectName)),
			projectchangelog.IsDeletedEQ(false)).
		Order(ent.Desc(projectchangelog.FieldID)).
		Limit(maxChangeLogRows).
		All(ctx)
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OKWithData(projectLogs)
}

// maxChangeLogRows get_project_change_logs 返回的最大条数
const maxChangeLogRows = 200

// GetProjectByName 按名称查询未软删除的项目（审查 M2）。
//
// 旧实现不过滤 IsDeleted，导致软删除项目仍能通过 get_project_by_name /
// get_all_files 对外服务、客户端 check_update 也照样认为"有更新"——
// 软删除对已部署客户端形同虚设。
func GetProjectByName(ctx context.Context, projectName string) models.CommonResponse {
	p, err := db.Client.Project.Query().
		Where(project.NameEQ(projectName), project.IsDeletedEQ(false)).
		First(ctx)
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OKWithData(p)
}

func PublishVersion(ctx context.Context, projectName string, version string, logs []string, timeStr string, afterApplyUpdateScript string) models.CommonResponse {
	// 校验时间格式（必须匹配 "2006-01-02 15:04:05"）
	if timeStr != "" {
		if _, err := time.Parse("2006-01-02 15:04:05", timeStr); err != nil {
			return models.NG("时间格式错误，正确格式: yyyy-MM-dd HH:mm:ss")
		}
	}
	var changelog *ent.ProjectChangeLog
	// 项目 ignore 规则提出来供事务提交后的清单固化使用（与 get_all_files 同口径过滤）
	var manifestIgnoreFolders, manifestIgnoreFiles []string
	err := db.WithTx(ctx, func(tx *ent.Tx) error {
		// 获取项目实体（用于关联变更日志，仅未软删除的项目）
		p, err := tx.Project.Query().Where(project.NameEQ(projectName), project.IsDeletedEQ(false)).First(ctx)
		if err != nil {
			return err
		}
		manifestIgnoreFolders, manifestIgnoreFiles = p.IgnoreFolders, p.IgnoreFiles
		// 防倒退校验（精简计划条目 8b）：新版本必须严格大于该项目已发布的
		// 最新版本号，否则拒绝（防手滑倒退——重发旧版本号会让已更新的客户端
		// "卡"在旧内容上：check 的 needUpdate 只认"严格更高"）。
		// 用户决策（计划文档 §五.2）：不留 --allow-downgrade 例外口子，
		// 确需回退内容时走"发布更高版本号"的正路。
		allLogs, lErr := tx.ProjectChangeLog.Query().
			Where(projectchangelog.ProjectID(p.ID), projectchangelog.IsDeletedEQ(false)).
			All(ctx)
		if lErr != nil {
			return lErr
		}
		if len(allLogs) > 0 {
			latest := allLogs[0].Version
			for _, l := range allLogs[1:] {
				if CompareVersion(l.Version, latest) > 0 {
					latest = l.Version
				}
			}
			if CompareVersion(version, latest) <= 0 {
				return fmt.Errorf("版本 %s 不大于已发布的最新版本 %s（防倒退保护；如需回退内容，请发布更高的版本号）", version, latest)
			}
		}
		// 更新项目版本号
		_, err = tx.Project.Update().
			Where(project.NameEQ(projectName), project.IsDeletedEQ(false)).
			SetVersion(version).
			Save(ctx)
		if err != nil {
			return err
		}
		// 创建变更日志记录
		create := tx.ProjectChangeLog.Create().
			SetProject(p).
			SetVersion(version).
			SetLogs(logs).
			SetTime(timeStr)
		if afterApplyUpdateScript != "" {
			create = create.SetAfterApplyUpdateScript(afterApplyUpdateScript)
		}
		changelog, err = create.Save(ctx)
		return err
	})
	if err != nil {
		return models.NGWithError(err)
	}

	// #4 删除同步：发布成功后把当前项目文件清单固化到数据目录（.updator_manifest.json），
	// 客户端 download_update 依此清单删除版本目录中"发布端已删除"的文件。
	// 写失败不阻断发布（返回成功）：清单缺失时客户端跳过删除（与旧行为兼容，仅退化为不删）。
	if mErr := writeReleaseManifest(projectName, manifestIgnoreFolders, manifestIgnoreFiles); mErr != nil {
		// 服务端日志留痕（不引入新依赖，沿用标准 log）
		log.Printf("[manifest] publish version %s for project %s: write manifest failed: %v",
			version, projectName, mErr)
	}

	return models.OKWithData(changelog)
}

// ManifestFileName 发布清单文件名（位于项目数据目录根部）。
// 名称带 .updator_ 前缀：与上传中间态（.chunks/.merging）区分，
// 且不在项目 ignore 规则默认覆盖范围内，walk 时由 IsUploadTempPath 之外的
// manifest 专用跳过逻辑排除（见 collectFileStamps 调用方）。
const ManifestFileName = ".updator_manifest.json"

// writeReleaseManifest 把项目当前文件清单（相对路径集合，与 get_all_files 同过滤口径）
// 写入 data/{project}/.updator_manifest.json。原子写：临时文件 + rename。
// 过滤口径对齐（审查修复）：复用 collectFileStamps —— 跳过上传中间态（.chunks/.merging）、
// 应用项目 ignore 规则、排除清单自身；此前版本只排除了清单自身，注释声称的"同口径"
// 并未真正实现（方向上清单只多不少、不会造成客户端误删，但属语义债，且清单内
// 出现 .merging 等中间态路径会让客户端把"下轮消失的中间态"误判为"服务端删除"）。
func writeReleaseManifest(projectName string, ignoreFolders, ignoreFiles []string) error {
	workDir, err := GetProjectWorkPath(projectName)
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(workDir); os.IsNotExist(statErr) {
		return nil // 项目目录不存在（刚创建未上传）：空清单无意义，跳过
	}

	// 复用 get_all_files 的收集逻辑（collectFileStamps）：
	// 完整过滤口径（中间态 + ignore 规则 + 清单自排除），只需相对路径、不算哈希。
	stamps, err := collectFileStamps(workDir, ignoreFolders, ignoreFiles)
	if err != nil {
		return fmt.Errorf("collect file stamps for manifest: %w", err)
	}
	relPaths := make([]string, 0, len(stamps))
	for _, s := range stamps {
		relPaths = append(relPaths, s.relPath)
	}

	data, mErr := json.MarshalIndent(struct {
		Files []string `json:"files"`
	}{Files: relPaths}, "", "  ")
	if mErr != nil {
		return mErr
	}

	manifestPath := filepath.Join(workDir, ManifestFileName)
	tmpPath := manifestPath + ".tmp"
	if wErr := ioutil.WriteFile(tmpPath, data, 0644); wErr != nil {
		return wErr
	}
	if rErr := os.Rename(tmpPath, manifestPath); rErr != nil {
		os.Remove(tmpPath)
		return rErr
	}
	return nil
}

// CompareVersion 语义化版本比较（与 client 端 cmd/common.go 口径一致）：
// v1 > v2 返回 1，v1 < v2 返回 -1，相等返回 0。
// 支持 v 前缀、预发布（1.0.0-beta < 1.0.0）、构建元数据忽略、非数字段按 0。
func CompareVersion(v1, v2 string) int {
	v1 = strings.TrimPrefix(strings.TrimPrefix(v1, "V"), "v")
	v2 = strings.TrimPrefix(strings.TrimPrefix(v2, "V"), "v")
	p1, pre1 := splitSemverParts(v1)
	p2, pre2 := splitSemverParts(v2)

	maxLen := len(p1)
	if len(p2) > maxLen {
		maxLen = len(p2)
	}
	for i := 0; i < maxLen; i++ {
		var n1, n2 int
		if i < len(p1) {
			n1, _ = strconv.Atoi(p1[i])
		}
		if i < len(p2) {
			n2, _ = strconv.Atoi(p2[i])
		}
		if n1 > n2 {
			return 1
		}
		if n1 < n2 {
			return -1
		}
	}
	if pre1 == "" && pre2 == "" {
		return 0
	}
	if pre1 == "" {
		return 1
	}
	if pre2 == "" {
		return -1
	}
	return comparePrereleaseParts(pre1, pre2)
}

// splitSemverParts 拆 "1.2.3-beta.1+build" 为 (["1","2","3"], "beta.1")。
func splitSemverParts(v string) ([]string, string) {
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	core, pre := v, ""
	if i := strings.IndexByte(v, '-'); i >= 0 {
		core, pre = v[:i], v[i+1:]
	}
	if core == "" {
		return nil, pre
	}
	return strings.Split(core, "."), pre
}

// comparePrereleaseParts 比较预发布段：数字段按数值且 < 字母段；字典序；段数多者大。
func comparePrereleaseParts(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	maxLen := len(as)
	if len(bs) > maxLen {
		maxLen = len(bs)
	}
	for i := 0; i < maxLen; i++ {
		if i >= len(as) {
			return -1
		}
		if i >= len(bs) {
			return 1
		}
		ai, bi := as[i], bs[i]
		aNum, bNum := isNumericPart(ai), isNumericPart(bi)
		if aNum && bNum {
			an, _ := strconv.Atoi(ai)
			bn, _ := strconv.Atoi(bi)
			if an > bn {
				return 1
			}
			if an < bn {
				return -1
			}
			continue
		}
		if aNum != bNum {
			if aNum {
				return -1
			}
			return 1
		}
		if ai > bi {
			return 1
		}
		if ai < bi {
			return -1
		}
	}
	return 0
}

// isNumericPart 判断 semver 标识符是否纯数字。
func isNumericPart(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func DeleteProject(ctx context.Context, projectName string) models.CommonResponse {
	affected, err := db.Client.Project.Update().Where(project.NameEQ(projectName), project.IsDeletedEQ(false)).
		SetIsDeleted(true).
		Save(ctx)
	if err != nil {
		return models.NGWithError(err)
	}
	if affected == 0 {
		return models.NG(fmt.Sprintf("项目不存在: %s", projectName))
	}
	return models.OK()
}
