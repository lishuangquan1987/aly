// +build windows

// Package bridge 以子进程方式调用 aly-client.exe 并解析其 JSON 输出。
package bridge

import (
	"encoding/json"
	"strings"

	"aly/client/aly-client/model"
)

// Kind 表示一行 stdout 的语义。
type Kind int

const (
	// KindIgnore 非 JSON / 空行 / 无法识别 → 跳过。
	KindIgnore Kind = iota
	// KindResult 最终结果（单行 JSON，data 为具体结构）。
	KindResult
	// KindProgress 下载进度行。
	KindProgress
	// KindDone 下载完成行（isSuccess:true 且 data:null）。
	KindDone
	// KindFail 进度失败行（isSuccess:false 且带进度数据）。
	KindFail
)

// Parsed 是一行的解析结果。
type Parsed struct {
	Kind     Kind
	Success  bool
	ErrMsg   string
	Check    *model.CheckUpdateData
	Self     *model.CheckSelfUpdateData
	Progress *model.DownloadProgress
	Rollback *model.RollbackData
	Versions *model.RollbackListData
}

// envelope 先取出 data 的原始字节，再按命令类型二次解析。
type envelope struct {
	IsSuccess bool            `json:"isSuccess"`
	ErrMsg    string          `json:"errorMsg"`
	Data      json.RawMessage `json:"data"`
}

// ParseLine 解析一行 stdout。
//
// 非 JSON 行返回 KindIgnore 且 err==nil（调用方直接跳过即可）。
//
// 结果类型判定用 **键存在性** 而非字段零值：aly-client 在
// `has_update=false` 且 `current_version` 为空时，CheckUpdateData 的
// 所有字段都是零值，仅凭零值判断会把它误判成 CheckSelfUpdateData。
// 各 DTO 的判别键互不重叠：
//
//	DownloadProgress   index (且 status 非空)
//	CheckUpdateData    has_update
//	CheckSelfUpdateData need_update
//	RollbackListData   versions
//	RollbackData       version
func ParseLine(line string) (Parsed, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.HasPrefix(trimmed, "{") {
		return Parsed{Kind: KindIgnore}, nil
	}

	var env envelope
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		// 形似 JSON 但解析失败：按忽略处理，不中断流程
		return Parsed{Kind: KindIgnore}, nil
	}

	p := Parsed{Success: env.IsSuccess, ErrMsg: env.ErrMsg}

	if len(env.Data) == 0 || string(env.Data) == "null" {
		if env.IsSuccess {
			p.Kind = KindDone // 成功且 data:null = 下载完成行
		} else {
			p.Kind = KindResult // 失败结果（errorMsg 携带原因）
		}
		return p, nil
	}

	// 取 data 的键集合，用于判定类型
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &keys); err != nil {
		return Parsed{Kind: KindIgnore}, nil
	}

	switch {
	case hasKey(keys, "index") && hasKey(keys, "status"):
		var prog model.DownloadProgress
		if err := json.Unmarshal(env.Data, &prog); err != nil {
			return Parsed{Kind: KindIgnore}, nil
		}
		p.Progress = &prog
		if env.IsSuccess {
			p.Kind = KindProgress
		} else {
			p.Kind = KindFail
		}
		return p, nil

	case hasKey(keys, "has_update"):
		var chk model.CheckUpdateData
		if err := json.Unmarshal(env.Data, &chk); err != nil {
			return Parsed{Kind: KindIgnore}, nil
		}
		p.Check = &chk
		p.Kind = KindResult
		return p, nil

	case hasKey(keys, "need_update"):
		var self model.CheckSelfUpdateData
		if err := json.Unmarshal(env.Data, &self); err != nil {
			return Parsed{Kind: KindIgnore}, nil
		}
		p.Self = &self
		p.Kind = KindResult
		return p, nil

	case hasKey(keys, "versions"):
		var vers model.RollbackListData
		if err := json.Unmarshal(env.Data, &vers); err != nil {
			return Parsed{Kind: KindIgnore}, nil
		}
		p.Versions = &vers
		p.Kind = KindResult
		return p, nil

	case hasKey(keys, "version"):
		var rb model.RollbackData
		if err := json.Unmarshal(env.Data, &rb); err != nil {
			return Parsed{Kind: KindIgnore}, nil
		}
		p.Rollback = &rb
		p.Kind = KindResult
		return p, nil
	}

	return Parsed{Kind: KindIgnore}, nil
}

// hasKey 判断 JSON 对象是否包含某键（不看值）。
func hasKey(m map[string]json.RawMessage, k string) bool {
	_, ok := m[k]
	return ok
}
