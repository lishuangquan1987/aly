package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
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

// buildManifestTree 创建如下测试目录结构并返回根路径：
//
//	a.txt            ← 删除集内（将被删）
//	sub/b.txt        ← 删除集外（保留）
//	sub/old.txt      ← 删除集内（将被删）
//	keep/d.txt       ← 删除集外（保留）
//	runtime/cfg.ini  ← 删除集外（保留，模拟 materialize 拷入的运行时数据）
func buildManifestTree(t *testing.T) string {
	t.Helper()
	root, err := ioutil.TempDir("", "manifest-clean")
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
	write("sub/old.txt")
	write("keep/d.txt")
	write("runtime/cfg.ini")
	return root
}

// TestRemoveFilesByRelPathsDeleteSet 验证差量删除只删集合内的文件，
// 集合外文件（含运行时数据）一律保留。
func TestRemoveFilesByRelPathsDeleteSet(t *testing.T) {
	root := buildManifestTree(t)
	defer os.RemoveAll(root)

	deleteSet := map[string]bool{
		"a.txt":       true,
		"sub/old.txt": true,
	}
	removed, err := RemoveFilesByRelPaths(root, deleteSet)
	if err != nil {
		t.Fatalf("RemoveFilesByRelPaths 失败: %v", err)
	}
	sort.Strings(removed)
	if !reflect.DeepEqual(removed, []string{"a.txt", "sub/old.txt"}) {
		t.Errorf("removed = %v, 期望 [a.txt sub/old.txt]", removed)
	}

	assertMissing(t, filepath.Join(root, "a.txt"))
	assertMissing(t, filepath.Join(root, "sub", "old.txt"))
	assertExists(t, filepath.Join(root, "sub", "b.txt"))
	assertExists(t, filepath.Join(root, "keep", "d.txt"))
	assertExists(t, filepath.Join(root, "runtime", "cfg.ini"))
}

// TestRemoveFilesByRelPathsEmptyDir 验证删除后变空的目录被清理。
func TestRemoveFilesByRelPathsEmptyDir(t *testing.T) {
	root, err := ioutil.TempDir("", "manifest-empty-dir")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)
	full := filepath.Join(root, "gone", "x.txt")
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}
	if err := ioutil.WriteFile(full, []byte("x"), 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}
	if _, err := RemoveFilesByRelPaths(root, map[string]bool{"gone/x.txt": true}); err != nil {
		t.Fatalf("RemoveFilesByRelPaths 失败: %v", err)
	}
	assertMissing(t, filepath.Join(root, "gone"))
}

// TestRemoveFilesByRelPathsNilNoop 验证 nil / 空集合时不做任何删除。
func TestRemoveFilesByRelPathsNilNoop(t *testing.T) {
	root := buildManifestTree(t)
	defer os.RemoveAll(root)

	for _, set := range []map[string]bool{nil, {}} {
		removed, err := RemoveFilesByRelPaths(root, set)
		if err != nil {
			t.Fatalf("RemoveFilesByRelPaths(%v) 失败: %v", set, err)
		}
		if len(removed) != 0 {
			t.Errorf("nil/空集合不应删除任何文件, removed = %v", removed)
		}
	}
	assertExists(t, filepath.Join(root, "a.txt"))
	assertExists(t, filepath.Join(root, "sub", "b.txt"))
}

// TestRemoveFilesByRelPathsMissingFile 验证删除集中不存在的文件被静默忽略。
func TestRemoveFilesByRelPathsMissingFile(t *testing.T) {
	root := buildManifestTree(t)
	defer os.RemoveAll(root)

	removed, err := RemoveFilesByRelPaths(root, map[string]bool{
		"a.txt":    true,
		"nope.txt": true,
		"sub/gone": true,
	})
	if err != nil {
		t.Fatalf("RemoveFilesByRelPaths 失败: %v", err)
	}
	if len(removed) != 1 || removed[0] != "a.txt" {
		t.Errorf("removed = %v, 期望 [a.txt]", removed)
	}
}

// TestCachedManifestRoundTrip 验证清单缓存写读往返与 .tmp 不残留。
func TestCachedManifestRoundTrip(t *testing.T) {
	dir, err := ioutil.TempDir("", "manifest-cache")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)

	files := []string{"a.txt", "sub/b.txt", "sub/old.txt"}
	if err := WriteCachedManifest(dir, files); err != nil {
		t.Fatalf("WriteCachedManifest 失败: %v", err)
	}
	got, err := ReadCachedManifest(dir)
	if err != nil {
		t.Fatalf("ReadCachedManifest 失败: %v", err)
	}
	want := map[string]bool{"a.txt": true, "sub/b.txt": true, "sub/old.txt": true}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("读出清单 = %v, 期望 %v", got, want)
	}
	assertMissing(t, filepath.Join(dir, cachedManifestFile+".tmp"))
}

// TestCachedManifestRoundTripOverwrite 验证缓存覆盖写（原子替换旧文件）。
func TestCachedManifestRoundTripOverwrite(t *testing.T) {
	dir, err := ioutil.TempDir("", "manifest-cache")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)

	if err := WriteCachedManifest(dir, []string{"a.txt", "old.txt"}); err != nil {
		t.Fatalf("首次 WriteCachedManifest 失败: %v", err)
	}
	if err := WriteCachedManifest(dir, []string{"a.txt", "new.txt"}); err != nil {
		t.Fatalf("覆盖 WriteCachedManifest 失败: %v", err)
	}
	got, err := ReadCachedManifest(dir)
	if err != nil {
		t.Fatalf("ReadCachedManifest 失败: %v", err)
	}
	if got["old.txt"] {
		t.Errorf("覆盖写后 old.txt 仍在缓存中: %v", got)
	}
	if !got["new.txt"] {
		t.Errorf("覆盖写后 new.txt 缺失: %v", got)
	}
}

// TestReadCachedManifestNotExist 验证文件不存在返回 (nil, nil)。
func TestReadCachedManifestNotExist(t *testing.T) {
	dir, err := ioutil.TempDir("", "manifest-cache")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)

	got, err := ReadCachedManifest(dir)
	if err != nil {
		t.Fatalf("不存在时应返回 (nil, nil), got err: %v", err)
	}
	if got != nil {
		t.Errorf("不存在时应返回 nil, got %v", got)
	}
}

// TestReadCachedManifestCorrupt 验证损坏文件返回 error
// （宁可禁用删除同步也不用坏数据做差量）。
func TestReadCachedManifestCorrupt(t *testing.T) {
	dir, err := ioutil.TempDir("", "manifest-cache")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)

	if err := ioutil.WriteFile(filepath.Join(dir, cachedManifestFile),
		[]byte("{broken json"), 0644); err != nil {
		t.Fatalf("写入损坏文件失败: %v", err)
	}
	if _, err := ReadCachedManifest(dir); err == nil {
		t.Errorf("损坏清单应返回 error")
	}
}

// TestClearCachedManifest 验证清理：存在则删、不存在不报错。
func TestClearCachedManifest(t *testing.T) {
	dir, err := ioutil.TempDir("", "manifest-cache")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)

	ClearCachedManifest(dir) // 不存在：不应报错
	if err := WriteCachedManifest(dir, []string{"a.txt"}); err != nil {
		t.Fatalf("WriteCachedManifest 失败: %v", err)
	}
	ClearCachedManifest(dir)
	assertMissing(t, filepath.Join(dir, cachedManifestFile))
}

// TestDeleteSyncDiffScenario 端到端差量语义验证：
// 旧清单 {a,b,c} → 新清单 {a,b,d} ⇒ 差量删除集 = {c}。
// 验证"服务端删了才删"——不在任何清单中的运行时文件不受影响。
func TestDeleteSyncDiffScenario(t *testing.T) {
	root := buildManifestTree(t)
	defer os.RemoveAll(root)

	oldManifest := []string{"a.txt", "sub/b.txt", "sub/old.txt", "keep/d.txt"}
	newManifest := []string{"a.txt", "sub/b.txt", "keep/d.txt", "runtime/cfg.ini"}

	oldSet := map[string]bool{}
	for _, rel := range oldManifest {
		oldSet[rel] = true
	}
	newSet := map[string]bool{}
	for _, rel := range newManifest {
		newSet[rel] = true
	}
	deleteSet := map[string]bool{}
	for rel := range oldSet {
		if !newSet[rel] {
			deleteSet[rel] = true
		}
	}
	if len(deleteSet) != 1 || !deleteSet["sub/old.txt"] {
		t.Fatalf("差量计算错误: %v, 期望仅 [sub/old.txt]", deleteSet)
	}

	removed, err := RemoveFilesByRelPaths(root, deleteSet)
	if err != nil {
		t.Fatalf("RemoveFilesByRelPaths 失败: %v", err)
	}
	if len(removed) != 1 || removed[0] != "sub/old.txt" {
		t.Errorf("removed = %v, 期望 [sub/old.txt]", removed)
	}
	assertMissing(t, filepath.Join(root, "sub", "old.txt"))
	assertExists(t, filepath.Join(root, "runtime", "cfg.ini"))
}
