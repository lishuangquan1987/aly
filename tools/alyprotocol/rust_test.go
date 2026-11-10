package main

import (
	"os"
	"strings"
	"testing"
)

func rustTestConfig() *RustConfig {
	return &RustConfig{
		Types: map[string]RustTypeConfig{"Item": {}, "Data": {}},
		Skip:  []string{"Output"},
	}
}

func TestRenderRustGeneratesStructs(t *testing.T) {
	s := mustSchema(t, goTestSchema)
	file, err := RenderRust(s, rustTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "models_gen.rs" {
		t.Fatalf("file name = %q", file.Name)
	}
	out := string(file.Content)
	for _, want := range []string{
		"use serde::Deserialize;",
		"#[derive(Debug, Clone, PartialEq, Deserialize)]",
		"pub struct Data {",
		"pub status: String,",
		"pub count: i64,",
		"#[serde(default)]",
		"pub optional_note: Option<String>,",
		"pub items: Vec<Item>,",
		"pub tags: Option<Vec<String>>,",
		"impl Data {",
		`pub const STATUS_BEGIN: &'static str = "BEGIN";`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderRustRequiresCoverage(t *testing.T) {
	s := mustSchema(t, goTestSchema)
	cfg := &RustConfig{Types: map[string]RustTypeConfig{}, Skip: []string{"Output"}}
	if _, err := RenderRust(s, cfg); err == nil {
		t.Fatal("expected coverage error")
	}
}

func TestLoadRustConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/rust.json"
	bad := `{"types":{"Data":{}},"skip":["Output"],"extra":1}`
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRustConfig(path); err == nil {
		t.Fatal("expected unknown-field error")
	}
	good := `{"types":{"Data":{}},"skip":["Output"]}`
	if err := os.WriteFile(path, []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadRustConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Types["Data"].Name != "Data" {
		t.Fatalf("default name = %q", cfg.Types["Data"].Name)
	}
}
