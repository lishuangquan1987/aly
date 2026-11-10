package main

import (
	"os"
	"strings"
	"testing"
)

func cppTestConfig() *CppConfig {
	return &CppConfig{
		Types: map[string]CppTypeConfig{"Item": {}, "Data": {}},
		Skip:  []string{"Output"},
	}
}

func TestRenderCppGeneratesHeader(t *testing.T) {
	s := mustSchema(t, goTestSchema)
	file, err := RenderCpp(s, cppTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "models_gen.h" {
		t.Fatalf("file name = %q", file.Name)
	}
	out := string(file.Content)
	for _, want := range []string{
		"#pragma once",
		"namespace aly {",
		"struct Item {",
		"struct Data {",
		"std::string status;",
		"long long count;",
		"Optional<std::string> optional_note;",
		"std::vector<Item> items;",
		"Optional<std::vector<std::string>> tags;",
		"jsonOptString(value, \"optional_note\")",
		"jsonObjectList< Item >(value, \"items\", &ItemFromJson)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Index(out, "struct Item {") > strings.Index(out, "struct Data {") {
		t.Errorf("Item must be declared before Data")
	}
}

func TestCppDependencyOrder(t *testing.T) {
	s := mustSchema(t, `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "x-protocol-version": "1.0.0",
  "$defs": {
    "Holder": {"type": "object", "additionalProperties": false, "properties": {"items": {"type": "array", "items": {"$ref": "#/$defs/Item"}}}, "required": ["items"]},
    "Item": {"type": "object", "additionalProperties": false, "properties": {"id": {"type": "string"}}, "required": ["id"]},
    "Output": {"type": "object", "additionalProperties": false, "properties": {}}
  }
}`)
	cfg := &CppConfig{Types: map[string]CppTypeConfig{"Holder": {}, "Item": {}}, Skip: []string{"Output"}}
	file, err := RenderCpp(s, cfg)
	if err != nil {
		t.Fatal(err)
	}
	out := string(file.Content)
	if strings.Index(out, "struct Item {") > strings.Index(out, "struct Holder {") {
		t.Errorf("dependency order wrong:\n%s", out)
	}
}

func TestRenderCppRequiresCoverage(t *testing.T) {
	s := mustSchema(t, goTestSchema)
	cfg := &CppConfig{Types: map[string]CppTypeConfig{}, Skip: []string{"Output"}}
	if _, err := RenderCpp(s, cfg); err == nil {
		t.Fatal("expected coverage error")
	}
}

func TestLoadCppConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/cpp.json"
	bad := `{"types":{"Data":{}},"skip":["Output"],"extra":1}`
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadCppConfig(path); err == nil {
		t.Fatal("expected unknown-field error")
	}
	good := `{"types":{"Data":{}},"skip":["Output"]}`
	if err := os.WriteFile(path, []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadCppConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Types["Data"].Name != "Data" {
		t.Fatalf("default name = %q", cfg.Types["Data"].Name)
	}
}
