package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// TestRemoveFilesNotInManifest 验证 #4 删除同步的清单外文件清理：
//   - 清单内文件保留；
//   - 清单外文件删除；
//   - 因此变空的子目录被清理；
//   - 非空父目录保留；
//   - manifest 为 nil 时不做任何删除（清单缺失的保守行为）。
func TestRemoveFilesNotInManifest(t *testing.T) {
	root, err := ioutil.TempDir("", "manifest-clean-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	// 构造目录树：
	//   keep.exe            （清单内 → 保留）
	//   sub/keep.dll        （清单内 → 保留，sub 非空）
	//   sub/deleted.ini     （清单外 → 删除）
	//   old/legacy.dat      （清单外 → 删除，old 变空 → 目录清理）
	//   keep/alone.txt      （清单内 → 保留，keep 目录不删）
	mustWrite := func(rel, content string) {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
		if err := ioutil.WriteFile(abs, []byte(content), 0644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
	}
	mustWrite("keep.exe", "k")
	mustWrite("sub/keep.dll", "k")
	mustWrite("sub/deleted.ini", "d")
	mustWrite("old/legacy.dat", "d")
	mustWrite("keep/alone.txt", "k")

	manifest := map[string]bool{
		"keep.exe":       true,
		"sub/keep.dll":   true,
		"keep/alone.txt": true,
	}

	removed, err := RemoveFilesNotInManifest(root, manifest)
	if err != nil {
		t.Fatalf("RemoveFilesNotInManifest 失败: %v", err)
	}
	if len(removed) != 2 {
		t.Fatalf("应删除 2 个文件，实际 %d: %v", len(removed), removed)
	}
	removedSet := map[string]bool{}
	for _, r := range removed {
		removedSet[r] = true
	}
	if !removedSet["sub/deleted.ini"] || !removedSet["old/legacy.dat"] {
		t.Errorf("删除清单不符: %v", removed)
	}

	// 清单内文件应全部保留
	for _, keep := range []string{"keep.exe", "sub/keep.dll", "keep/alone.txt"} {
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(keep))); statErr != nil {
			t.Errorf("清单内文件 %s 不应被删除: %v", keep, statErr)
		}
	}
	// 清单外文件应已删除
	for _, gone := range []string{"sub/deleted.ini", "old/legacy.dat"} {
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(gone))); !os.IsNotExist(statErr) {
			t.Errorf("清单外文件 %s 应被删除", gone)
		}
	}
	// old 目录变空应被清理；sub/keep 非空应保留
	if _, statErr := os.Stat(filepath.Join(root, "old")); !os.IsNotExist(statErr) {
		t.Errorf("空目录 old 应被清理")
	}
	if _, statErr := os.Stat(filepath.Join(root, "sub")); statErr != nil {
		t.Errorf("非空目录 sub 不应被清理: %v", statErr)
	}

	// nil 清单：不删除任何东西
	root2, err := ioutil.TempDir("", "manifest-nil-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root2)
	mustWrite2 := func(rel string) {
		abs := filepath.Join(root2, rel)
		if err := ioutil.WriteFile(abs, []byte("x"), 0644); err != nil {
			t.Fatalf("写文件失败: %v", err)
		}
	}
	mustWrite2("a.txt")
	mustWrite2("b.txt")
	removed2, err := RemoveFilesNotInManifest(root2, nil)
	if err != nil {
		t.Fatalf("nil manifest 不应报错: %v", err)
	}
	if len(removed2) != 0 {
		t.Errorf("nil manifest 不应删除任何文件，实际 %v", removed2)
	}
	if _, statErr := os.Stat(filepath.Join(root2, "a.txt")); statErr != nil {
		t.Errorf("nil manifest 时文件不应被删除: %v", statErr)
	}
}
