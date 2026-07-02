package models

import "encoding/json"

// ─── 传输层：解析服务端响应（camelCase，与 server 对齐）────────────────────

// CommonResponse 服务端统一响应格式
type CommonResponse struct {
	IsSuccess bool            `json:"isSuccess"`
	ErrorMsg  string          `json:"errorMsg"`
	Data      json.RawMessage `json:"data"`
}

// Project 服务端 ent 生成的 Project（GET 响应 snake_case）
type Project struct {
	ID            int      `json:"id"`
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	Version       string   `json:"version"`
	ForceUpdate   bool     `json:"force_update"`
	IgnoreFolders []string `json:"ignore_folders"`
	IgnoreFiles   []string `json:"ignore_files"`
	CreatedAt     string   `json:"created_at"`
	IsDeleted     bool     `json:"is_deleted"`
}

// ProjectChangeLog 服务端 ent 生成的 ProjectChangeLog
type ProjectChangeLog struct {
	ID                      int      `json:"id"`
	ProjectID               int      `json:"project_id"`
	Version                 string   `json:"version"`
	Logs                    []string `json:"logs"`
	Time                    string   `json:"time"`
	CreatedAt               string   `json:"created_at"`
	IsDeleted               bool     `json:"is_deleted"`
	AfterApplyUpdateScript  string   `json:"after_apply_update_script"`
}

// FileInfo 服务端 models.FileInfo（camelCase）
type FileInfo struct {
	FileAbsolutePath string `json:"fileAbsolutePath"`
	FileRelativePath string `json:"fileRelativePath"`
	LastUpdateTime   string `json:"lastUpdateTime"`
	FileSize         int64  `json:"fileSize"`
	MD5              string `json:"md5"`
	SHA256           string `json:"sha256"`
}

// ServerOSInfo 服务端系统信息
type ServerOSInfo struct {
	OS              string  `json:"os"`
	Platform        string  `json:"platform"`
	GOARCH          string  `json:"goARCH"`
	Version         string  `json:"version"`
	NumCPU          int     `json:"numCPU"`
	CPUName         string  `json:"cpuName"`
	CPUMhz          float64 `json:"cpuMhz"`
	DiskUsed        float64 `json:"diskUsed"`
	DiskFree        float64 `json:"diskFree"`
	DiskTotal       float64 `json:"diskTotal"`
	DiskUsedPercent float64 `json:"diskUsedPercent"`
}

// ─── 请求 DTO（POST 请求体，camelCase）───────────────────────────────────

// ProjectConfigRequest POST /api/project/create_project 和 update_project 共用
type ProjectConfigRequest struct {
	Name          string   `json:"name"`
	Title         string   `json:"title"`
	IsForceUpdate bool     `json:"isForceUpdate"`
	IgnoreFolders []string `json:"ignoreFolders"`
	IgnoreFiles   []string `json:"ignoreFiles"`
}

// PublishVersionRequest POST /api/project/publish_version
type PublishVersionRequest struct {
	ProjectName            string   `json:"projectName"`
	Version                string   `json:"version"`
	Logs                   []string `json:"logs"`
	Time                   string   `json:"time"`
	AfterApplyUpdateScript string   `json:"afterApplyUpdateScript"`
}

// ─── CLI 输出模型（camelCase）────────────────────────────────────────────

// Output CLI JSON 标准输出包装
type Output struct {
	IsSuccess bool        `json:"isSuccess"`
	ErrMsg    string      `json:"errorMsg"`
	Data      interface{} `json:"data"`
}

// FileStatusItem status/diff 输出中的单个文件状态
type FileStatusItem struct {
	RelativePath string `json:"relativePath"`
	Status       string `json:"status"` // new / modified / deleted / unchanged
	LocalMd5     string `json:"localMd5"`
	LocalSize    int64  `json:"localSize"`
	ServerMd5    string `json:"serverMd5,omitempty"`
	ServerSize   int64  `json:"serverSize,omitempty"`
}

// StatusData status 命令的 data 字段
type StatusData struct {
	Staged    []FileStatusItem `json:"staged"`
	Unstaged  []FileStatusItem `json:"unstaged"`
	Unchanged []FileStatusItem `json:"unchanged"`
}

// ─── 内部模型 ──────────────────────────────────────────────────────────
// LocalFile 本地扫描结果
type LocalFile struct {
	AbsolutePath string
	RelativePath string
	Size         int64
	ModTime      string
	MD5          string
	SHA256       string
}

// FileDiff 文件差异（内部比对用）
type FileDiff struct {
	RelativePath string
	Status       string // new / modified / deleted / unchanged
	LocalMd5     string
	LocalSize    int64
	ServerMd5    string
	ServerSize   int64
}

// UploadProgress 是 push 命令的过程进度输出（每行一个 JSON，仅 --json 模式）
type UploadProgress struct {
	Index    int    `json:"index"`
	Total    int    `json:"total"`
	File     string `json:"file"`
	Status   string `json:"status"` // START / DONE / FAIL
	FileSize int64  `json:"file_size"`
	Error    string `json:"error,omitempty"`
}
