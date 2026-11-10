package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepoSchemaMatchesGeneratedModels 对真实仓库跑完整流程：
// schema ↔ dto.go 漂移校验 + Models/Generated 与生成器输出逐字节比对。
func TestRepoSchemaMatchesGeneratedModels(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, schemaRelPath)); err != nil {
		t.Fatalf("仓库根推断错误（%s）: %v", root, err)
	}
	if err := process(root, true); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCoverageRejectsUnmappedDefinition(t *testing.T) {
	s := mustSchema(t, renderTestSchema)
	cfg := &Config{Go: GoSection{Structs: map[string]string{"Output": "Output"}}}
	err := validateCoverage(s, cfg)
	if err == nil {
		t.Fatal("expected coverage error")
	}
	if !strings.Contains(err.Error(), "DownloadProgress") {
		t.Fatalf("error should name the uncovered definition: %v", err)
	}
}
