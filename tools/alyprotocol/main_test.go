package main

import (
	"os"
	"path/filepath"
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