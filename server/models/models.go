package models

import (
	"time"
)

type FileInfo struct {
	FileAbsolutePath string    `json:"fileAbsolutePath"`
	FileRelativePath string    `json:"fileRelativePath"`
	LastUpdateTime   time.Time `json:"lastUpdateTime"`
	FileSize         int64     `json:"fileSize"`
	MD5              string    `json:"md5"`
	SHA256           string    `json:"sha256"`
}

type CommonResponse struct {
	IsSuccess bool        `json:"isSuccess"`
	ErrorMsg  string      `json:"errorMsg"`
	Data      any         `json:"data"`
}

func OK() CommonResponse {
	return CommonResponse{
		IsSuccess: true,
		ErrorMsg:  "",
		Data:      nil,
	}
}

func OKWithData(data any) CommonResponse {
	return CommonResponse{
		IsSuccess: true,
		ErrorMsg:  "",
		Data:      data,
	}
}

func NG(msg string) CommonResponse {
	return CommonResponse{
		IsSuccess: false,
		ErrorMsg:  msg,
		Data:      nil,
	}
}
func NGWithError(err error) CommonResponse {
	return CommonResponse{
		IsSuccess: false,
		ErrorMsg:  err.Error(),
		Data:      nil,
	}
}

// 模型-系统信息
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
