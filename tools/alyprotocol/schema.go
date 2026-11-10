package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Schema 是 spec/aly-protocol.schema.json 的内存表示。
// Defs 保持文件中的定义顺序，Properties 保持文件中的属性顺序 —— 生成物必须可复现。
type Schema struct {
	ProtocolVersion string
	Defs            []*Object
	byName          map[string]*Object
}

// Object 对应一个 $defs 定义（type=object）。
type Object struct {
	Name        string
	Description string
	Properties  []*Property
	Required    []string
	byName      map[string]*Property
}

// Property 对应对象的一个属性。Type 与 Ref 互斥；数组的元素类型放在 ItemsType/ItemsRef。
type Property struct {
	Name        string
	Description string
	Type        string
	Ref         string
	ItemsType   string
	ItemsRef    string
	Enum        []string
}

var (
	protocolVersionRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	propertyNameRe    = regexp.MustCompile(`^[a-z][A-Za-z0-9_]*$`)
)

// ParseSchema 解析并校验协议 schema。
func ParseSchema(raw []byte) (*Schema, error) {
	var top struct {
		Schema  string          `json:"$schema"`
		Version string          `json:"x-protocol-version"`
		Defs    json.RawMessage `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("schema 不是合法 JSON: %v", err)
	}
	if !strings.Contains(top.Schema, "draft-07") {
		return nil, fmt.Errorf("schema 必须声明 draft-07，实际为 %q", top.Schema)
	}
	if !protocolVersionRe.MatchString(top.Version) {
		return nil, fmt.Errorf("x-protocol-version 必须是 MAJOR.MINOR.PATCH，实际为 %q", top.Version)
	}

	names, rawDefs, err := decodeObject(top.Defs)
	if err != nil {
		return nil, fmt.Errorf("解析 $defs: %v", err)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("$defs 为空")
	}

	s := &Schema{ProtocolVersion: top.Version, byName: make(map[string]*Object, len(names))}
	for _, name := range names {
		obj, err := parseObject(name, rawDefs[name])
		if err != nil {
			return nil, err
		}
		s.Defs = append(s.Defs, obj)
		s.byName[name] = obj
	}
	for _, obj := range s.Defs {
		for _, p := range obj.Properties {
			if err := s.checkRef(p.Ref); err != nil {
				return nil, fmt.Errorf("%s.%s: %v", obj.Name, p.Name, err)
			}
			if err := s.checkRef(p.ItemsRef); err != nil {
				return nil, fmt.Errorf("%s.%s[]: %v", obj.Name, p.Name, err)
			}
		}
	}
	return s, nil
}

func (s *Schema) checkRef(ref string) error {
	if ref == "" {
		return nil
	}
	if !strings.HasPrefix(ref, "#/$defs/") {
		return fmt.Errorf("$ref 必须指向 #/$defs/，实际为 %q", ref)
	}
	name := strings.TrimPrefix(ref, "#/$defs/")
	if s.byName[name] == nil {
		return fmt.Errorf("$ref 指向不存在的定义 %q", name)
	}
	return nil
}

func parseObject(name string, raw json.RawMessage) (*Object, error) {
	var def struct {
		Type                 string          `json:"type"`
		Description          string          `json:"description"`
		AdditionalProperties *bool           `json:"additionalProperties"`
		Required             []string        `json:"required"`
		Properties           json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &def); err != nil {
		return nil, fmt.Errorf("解析定义 %s: %v", name, err)
	}
	if def.Type != "object" {
		return nil, fmt.Errorf("定义 %s 的 type 必须是 object，实际为 %q", name, def.Type)
	}
	if def.AdditionalProperties == nil || *def.AdditionalProperties {
		return nil, fmt.Errorf("定义 %s 必须显式设置 additionalProperties:false（严格模式用于拦截协议漂移）", name)
	}

	propNames, propRaw, err := decodeObject(def.Properties)
	if err != nil {
		return nil, fmt.Errorf("解析定义 %s 的 properties: %v", name, err)
	}

	obj := &Object{
		Name:        name,
		Description: def.Description,
		byName:      make(map[string]*Property, len(propNames)),
	}
	for _, pn := range propNames {
		p, err := parseProperty(name, pn, propRaw[pn])
		if err != nil {
			return nil, err
		}
		obj.Properties = append(obj.Properties, p)
		obj.byName[pn] = p
	}

	seen := make(map[string]bool, len(def.Required))
	for _, r := range def.Required {
		if seen[r] {
			return nil, fmt.Errorf("定义 %s 的 required 重复: %q", name, r)
		}
		seen[r] = true
		if obj.byName[r] == nil {
			return nil, fmt.Errorf("定义 %s 的 required 含不存在的属性 %q", name, r)
		}
	}
	obj.Required = def.Required
	return obj, nil
}

func parseProperty(defName, propName string, raw json.RawMessage) (*Property, error) {
	var p struct {
		Type        string   `json:"type"`
		Description string   `json:"description"`
		Ref         string   `json:"$ref"`
		Enum        []string `json:"enum"`
		Items       struct {
			Type string `json:"type"`
			Ref  string `json:"$ref"`
		} `json:"items"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("解析属性 %s.%s: %v", defName, propName, err)
	}
	if !propertyNameRe.MatchString(propName) {
		return nil, fmt.Errorf("属性名 %s.%s 必须是 lowerCamelCase 或 snake_case（小写开头）", defName, propName)
	}
	if p.Type != "" && p.Ref != "" {
		return nil, fmt.Errorf("属性 %s.%s 不能同时有 type 和 $ref", defName, propName)
	}
	switch p.Type {
	case "":
		// 无 type 的"任意值"，仅 Output.data 使用。
		if p.Ref != "" || p.Items.Type != "" || p.Items.Ref != "" {
			return nil, fmt.Errorf("属性 %s.%s 无 type 时不能有 $ref/items", defName, propName)
		}
	case "object":
		if p.Ref == "" {
			return nil, fmt.Errorf("属性 %s.%s 的 object 必须用 $ref 引用 $defs", defName, propName)
		}
	case "array":
		if (p.Items.Type == "") == (p.Items.Ref == "") {
			return nil, fmt.Errorf("属性 %s.%s 是数组，items 必须且只能声明 type 或 $ref 之一", defName, propName)
		}
	case "string", "boolean", "integer", "number":
		// ok
	default:
		return nil, fmt.Errorf("属性 %s.%s 的类型 %q 不受支持", defName, propName, p.Type)
	}
	if len(p.Enum) > 0 && p.Type != "string" {
		return nil, fmt.Errorf("属性 %s.%s 有 enum 但 type 不是 string", defName, propName)
	}

	return &Property{
		Name:        propName,
		Description: p.Description,
		Type:        p.Type,
		Ref:         p.Ref,
		ItemsType:   p.Items.Type,
		ItemsRef:    p.Items.Ref,
		Enum:        p.Enum,
	}, nil
}

// decodeObject 按文件顺序返回对象的键，并检测重复键。
func decodeObject(raw []byte) ([]string, map[string]json.RawMessage, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, nil, fmt.Errorf("期望 JSON 对象，实际为 %v", tok)
	}
	var names []string
	vals := make(map[string]json.RawMessage)
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, nil, fmt.Errorf("对象键不是字符串: %v", keyTok)
		}
		var val json.RawMessage
		if err := dec.Decode(&val); err != nil {
			return nil, nil, err
		}
		if _, dup := vals[key]; dup {
			return nil, nil, fmt.Errorf("重复的键 %q", key)
		}
		names = append(names, key)
		vals[key] = val
	}
	if _, err := dec.Token(); err != nil {
		return nil, nil, err
	}
	return names, vals, nil
}

// Def 返回指定定义；不存在时返回 nil。
func (s *Schema) Def(name string) *Object {
	return s.byName[name]
}

// DefNames 按文件顺序返回定义名。
func (s *Schema) DefNames() []string {
	names := make([]string, len(s.Defs))
	for i, d := range s.Defs {
		names[i] = d.Name
	}
	return names
}

// Property 返回指定属性；不存在时返回 nil。
func (o *Object) Property(name string) *Property {
	return o.byName[name]
}

// PropertyNames 按文件顺序返回属性名。
func (o *Object) PropertyNames() []string {
	names := make([]string, len(o.Properties))
	for i, p := range o.Properties {
		names[i] = p.Name
	}
	return names
}

// IsRequired 判断属性是否在 required 中。
func (o *Object) IsRequired(name string) bool {
	for _, r := range o.Required {
		if r == name {
			return true
		}
	}
	return false
}
