package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"zap/publish-cli/pkg/models"
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
		hc:        &http.Client{Timeout: 500 * time.Second},
	}
}

// ─── 通用方法 ──────────────────────────────────────────────────────────

func (c *Client) get(path string, result interface{}) error {
	resp, err := c.hc.Get(c.ServerURL + path)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("GET %s returned HTTP %d (failed to read body: %v)", path, resp.StatusCode, readErr)
		}
		return fmt.Errorf("GET %s returned HTTP %d: %s", path, resp.StatusCode, string(body))
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("GET %s read body: %w", path, readErr)
	}
	var cr models.CommonResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return fmt.Errorf("GET %s parse response: %w", path, err)
	}
	if !cr.IsSuccess {
		return fmt.Errorf("GET %s: %s", path, cr.ErrorMsg)
	}
	if result != nil && cr.Data != nil {
		return json.Unmarshal(cr.Data, result)
	}
	return nil
}

func (c *Client) post(path string, reqBody interface{}, result interface{}) error {
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("POST %s marshal request: %w", path, err)
	}
	resp, err := c.hc.Post(c.ServerURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("POST %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("POST %s returned HTTP %d (failed to read body: %v)", path, resp.StatusCode, readErr)
		}
		return fmt.Errorf("POST %s returned HTTP %d: %s", path, resp.StatusCode, string(body))
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("POST %s read body: %w", path, readErr)
	}
	var cr models.CommonResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return fmt.Errorf("POST %s parse response: %w", path, err)
	}
	if !cr.IsSuccess {
		return fmt.Errorf("POST %s: %s", path, cr.ErrorMsg)
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
func (c *Client) CreateProject(req models.ProjectConfigRequest) (*models.Project, error) {
	var project models.Project
	if err := c.post("/api/project/create_project", req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// UpdateProject 更新项目配置
func (c *Client) UpdateProject(req models.ProjectConfigRequest) error {
	return c.post("/api/project/update_project", req, nil)
}

// SetProjectForceUpdate 只更新项目的强制更新标记，不涉及其他字段
func (c *Client) SetProjectForceUpdate(projectName string, forceUpdate bool) error {
	return c.post("/api/project/set_force_update", map[string]interface{}{
		"projectName": projectName,
		"forceUpdate": forceUpdate,
	}, nil)
}

// DeleteProject 软删除项目
func (c *Client) DeleteProject(projectName string) error {
	path := fmt.Sprintf("/api/project/delete_project/%s", url.PathEscape(projectName))
	return c.post(path, struct{}{}, nil)
}

// GetProjectChangeLogs 获取变更日志（按项目名称）
func (c *Client) GetProjectChangeLogs(projectName string) ([]models.ProjectChangeLog, error) {
	var logs []models.ProjectChangeLog
	path := fmt.Sprintf("/api/project/get_project_change_logs/%s", url.PathEscape(projectName))
	if err := c.get(path, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

// GetProjectOSInfo 获取服务端系统信息（按项目名称）
func (c *Client) GetProjectOSInfo(projectName string) ([]models.ServerOSInfo, error) {
	var info []models.ServerOSInfo
	path := fmt.Sprintf("/api/project/get_project_os_info/%s", url.PathEscape(projectName))
	if err := c.get(path, &info); err != nil {
		return nil, err
	}
	return info, nil
}

// GetServerInfo 获取服务端系统信息（无需指定项目）
func (c *Client) GetServerInfo() ([]models.ServerOSInfo, error) {
	var info []models.ServerOSInfo
	if err := c.get("/api/server/info", &info); err != nil {
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

// GetAllFiles 获取服务端文件列表（按项目名称）
func (c *Client) GetAllFiles(projectName string) ([]models.FileInfo, error) {
	var files []models.FileInfo
	path := fmt.Sprintf("/api/file/get_all_files/%s", url.PathEscape(projectName))
	if err := c.get(path, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// UploadFile 上传单个文件（multipart，流式传输避免大文件 OOM）
func (c *Client) UploadFile(localPath, projectName, relativeFileName string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	writeErrCh := make(chan error, 1)

	// 在 goroutine 中写入 multipart 数据，通过 Pipe 流式传输
	// file.Close 在 goroutine 内完成，避免主 goroutine 提前关闭文件
	go func() {
		defer file.Close()
		var writeErr error
		defer func() {
			// panic recovery：确保 goroutine panic 不会导致 Pipe 静默成功
			if r := recover(); r != nil {
				writeErr = fmt.Errorf("panic in multipart writer: %v", r)
			}
			// 确保 writer 关闭以结束 multipart 边界
			if closeErr := writer.Close(); closeErr != nil && writeErr == nil {
				writeErr = closeErr
			}
			pw.CloseWithError(writeErr)
			writeErrCh <- writeErr
		}()

		// file 字段
		part, err := writer.CreateFormFile("file", filepath.Base(localPath))
		if err != nil {
			writeErr = fmt.Errorf("create form file: %w", err)
			return
		}
		if _, err := io.Copy(part, file); err != nil {
			writeErr = fmt.Errorf("copy file to part: %w", err)
			return
		}

		// projectName 字段
		if err := writer.WriteField("projectName", projectName); err != nil {
			writeErr = fmt.Errorf("write projectName field: %w", err)
			return
		}
		// relativeFileName 字段
		if err := writer.WriteField("relativeFileName", relativeFileName); err != nil {
			writeErr = fmt.Errorf("write relativeFileName field: %w", err)
			return
		}
	}()

	resp, err := c.hc.Post(c.ServerURL+"/api/file/upload_file", writer.FormDataContentType(), pr)
	writeErr := <-writeErrCh
	if writeErr != nil {
		return fmt.Errorf("multipart write: %w", writeErr)
	}
	if err != nil {
		return fmt.Errorf("upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("upload file returned HTTP %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("upload file returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("upload file read response: %w", readErr)
	}
	var cr models.CommonResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return fmt.Errorf("upload file parse response: %w", err)
	}
	if !cr.IsSuccess {
		return fmt.Errorf("upload file: %s", cr.ErrorMsg)
	}
	return nil
}
