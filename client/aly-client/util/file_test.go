package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"aly/client/aly-client/config"
)

// buildTestTree 创建如下测试目录结构并返回根路径：
//
//	a.txt
//	sub/b.txt
//	logs/c.log
//	keep/d.txt
func buildTestTree(t *testing.T) string {
	t.Helper()
	root, err := ioutil.TempDir("", "copy-exclude-src")
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
	write("sub/b.txt")
	write("logs/c.log")
	write("keep/d.txt")
	return root
}

// assertExists 断言文件存在
func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("期望文件存在: %s (%v)", path, err)
	}
}

// assertMissing 断言文件不存在
func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("期望文件不存在: %s", path)
	}
}

// TestCopyDirWithExcludeUnCopyFolders 验证"不复制文件夹"（un_copy_folders）：
// 复制时整目录跳过，目录内文件不进入目标目录。
func TestCopyDirWithExcludeUnCopyFolders(t *testing.T) {
	src := buildTestTree(t)
	defer os.RemoveAll(src)
	dst, err := ioutil.TempDir("", "copy-exclude-dst")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dst)

	// 模拟 apply_update：不复制 logs 文件夹
	unCopyFolders := []string{"logs"}
	err = CopyDirWithExclude(src, dst,
		func(relPath string) bool { return config.ShouldSkipFile(relPath, nil) },
		func(relPath string) bool { return config.ShouldSkipFolder(relPath, unCopyFolders) },
	)
	if err != nil {
		t.Fatalf("CopyDirWithExclude 失败: %v", err)
	}

	assertExists(t, filepath.Join(dst, "a.txt"))
	assertExists(t, filepath.Join(dst, "sub", "b.txt"))
	assertExists(t, filepath.Join(dst, "keep", "d.txt"))
	assertMissing(t, filepath.Join(dst, "logs", "c.log"))
}

// TestCopyDirWithExcludeUnCopyFiles 验证"不复制文件"（un_copy_files）：
// 仅跳过指定文件，其余文件正常复制。
func TestCopyDirWithExcludeUnCopyFiles(t *testing.T) {
	src := buildTestTree(t)
	defer os.RemoveAll(src)
	dst, err := ioutil.TempDir("", "copy-exclude-dst")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dst)

	// 模拟 apply_update：不复制 c.log
	unCopyFiles := []string{"c.log"}
	err = CopyDirWithExclude(src, dst,
		func(relPath string) bool { return config.ShouldSkipFile(relPath, unCopyFiles) },
		func(relPath string) bool { return config.ShouldSkipFolder(relPath, nil) },
	)
	if err != nil {
		t.Fatalf("CopyDirWithExclude 失败: %v", err)
	}

	assertExists(t, filepath.Join(dst, "a.txt"))
	assertExists(t, filepath.Join(dst, "sub", "b.txt"))
	assertExists(t, filepath.Join(dst, "keep", "d.txt"))
	assertMissing(t, filepath.Join(dst, "logs", "c.log"))
}

// TestCopyDirWithExcludeNothingSkipped 验证未配置不复制规则时全部复制。
func TestCopyDirWithExcludeNothingSkipped(t *testing.T) {
	src := buildTestTree(t)
	defer os.RemoveAll(src)
	dst, err := ioutil.TempDir("", "copy-exclude-dst")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dst)

	err = CopyDirWithExclude(src, dst, nil, nil)
	if err != nil {
		t.Fatalf("CopyDirWithExclude 失败: %v", err)
	}

	assertExists(t, filepath.Join(dst, "a.txt"))
	assertExists(t, filepath.Join(dst, "sub", "b.txt"))
	assertExists(t, filepath.Join(dst, "logs", "c.log"))
	assertExists(t, filepath.Join(dst, "keep", "d.txt"))
}
