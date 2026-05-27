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
}
