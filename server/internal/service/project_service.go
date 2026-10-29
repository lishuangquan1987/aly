package service

import (
	"fmt"
	"aly/server/ent"
	"aly/server/ent/project"
	"aly/server/ent/projectchangelog"
	"aly/server/internal/db"
	"aly/server/models"
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/utils-go/ngo/datetime"
	"github.com/utils-go/ngo/io/path"
)

const (
	// DefaultVersion 创建项目时的初始版本号
	DefaultVersion = "V1.0.0"
	// DataDirName 服务端数据存储目录名
	DataDirName = "data"
)

func GetProjectWorkPath(projectName string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)

	return path.Combine(dir, DataDirName, projectName), nil
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
		// Mutation 返回的是 mutation 对象，无法直接检查 affected 行数。
		// 这里再查一次确认条目存在（防御并发软删除后更新）。
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
	projectLogs, err := db.Client.ProjectChangeLog.
		Query().
		Where(projectchangelog.HasProjectWith(project.NameEQ(projectName)),
			projectchangelog.IsDeletedEQ(false)).
		Order(ent.Desc(projectchangelog.FieldID)).
		All(ctx)
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OKWithData(projectLogs)
}

func GetProjectByName(ctx context.Context, projectName string) models.CommonResponse {
	p, err := db.Client.Project.Query().Where(project.NameEQ(projectName)).First(ctx)
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
	err := db.WithTx(ctx, func(tx *ent.Tx) error {
		// 获取项目实体（用于关联变更日志，仅未软删除的项目）
		p, err := tx.Project.Query().Where(project.NameEQ(projectName), project.IsDeletedEQ(false)).First(ctx)
		if err != nil {
			return err
		}
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

	return models.OKWithData(changelog)
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
