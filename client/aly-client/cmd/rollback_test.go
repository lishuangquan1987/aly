package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aly/client/aly-client/config"
)

// setupRollbackLayout 构造 PackageFolder 布局（UpdateFolder + ApplicationFolder + 各版本目录），
// 返回 pkgDir（PackageFolder 根）。
//   - UpdateFolder/client.json    main_exe_relative_path = ../ApplicationFolder/app.exe
//   - UpdateFolder/version.json   由参数指定
//   - ApplicationFolder/.updator/shared.json
func setupRollbackLayout(t *testing.T, versionJSON string) string {
	t.Helper()
	root, err := ioutil.TempDir("", "rollback-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(update, "client.json"),
		[]byte(`{"main_exe_relative_path":"../ApplicationFolder/app.exe","must_close_process_name":[]}`), 0644); err != nil {
		t.Fatalf("写 client.json 失败: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(update, "version.json"), []byte(versionJSON), 0644); err != nil {
		t.Fatalf("写 version.json 失败: %v", err)
	}
	app := filepath.Join(root, "ApplicationFolder")
	if err := os.MkdirAll(filepath.Join(app, ".updator"), 0755); err != nil {
		t.Fatalf("创建 ApplicationFolder/.updator 失败: %v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(app, ".updator", "shared.json"),
		[]byte(`{"server_url":"http://127.0.0.1:2000","project_name":"test"}`), 0644); err != nil {
		t.Fatalf("写 shared.json 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })
	return root
}

func writePkgFile(t *testing.T, pkg, rel, content string) {
	t.Helper()
	full := filepath.Join(pkg, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("创建目录 %s 失败: %v", filepath.Dir(full), err)
	}
	if err := ioutil.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("写文件 %s 失败: %v", rel, err)
	}
}

func assertPkgExists(t *testing.T, pkg, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(pkg, rel)); err != nil {
		t.Errorf("期望文件/目录存在: %s (%v)", rel, err)
	}
}

func assertPkgMissing(t *testing.T, pkg, rel string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(pkg, rel)); err == nil {
		t.Errorf("期望文件/目录不存在: %s", rel)
	}
}

// captureStdout 捕获 fn 执行期间的 stdout 输出
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("创建管道失败: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	data, _ := ioutil.ReadAll(r)
	return string(data)
}

const downloadedVersionJSON = `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "downloaded"
}`

// TestRollbackDownloadedUsesActiveVersionForBackup 验证 #5 修复：
// downloaded 状态下执行 rollback，备份目录名必须取 MainFolder 的真实内容版本（VersionPrevious），
// 待下载目录（ApplicationFolder_2.0）不得被当作备份旁移删除，version.json 语义正确。
//
// 场景：MainFolder=V1，V0 备份存在，V2 已下载未应用 → rollback --version 0.9
func TestRollbackDownloadedUsesActiveVersionForBackup(t *testing.T) {
	pkg := setupRollbackLayout(t, downloadedVersionJSON)

	writePkgFile(t, pkg, "ApplicationFolder/v1.txt", "v1")             // 当前主程序内容 = V1
	writePkgFile(t, pkg, "ApplicationFolder_0.9/v09.txt", "v09")       // V0 备份（回滚目标）
	writePkgFile(t, pkg, "ApplicationFolder_2.0/v2.pending", "v2pend") // 待下载目录

	os.Args = []string{"aly-client", "rollback", "--version", "0.9"}
	Rollback()

	// MainFolder = V0 内容
	assertPkgExists(t, pkg, "ApplicationFolder/v09.txt")
	assertPkgMissing(t, pkg, "ApplicationFolder/v1.txt")
	// 备份目录以真实内容版本 V1 命名（不再错配成 V2）
	assertPkgExists(t, pkg, "ApplicationFolder_1.0/v1.txt")
	// 待下载目录保留（未被误当备份旁移后删除）
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/v2.pending")
	// version.json：Version=0.9, VersionPrevious=1.0, applied
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi.Version != "0.9" || vi.VersionPrevious != "1.0" || vi.VersionStatus != config.VersionStatusApplied {
		t.Errorf("version.json 应为 Version=0.9/VersionPrevious=1.0/applied，实际 %+v", vi)
	}
}

// TestRollbackRejectsPendingVersion 验证 #5 守卫：
// downloaded 状态下回滚到"待应用的下载版本"（== versionInfo.Version）必须被拒绝，不做任何改动。
func TestRollbackRejectsPendingVersion(t *testing.T) {
	pkg := setupRollbackLayout(t, downloadedVersionJSON)

	writePkgFile(t, pkg, "ApplicationFolder/v1.txt", "v1")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/v2.pending", "v2pend")

	os.Args = []string{"aly-client", "rollback", "--version", "2.0"}
	Rollback()

	// 无任何改动
	assertPkgExists(t, pkg, "ApplicationFolder/v1.txt")
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/v2.pending")
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi.Version != "2.0" || vi.VersionStatus != config.VersionStatusDownloaded {
		t.Errorf("version.json 应保持 Version=2.0/downloaded，实际 %+v", vi)
	}
}

// TestRollbackCrashRecoveryUsesPersistedActiveVersion 验证 #5 崩溃恢复：
// rollback 在 downloaded 状态启动、写完 status=applying 后、正式替换前崩溃，
// 再次 rollback 重做替换时，备份目录名应取持久化的 rollback_previous（真实内容版本 1.0），
// 而非 versionInfo.Version（待应用版本 2.0）；待下载目录不得被当作备份旁移删除。
func TestRollbackCrashRecoveryUsesPersistedActiveVersion(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applying",
  "rollback_previous": "1.0"
}`)

	// MainFolder 存在（崩溃发生在备份改名之前），内容 = V1
	writePkgFile(t, pkg, "ApplicationFolder/v1.txt", "v1")
	writePkgFile(t, pkg, "ApplicationFolder_0.9/v09.txt", "v09")     // 回滚目标
	writePkgFile(t, pkg, "ApplicationFolder_2.0/v2.pending", "v2pd") // 待下载目录
	// ApplicationFolder_1.0 不存在（V1 在 MainFolder 中，待重做时创建）

	os.Args = []string{"aly-client", "rollback", "--version", "0.9"}
	Rollback()

	// MainFolder = 回滚目标 V0 内容
	assertPkgExists(t, pkg, "ApplicationFolder/v09.txt")
	assertPkgMissing(t, pkg, "ApplicationFolder/v1.txt")
	// 备份目录以真实内容版本 V1 命名（而非待应用版本 2.0）
	assertPkgExists(t, pkg, "ApplicationFolder_1.0/v1.txt")
	// 待下载目录保留
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/v2.pending")
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi.Version != "0.9" || vi.VersionPrevious != "1.0" || vi.VersionStatus != config.VersionStatusApplied {
		t.Errorf("version.json 应为 Version=0.9/VersionPrevious=1.0/applied，实际 %+v", vi)
	}
	if vi.RollbackPrevious != "" {
		t.Errorf("崩溃恢复后 rollback_previous 应被清空，实际 %q", vi.RollbackPrevious)
	}
}

// TestRollbackRejectsNoOpActiveVersion 验证 #5 守卫（审查发现1）：
// downloaded 状态下回滚到"当前活动版本"（VersionPrevious）是无操作，必须被拒绝，
// 避免走"旁移→目标丢失→apply rename failed"的误导失败路径。
func TestRollbackRejectsNoOpActiveVersion(t *testing.T) {
	pkg := setupRollbackLayout(t, downloadedVersionJSON)

	writePkgFile(t, pkg, "ApplicationFolder/v1.txt", "v1")
	// 活动版本目录残留存在（异常/残留场景，stat 检查可通过）
	writePkgFile(t, pkg, "ApplicationFolder_1.0/v1-backup.txt", "v1backup")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/v2.pending", "v2pend")

	os.Args = []string{"aly-client", "rollback", "--version", "1.0"}
	Rollback()

	// 无任何改动：MainFolder 未变、待下载目录保留、活动版本残留目录保留
	assertPkgExists(t, pkg, "ApplicationFolder/v1.txt")
	assertPkgExists(t, pkg, "ApplicationFolder_1.0/v1-backup.txt")
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/v2.pending")
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi.Version != "2.0" || vi.VersionPrevious != "1.0" || vi.VersionStatus != config.VersionStatusDownloaded {
		t.Errorf("version.json 应保持不变，实际 %+v", vi)
	}
}

// TestListRollbackExcludesPending 验证 #5 配套：
// downloaded 状态下待应用版本不出现在回滚列表，CurrentVersion 显示真实内容版本（VersionPrevious）。
func TestListRollbackExcludesPending(t *testing.T) {
	pkg := setupRollbackLayout(t, downloadedVersionJSON)

	writePkgFile(t, pkg, "ApplicationFolder/v1.txt", "v1")
	writePkgFile(t, pkg, "ApplicationFolder_0.9/v09.txt", "v09")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/v2.pending", "v2pend")

	os.Args = []string{"aly-client", "list_rollback_versions"}
	out := captureStdout(t, ListRollbackVersions)

	if !strings.Contains(out, `"0.9"`) {
		t.Errorf("回滚列表应包含 0.9，实际输出: %s", out)
	}
	if strings.Contains(out, `"2.0"`) {
		t.Errorf("回滚列表不应包含待应用版本 2.0，实际输出: %s", out)
	}
	if !strings.Contains(out, `"current_version":"1.0"`) {
		t.Errorf("current_version 应显示真实内容版本 1.0，实际输出: %s", out)
	}
}

// TestRollbackResumesInterruptedAfterTargetConsumed 验证 Bug#1 B2'：
// 回滚激活已完成但未写 applied（status=applying + rollback_previous + rollback_target，
// 目标目录已被消耗），重跑 rollback --version 不再报 "version not found"，
// 而是委托 resumeRollback 补写 applied、保持回滚结果。
func TestRollbackResumesInterruptedAfterTargetConsumed(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "2.0",
  "version": "3.0",
  "version_status": "applying",
  "rollback_previous": "2.0",
  "rollback_target": "1.0"
}`)

	// 现场：MainFolder=1.0 内容（回滚已完成），目标目录 App_1.0 已消耗，
	// 备份 App_2.0 与待应用目录 App_3.0 仍在
	writePkgFile(t, pkg, "ApplicationFolder/v1.txt", "v1")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/v2.txt", "v2")
	writePkgFile(t, pkg, "ApplicationFolder_3.0/v3.txt", "v3")

	os.Args = []string{"aly-client", "rollback", "--version", "1.0"}
	Rollback()

	// 回滚结果保持：MainFolder 仍为 1.0 内容，目标目录不重建
	assertPkgExists(t, pkg, "ApplicationFolder/v1.txt")
	assertPkgMissing(t, pkg, "ApplicationFolder_1.0")
	// 待应用目录未被触碰
	assertPkgExists(t, pkg, "ApplicationFolder_3.0/v3.txt")

	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi.Version != "1.0" || vi.VersionPrevious != "2.0" || vi.VersionStatus != config.VersionStatusApplied {
		t.Errorf("应补写 Version=1.0/VersionPrevious=2.0/applied，实际 %+v", vi)
	}
	if vi.RollbackPrevious != "" || vi.RollbackTarget != "" {
		t.Errorf("回滚标记应被清空，实际 %+v", vi)
	}
}

// TestResumeRollbackRedoSwap 验证 resumeRollback 分支 2：MainFolder 存在 + 目标目录存在
// → 重做回滚替换（仅重命名），备份以 rollback_previous 命名。
func TestResumeRollbackRedoSwap(t *testing.T) {
	root := setupRollbackLayout(t, `{
  "version_previous": "2.0",
  "version": "3.0",
  "version_status": "applying",
  "rollback_previous": "2.0",
  "rollback_target": "1.0"
}`)
	// MainFolder = 2.0 内容（回滚前的活动版本），目标目录 1.0 存在（崩溃发生在备份改名前）
	writePkgFile(t, root, "ApplicationFolder/v2.txt", "v2")
	writePkgFile(t, root, "ApplicationFolder_1.0/v1.txt", "v1")

	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(root, "ApplicationFolder"),
	}
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if err := resumeRollback(fc, vi, 5*time.Second, "1.0"); err != nil {
		t.Fatalf("resumeRollback 分支2 失败: %v", err)
	}

	// 目标版本激活，备份 = 回滚前版本 2.0
	assertPkgExists(t, root, "ApplicationFolder/v1.txt")
	assertPkgMissing(t, root, "ApplicationFolder/v2.txt")
	assertPkgMissing(t, root, "ApplicationFolder_1.0")
	assertPkgExists(t, root, "ApplicationFolder_2.0/v2.txt")

	vi2, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi2.Version != "1.0" || vi2.VersionPrevious != "2.0" || vi2.VersionStatus != config.VersionStatusApplied {
		t.Errorf("应恢复为 Version=1.0/VersionPrevious=2.0/applied，实际 %+v", vi2)
	}
	if vi2.RollbackPrevious != "" || vi2.RollbackTarget != "" {
		t.Errorf("回滚标记应被清空，实际 %+v", vi2)
	}
}

// TestResumeRollbackRestoreBackup 验证 resumeRollback 分支 4：MainFolder 缺失 + 目标目录缺失
// + 备份存在（B2' 且主目录丢失）→ 放弃回滚，恢复回滚前版本备份。
func TestResumeRollbackRestoreBackup(t *testing.T) {
	root := setupRollbackLayout(t, `{
  "version_previous": "2.0",
  "version": "3.0",
  "version_status": "applying",
  "rollback_previous": "2.0",
  "rollback_target": "1.0"
}`)
	// 主目录缺失；目标目录 1.0 缺失；备份 2.0 存在
	if err := os.RemoveAll(filepath.Join(root, "ApplicationFolder")); err != nil {
		t.Fatalf("移除 MainFolder 失败: %v", err)
	}
	writePkgFile(t, root, "ApplicationFolder_2.0/v2.txt", "v2")

	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(root, "ApplicationFolder"),
	}
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if err := resumeRollback(fc, vi, 5*time.Second, "1.0"); err != nil {
		t.Fatalf("resumeRollback 分支4 失败: %v", err)
	}

	// 恢复回滚前版本 2.0 到主目录
	assertPkgExists(t, root, "ApplicationFolder/v2.txt")
	assertPkgMissing(t, root, "ApplicationFolder_2.0")

	vi2, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi2.Version != "2.0" || vi2.VersionStatus != config.VersionStatusApplied {
		t.Errorf("应归位为 Version=2.0/applied（放弃回滚），实际 %+v", vi2)
	}
}

// TestResumeRollbackLegacyAbortNoTarget 验证老数据兜底：applying + rollback_previous 但
// 无 rollback_target 且无 CLI 目标时，安全放弃回滚（归位到回滚前版本），绝不升级。
func TestResumeRollbackLegacyAbortNoTarget(t *testing.T) {
	root := setupRollbackLayout(t, `{
  "version_previous": "2.0",
  "version": "3.0",
  "version_status": "applying",
  "rollback_previous": "2.0"
}`)
	// MainFolder 存在（回滚前的活动版本 2.0 内容）
	writePkgFile(t, root, "ApplicationFolder/v2.txt", "v2")

	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(root, "ApplicationFolder"),
	}
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if err := resumeRollback(fc, vi, 5*time.Second, ""); err != nil {
		t.Fatalf("resumeRollback 老数据兜底失败: %v", err)
	}

	// 归位到回滚前版本 2.0（不升级到 3.0）
	assertPkgExists(t, root, "ApplicationFolder/v2.txt")
	vi2, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi2.Version != "2.0" || vi2.VersionStatus != config.VersionStatusApplied {
		t.Errorf("应归位为 Version=2.0/applied，实际 %+v", vi2)
	}
}
