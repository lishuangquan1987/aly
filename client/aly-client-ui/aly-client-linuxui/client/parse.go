package client

import (
	"encoding/json"
	"strings"
)

// Kind 表示一行 stdout 的语义。
type Kind int

const (
	KindIgnore Kind = iota
	KindResult
	KindProgress
	KindDone
	KindFail
)

// Parsed 是一行的解析结果。
type Parsed struct {
	Kind       Kind
	Success    bool
	ErrMsg     string
	Check      *CheckUpdate
	Self       *CheckSelfUpdate
	SelfUpdate *SelfUpdate
	Progress   *Progress
	Rollback   *Rollback
	Versions   *RollbackList
}

type envelope struct {
	IsSuccess bool            `json:"isSuccess"`
	ErrMsg    string          `json:"errorMsg"`
	Data      json.RawMessage `json:"data"`
}

// ParseLine 解析一行 stdout。判定用键存在性，避免全零值误判类型。
func ParseLine(line string) (Parsed, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || !strings.HasPrefix(trimmed, "{") {
		return Parsed{Kind: KindIgnore}, nil
	}
	var env envelope
	if err := json.Unmarshal([]byte(trimmed), &env); err != nil {
		return Parsed{Kind: KindIgnore}, nil
	}
	p := Parsed{Success: env.IsSuccess, ErrMsg: env.ErrMsg}

	if len(env.Data) == 0 || string(env.Data) == "null" {
		if env.IsSuccess {
			p.Kind = KindDone
		} else {
			p.Kind = KindResult
		}
		return p, nil
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(env.Data, &keys); err != nil {
		return Parsed{Kind: KindIgnore}, nil
	}
	switch {
	case hasKey(keys, "index") && hasKey(keys, "status"):
		var prog Progress
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
		var chk CheckUpdate
		if err := json.Unmarshal(env.Data, &chk); err != nil {
			return Parsed{Kind: KindIgnore}, nil
		}
		p.Check = &chk
		p.Kind = KindResult
		return p, nil
	case hasKey(keys, "need_update"):
		var self CheckSelfUpdate
		if err := json.Unmarshal(env.Data, &self); err != nil {
			return Parsed{Kind: KindIgnore}, nil
		}
		p.Self = &self
		p.Kind = KindResult
		return p, nil
	case hasKey(keys, "updated"):
		var su SelfUpdate
		if err := json.Unmarshal(env.Data, &su); err != nil {
			return Parsed{Kind: KindIgnore}, nil
		}
		p.SelfUpdate = &su
		p.Kind = KindResult
		return p, nil
	case hasKey(keys, "versions"):
		var vers RollbackList
		if err := json.Unmarshal(env.Data, &vers); err != nil {
			return Parsed{Kind: KindIgnore}, nil
		}
		p.Versions = &vers
		p.Kind = KindResult
		return p, nil
	case hasKey(keys, "version"):
		var rb Rollback
		if err := json.Unmarshal(env.Data, &rb); err != nil {
			return Parsed{Kind: KindIgnore}, nil
		}
		p.Rollback = &rb
		p.Kind = KindResult
		return p, nil
	}
	return Parsed{Kind: KindIgnore}, nil
}

func hasKey(m map[string]json.RawMessage, k string) bool {
	_, ok := m[k]
	return ok
}
