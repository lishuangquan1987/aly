package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"publish-cli/pkg/models"
)

// Client HTTP API 客户端
type Client struct {
	ServerURL string
	hc        *http.Client
}

// NewClient 创建客户端
func NewClient(serverURL string) *Client {
	return &Client{
		ServerURL: serverURL,
		hc:        &http.Client{},
	}
}

// ─── 通用方法 ──────────────────────────────────────────────────────────

func (c *Client) get(path string, result interface{}) error {
	resp, err := c.hc.Get(c.ServerURL + path)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var cr models.CommonResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if !cr.IsSuccess {
		return fmt.Errorf("%s", cr.ErrorMsg)
	}
	if result != nil && cr.Data != nil {
		return json.Unmarshal(cr.Data, result)
	}
	return nil
}

func (c *Client) post(path string, reqBody interface{}, result interface{}) error {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	resp, err := c.hc.Post(c.ServerURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var cr models.CommonResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if !cr.IsSuccess {
		return fmt.Errorf("%s", cr.ErrorMsg)
	}
	if result != nil && cr.Data != nil {
		return json.Unmarshal(cr.Data, result)
	}
	return nil
}

// ─── Project API ───────────────────────────────────────────────────────

// GetAllProjects 获取所有项目
func (c *Client) GetAllProjects() ([]models.Project, error) {
	var projects []models.Project
	if err := c.get("/api/project/get_all_projects", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// CreateProject 创建项目
func (c *Client) CreateProject(req models.CreateProjectRequest) (*models.Project, error) {
	var project models.Project
	if err := c.post("/api/project/create_project", req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// UpdateProject 更新项目配置
func (c *Client) UpdateProject(req models.UpdateProjectRequest) error {
	return c.post("/api/project/update_project", req, nil)
}

// DeleteProject 软删除项目
func (c *Client) DeleteProject(projectID int) error {
	path := fmt.Sprintf("/api/project/delete_project/%d", projectID)
	return c.post(path, nil, nil)
}

// GetProjectChangeLogs 获取变更日志
func (c *Client) GetProjectChangeLogs(projectID int) ([]models.ProjectChangeLog, error) {
	var logs []models.ProjectChangeLog
	path := fmt.Sprintf("/api/project/get_project_change_logs/%d", projectID)
	if err := c.get(path, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

// GetProjectOSInfo 获取服务端系统信息
func (c *Client) GetProjectOSInfo(projectID int) ([]models.ServerOSInfo, error) {
	var info []models.ServerOSInfo
	path := fmt.Sprintf("/api/project/get_project_os_info/%d", projectID)
	if err := c.get(path, &info); err != nil {
		return nil, err
	}
	return info, nil
}

// PublishVersion 发布新版本
func (c *Client) PublishVersion(req models.PublishVersionRequest) (*models.ProjectChangeLog, error) {
	var log models.ProjectChangeLog
	if err := c.post("/api/project/publish_version", req, &log); err != nil {
		return nil, err
	}
	return &log, nil
}

// ─── File API ──────────────────────────────────────────────────────────

// GetAllFiles 获取服务端文件列表
func (c *Client) GetAllFiles(projectID int) ([]models.FileInfo, error) {
	var files []models.FileInfo
	path := fmt.Sprintf("/api/file/get_all_files/%d", projectID)
	if err := c.get(path, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// UploadFile 上传单个文件（multipart）
func (c *Client) UploadFile(localPath, projectName, relativeFileName string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// file 字段
	part, err := writer.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}

	// projectName 字段
	writer.WriteField("projectName", projectName)
	// relativeFileName 字段
	writer.WriteField("relativeFileName", relativeFileName)

	writer.Close()

	resp, err := c.hc.Post(c.ServerURL+"/api/file/upload_file", writer.FormDataContentType(), body)
	if err != nil {
		return fmt.Errorf("upload file: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var cr models.CommonResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if !cr.IsSuccess {
		return fmt.Errorf("%s", cr.ErrorMsg)
	}
	return nil
}
