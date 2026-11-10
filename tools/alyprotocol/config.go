package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// Config 对应 tools/alyprotocol/csharp.json：C# 生成与控制映射。
type Config struct {
	Types      map[string]TypeConfig   `json:"types"`
	Properties map[string]PropOverride `json:"properties"`
	Go         GoSection               `json:"go"`
}

type GoSection struct {
	Structs map[string]string `json:"structs"`
}

type TypeConfig struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Visibility string `json:"visibility"`
	Partial    bool   `json:"partial"`
}

type PropOverride struct {
	Type string `json:"type"`
}

// LoadConfig 严格读取配置：未知字段、非法可见性、缺少命名空间都直接报错。
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置 %s: %v", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("解析配置 %s: %v", path, err)
	}
	var extra struct{}
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("配置 %s 含多余内容", path)
	}
	if err := cfg.normalizeAndValidate(); err != nil {
		return nil, fmt.Errorf("配置 %s: %v", path, err)
	}
	return &cfg, nil
}

func (c *Config) normalizeAndValidate() error {
	if len(c.Types) == 0 {
		return fmt.Errorf("types 不能为空")
	}
	for defName, tc := range c.Types {
		if tc.Name == "" {
			tc.Name = defName
		}
		if tc.Visibility == "" {
			tc.Visibility = "public"
		}
		if tc.Visibility != "public" && tc.Visibility != "internal" {
			return fmt.Errorf("类型 %s 的 visibility 只能是 public 或 internal，实际为 %q", defName, tc.Visibility)
		}
		if strings.TrimSpace(tc.Namespace) == "" {
			return fmt.Errorf("类型 %s 缺少 namespace", defName)
		}
		c.Types[defName] = tc
	}
	for key, ov := range c.Properties {
		if !strings.Contains(key, ".") {
			return fmt.Errorf("属性覆盖 %q 必须是 <Def>.<property> 形式", key)
		}
		if strings.TrimSpace(ov.Type) == "" {
			return fmt.Errorf("属性覆盖 %q 缺少 type", key)
		}
	}
	for defName, structName := range c.Go.Structs {
		if strings.TrimSpace(defName) == "" || strings.TrimSpace(structName) == "" {
			return fmt.Errorf("go.structs 含空映射")
		}
	}
	return nil
}
