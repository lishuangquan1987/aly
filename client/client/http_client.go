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

	"clientupdator/client/model"
)

var httpClient = &http.Client{
	Timeout: 300 * time.Second,
}

// doGet 执行 GET 请求并返回响应体
func doGet(url string) ([]byte, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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

// GetAllProjects 获取所有项目列表
func GetAllProjects(serverURL string) ([]model.Project, error) {
	url := strings.TrimRight(serverURL, "/") + "/api/project/get_all_projects"
	data, err := doGet(url)
	if err != nil {
		return nil, err
	}

	resp, err := parseResponse(data)
	if err != nil {
		return nil, err
	}

	var projects []model.Project
	if err := json.Unmarshal(resp.Data, &projects); err != nil {
		return nil, fmt.Errorf("解析项目列表失败: %v", err)
	}

	return projects, nil
}

// FindProjectByName 根据项目名称查找项目
func FindProjectByName(serverURL string, name string) (*model.Project, error) {
	projects, err := GetAllProjects(serverURL)
	if err != nil {
		return nil, err
	}

	for i := range projects {
		if projects[i].Name == name && !projects[i].IsDeleted {
			return &projects[i], nil
		}
	}

	return nil, fmt.Errorf("未找到项目: %s", name)
}

// GetProjectChangeLogs 获取项目的变更日志
func GetProjectChangeLogs(serverURL string, projectID int) ([]model.ProjectChangeLog, error) {
	url := fmt.Sprintf("%s/api/project/get_project_change_logs/%d",
		strings.TrimRight(serverURL, "/"), projectID)
	data, err := doGet(url)
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

// GetAllFiles 获取项目的所有文件列表
func GetAllFiles(serverURL string, projectID int) ([]model.FileInfo, error) {
	url := fmt.Sprintf("%s/api/file/get_all_files/%d",
		strings.TrimRight(serverURL, "/"), projectID)
	data, err := doGet(url)
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
	url := fmt.Sprintf("%s/api/file/download_file?path=%s",
		strings.TrimRight(serverURL, "/"),
		urlEncodePath(serverFilePath))

	resp, err := httpClient.Get(url)
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

func urlEncodePath(s string) string {
	// 将反斜杠替换为正斜杠
	s = strings.Replace(s, "\\", "/", -1)
	// 对路径的每个段进行 URL 编码，保留 / 分隔符
	parts := strings.Split(s, "/")
	for i := range parts {
		parts[i] = url.QueryEscape(parts[i])
	}
	return strings.Join(parts, "/")
}
