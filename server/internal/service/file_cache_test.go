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

// TestGetProjectFileListCacheHitDoesNotRecomputeHash 验证缓存核心价值：
// **命中缓存时不再重新计算 md5/sha256**。
//
// 做法：首次构建缓存后，把文件内容替换为"相同长度"的新内容，并把 mtime 恢复为原值
// （保持指纹（relPath+size+mtime）不变）——若命中缓存直接返回旧哈希，则证明没有重算；
// 再手动失效后调用，应返回新哈希，证明失效后确实重建。
func TestGetProjectFileListCacheHitDoesNotRecomputeHash(t *testing.T) {
	workDir := setupCacheTestDir(t)
	projectName := "proj-cache-no-recompute"
	InvalidateProjectFileList(projectName)

	// 首次：构建缓存
	list1, err := GetProjectFileList(projectName, workDir, nil, nil)
	if err != nil {
		t.Fatalf("首次构建失败: %v", err)
	}
	if len(list1) != 2 {
		t.Fatalf("应返回 2 个文件，实际 %d", len(list1))
	}
	// 记录各文件的旧 md5/sha256
	oldMD5 := map[string]string{}
	oldSHA256 := map[string]string{}
	for _, fi := range list1 {
		oldMD5[fi.FileRelativePath] = fi.MD5
		oldSHA256[fi.FileRelativePath] = fi.SHA256
	}

	// 篡改文件内容：相同长度（保持 size 指纹不变），并恢复 mtime
	rel := "app.exe"
	abs := filepath.Join(workDir, rel)
	before, err := os.Stat(abs)
	if err != nil {
		t.Fatalf("stat %s 失败: %v", abs, err)
	}
	// 相同长度的新内容（"binary-content-2" 与 "binary-content-1" 等长）
	newContent := []byte("binary-content-2")
	if len(newContent) != len([]byte("binary-content-1")) {
		t.Fatal("测试前提错误：新旧内容必须等长")
	}
	if err := ioutil.WriteFile(abs, newContent, 0644); err != nil {
		t.Fatalf("写新内容失败: %v", err)
	}
	// 恢复 mtime，保持指纹不变
	if err := os.Chtimes(abs, before.ModTime(), before.ModTime()); err != nil {
		t.Fatalf("恢复 mtime 失败: %v", err)
	}

	// 关键断言 1：指纹未变 → 命中缓存 → 返回旧哈希（证明没有重新计算）
	list2, err := GetProjectFileList(projectName, workDir, nil, nil)
	if err != nil {
		t.Fatalf("命中缓存失败: %v", err)
	}
	var gotMD5, gotSHA256 string
	for _, fi := range list2 {
		if fi.FileRelativePath == rel {
			gotMD5 = fi.MD5
			gotSHA256 = fi.SHA256
		}
	}
	if gotMD5 != oldMD5[rel] {
		t.Errorf("缓存命中应返回旧 md5（未重算），got=%s want=%s", gotMD5, oldMD5[rel])
	}
	if gotSHA256 != oldSHA256[rel] {
		t.Errorf("缓存命中应返回旧 sha256（未重算），got=%s want=%s", gotSHA256, oldSHA256[rel])
	}

	// 关键断言 2：手动失效后重新构建 → 返回新哈希（证明失效后确实重算）
	InvalidateProjectFileList(projectName)
	list3, err := GetProjectFileList(projectName, workDir, nil, nil)
	if err != nil {
		t.Fatalf("失效后重建失败: %v", err)
	}
	var newMD5 string
	for _, fi := range list3 {
		if fi.FileRelativePath == rel {
			newMD5 = fi.MD5
		}
	}
	if newMD5 == oldMD5[rel] {
		t.Error("失效后应重新计算哈希（新内容 -> 新 md5），实际仍返回旧 md5")
	}
}

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
