package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mustMkdirFile 创建目录并在其中写入一个文件
func mustMkdirFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll(%s) 失败: %v", dir, err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%s) 失败: %v", filepath.Join(dir, name), err)
	}
}

// TestRenameDirWithKillOverExistingTarget 回归测试：
// 目标目录已存在且非空（上一次更新留下的旧版本备份）时，
// 直接 os.Rename 会报 Access denied（MoveFileEx 无法覆盖非空目录，与占用无关）。
// renameDirWithKill 应先把目标挪到 to+".old"，再把 from 重命名为 to。
func TestRenameDirWithKillOverExistingTarget(t *testing.T) {
	root, err := ioutil.TempDir("", "rename-kill-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "win-x64")
	to := filepath.Join(root, "win-x64_1.0")

	// 源目录（当前主程序目录）
	mustMkdirFile(t, from, "app.exe", "new app")
	// 目标目录已存在且非空（旧版本备份）
	mustMkdirFile(t, to, "old.exe", "old app")

	if err := renameDirWithKill(from, to, 5*time.Second); err != nil {
		t.Fatalf("目标存在时应自动挪开后成功，实际失败: %v", err)
	}

	// from 应已不存在，内容移动到 to
	if _, err := os.Stat(from); !os.IsNotExist(err) {
		t.Errorf("源目录 %s 应已不存在", from)
	}
	if _, err := os.Stat(filepath.Join(to, "app.exe")); err != nil {
		t.Errorf("目标 %s 应包含新内容 app.exe: %v", to, err)
	}
	// 旧备份应被挪到 to.old
	aside := to + ".old"
	if _, err := os.Stat(filepath.Join(aside, "old.exe")); err != nil {
		t.Errorf("旧备份应被挪到 %s (old.exe): %v", aside, err)
	}
}

// TestRenameDirWithKillNoTarget 验证目标不存在时正常重命名，不产生 .old 目录。
func TestRenameDirWithKillNoTarget(t *testing.T) {
	root, err := ioutil.TempDir("", "rename-kill-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "win-x64")
	to := filepath.Join(root, "win-x64_2.0")
	mustMkdirFile(t, from, "app.exe", "new app")

	if err := renameDirWithKill(from, to, 5*time.Second); err != nil {
		t.Fatalf("renameDirWithKill 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(to, "app.exe")); err != nil {
		t.Errorf("目标 %s 应包含 app.exe: %v", to, err)
	}
	if _, err := os.Stat(to + ".old"); !os.IsNotExist(err) {
		t.Errorf("目标不存在时不应产生 %s.old", to)
	}
}
