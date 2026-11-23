package diff

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"aly/publish-cli/pkg/models"
)

// buildScanTree 创建如下测试目录结构并返回根路径：
//
//	a.txt
//	node_modules/x.js
//	src/main.go
//	logs/app.log
//	.updator/shared.json
func buildScanTree(t *testing.T) string {
	t.Helper()
	root, err := ioutil.TempDir("", "scan-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	write := func(rel string) {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("创建目录 %s 失败: %v", filepath.Dir(full), err)
		}
		if err := ioutil.WriteFile(full, []byte(rel), 0644); err != nil {
			t.Fatalf("写入文件 %s 失败: %v", full, err)
		}
	}
	write("a.txt")
	write("node_modules/x.js")
	write("src/main.go")
	write("logs/app.log")
	write(".updator/shared.json")
	return root
}

// scanRelSet 扫描目录并返回相对路径集合
func scanRelSet(t *testing.T, root string, ignoreFolders, ignoreFiles []string) map[string]bool {
	t.Helper()
	files, err := ScanDirectory(root, ignoreFolders, ignoreFiles)
	if err != nil {
		t.Fatalf("ScanDirectory 失败: %v", err)
	}
	set := make(map[string]bool)
	for _, f := range files {
		set[f.RelativePath] = true
	}
	return set
}

// TestScanDirectoryIgnoreFolders 验证"忽略文件夹"（ignore_folders）功能：
// 被忽略的文件夹及其子文件不进入文件列表。
func TestScanDirectoryIgnoreFolders(t *testing.T) {
	root := buildScanTree(t)
	defer os.RemoveAll(root)

	set := scanRelSet(t, root, []string{"node_modules", "logs"}, nil)

	assertHas := func(rel string) {
		if !set[rel] {
			t.Errorf("期望文件列表包含 %s", rel)
		}
	}
	assertMissing := func(rel string) {
		if set[rel] {
			t.Errorf("文件列表不应包含被忽略的 %s", rel)
		}
	}
	assertHas("a.txt")
	assertHas("src/main.go")
	assertHas(".updator/shared.json")
	assertMissing("node_modules/x.js")
	assertMissing("logs/app.log")
}

// TestScanDirectoryIgnoreFolderNested 验证忽略文件夹支持子路径前缀：
// 忽略 src 时，src 下所有文件被跳过。
func TestScanDirectoryIgnoreFolderNested(t *testing.T) {
	root := buildScanTree(t)
	defer os.RemoveAll(root)

	set := scanRelSet(t, root, []string{"src"}, nil)

	if set["src/main.go"] {
		t.Error("src/main.go 应被忽略（忽略文件夹 src）")
	}
	if !set["a.txt"] {
		t.Error("期望文件列表包含 a.txt")
	}
}

// TestScanDirectoryIgnoreFiles 验证"忽略文件"（ignore_files）功能：
// 支持 *.ext 后缀通配与精确文件名。
func TestScanDirectoryIgnoreFiles(t *testing.T) {
	root := buildScanTree(t)
	defer os.RemoveAll(root)

	set := scanRelSet(t, root, nil, []string{"*.log"})

	if set["logs/app.log"] {
		t.Error("logs/app.log 应被忽略（*.log）")
	}
	if !set["a.txt"] {
		t.Error("期望文件列表包含 a.txt")
	}
	if !set["node_modules/x.js"] {
		t.Error("期望文件列表包含 node_modules/x.js")
	}
}

// TestScanDirectoryNoIgnore 验证未配置忽略规则时文件全部收录。
func TestScanDirectoryNoIgnore(t *testing.T) {
	root := buildScanTree(t)
	defer os.RemoveAll(root)

	set := scanRelSet(t, root, nil, nil)

	for _, rel := range []string{"a.txt", "node_modules/x.js", "src/main.go", "logs/app.log", ".updator/shared.json"} {
		if !set[rel] {
			t.Errorf("期望文件列表包含 %s", rel)
		}
	}
}

// TestDiffKeepsServerOnlyFiles is the regression for removing delete sync:
// a file that exists only on the server must not become a difference (it used
// to be "deleted", which made the publisher delete it from the server).
func TestDiffKeepsServerOnlyFiles(t *testing.T) {
	local := []models.LocalFile{
		{RelativePath: "keep.txt", MD5: "aaa", Size: 3},
		{RelativePath: "changed.txt", MD5: "new", Size: 3},
	}
	server := []models.FileInfo{
		{FileRelativePath: "keep.txt", MD5: "aaa", FileSize: 3},
		{FileRelativePath: "changed.txt", MD5: "old", FileSize: 3},
		{FileRelativePath: "server-only.dll", MD5: "bbb", FileSize: 5},
	}
	diffs := Diff(local, server)
	if len(diffs) != 2 {
		t.Fatalf("diff 返回 %d 项，服务端独有文件不应出现: %+v", len(diffs), diffs)
	}
	if diffs[0].RelativePath != "changed.txt" || diffs[0].Status != "modified" {
		t.Fatalf("第一项应为 changed.txt/modified: %+v", diffs[0])
	}
	if diffs[1].RelativePath != "keep.txt" || diffs[1].Status != "unchanged" {
		t.Fatalf("第二项应为 keep.txt/unchanged: %+v", diffs[1])
	}
}
