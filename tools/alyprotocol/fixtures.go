package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Scenario 是一个可被假客户端回放、并受 schema 校验的命令场景。
type Scenario struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Commands    map[string]*ScenarioCommand `json:"commands"`
}

// ScenarioCommand 是一条命令的脚本：逐行 stdout 与退出码。
type ScenarioCommand struct {
	Lines       []json.RawMessage `json:"lines"`
	LineDelayMS int               `json:"line_delay_ms"`
	ExitCode    int               `json:"exit_code"`
}

// commandRule 描述某条命令最终行/进度行的载荷约束。
type commandRule struct {
	def       string // data 对应的 schema 定义；空表示只能是 null
	allowNull bool
	progress  bool // download_update：多行进度 + 最后一行完成/失败
}

var scenarioCommandRules = map[string]commandRule{
	"check_update":           {def: "CheckUpdateData"},
	"check_diff":             {def: "CheckDiffData"},
	"download_update":        {progress: true},
	"apply_update":           {allowNull: true},
	"rollback":               {def: "RollbackData", allowNull: true},
	"list_rollback_versions": {def: "RollbackListData"},
	"check_self_update":      {def: "CheckSelfUpdateData"},
	"self_update":            {def: "SelfUpdateData"},
	"clean":                  {def: "CleanData"},
}

// ParseScenario 严格解析场景文件：未知字段直接报错，避免拼错字段被静默忽略。
func ParseScenario(raw []byte) (*Scenario, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var sc Scenario
	if err := dec.Decode(&sc); err != nil {
		return nil, fmt.Errorf("解析 scenario: %v", err)
	}
	var extra struct{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("scenario 含多余内容")
	}
	return &sc, nil
}

// LoadScenarios 读取目录下全部 *.json 场景（按文件名排序）。
func LoadScenarios(dir string) ([]*Scenario, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("读取 fixtures 目录 %s: %v", dir, err)
	}
	var scenarios []*Scenario
	names := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("读取 %s: %v", path, err)
		}
		sc, err := ParseScenario(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", e.Name(), err)
		}
		if prev, dup := names[sc.Name]; dup {
			return nil, fmt.Errorf("%s: scenario name %q 与 %s 重复", e.Name(), sc.Name, prev)
		}
		names[sc.Name] = e.Name()
		scenarios = append(scenarios, sc)
	}
	if len(scenarios) == 0 {
		return nil, fmt.Errorf("fixtures 目录 %s 下没有场景文件", dir)
	}
	return scenarios, nil
}

// ValidateScenario 校验场景里的每一行输出：信封、载荷、null 位置、退出码。
func ValidateScenario(s *Schema, sc *Scenario) []error {
	var errs []error
	if strings.TrimSpace(sc.Name) == "" {
		errs = append(errs, fmt.Errorf("scenario 缺少 name"))
	}
	if len(sc.Commands) == 0 {
		errs = append(errs, fmt.Errorf("scenario %s 没有 commands", sc.Name))
	}
	for _, cmdName := range sortedStringKeys(sc.Commands) {
		cmd := sc.Commands[cmdName]
		if cmd == nil {
			errs = append(errs, fmt.Errorf("scenario %s: %s 为空", sc.Name, cmdName))
			continue
		}
		rule, ok := scenarioCommandRules[cmdName]
		if !ok {
			errs = append(errs, fmt.Errorf("scenario %s: 未知命令 %q", sc.Name, cmdName))
			continue
		}
		if cmd.ExitCode != 0 {
			errs = append(errs, fmt.Errorf("scenario %s: %s.exit_code 必须为 0（业务失败也以 0 退出，见 host-contract §2）", sc.Name, cmdName))
		}
		if cmd.LineDelayMS < 0 {
			errs = append(errs, fmt.Errorf("scenario %s: %s.line_delay_ms 不能为负", sc.Name, cmdName))
		}
		if len(cmd.Lines) == 0 {
			errs = append(errs, fmt.Errorf("scenario %s: %s 没有输出行", sc.Name, cmdName))
			continue
		}
		if rule.progress {
			errs = append(errs, validateDownloadStream(s, sc.Name, cmdName, cmd.Lines)...)
			continue
		}
		errs = append(errs, validateSingleLine(s, sc.Name, cmdName, rule, cmd.Lines)...)
	}
	return errs
}

func validateSingleLine(s *Schema, scenarioName, cmdName string, rule commandRule, lines []json.RawMessage) []error {
	var errs []error
	prefix := fmt.Sprintf("scenario %s: %s", scenarioName, cmdName)
	if len(lines) != 1 {
		errs = append(errs, fmt.Errorf("%s 必须恰好输出 1 行，实际 %d 行", prefix, len(lines)))
	}
	env, err := parseEnvelope(lines[0])
	if err != nil {
		return append(errs, fmt.Errorf("%s 第 1 行: %v", prefix, err))
	}
	if isJSONNull(env.Data) {
		if !rule.allowNull {
			errs = append(errs, fmt.Errorf("%s 第 1 行: data 不能为 null", prefix))
		}
		return errs
	}
	if rule.def == "" {
		errs = append(errs, fmt.Errorf("%s 第 1 行: 该命令不允许 data 载荷", prefix))
		return errs
	}
	for _, e := range validateObject(s, rule.def, env.Data) {
		errs = append(errs, fmt.Errorf("%s 第 1 行 data: %v", prefix, e))
	}
	return errs
}

func validateDownloadStream(s *Schema, scenarioName, cmdName string, lines []json.RawMessage) []error {
	var errs []error
	prefix := fmt.Sprintf("scenario %s: %s", scenarioName, cmdName)
	for i, line := range lines {
		env, err := parseEnvelope(line)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s 第 %d 行: %v", prefix, i+1, err))
			continue
		}
		last := i == len(lines)-1
		if isJSONNull(env.Data) {
			if !last {
				errs = append(errs, fmt.Errorf("%s 第 %d 行: data:null 只能出现在最后一行", prefix, i+1))
			}
			continue
		}
		if last {
			if env.IsSuccess {
				errs = append(errs, fmt.Errorf("%s 第 %d 行: 最后一行带 data 载荷时必须 isSuccess=false（部分失败）", prefix, i+1))
			}
			for _, e := range validateObject(s, "DownloadUpdateData", env.Data) {
				errs = append(errs, fmt.Errorf("%s 第 %d 行 data: %v", prefix, i+1, e))
			}
			continue
		}
		for _, e := range validateObject(s, "DownloadProgress", env.Data) {
			errs = append(errs, fmt.Errorf("%s 第 %d 行 data: %v", prefix, i+1, e))
		}
	}
	return errs
}

// ValidateFixtureCoverage 要求 9 条命令都至少有一个场景；新增命令忘记加样例会直接失败。
func ValidateFixtureCoverage(scenarios []*Scenario) []error {
	covered := make(map[string]bool)
	for _, sc := range scenarios {
		for cmd := range sc.Commands {
			covered[cmd] = true
		}
	}
	var errs []error
	for _, cmd := range sortedStringKeys(scenarioCommandRules) {
		if !covered[cmd] {
			errs = append(errs, fmt.Errorf("fixtures 未覆盖命令 %q", cmd))
		}
	}
	return errs
}

type envelope struct {
	IsSuccess bool
	ErrorMsg  string
	Data      json.RawMessage
}

func parseEnvelope(raw []byte) (*envelope, error) {
	names, vals, err := decodeObject(raw)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{"isSuccess": true, "errorMsg": true, "data": true}
	for _, n := range names {
		if !allowed[n] {
			return nil, fmt.Errorf("信封含未知字段 %q", n)
		}
	}
	for _, req := range []string{"isSuccess", "errorMsg", "data"} {
		if _, ok := vals[req]; !ok {
			return nil, fmt.Errorf("信封缺少字段 %q", req)
		}
	}
	if isJSONNull(vals["isSuccess"]) {
		return nil, fmt.Errorf("isSuccess 不能为 null")
	}
	var isSuccess bool
	if err := json.Unmarshal(vals["isSuccess"], &isSuccess); err != nil {
		return nil, fmt.Errorf("isSuccess 必须是 boolean")
	}
	if isJSONNull(vals["errorMsg"]) {
		return nil, fmt.Errorf("errorMsg 不能为 null")
	}
	var errMsg string
	if err := json.Unmarshal(vals["errorMsg"], &errMsg); err != nil {
		return nil, fmt.Errorf("errorMsg 必须是 string")
	}
	return &envelope{IsSuccess: isSuccess, ErrorMsg: errMsg, Data: vals["data"]}, nil
}

func isJSONNull(raw json.RawMessage) bool {
	return string(bytes.TrimSpace(raw)) == "null"
}

func validateObject(s *Schema, defName string, raw json.RawMessage) []error {
	obj := s.Def(defName)
	if obj == nil {
		return []error{fmt.Errorf("schema 缺少定义 %q", defName)}
	}
	names, vals, err := decodeObject(raw)
	if err != nil {
		return []error{err}
	}
	var errs []error
	for _, req := range obj.Required {
		if _, ok := vals[req]; !ok {
			errs = append(errs, fmt.Errorf("缺少必填字段 %q", req))
		}
	}
	for _, n := range names {
		if obj.Property(n) == nil {
			errs = append(errs, fmt.Errorf("未登记字段 %q", n))
		}
	}
	for _, p := range obj.Properties {
		v, ok := vals[p.Name]
		if !ok {
			continue
		}
		for _, e := range validateValue(s, p, v) {
			errs = append(errs, fmt.Errorf("%s: %v", p.Name, e))
		}
	}
	return errs
}

func validateValue(s *Schema, p *Property, raw json.RawMessage) []error {
	if isJSONNull(raw) {
		if p.Type == "" {
			return nil // 任意值（Output.data）允许 null
		}
		return []error{fmt.Errorf("不能为 null")}
	}
	switch p.Type {
	case "":
		return nil
	case "string":
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return []error{fmt.Errorf("必须是 string")}
		}
		if len(p.Enum) > 0 && !containsString(p.Enum, v) {
			return []error{fmt.Errorf("值 %q 不在枚举 %v 中", v, p.Enum)}
		}
	case "boolean":
		var v bool
		if err := json.Unmarshal(raw, &v); err != nil {
			return []error{fmt.Errorf("必须是 boolean")}
		}
	case "integer":
		var v int64
		if err := json.Unmarshal(raw, &v); err != nil {
			return []error{fmt.Errorf("必须是整数")}
		}
	case "number":
		var v float64
		if err := json.Unmarshal(raw, &v); err != nil {
			return []error{fmt.Errorf("必须是数字")}
		}
	case "array":
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err != nil {
			return []error{fmt.Errorf("必须是数组")}
		}
		var errs []error
		for i, item := range items {
			if p.ItemsRef != "" {
				for _, e := range validateObject(s, refName(p.ItemsRef), item) {
					errs = append(errs, fmt.Errorf("[%d]: %v", i, e))
				}
				continue
			}
			for _, e := range validateValue(s, &Property{Type: p.ItemsType}, item) {
				errs = append(errs, fmt.Errorf("[%d]: %v", i, e))
			}
		}
		return errs
	case "object":
		if p.Ref == "" {
			return []error{fmt.Errorf("object 必须用 $ref 指定定义")}
		}
		return validateObject(s, refName(p.Ref), raw)
	default:
		return []error{fmt.Errorf("不支持的 schema 类型 %q", p.Type)}
	}
	return nil
}

func refName(ref string) string {
	return strings.TrimPrefix(ref, "#/$defs/")
}

func containsString(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

func sortedStringKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
