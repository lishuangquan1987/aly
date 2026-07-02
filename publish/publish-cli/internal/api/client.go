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
	"strings"
	"time"

	"aly/publish-cli/pkg/models"
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

// ─── 分片上传常量 ──────────────────────────────────────────────────────

const chunkSize = 50 * 1024 // 50 KiB（跨子网防火墙通常允许 <64KB 的 POST body）

// ─── 分片上传逻辑 ──────────────────────────────────────────────────────

// isTransientUploadError 判断是否为可重试的临时网络错误
func isTransientUploadError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "closed pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "forcibly closed") ||
		strings.Contains(msg, "timeout")
}

// UploadFile 上传单个文件：小文件直接上传，大文件分片上传
func (c *Client) UploadFile(localPath, projectName, relativeFileName string) error {
	fi, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if fi.Size() <= chunkSize {
		return c.uploadFileDirect(localPath, projectName, relativeFileName)
	}
	return c.uploadFileChunked(localPath, projectName, relativeFileName, fi.Size())
}

// uploadFileDirect 直接上传小文件（内存缓冲，无 io.Pipe）
func (c *Client) uploadFileDirect(localPath, projectName, relativeFileName string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	body := multipartFormBuffer(data, filepath.Base(localPath), map[string]string{
		"projectName":      projectName,
		"relativeFileName": relativeFileName,
	})

	resp, err := c.hc.Post(c.ServerURL+"/api/file/upload_file", body.contentType, &body.buf)
	if err != nil {
		return fmt.Errorf("upload file: %w", err)
	}
	defer resp.Body.Close()

	return checkResponse(resp)
}

// uploadFileChunked 分片上传大文件
func (c *Client) uploadFileChunked(localPath, projectName, relativeFileName string, fileSize int64) error {
	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// 顺序上传分片（每个分片有独立重试）
	for i := 0; i < totalChunks; i++ {
		offset := int64(i) * chunkSize
		remaining := fileSize - offset
		chunkLen := chunkSize
		if remaining < int64(chunkSize) {
			chunkLen = int(remaining)
		}
		if chunkLen <= 0 {
			break
		}

		buf := make([]byte, chunkLen)
		if _, err := file.ReadAt(buf, offset); err != nil {
			return fmt.Errorf("read chunk %d: %w", i, err)
		}

		if err := c.uploadChunkWithRetry(buf, projectName, relativeFileName, i, totalChunks); err != nil {
			return fmt.Errorf("upload chunk %d/%d: %w", i, totalChunks, err)
		}
	}

	// 通知服务端合并分片
	return c.completeChunks(projectName, relativeFileName, totalChunks)
}

// completeChunks 通知服务端合并所有分片
func (c *Client) completeChunks(projectName, relativeFileName string, totalChunks int) error {
	body := multipartFormBuffer(nil, "", map[string]string{
		"projectName":      projectName,
		"relativeFileName": relativeFileName,
		"totalChunks":      fmt.Sprintf("%d", totalChunks),
	})

	resp, err := c.hc.Post(c.ServerURL+"/api/file/upload_chunk_complete", body.contentType, &body.buf)
	if err != nil {
		return fmt.Errorf("chunk complete: %w", err)
	}
	defer resp.Body.Close()

	return checkResponse(resp)
}

// uploadChunkWithRetry 上传单个分片（带重试）
func (c *Client) uploadChunkWithRetry(data []byte, projectName, relativeFileName string, chunkIndex, totalChunks int) error {
	maxRetries := 3
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(200*(1<<attempt)) * time.Millisecond
			time.Sleep(backoff)
		}
		err := c.uploadChunkOnce(data, projectName, relativeFileName, chunkIndex, totalChunks)
		if err == nil {
			return nil
		}
		if !isTransientUploadError(err) {
			return err
		}
		lastErr = err
	}
	return fmt.Errorf("chunk %d/%d failed after %d retries: %w", chunkIndex, totalChunks, maxRetries, lastErr)
}

// uploadChunkOnce 单次上传分片（内存缓冲，无 io.Pipe）
func (c *Client) uploadChunkOnce(data []byte, projectName, relativeFileName string, chunkIndex, totalChunks int) error {
	body := multipartFormBuffer(data, filepath.Base(relativeFileName), map[string]string{
		"projectName":      projectName,
		"relativeFileName": relativeFileName,
		"chunkIndex":       fmt.Sprintf("%d", chunkIndex),
		"totalChunks":      fmt.Sprintf("%d", totalChunks),
	})

	resp, err := c.hc.Post(c.ServerURL+"/api/file/upload_chunk", body.contentType, &body.buf)
	if err != nil {
		return fmt.Errorf("upload chunk: %w", err)
	}
	defer resp.Body.Close()

	return checkResponse(resp)
}

// ─── multipart 辅助 ─────────────────────────────────────────────────────

type multipartForm struct {
	contentType string
	buf         bytes.Buffer
}

// multipartFormBuffer 构造 multipart/form-data 请求体（完全在内存中，无 io.Pipe）
// fileData 为 nil 时跳过 file 字段（用于 complete 请求）
func multipartFormBuffer(fileData []byte, fileName string, extraFields map[string]string) multipartForm {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if fileData != nil {
		part, _ := writer.CreateFormFile("file", fileName)
		part.Write(fileData)
	}

	for key, value := range extraFields {
		writer.WriteField(key, value)
	}

	writer.Close()
	return multipartForm{contentType: writer.FormDataContentType(), buf: buf}
}

// checkResponse 检查通用响应
func checkResponse(resp *http.Response) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("HTTP %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("read response: %w", readErr)
	}
	var cr models.CommonResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if !cr.IsSuccess {
		return fmt.Errorf("%s", cr.ErrorMsg)
	}
	return nil
}
