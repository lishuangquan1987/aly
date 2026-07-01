package client

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aly/client/aly-client/model"
)

var httpClient = &http.Client{
	Timeout: 300 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     90 * time.Second,
		DisableCompression:  false,
	},
}

// doGet 执行 GET 请求并返回响应体
func doGet(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("服务器返回状态码: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %v", err)
	}

	return body, nil
}

// parseResponse 解析 CommonResponse 并提取 Data 字段
func parseResponse(data []byte) (*model.CommonResponse, error) {
	var resp model.CommonResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	if !resp.IsSuccess {
		return nil, fmt.Errorf("服务器错误: %s", resp.ErrorMsg)
	}

	return &resp, nil
}

// buildAPIURL 校验并构建 API URL，确保 serverURL 格式合法
func buildAPIURL(serverURL, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("API 路径不能为空")
	}
	// 自动补全协议前缀，兼容旧配置（如 localhost:8080）
	if !strings.Contains(serverURL, "://") {
		serverURL = "http://" + serverURL
	}
	base, err := url.Parse(strings.TrimRight(serverURL, "/"))
	if err != nil {
		return "", fmt.Errorf("无效的服务器地址: %v", err)
	}
	if base.Host == "" {
		return "", fmt.Errorf("服务器地址缺少主机名")
	}
	// 仅使用 origin 部分，避免 serverURL 中的路径前缀导致重复
	return base.Scheme + "://" + base.Host + "/" + strings.TrimPrefix(path, "/"), nil
}

// GetProjectByName 根据项目名称获取项目（走独立接口，不再拉全量列表）
func GetProjectByName(serverURL string, name string) (*model.Project, error) {
	u, err := buildAPIURL(serverURL, "api/project/get_project_by_name/"+urlEncodePath(name))
	if err != nil {
		return nil, err
	}
	data, err := doGet(u)
	if err != nil {
		return nil, err
	}

	resp, err := parseResponse(data)
	if err != nil {
		return nil, err
	}

	var project model.Project
	if err := json.Unmarshal(resp.Data, &project); err != nil {
		return nil, fmt.Errorf("解析项目信息失败: %v", err)
	}

	return &project, nil
}

// FindProjectByName 根据项目名称查找项目（使用独立接口，不再拉全量列表）
func FindProjectByName(serverURL string, name string) (*model.Project, error) {
	project, err := GetProjectByName(serverURL, name)
	if err != nil {
		return nil, err
	}
	if project.IsDeleted {
		return nil, fmt.Errorf("未找到项目: %s", name)
	}
	return project, nil
}

// GetProjectChangeLogs 获取项目的变更日志（按项目名称）
func GetProjectChangeLogs(serverURL string, projectName string) ([]model.ProjectChangeLog, error) {
	pn := urlEncodePath(projectName)
	u, err := buildAPIURL(serverURL, "api/project/get_project_change_logs/"+pn)
	if err != nil {
		return nil, err
	}
	data, err := doGet(u)
	if err != nil {
		return nil, err
	}

	resp, err := parseResponse(data)
	if err != nil {
		return nil, err
	}

	var logs []model.ProjectChangeLog
	if err := json.Unmarshal(resp.Data, &logs); err != nil {
		return nil, fmt.Errorf("解析变更日志失败: %v", err)
	}

	return logs, nil
}

// GetAllFiles 获取项目的所有文件列表（按项目名称）
func GetAllFiles(serverURL string, projectName string) ([]model.FileInfo, error) {
	pn := urlEncodePath(projectName)
	u, err := buildAPIURL(serverURL, "api/file/get_all_files/"+pn)
	if err != nil {
		return nil, err
	}
	data, err := doGet(u)
	if err != nil {
		return nil, err
	}

	resp, err := parseResponse(data)
	if err != nil {
		return nil, err
	}

	var files []model.FileInfo
	if err := json.Unmarshal(resp.Data, &files); err != nil {
		return nil, fmt.Errorf("解析文件列表失败: %v", err)
	}

	return files, nil
}

// DownloadFile 从服务端下载文件并保存到本地
func DownloadFile(serverURL string, serverFilePath string, localPath string) error {
	u, err := buildAPIURL(serverURL, "api/file/download_file?path="+urlEncodePath(serverFilePath))
	if err != nil {
		return err
	}

	resp, err := httpClient.Get(u)
	if err != nil {
		return fmt.Errorf("下载请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(localPath)
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}

// DownloadFileWithResume downloads a file, resuming from partial download if a .part file exists.
// largeFileThreshold is the size in bytes above which resume is attempted (e.g., 100*1024*1024 for 100MB).
func DownloadFileWithResume(serverURL string, serverFilePath string, localPath string, serverFileSize int64, largeFileThreshold int64) error {
	requestURL, err := buildAPIURL(serverURL, "api/file/download_file?path="+urlEncodePath(serverFilePath))
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory failed: %v", err)
	}

	// For files larger than threshold, check for existing partial download
	partPath := localPath + ".part"
	if serverFileSize > largeFileThreshold {
		// Check if completed file already exists
		if localInfo, localErr := os.Stat(localPath); localErr == nil && localInfo.Size() == serverFileSize {
			return nil
		}

		// Check if partial .part file exists for resume
		if partInfo, partErr := os.Stat(partPath); partErr == nil && partInfo.Size() > 0 && partInfo.Size() < serverFileSize {
			// Resume from .part file
			f, err := os.OpenFile(partPath, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("open part file failed: %v", err)
			}

			req, err := http.NewRequest("GET", requestURL, nil)
			if err != nil {
				f.Close()
				return fmt.Errorf("create request failed: %v", err)
			}
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", partInfo.Size()))

			resp, err := httpClient.Do(req)
			if err != nil {
				f.Close()
				return fmt.Errorf("resume request failed: %v", err)
			}

			if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				f.Close()
				// Server doesn't support resume, remove .part and start over
				os.Remove(partPath)
				return DownloadFile(serverURL, serverFilePath, localPath)
			}

			_, copyErr := io.Copy(f, resp.Body)
			resp.Body.Close()
			f.Close()

			if copyErr != nil {
				return fmt.Errorf("resume write failed: %v", copyErr)
			}

			// Verify download is complete before renaming
			if checkInfo, checkErr := os.Stat(partPath); checkErr == nil && checkInfo.Size() == serverFileSize {
				if err := os.Rename(partPath, localPath); err != nil {
					return fmt.Errorf("rename part file failed: %v", err)
				}
				return nil
			}
			// Download not complete after resume, fall through to fresh download
			os.Remove(partPath)
		}

		// No valid .part file or resume didn't complete - remove any leftover .part
		os.Remove(partPath)
	}

	// Download to .part file, then rename atomically
	f, err := os.Create(partPath)
	if err != nil {
		return fmt.Errorf("create file failed: %v", err)
	}

	resp, err := httpClient.Get(requestURL)
	if err != nil {
		f.Close()
		os.Remove(partPath)
		return fmt.Errorf("download failed: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		f.Close()
		os.Remove(partPath)
		return fmt.Errorf("download failed, status: %d", resp.StatusCode)
	}

	_, copyErr := io.Copy(f, resp.Body)
	resp.Body.Close()
	f.Close()

	if copyErr != nil {
		os.Remove(partPath)
		return fmt.Errorf("write file failed: %v", copyErr)
	}

	if err := os.Rename(partPath, localPath); err != nil {
		return fmt.Errorf("rename part file failed: %v", err)
	}
	return nil
}

func urlEncodePath(s string) string {
	// 将反斜杠替换为正斜杠
	s = strings.Replace(s, "\\", "/", -1)
	// 对路径的每个段进行 URL 编码，保留 / 分隔符
	parts := strings.Split(s, "/")
	for i := range parts {
		// 拒绝路径遍历
		if parts[i] == ".." {
			return ""
		}
		parts[i] = url.PathEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
