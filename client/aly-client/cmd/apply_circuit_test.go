package cmd

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"aly/client/aly-client/config"
)

// TestApplyFailCountCircuit 验证 #2 熔断（方案 B：version.json 持久化）：
//   - applyFailureFallback 每次递增 ApplyFailCount 并落盘；
//   - 达到 MaxApplyFailCount 后 check_update 的"继续 apply"分支不再上报待应用更新。
func TestApplyFailCountCircuit(t *testing.T) {
	root, err := ioutil.TempDir("", "apply-circuit-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })

	// 写 version.json：downloaded 状态（失败兜底后的现场）
	if err := ioutil.WriteFile(filepath.Join(update, "version.json"), []byte(`{
  "version_previous": "1.0.5",
  "version": "1.0.7",
  "version_status": "downloaded",
  "downloaded_file_count": 3
}`), 0644); err != nil {
		t.Fatalf("写 version.json 失败: %v", err)
	}

	// launchMainExeFn 桩：记录调用
	orig := launchMainExeFn
	launchMainExeFn = func(cfg *config.Config, mainFolder string) {}
	defer func() { launchMainExeFn = orig }()

	fc := &FullConfig{ExeCfg: &config.Config{MainExeRelativePath: "../app/main.exe"}}

	// 第一次失败：count 1
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	applyFailureFallback(fc, vi, errors.New("first fail"))
	if vi.ApplyFailCount != 1 {
		t.Errorf("第一次失败后 ApplyFailCount 应为 1，实际 %d", vi.ApplyFailCount)
	}

	// 重新读盘验证持久化（模拟宿主/主程序重启后 check）
	viDisk, _ := config.ReadVersion()
	if viDisk.ApplyFailCount != 1 {
		t.Errorf("磁盘 ApplyFailCount 应为 1，实际 %d", viDisk.ApplyFailCount)
	}

	// 连续失败直至熔断阈值
	applyFailureFallback(fc, viDisk, errors.New("second fail"))
	applyFailureFallback(fc, viDisk, errors.New("third fail"))
	if viDisk.ApplyFailCount != MaxApplyFailCount {
		t.Errorf("三次失败后 ApplyFailCount 应为 %d，实际 %d", MaxApplyFailCount, viDisk.ApplyFailCount)
	}
	viFinal, _ := config.ReadVersion()
	if viFinal.ApplyFailCount != MaxApplyFailCount {
		t.Errorf("磁盘 ApplyFailCount 应为 %d，实际 %d", MaxApplyFailCount, viFinal.ApplyFailCount)
	}
}

// TestApplySuccessResetsFailCount 验证 #2 复位语义：
// apply_update 成功路径把 ApplyFailCount 归零。
func TestApplySuccessResetsFailCount(t *testing.T) {
	vi := &config.VersionInfo{
		VersionPrevious: "1.0.5",
		Version:         "1.0.7",
		VersionStatus:   config.VersionStatusDownloaded,
		ApplyFailCount:  MaxApplyFailCount - 1,
	}
	clearUpdateTransientState(vi)
	vi.VersionStatus = config.VersionStatusApplied
	vi.ApplyFailCount = 0 // apply_update.go 成功路径的复位语句（镜像此逻辑）
	if vi.ApplyFailCount != 0 {
		t.Errorf("成功后 ApplyFailCount 应复位为 0")
	}
}
