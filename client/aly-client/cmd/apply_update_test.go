package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aly/client/aly-client/config"
)

// TestApplyUpdateResumesInterruptedRollback 验证 Bug#1：SDK 宿主实际调用的 apply_update
// 在回滚中断现场（applying + rollback_previous + 主目录缺失 + 目标目录存在）委托 resumeRollback
// 完成回滚，绝不把回滚当成升级（不激活待应用版本目录 3.0）。
func TestApplyUpdateResumesInterruptedRollback(t *testing.T) {
	root, err := ioutil.TempDir("", "apply-rollback-recover")
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
	// 回滚中断现场：主目录缺失，回滚目标目录 1.0 存在，待应用版本目录 3.0 存在
	if err := ioutil.WriteFile(filepath.Join(update, "version.json"), []byte(`{
  "version_previous": "2.0",
  "version": "3.0",
  "version_status": "applying",
  "rollback_previous": "2.0",
  "rollback_target": "1.0"
}`), 0644); err != nil {
		t.Fatalf("写 version.json 失败: %v", err)
	}
	// 回滚目标目录（含配置，loadFullConfig 在主目录缺失时回退读取）
	writePkgFile(t, root, "ApplicationFolder_1.0/v1.txt", "v1")
	writePkgFile(t, root, "ApplicationFolder_1.0/.updator/shared.json",
		`{"server_url":"http://127.0.0.1:2000","project_name":"recover-proj"}`)
	// 待应用版本目录（绝不能被激活成"升级"）
	writePkgFile(t, root, "ApplicationFolder_3.0/v3.txt", "v3")
	writePkgFile(t, root, "ApplicationFolder_3.0/.updator/shared.json",
		`{"server_url":"http://127.0.0.1:2000","project_name":"recover-proj"}`)

	os.Args = []string{"aly-client", "apply_update"}
	ApplyUpdate()

	// 主目录被回滚目标 1.0 激活
	assertPkgExists(t, root, "ApplicationFolder/v1.txt")
	// 待应用版本目录 3.0 未被消耗（没有被当成升级应用）
	assertPkgExists(t, root, "ApplicationFolder_3.0/v3.txt")
	assertPkgMissing(t, root, "ApplicationFolder_1.0")

	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi.Version != "1.0" || vi.VersionPrevious != "2.0" || vi.VersionStatus != config.VersionStatusApplied {
		t.Errorf("应恢复为 Version=1.0/VersionPrevious=2.0/applied，实际 %+v", vi)
	}
	if vi.RollbackPrevious != "" || vi.RollbackTarget != "" {
		t.Errorf("回滚标记应被清空，实际 %+v", vi)
	}
}

// TestApplyReplacementRejectsVersionEqualsPrevious 验证防御守卫：Version == VersionPrevious
// （异常/遗留状态）时拒绝替换，避免"旁移-备份-激活自毁"静默装回旧版本（Bug#3 变体）。
func TestApplyReplacementRejectsVersionEqualsPrevious(t *testing.T) {
	root, err := ioutil.TempDir("", "apply-reject-eq")
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
	if err := ioutil.WriteFile(filepath.Join(update, "version.json"), []byte(`{
  "version_previous": "2.0",
  "version": "2.0",
  "version_status": "downloaded"
}`), 0644); err != nil {
		t.Fatalf("写 version.json 失败: %v", err)
	}
	writePkgFile(t, root, "ApplicationFolder/v1.txt", "v1")
	writePkgFile(t, root, "ApplicationFolder_2.0/v2.txt", "v2")

	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(root, "ApplicationFolder"),
	}
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	versionDir := filepath.Join(root, "ApplicationFolder_2.0")
	err = applyReplacement(fc, vi, versionDir, 5*time.Second, newRenameProbeState())
	if err == nil {
		t.Fatal("Version == VersionPrevious 时应拒绝替换，实际成功")
	}
	// 磁盘未被破坏：主目录仍是旧内容，版本目录未被旁移消耗
	assertPkgExists(t, root, "ApplicationFolder/v1.txt")
	assertPkgExists(t, root, "ApplicationFolder_2.0/v2.txt")
}
