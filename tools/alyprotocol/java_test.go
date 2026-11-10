package main

import (
	"os"
	"strings"
	"testing"
)

func javaTestConfig() *JavaConfig {
	return &JavaConfig{
		Types: map[string]JavaTypeConfig{"Item": {}, "Data": {}},
		Skip:  []string{"Output"},
	}
}

func TestRenderJavaGeneratesClasses(t *testing.T) {
	s := mustSchema(t, goTestSchema)
	files, err := RenderJava(s, javaTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files[0].Name != "Item.java" || files[1].Name != "Data.java" {
		t.Fatalf("files = %v", fileNames(files))
	}
	out := string(files[1].Content)
	for _, want := range []string{
		"package io.github.lishuangquan1987.alyclient.models;",
		"public final class Data {",
		"public String status;",
		"public long count;",
		"public String optionalNote;",
		"public List<Item> items;",
		"public static final String STATUS_BEGIN = \"BEGIN\";",
		"public static Data fromMap(Map<String, Object> data)",
		"value.count = Json.lng(data, \"count\", 0L);",
		"value.optionalNote = Json.optStr(data, \"optional_note\");",
		"Json.mapList(data, \"items\", Item::fromMap)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestJavaOptionalScalarsAreBoxed(t *testing.T) {
	s := mustSchema(t, goTestSchema)
	typ, err := javaType(s, javaTestConfig(), &Property{Name: "flag", Type: "boolean"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if typ != "Boolean" {
		t.Fatalf("optional boolean type = %q", typ)
	}
}

func TestRenderJavaRequiresCoverage(t *testing.T) {
	s := mustSchema(t, goTestSchema)
	cfg := &JavaConfig{Types: map[string]JavaTypeConfig{}, Skip: []string{"Output"}}
	if _, err := RenderJava(s, cfg); err == nil {
		t.Fatal("expected coverage error")
	}
}

func TestLoadJavaConfig(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/java.json"
	bad := `{"types":{"Data":{}},"skip":["Output"],"extra":1}`
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadJavaConfig(path); err == nil {
		t.Fatal("expected unknown-field error")
	}
	good := `{"types":{"Data":{}},"skip":["Output"]}`
	if err := os.WriteFile(path, []byte(good), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadJavaConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Types["Data"].Name != "Data" {
		t.Fatalf("default name = %q", cfg.Types["Data"].Name)
	}
}
