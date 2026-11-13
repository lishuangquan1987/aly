package main

import (
	"encoding/json"
	"strings"
)

type pushKind int

const (
	pushIgnored pushKind = iota
	pushProgress
	pushFailure
	pushSuccess
)

type pushEvent struct {
	Kind     pushKind
	Progress UploadProgress
	Error    string
}

// parsePushLine classifies one line of `push --json`: per-file progress,
// the final success line, an error, or noise.
func parsePushLine(line string) pushEvent {
	line = strings.TrimSpace(line)
	if line == "" || !strings.HasPrefix(line, "{") {
		return pushEvent{Kind: pushIgnored}
	}
	var out cliOutput
	if err := json.Unmarshal([]byte(line), &out); err != nil {
		return pushEvent{Kind: pushIgnored}
	}
	data := strings.TrimSpace(string(out.Data))
	if data != "" && data != "null" {
		var progress UploadProgress
		if err := json.Unmarshal(out.Data, &progress); err == nil && progress.Status != "" {
			ev := pushEvent{Kind: pushProgress, Progress: progress}
			if progress.Status == "FAIL" && progress.Error != "" {
				ev.Error = progress.File + ": " + progress.Error
			}
			return ev
		}
	}
	if !out.IsSuccess {
		msg := strings.TrimSpace(out.ErrorMsg)
		if msg == "" && data != "" && data != "null" {
			var failure struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(out.Data, &failure) == nil {
				msg = strings.TrimSpace(failure.Error)
			}
		}
		return pushEvent{Kind: pushFailure, Error: msg}
	}
	if data != "" && data != "null" && strings.HasPrefix(data, "{") {
		return pushEvent{Kind: pushSuccess}
	}
	return pushEvent{Kind: pushIgnored}
}

func statusDisplay(status string, lang string) string {
	if lang == "en" {
		switch status {
		case "new":
			return "New"
		case "modified":
			return "Modified"
		case "deleted":
			return "Deleted"
		case "unchanged":
			return "Unchanged"
		}
		return status
	}
	switch status {
	case "new":
		return "新增"
	case "modified":
		return "修改"
	case "deleted":
		return "删除"
	case "unchanged":
		return "无变化"
	}
	return status
}
