package model

import "encoding/json"

// --- 服务端响应解析模型（与服务端 JSON 字段严格对应）---

// CommonResponse 对服务端的统一响应（camelCase）
type CommonResponse struct {
	IsSuccess bool            `json:"isSuccess"`
	ErrorMsg  string          `json:"errorMsg"`
	Data      json.RawMessage `json:"data"`
}

// Project 对应服务端 ent 生成的 Project 结构（snake_case）
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

// ProjectChangeLog 对应服务端 ent 生成的 ProjectChangeLog 结构
type ProjectChangeLog struct {
	ID        int      `json:"id"`
	Version   string   `json:"version"`
	Logs      []string `json:"logs"`
	Time      string   `json:"time"`
	CreatedAt string   `json:"created_at"`
	IsDeleted bool     `json:"is_deleted"`
}

// FileInfo 对应服务端 models.FileInfo（camelCase + 小写 md5/sha256）
type FileInfo struct {
	FileAbsolutePath string `json:"fileAbsolutePath"`
	FileRelativePath string `json:"fileRelativePath"`
	LastUpdateTime   string `json:"lastUpdateTime"`
	FileSize         int64  `json:"fileSize"`
	MD5              string `json:"md5"`
	SHA256           string `json:"sha256"`
}

// --- 命令行输出模型（camelCase，server/client/publish-cli 三端统一）---

// Output 是所有命令的标准输出包装（camelCase，三端统一）
type Output struct {
	IsSuccess bool        `json:"isSuccess"`
	ErrMsg    string      `json:"errorMsg"`
	Data      interface{} `json:"data"`
}

// CheckUpdateData 是 check_update 命令 data 字段
type CheckUpdateData struct {
	HasUpdate      bool   `json:"has_update"`
	CurrentVersion string `json:"current_version"`
	NewVersion     string `json:"new_version,omitempty"`
	ForceUpdate    *bool  `json:"force_update,omitempty"`
}

// DiffFileItem 表示 check_diff 输出中的单个差异文件
type DiffFileItem struct {
	Path         string `json:"path"`
	LocalMD5     string `json:"local_md5"`
	LocalSize    int64  `json:"local_size"`
	LocalSHA256  string `json:"local_sha256"`
	ServerMD5    string `json:"server_md5"`
	ServerSize   int64  `json:"server_size"`
	ServerSHA256 string `json:"server_sha256"`
}

// CheckDiffData 是 check_diff 命令 data 字段
type CheckDiffData struct {
	NewVersion string         `json:"new_version"`
	Files      []DiffFileItem `json:"files"`
}

// DownloadUpdateData 是 download_update 命令 data 字段
type DownloadUpdateData struct {
	Version string `json:"version"`
}

// DownloadProgress 是 download_update 命令的过程进度输出（每行一个 JSON）
type DownloadProgress struct {
	Index    int    `json:"index"`
	Total    int    `json:"total"`
	File     string `json:"file"`
	Status   string `json:"status"`   // START / DONE / SKIP / FAIL
	FileSize int64  `json:"file_size"`
	Error    string `json:"error,omitempty"`
}

// RollbackListData 是 list_rollback_versions 命令 data 字段
type RollbackListData struct {
	CurrentVersion string   `json:"current_version"`
	Versions       []string `json:"versions"`
}

// RollbackData 是 rollback 命令 data 字段
type RollbackData struct {
	Version string `json:"version"`
}

// CheckSelfUpdateData 是 check_self_update 命令 data 字段
type CheckSelfUpdateData struct {
	NeedUpdate bool `json:"need_update"`
}
