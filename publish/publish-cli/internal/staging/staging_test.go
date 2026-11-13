package staging

// 暂存区修复的回归测试（审查 M21 / M20 相邻 / B6 / B7）。

import (
	"os"
	"path/filepath"
	"testing"

	"aly/publish-cli/pkg/models"
)

func TestNormalizeRelPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a/b.txt", "a/b.txt"},
		{`a\b.txt`, "a/b.txt"},           // 反斜杠
		{"./a/b.txt", "a/b.txt"},         // 前导 ./
		{"a//b.txt", "a/b.txt"},          // 重复分隔符
		{"a/./b.txt", "a/b.txt"},
		{"  a/b.txt  ", "a/b.txt"},       // 首尾空白
		{"a/../b.txt", "b.txt"},          // Clean 后合法
		{"../evil.txt", ""},              // 逃逸必须拒绝
		{"..", ""},
		{".", ""},
		{"", ""},
		{`a\..\..\evil.txt`, ""},
	}
	for _, c := range cases {
		if got := normalizeRelPath(c.in); got != c.want {
			t.Fatalf("normalizeRelPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestAddWithStatusNormalizes 审查 M21：同一文件的多种写法不能既 staged 又 unstaged
func TestAddWithStatusNormalizes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "f.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	// 用反斜杠 + ./ 的写法添加
	if err := AddWithStatus(root, []string{`sub\f.txt`}, nil); err != nil {
		t.Fatalf("add: %v", err)
	}
	// 再用正斜杠写法添加：必须识别为已存在，不得重复
	if err := AddWithStatus(root, []string{"./sub/f.txt"}, nil); err != nil {
		t.Fatalf("add2: %v", err)
	}
	files, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("同一文件被暂存了 %d 次（应为 1 次）: %+v", len(files), files)
	}
	if files[0].RelativePath != "sub/f.txt" {
		t.Fatalf("暂存路径未归一化: %q", files[0].RelativePath)
	}
}

// TestAddWithStatusAcceptsDeleted 审查 B6：删除的文件必须能进暂存区
// （旧实现会去 HashFile 已不存在的文件而直接报错）
func TestAddWithStatusAcceptsDeleted(t *testing.T) {
	root := t.TempDir()
	err := AddWithStatus(root, []string{"gone/old.txt"}, map[string]string{"gone/old.txt": "deleted"})
	if err != nil {
		t.Fatalf("deleted 文件不应导致 add 失败: %v", err)
	}
	files, err := Load(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(files) != 1 || files[0].Status != "deleted" {
		t.Fatalf("deleted 文件未正确进入暂存区: %+v", files)
	}
}

// TestVerifySkipsDeleted 审查 B6：Verify 不得对已删除文件重新计算哈希
func TestVerifySkipsDeleted(t *testing.T) {
	root := t.TempDir()
	if err := AddWithStatus(root, []string{"gone.txt"}, map[string]string{"gone.txt": "deleted"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	conflicts, err := Verify(root)
	if err != nil {
		t.Fatalf("verify 不应因 deleted 文件报错: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("deleted 文件不该出现在冲突列表: %v", conflicts)
	}
}

func saveStaged(t *testing.T, root string, files []models.FileStatusItem) {
	t.Helper()
	if err := Save(root, files); err != nil {
		t.Fatalf("save: %v", err)
	}
}

// TestSyncRequiresManifest 审查 B7：已上传但未进发布清单的文件必须保留
func TestSyncRequiresManifest(t *testing.T) {
	root := t.TempDir()
	saveStaged(t, root, []models.FileStatusItem{
		{RelativePath: "a.txt", Status: "modified", LocalMd5: "AAA"},
		{RelativePath: "b.txt", Status: "new", LocalMd5: "BBB"},
	})
	serverMD5 := map[string]string{"a.txt": "AAA", "b.txt": "BBB"}

	// 清单只含 a.txt：a 被移除（已发布），b 保留（上传成功但版本未建成）
	removed, err := Sync(root, serverMD5, map[string]bool{"a.txt": true})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(removed) != 1 || removed[0] != "a.txt" {
		t.Fatalf("应只移除已进清单的文件，实际: %v", removed)
	}
	left, _ := Load(root)
	if len(left) != 1 || left[0].RelativePath != "b.txt" {
		t.Fatalf("未进清单的文件必须保留以便重试 push: %+v", left)
	}

	// manifestFiles == nil（老服务器）：降级为旧行为，MD5 一致即移除
	removed, err = Sync(root, serverMD5, nil)
	if err != nil {
		t.Fatalf("sync legacy: %v", err)
	}
	if len(removed) != 1 || removed[0] != "b.txt" {
		t.Fatalf("老服务器应降级为旧行为，实际: %v", removed)
	}
}

// TestSyncKeepsChangedFiles 内容已变（MD5 不同）的文件不得被移除
func TestSyncKeepsChangedFiles(t *testing.T) {
	root := t.TempDir()
	saveStaged(t, root, []models.FileStatusItem{
		{RelativePath: "a.txt", Status: "modified", LocalMd5: "NEW"},
	})
	removed, err := Sync(root, map[string]string{"a.txt": "OLD"}, map[string]bool{"a.txt": true})
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if len(removed) != 0 {
		t.Fatalf("内容已变的文件不该被移除: %v", removed)
	}
}
