package service

import (
	"zap/server/ent"
	"zap/server/ent/project"
	"zap/server/ent/projectchangelog"
	"zap/server/internal/db"
	"zap/server/models"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/utils-go/ngo/datetime"
	"github.com/utils-go/ngo/io/path"
)

func GetProjectWorkPath(projectName string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)

	return path.Combine(dir, "data", projectName), nil
}

func CreateProjectWithFirstLog(ctx context.Context, name string, title string, isForceUpdate bool, ignoreFolders []string, ignoreFiles []string) models.CommonResponse {
	var project *ent.Project
	err := db.WithTx(ctx, func(tx *ent.Tx) error {
		//插入项目
		var err error
		project, err = tx.Project.Create().
			SetName(name).
			SetTitle(title).
			SetVersion("V1.0.0").
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
			SetVersion("V1.0.0").
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
		//更新项目
		var err error
		_, err = tx.Project.Update().
			Where(project.NameEQ(name)).
			SetTitle(title).
			SetForceUpdate(isForceUpdate).
			SetIgnoreFolders(ignoreFolders).
			SetIgnoreFiles(ignoreFiles).
			Save(ctx)
		return err
	})
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OK()
}

// SetForceUpdate 只更新项目的 force_update 字段，避免 read-modify-write 竞态
func SetForceUpdate(ctx context.Context, projectName string, forceUpdate bool) models.CommonResponse {
	err := db.WithTx(ctx, func(tx *ent.Tx) error {
		_, err := tx.Project.Update().
			Where(project.NameEQ(projectName)).
			SetForceUpdate(forceUpdate).
			Save(ctx)
		return err
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
		// 获取项目实体（用于关联变更日志）
		p, err := tx.Project.Query().Where(project.NameEQ(projectName)).First(ctx)
		if err != nil {
			return err
		}
		// 更新项目版本号
		_, err = tx.Project.Update().
			Where(project.NameEQ(projectName)).
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

func DeleteProject(ctx context.Context, projectName string) models.CommonResponse {
	_, err := db.Client.Project.Update().Where(project.NameEQ(projectName)).
		SetIsDeleted(true).
		Save(ctx)
	if err != nil {
		return models.NGWithError(err)
	}
	return models.OK()
}
