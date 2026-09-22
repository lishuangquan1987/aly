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

// TestApplyFailureFallbackKeepsApplyingWhenMainFolderLost 验证 #7 修复：
// 主目录缺失（备份改名成功、应用改名与回滚改名都失败）但版本目录仍存在时，
// 失败兜底必须保持 applying 状态（不降级为 downloaded），
// 让下次 apply/check 走崩溃恢复分支完成 versionDir → MainFolder 重命名。
func TestApplyFailureFallbackKeepsApplyingWhenMainFolderLost(t *testing.T) {
	root, err := ioutil.TempDir("", "apply-fail-lost-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })

	// PackageFolder 布局：UpdateFolder（含 client.json/version.json）+
	// ApplicationFolder（主目录，缺失！）+ ApplicationFolder_1.0.7（版本目录，存在）
	pkg := filepath.Join(root)
	if err := ioutil.WriteFile(filepath.Join(update, "client.json"),
		[]byte(`{"main_exe_relative_path":"../ApplicationFolder/app.exe","must_close_process_name":[]}`), 0644); err != nil {
		t.Fatalf("写 client.json 失败: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(update, "version.json"), []byte(`{
  "version_previous": "1.0.5",
  "version": "1.0.7",
  "version_status": "applying"
}`), 0644); err != nil {
		t.Fatalf("写 version.json 失败: %v", err)
	}
	// 主目录缺失，版本目录存在（崩溃恢复的修复对象）
	versionDir := filepath.Join(pkg, "ApplicationFolder_1.0.7")
	if err := os.MkdirAll(filepath.Join(versionDir, ".updator"), 0755); err != nil {
		t.Fatalf("创建版本目录失败: %v", err)
	}

	var launched []string
	orig := launchMainExeFn
	launchMainExeFn = func(cfg *config.Config, mainFolder string) {
		launched = append(launched, cfg.MainExeRelativePath)
	}
	defer func() { launchMainExeFn = orig }()

	fc := &FullConfig{ExeCfg: &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"}}
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}

	msg := applyFailureFallback(fc, vi, errors.New("boom: rename denied"))

	// 主目录缺失 + 版本目录存在 → 必须保持 applying（内存 + 磁盘）
	if vi.VersionStatus != config.VersionStatusApplying {
		t.Errorf("主目录缺失时 status 应保持 applying，实际 %q", vi.VersionStatus)
	}
	vi2, _ := config.ReadVersion()
	if vi2.VersionStatus != config.VersionStatusApplying {
		t.Errorf("磁盘 version.json status 应保持 applying，实际 %q", vi2.VersionStatus)
	}
	// 主目录已丢失：不应尝试启动旧 exe（exe 位于缺失目录下，启动必然失败），
	// 由崩溃恢复分支完成重命名后启动主程序。
	if len(launched) != 0 {
		t.Errorf("主目录缺失时不应启动旧 exe，实际启动 %d 次", len(launched))
	}
	// 返回的错误信息包含原始错误
	if !strings.Contains(msg, "boom: rename denied") {
		t.Errorf("错误信息应包含原始错误，实际 %q", msg)
	}
}

// TestLoadFullConfigFromVersionDir 验证崩溃恢复的配置回退（#7 延伸）：
// MainFolder 缺失（apply 中途崩溃，主目录被改名走）时，loadFullConfig 应能
// 从版本目录读取 shared.json，使崩溃恢复分支可执行。
func TestLoadFullConfigFromVersionDir(t *testing.T) {
	root, err := ioutil.TempDir("", "loadcfg-recover")
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

	// 构造：MainFolder 缺失；版本目录 ApplicationFolder_1.0.6 存在且含 shared.json
	if _, err := os.Stat(filepath.Join(root, "ApplicationFolder")); !os.IsNotExist(err) {
		t.Fatal("预置：MainFolder 应缺失")
	}
	verDir := filepath.Join(root, "ApplicationFolder_1.0.6")
	if err := os.MkdirAll(filepath.Join(verDir, ".updator"), 0755); err != nil {
		t.Fatalf("创建版本目录失败: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(verDir, ".updator", "shared.json"),
		[]byte(`{"server_url":"http://127.0.0.1:2000","project_name":"recover-proj"}`), 0644); err != nil {
		t.Fatalf("写 shared.json 失败: %v", err)
	}

	fc, err := loadFullConfig("", "", "")
	if err != nil {
		t.Fatalf("MainFolder 缺失时 loadFullConfig 应回退到版本目录成功，实际失败: %v", err)
	}
	if fc.Shared.ServerURL != "http://127.0.0.1:2000" || fc.Shared.ProjectName != "recover-proj" {
		t.Errorf("应回退读取版本目录的 shared.json，实际 %+v", fc.Shared)
	}
}

// TestLoadFullConfigPrefersVersionJSONDir 验证回退优先使用 version.json 记录的版本目录：
// 存在更高版本目录时，也应优先取"正在应用的版本"（崩溃恢复的目标），而非最高版本。
func TestLoadFullConfigPrefersVersionJSONDir(t *testing.T) {
	root, err := ioutil.TempDir("", "loadcfg-prefer")
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
	// version.json 记录正在应用 1.0.6（崩溃恢复目标）
	if err := ioutil.WriteFile(filepath.Join(update, "version.json"),
		[]byte(`{"version_previous":"1.0.5","version":"1.0.6","version_status":"applying"}`), 0644); err != nil {
		t.Fatalf("写 version.json 失败: %v", err)
	}

	// 两个版本目录：1.0.6（正在应用）与 1.0.7（更高，但非本次恢复目标）
	for _, v := range []string{"1.0.6", "1.0.7"} {
		d := filepath.Join(root, "ApplicationFolder_"+v, ".updator")
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("创建版本目录失败: %v", err)
		}
		content := `{"server_url":"http://127.0.0.1:2000","project_name":"proj-` + v + `"}`
		if err := ioutil.WriteFile(filepath.Join(d, "shared.json"), []byte(content), 0644); err != nil {
			t.Fatalf("写 shared.json 失败: %v", err)
		}
	}

	fc, err := loadFullConfig("", "", "")
	if err != nil {
		t.Fatalf("loadFullConfig 失败: %v", err)
	}
	if fc.Shared.ProjectName != "proj-1.0.6" {
		t.Errorf("应优先 version.json 记录的 1.0.6 版本目录，实际 %q", fc.Shared.ProjectName)
	}
}

// TestLoadFullConfigNoFallbackOnCorruptShared 验证 MainFolder 存在但 shared.json 损坏时，
// 不做静默回退（保持原错误上报），避免掩盖配置损坏问题。
func TestLoadFullConfigNoFallbackOnCorruptShared(t *testing.T) {
	root, err := ioutil.TempDir("", "loadcfg-corrupt")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	appDir := filepath.Join(root, "ApplicationFolder", ".updator")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	if err := os.MkdirAll(appDir, 0755); err != nil {
		t.Fatalf("创建 ApplicationFolder 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })

	if err := ioutil.WriteFile(filepath.Join(update, "client.json"),
		[]byte(`{"main_exe_relative_path":"../ApplicationFolder/app.exe","must_close_process_name":[]}`), 0644); err != nil {
		t.Fatalf("写 client.json 失败: %v", err)
	}
	// MainFolder 存在，但 shared.json 是非法 JSON（损坏）
	if err := ioutil.WriteFile(filepath.Join(appDir, "shared.json"),
		[]byte(`{not-valid-json`), 0644); err != nil {
		t.Fatalf("写损坏 shared.json 失败: %v", err)
	}
	// 同时放一个可用的版本目录，确认不会被误用
	verDir := filepath.Join(root, "ApplicationFolder_1.0.6", ".updator")
	if err := os.MkdirAll(verDir, 0755); err != nil {
		t.Fatalf("创建版本目录失败: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(verDir, "shared.json"),
		[]byte(`{"server_url":"http://127.0.0.1:2000","project_name":"other"}`), 0644); err != nil {
		t.Fatalf("写 shared.json 失败: %v", err)
	}

	if _, err := loadFullConfig("", "", ""); err == nil {
		t.Error("MainFolder 存在但 shared.json 损坏时应报错，不应静默回退到版本目录")
	}
}
