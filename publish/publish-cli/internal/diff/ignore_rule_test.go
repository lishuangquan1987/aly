package diff

// ignore 规则的边界测试（审查 M19）。
//
// 旧实现的三处边界缺陷（本测试即回归锚）：
//   1. 目录规则只在根级生效："build" 匹配不到 "a/build"；
//   2. Windows 下大小写敏感："Build" 匹配不到实际 "build"；
//   3. 规则末尾带 "/" 时 HasPrefix 恒 false，静默完全失效。
// 另有跨平台不一致：filepath.Match + ToSlash 路径在 Windows 上 `*` 会跨目录。

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatchDirAnyLevel(t *testing.T) {
	cases := []struct {
		relPath string
		rule    string
		want    bool
	}{
		// 根级（旧实现也能过）
		{"build", "build", true},
		{"build/x.txt", "build", true},
		// 任意层级：旧实现失效，修复后必须命中
		{"a/build", "build", true},
		{"a/build/x.txt", "build", true},
		{"a/b/build", "build", true},
		{"a/b/c/build/x.txt", "build", true},
		// 前缀相似但不同名，不得误伤
		{"buildtools", "build", false},
		{"a/buildtools/x.txt", "build", false},
		{"a/mybuilder", "build", false},
		// 多段规则
		{"a/src/generated/x.go", "src/generated", true},
		{"generated", "src/generated", false},
		// 尾斜杠规则：旧实现静默失效
		{"build", "build/", true},
		{"a/build/x", "build/", true},
		// 前导 "./"
		{"build/x", "./build", true},
	}
	for _, c := range cases {
		if got := matchDir(c.relPath, c.rule); got != c.want {
			t.Fatalf("matchDir(%q, %q) = %v, want %v", c.relPath, c.rule, got, c.want)
		}
	}
}

func TestMatchDirCaseInsensitive(t *testing.T) {
	cases := [][3]string{
		{"build", "Build"},
		{"a/BUILD/x", "build"},
		{"Node_Modules", "node_modules"},
		{"a/Dist/x.js", "dist"},
	}
	for _, c := range cases {
		if !matchDir(c[0], c[1]) {
			t.Fatalf("matchDir(%q,%q) 应命中（大小写不敏感）", c[0], c[1])
		}
	}
}

func TestMatchFileDoesNotCrossDirectories(t *testing.T) {
	// 审查 M19 核心：`a*` 不得跨目录匹配整条路径
	cases := []struct {
		relPath string
		pattern string
		want    bool
	}{
		{"a/b/c", "a*", false},      // 旧实现 Windows 下为 true（跨目录）
		{"abc.txt", "a*", true},     // 同层应命中
		{"dir/abc.txt", "abc.txt", true}, // 文件名匹配
		{"x/y.log", "*.log", true},
		{"x/y.log", "y.log", true},
		{"x/y.log", "*.txt", false},
		{"a/b/c.txt", "a/*", false}, // `*` 只跨单段
		{"a/b.txt", "a/*", true},
		{"a.txt", "a.txt", true},
		{"A.TXT", "a.txt", true}, // Windows 语义：大小写不敏感
	}
	for _, c := range cases {
		if got := matchFile(c.relPath, c.pattern); got != c.want {
			t.Fatalf("matchFile(%q, %q) = %v, want %v", c.relPath, c.pattern, got, c.want)
		}
	}
}

func TestMatchFileInvalidGlobDoesNotPanic(t *testing.T) {
	// 未闭合的 "[" 属于非法 glob：提示并返回 false，绝不能 panic
	if matchFile("a.txt", "[abc") {
		t.Fatal("非法 glob 不应匹配")
	}
}

func TestScanDirectoryHonoursNestedIgnoreFolder(t *testing.T) {
	root := t.TempDir()
	// a/build/ignored.txt 必须被目录规则 "build" 排除
	for rel, content := range map[string]string{
		"a/build/ignored.txt": "x",
		"keep.txt":            "y",
	} {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	files, err := ScanDirectory(root, []string{"build"}, nil)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	for _, f := range files {
		if f.RelativePath == "a/build/ignored.txt" {
			t.Fatal("嵌套的 build 目录未被忽略（M19 回归）")
		}
	}
	found := false
	for _, f := range files {
		if f.RelativePath == "keep.txt" {
			found = true
		}
	}
	if !found {
		t.Fatal("非忽略文件被误删/漏扫")
	}
}
