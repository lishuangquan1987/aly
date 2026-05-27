package model

import (
	"encoding/json"
	"time"
)

// CommonResponse 对应服务端的统一响应结构
type CommonResponse struct {
	IsSuccess bool            `json:"isSuccess"`
	ErrorMsg  string          `json:"errorMsg"`
	Data      json.RawMessage `json:"data"`
}

// Project 对应服务端 ent 生成的 Project 结构（snake_case JSON tag）
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

// FileInfo 对应服务端 models.FileInfo（camelCase JSON tag）
type FileInfo struct {
	FileAbsolutePath string    `json:"fileAbsolutePath"`
	FileRelativePath string    `json:"fileRelativePath"`
	LastUpdateTime   time.Time `json:"lastUpdateTime"`
	FileSize         int64     `json:"fileSize"`
	MD5              string    `json:"md5"`
	SHA256           string    `json:"sha256"`
}

// --- 命令行输出模型 ---

// CheckUpdateOutput 是 check_update 命令的 JSON 输出
type CheckUpdateOutput struct {
	HasUpdate      bool   `json:"has_update"`
	CurrentVersion string `json:"current_version,omitempty"`
	NewVersion     string `json:"new_version,omitempty"`
	ForceUpdate    bool   `json:"force_update,omitempty"`
	Error          string `json:"error,omitempty"`
}

// DiffFile 表示 check_diff 输出中的单个差异文件
type DiffFile struct {
	Path       string `json:"path"`
	Status     string `json:"status"`
	LocalMD5   string `json:"local_md5"`
	ServerMD5  string `json:"server_md5"`
	ServerSize int64  `json:"server_size"`
}

// CheckDiffOutput 是 check_diff 命令的 JSON 输出
type CheckDiffOutput struct {
	NewVersion string     `json:"new_version"`
	Files      []DiffFile `json:"files"`
}

// SuccessOutput 是通用成功/错误输出
type SuccessOutput struct {
	Success bool   `json:"success"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

// RollbackListOutput 是 list_rollback_versions 命令的 JSON 输出
type RollbackListOutput struct {
	CurrentVersion string   `json:"current_version"`
	Versions       []string `json:"versions"`
}
