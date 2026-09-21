package cmd

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aly/client/aly-client/config"
)

// TestApplyFailureFallback 验证失败兜底：
// version.json 回退 downloaded、启动旧版本主程序（launch seam 被调用）、
// 返回的错误信息包含原始错误。
func TestApplyFailureFallback(t *testing.T) {
	root, err := ioutil.TempDir("", "apply-fail-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })

	// 写 version.json：applying 状态，version=1.0.7
	if err := ioutil.WriteFile(filepath.Join(update, "version.json"), []byte(`{
  "version_previous": "1.0.5",
  "version": "1.0.7",
  "version_status": "applying"
}`), 0644); err != nil {
		t.Fatalf("写 version.json 失败: %v", err)
	}

	// launchMainExeFn 桩：记录调用
	var launched []string
	orig := launchMainExeFn
	launchMainExeFn = func(cfg *config.Config, mainFolder string) {
		launched = append(launched, cfg.MainExeRelativePath)
	}
	defer func() { launchMainExeFn = orig }()

	fc := &FullConfig{ExeCfg: &config.Config{MainExeRelativePath: "../win-x64/YOFC.OTDR3001.exe"}}
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}

	msg := applyFailureFallback(fc, vi, errors.New("boom: rename denied"))

	// version.json 已回退 downloaded（内存 + 磁盘）
	if vi.VersionStatus != config.VersionStatusDownloaded {
		t.Errorf("内存中 status 应为 downloaded，实际 %q", vi.VersionStatus)
	}
	vi2, _ := config.ReadVersion()
	if vi2.VersionStatus != config.VersionStatusDownloaded {
		t.Errorf("磁盘 version.json status 应为 downloaded，实际 %q", vi2.VersionStatus)
	}
	// 启动旧版本主程序 1 次，且路径正确
	if len(launched) != 1 {
		t.Fatalf("应启动旧 exe 1 次，实际 %d", len(launched))
	}
	if launched[0] != "../win-x64/YOFC.OTDR3001.exe" {
		t.Errorf("应启动旧 exe 路径 %q，实际 %q", "../win-x64/YOFC.OTDR3001.exe", launched[0])
	}
	// 返回的错误信息包含原始错误
	if !strings.Contains(msg, "boom: rename denied") {
		t.Errorf("错误信息应包含原始错误，实际 %q", msg)
	}
}
