package service

import (
	"clientupdator/server/ent"
	"clientupdator/server/ent/project"
	"clientupdator/server/ent/projectchangelog"
	"clientupdator/server/internal/db"
	"clientupdator/server/models"
	"context"
	"os"
	"path/filepath"

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

func CreateProjectWithFirstLog(name string, title string, isForceUpdate bool, ignoreFolders []string, ignoreFiles []string) models.CommonResponse {
	ctx := context.Background()
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
		return models.NGWithError(err)
	}

	return models.OKWithData(project)
}

func UpdateProject(id int, title string, isForceUpdate bool, ignoreFolders []string, ignoreFiles []string) models.CommonResponse {
	ctx := context.Background()
	err := db.WithTx(ctx, func(tx *ent.Tx) error {
		//更新项目
		var err error
		_, err = tx.Project.Update().
			Where(project.IDEQ(id)).
			SetTitle(title).
			SetForceUpdate(isForceUpdate).
			SetIgnoreFolders(ignoreFolders).
			SetIgnoreFiles(ignoreFiles).
			Save(ctx)
		if err != nil {
			return err
		}
		return err
	})
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OK()
}

func GetAllProjects() models.CommonResponse {
	ctx := context.Background()
	projects, err := db.Client.Project.Query().Where(project.IsDeletedEQ(false)).All(ctx)
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OKWithData(projects)
}

func GetProjectChangeLogs(projectId int) models.CommonResponse {
	ctx := context.Background()
	projectLogs, err := db.Client.ProjectChangeLog.
		Query().
		Where(projectchangelog.HasProjectWith(project.IDEQ(projectId)),
			projectchangelog.IsDeletedEQ(false)).
		All(ctx)
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OKWithData(projectLogs)
}

func GetProjectById(projectId int) models.CommonResponse {
	ctx := context.Background()
	p, err := db.Client.Project.Query().Where(project.IDEQ(projectId)).First(ctx)
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OKWithData(p)
}

func PublishVersion(projectId int, version string, logs []string, time string) models.CommonResponse {
	ctx := context.Background()
	var changelog *ent.ProjectChangeLog
	err := db.WithTx(ctx, func(tx *ent.Tx) error {
		// 获取项目实体（用于关联变更日志）
		p, err := tx.Project.Get(ctx, projectId)
		if err != nil {
			return err
		}
		// 更新项目版本号
		_, err = tx.Project.Update().
			Where(project.IDEQ(projectId)).
			SetVersion(version).
			Save(ctx)
		if err != nil {
			return err
		}
		// 创建变更日志记录
		changelog, err = tx.ProjectChangeLog.Create().
			SetProject(p).
			SetVersion(version).
			SetLogs(logs).
			SetTime(time).
			Save(ctx)
		return err
	})
	if err != nil {
		return models.NGWithError(err)
	}

	return models.OKWithData(changelog)
}

func DeleteProject(projectId int) models.CommonResponse {
	ctx := context.Background()
	_, err := db.Client.Project.Update().Where(project.IDEQ(projectId)).
		SetIsDeleted(true).
		Save(ctx)
	if err != nil {
		return models.NGWithError(err)
	}
	return models.OK()
}
