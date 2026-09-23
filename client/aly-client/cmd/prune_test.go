package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"aly/client/aly-client/config"
)

// TestPruneVersionSnapshots 验证 Bug#6 配额清理：保留最近 keep 个候选快照，
// 保护 MainFolder / 当前版本目录 / 备份目录不被删除。
func TestPruneVersionSnapshots(t *testing.T) {
	root, err := ioutil.TempDir("", "prune-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })

	if err := ioutil.WriteFile(filepath.Join(update, "client.json"),
		[]byte(`{"main_exe_relative_path":"../ApplicationFolder/app.exe","must_close_process_name":[]}`), 0644); err != nil {
		t.Fatalf("写 client.json 失败: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(update, "version.json"),
		[]byte(`{"version_previous":"2.0.0","version":"3.0.0","version_status":"applied"}`), 0644); err != nil {
		t.Fatalf("写 version.json 失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ApplicationFolder"), 0755); err != nil {
		t.Fatalf("创建 MainFolder 失败: %v", err)
	}
	// 版本快照 1.0.0 ~ 6.0.0
	for _, v := range []string{"1.0.0", "2.0.0", "3.0.0", "4.0.0", "5.0.0", "6.0.0"} {
		if err := os.MkdirAll(filepath.Join(root, "ApplicationFolder_"+v), 0755); err != nil {
			t.Fatalf("创建版本目录 %s 失败: %v", v, err)
		}
	}

	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(root, "ApplicationFolder"),
	}
	pruneVersionSnapshots(fc, 3)

	// 最老的 1.0.0 被删；受保护的备份 2.0.0 与当前 3.0.0 保留；其余最新的 3 个候选（4/5/6）保留
	assertPkgMissing(t, root, "ApplicationFolder_1.0.0")
	for _, v := range []string{"2.0.0", "3.0.0", "4.0.0", "5.0.0", "6.0.0"} {
		assertPkgExists(t, root, "ApplicationFolder_"+v)
	}
}

// TestPruneVersionSnapshotsProtectsRollbackTarget 验证入口清理不会删掉 rollback 的 CLI 目标：
// 快照较多时，用户正要回滚到的旧版本目录必须受保护。
func TestPruneVersionSnapshotsProtectsRollbackTarget(t *testing.T) {
	root, err := ioutil.TempDir("", "prune-rollback-target")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })

	if err := ioutil.WriteFile(filepath.Join(update, "client.json"),
		[]byte(`{"main_exe_relative_path":"../ApplicationFolder/app.exe","must_close_process_name":[]}`), 0644); err != nil {
		t.Fatalf("写 client.json 失败: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(update, "version.json"),
		[]byte(`{"version_previous":"2.0.0","version":"3.0.0","version_status":"applied"}`), 0644); err != nil {
		t.Fatalf("写 version.json 失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ApplicationFolder"), 0755); err != nil {
		t.Fatalf("创建 MainFolder 失败: %v", err)
	}
	// 版本快照 1.0.0 ~ 7.0.0（1.0.0 是用户 rollback --version 1.0.0 的目标）
	for _, v := range []string{"1.0.0", "2.0.0", "3.0.0", "4.0.0", "5.0.0", "6.0.0", "7.0.0"} {
		if err := os.MkdirAll(filepath.Join(root, "ApplicationFolder_"+v), 0755); err != nil {
			t.Fatalf("创建版本目录 %s 失败: %v", v, err)
		}
	}

	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(root, "ApplicationFolder"),
	}
	// rollback 入口的调用形态：额外保护 CLI 目标 1.0.0
	pruneVersionSnapshots(fc, 3, "1.0.0")

	// 回滚目标 1.0.0 与保护集 2.0.0/3.0.0 保留；候选中最老的 4.0.0 被删
	assertPkgExists(t, root, "ApplicationFolder_1.0.0")
	assertPkgExists(t, root, "ApplicationFolder_2.0.0")
	assertPkgExists(t, root, "ApplicationFolder_3.0.0")
	assertPkgMissing(t, root, "ApplicationFolder_4.0.0")
	for _, v := range []string{"5.0.0", "6.0.0", "7.0.0"} {
		assertPkgExists(t, root, "ApplicationFolder_"+v)
	}
}

// TestPruneVersionSnapshotsKeepsAllWhenUnderQuota 验证快照数未超配额时不删任何目录。
func TestPruneVersionSnapshotsKeepsAllWhenUnderQuota(t *testing.T) {
	root, err := ioutil.TempDir("", "prune-keepall")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })

	if err := ioutil.WriteFile(filepath.Join(update, "client.json"),
		[]byte(`{"main_exe_relative_path":"../ApplicationFolder/app.exe","must_close_process_name":[]}`), 0644); err != nil {
		t.Fatalf("写 client.json 失败: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(update, "version.json"),
		[]byte(`{"version_previous":"2.0.0","version":"3.0.0","version_status":"applied"}`), 0644); err != nil {
		t.Fatalf("写 version.json 失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ApplicationFolder"), 0755); err != nil {
		t.Fatalf("创建 MainFolder 失败: %v", err)
	}
	// 仅一个候选快照 1.0.0（未超配额）
	if err := os.MkdirAll(filepath.Join(root, "ApplicationFolder_1.0.0"), 0755); err != nil {
		t.Fatalf("创建版本目录失败: %v", err)
	}

	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(root, "ApplicationFolder"),
	}
	pruneVersionSnapshots(fc, 3)

	assertPkgExists(t, root, "ApplicationFolder_1.0.0")
}
