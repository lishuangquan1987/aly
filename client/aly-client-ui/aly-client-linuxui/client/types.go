package client

// 与 aly-client 标准 JSON 输出对应的本地 DTO。Linux UI 是独立 Go module，
// 不 import GOPATH 里的 aly-client/model，避免 Go 1.10/1.27 模块混用；
// 这里以 JSON 字段为唯一契约。

type CheckUpdate struct {
	HasUpdate          bool     `json:"has_update"`
	NeedDownloadUpdate bool     `json:"need_download_update"`
	CurrentVersion     string   `json:"current_version"`
	NewVersion         string   `json:"new_version,omitempty"`
	ForceUpdate        *bool    `json:"force_update,omitempty"`
	ReleaseNotes       []string `json:"release_notes,omitempty"`
}

type Progress struct {
	Index           int    `json:"index"`
	Total           int    `json:"total"`
	File            string `json:"file"`
	Status          string `json:"status"`
	FileSize        int64  `json:"file_size"`
	Error           string `json:"error,omitempty"`
	DownloadedBytes int64  `json:"downloaded_bytes,omitempty"`
	TotalBytes      int64  `json:"total_bytes,omitempty"`
	SpeedBps        int64  `json:"speed_bps,omitempty"`
}

type RollbackList struct {
	CurrentVersion string   `json:"current_version"`
	Versions       []string `json:"versions"`
}

type Rollback struct {
	Version string `json:"version"`
}

type SelfUpdate struct {
	Updated bool   `json:"updated"`
	Reason  string `json:"reason"`
	OldPath string `json:"old_path,omitempty"`
}

type CheckSelfUpdate struct {
	NeedUpdate bool `json:"need_update"`
}
