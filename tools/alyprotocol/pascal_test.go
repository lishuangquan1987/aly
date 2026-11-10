package main

import (
	"os"
	"strings"
	"testing"
)

func pascalTestConfig() *PascalConfig {
	return &PascalConfig{
		Types: map[string]PascalTypeConfig{"Item": {}, "Data": {}},
		Skip:  []string{"Output"},
	}
}

func TestRenderPascalGeneratesUnit(t *testing.T) {
	s := mustSchema(t, goTestSchema)
	file, err := RenderPascal(s, pascalTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	if file.Name != "models_gen.pas" {
		t.Fatalf("file name = %q", file.Name)
	}
	out := string(file.Content)
	for _, want := range []string{
		"unit models_gen;",
		"{$mode delphi}",
		"TItem = record",
		"TData = record",
		"status: string;",
		"count: Int64;",
		"optional_note: string;",
		"items: array of TItem;",
		"tags: array of string;",
		"function TDataFromJson(Value: TJSONObject): TData;",
		"Result.optional_note := JsonStr(Value, 'optional_note');",
		"Result.items[I] := TItemFromJson(TJSONObject(TJSONArray(Data).Items[I]));",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Index(out, "TItem = record") > strings.Index(out, "TData = record") {
		t.Errorf("TItem must be declared before TData")
	}
}

func TestRenderPascalRequiresCoverage(t *testing.T) {
	s := mustSchema(t, goTestSchema)
	cfg := &PascalConfig{Types: map[string]PascalTypeConfig{}, Skip: []string{"Output"}}
	if _, err := RenderPascal(s, cfg); err == nil {
		t.Fatal("expected coverage error")
	}
}

func TestLoadPascalConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/pascal.json"
	bad := `{"types":{"Data":{}},"skip":["Output"],"extra":1}`
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPascalConfig(path); err == nil {
		t.Fatal("expected unknown-field error")
	}
	good := `{"types":{"Data":{}},"skip":["Output"]}`
	if err := os.WriteFile(path, []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadPascalConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Types["Data"].Name != "TData" {
		t.Fatalf("default name = %q", cfg.Types["Data"].Name)
	}
}
