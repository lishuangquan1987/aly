package service

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// setupCacheTestDir 创建临时项目目录与文件，返回 workDir。
func setupCacheTestDir(t *testing.T) string {
	t.Helper()
	root, err := ioutil.TempDir("", "file-cache-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })

	files := map[string]string{
		"app.exe":      "binary-content-1",
		"config/a.ini": "ini-content",
	}
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if err := ioutil.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
	}
	return root
}

// TestGetProjectFileListCachesAndInvalidates 验证缓存基本行为：
// 首次构建 → 命中缓存 → upload 失效 → 重新构建（含新文件）。
func TestGetProjectFileListCachesAndInvalidates(t *testing.T) {
	workDir := setupCacheTestDir(t)
	projectName := "proj-cache-test"
	InvalidateProjectFileList(projectName)

	// 首次：构建并缓存
	list1, err := GetProjectFileList(projectName, workDir, nil, nil)
	if err != nil {
		t.Fatalf("GetProjectFileList 首次失败: %v", err)
	}
	if len(list1) != 2 {
		t.Fatalf("首次应返回 2 个文件，实际 %d", len(list1))
	}
	if list1[0].MD5 == "" || list1[0].SHA256 == "" {
		t.Errorf("应包含 md5/sha256，实际 %+v", list1[0])
	}

	// 再次：命中缓存，结果一致（文件未变）
	list2, err := GetProjectFileList(projectName, workDir, nil, nil)
	if err != nil {
		t.Fatalf("GetProjectFileList 二次失败: %v", err)
	}
	if len(list2) != 2 || list2[0].MD5 != list1[0].MD5 {
		t.Errorf("缓存命中应返回一致结果，list1=%+v list2=%+v", list1, list2)
	}

	// 模拟 upload 新文件（不通过缓存接口）：失效后下次 get_all_files 重新缓存
	newFile := filepath.Join(workDir, "new.bin")
	if err := ioutil.WriteFile(newFile, []byte("new-content"), 0644); err != nil {
		t.Fatalf("写新文件失败: %v", err)
	}
	InvalidateProjectFileList(projectName)
	list3, err := GetProjectFileList(projectName, workDir, nil, nil)
	if err != nil {
		t.Fatalf("GetProjectFileList 失效后失败: %v", err)
	}
	if len(list3) != 3 {
		t.Errorf("失效后应重新构建出 3 个文件，实际 %d", len(list3))
	}
}

// TestGetProjectFileListIgnoreRules 验证忽略规则参与缓存内容（与 get_all_files 一致）。
func TestGetProjectFileListIgnoreRules(t *testing.T) {
	workDir := setupCacheTestDir(t)
	projectName := "proj-ignore-test"
	InvalidateProjectFileList(projectName)

	list, err := GetProjectFileList(projectName, workDir, []string{"config"}, nil)
	if err != nil {
		t.Fatalf("GetProjectFileList 失败: %v", err)
	}
	if len(list) != 1 || list[0].FileRelativePath != "app.exe" {
		t.Errorf("忽略 config 文件夹后应只剩 app.exe，实际 %+v", list)
	}
}

// TestGetProjectFileListMissingDir 验证 workDir 不存在时返回空列表（不缓存错误）。
func TestGetProjectFileListMissingDir(t *testing.T) {
	projectName := "proj-missing-test"
	InvalidateProjectFileList(projectName)

	list, err := GetProjectFileList(projectName, filepath.Join(os.TempDir(), "not-exist-"+time.Now().Format("150405.000000000")), nil, nil)
	if err != nil {
		t.Fatalf("workDir 不存在时应返回空列表而非错误: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("workDir 不存在时应返回空列表，实际 %d", len(list))
	}
}
