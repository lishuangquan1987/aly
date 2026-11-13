package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aly/client/aly-client/config"
)

// TestRollbackRejectsDownloadedWithoutPrevious M7：downloaded 且 version_previous 为空时，
// 回滚会退化成"用待应用版本号命名备份目录"（目录名与内容错配，与 F9 同型风险）。
// 与 F4/F10 保守口径一致：直接拒绝并给出指引，不改状态、不动磁盘。
func TestRollbackRejectsDownloadedWithoutPrevious(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "",
  "version": "2.0",
  "version_status": "downloaded"
}`)
	writePkgFile(t, pkg, "ApplicationFolder/cur.txt", "cur")
	writePkgFile(t, pkg, "ApplicationFolder_0.9/v09.txt", "v09")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/pending.txt", "pending")
	before := readFileOrFail(t, versionFilePath(pkg))

	os.Args = []string{"aly-client", "rollback", "--version", "0.9"}
	out := captureStdout(t, Rollback)

	if !strings.Contains(out, `"isSuccess":false`) {
		t.Fatalf("downloaded 且 Previous 为空时必须拒绝回滚，实际输出: %s", out)
	}
	if !strings.Contains(out, "version_previous 为空") {
		t.Errorf("错误信息应说明 version_previous 为空并给出指引，实际输出: %s", out)
	}
	if after := readFileOrFail(t, versionFilePath(pkg)); after != before {
		t.Errorf("拒绝路径不得改写 version.json，改前=%q 改后=%q", before, after)
	}
	// 磁盘现场未被破坏：无版本号备份目录（ApplicationFolder_）也不得被创建/旁移
	assertPkgExists(t, pkg, "ApplicationFolder/cur.txt")
	assertPkgExists(t, pkg, "ApplicationFolder_0.9/v09.txt")
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/pending.txt")
	assertPkgMissing(t, pkg, "ApplicationFolder_")
}

// TestResumeRollbackBothMissingDegrades M6：回滚续跑时主目录 / 目标版本 / 回滚前备份
// 全都不存在 → 不再只报错（RP 分支无条件早退会让熔断不可达），降级 downloaded +
// ApplyFailCount++ 并清空回滚现场，让 check 重下当前版本重建应用目录。
func TestResumeRollbackBothMissingDegrades(t *testing.T) {
	root := setupRollbackLayout(t, `{
  "version_previous": "2.0",
  "version": "3.0",
  "version_status": "applying",
  "rollback_previous": "2.0",
  "rollback_target": "1.0"
}`)
	// 主目录、目标 1.0、备份 2.0 全缺失（构造"全无现场"）
	if err := os.RemoveAll(filepath.Join(root, "ApplicationFolder")); err != nil {
		t.Fatalf("删除 MainFolder 失败: %v", err)
	}
	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(root, "ApplicationFolder"),
	}
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if err := resumeRollback(fc, vi, 5*time.Second, ""); err == nil {
		t.Fatal("全无现场时 resumeRollback 应返回 error")
	}
	persisted, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if persisted.VersionStatus != config.VersionStatusDownloaded {
		t.Errorf("应降级 downloaded，实际 %q", persisted.VersionStatus)
	}
	if persisted.ApplyFailCount != 1 {
		t.Errorf("应递增 ApplyFailCount 为 1，实际 %d", persisted.ApplyFailCount)
	}
	if persisted.RollbackPrevious != "" || persisted.RollbackTarget != "" {
		t.Errorf("降级后回滚现场应被清空，实际 %+v", persisted)
	}
}

// TestResumeRollbackFailureKeepsApplyingAndCounts M6：回滚续跑失败但现场仍可重试时，
// 保持 applying（保留 rollback_previous/target 供下次续跑）并递增 ApplyFailCount。
// 失败注入：rollback_previous 含 NUL（\u0000），使备份目录 stat 返回 EINVAL 而非 ENOENT。
// 不能用 Windows 非法字符 *：Linux 上它是合法文件名，stat 只会返回 ENOENT。
func TestResumeRollbackFailureKeepsApplyingAndCounts(t *testing.T) {
	root := setupRollbackLayout(t, `{
  "version_previous": "2.0",
  "version": "3.0",
  "version_status": "applying",
  "rollback_previous": "bad\u0000name",
  "rollback_target": "1.0"
}`)
	// 主目录存在；目标目录 1.0 存在（分支 2 条件），随后备份 stat 失败
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
	if err := resumeRollback(fc, vi, 5*time.Second, ""); err == nil {
		t.Fatal("stat 失败的现场应返回 error")
	}
	persisted, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if persisted.VersionStatus != config.VersionStatusApplying {
		t.Errorf("可重试失败应保持 applying，实际 %q", persisted.VersionStatus)
	}
	if persisted.ApplyFailCount != 1 {
		t.Errorf("应递增 ApplyFailCount 为 1，实际 %d", persisted.ApplyFailCount)
	}
	if persisted.RollbackPrevious != "bad\u0000name" || persisted.RollbackTarget != "1.0" {
		t.Errorf("回滚现场应保留供下次续跑，实际 %+v", persisted)
	}
	// 现场未被破坏
	assertPkgExists(t, root, "ApplicationFolder/v2.txt")
	assertPkgExists(t, root, "ApplicationFolder_1.0/v1.txt")
}

// TestResumeRollbackRedoSwapWithPreexistingAside M8：入口不再预删 .old 残留后，
// 已有的 .old 家族不得破坏分支 2 的旁移-备份-激活流程（成功路径仍会清理残留）。
func TestResumeRollbackRedoSwapWithPreexistingAside(t *testing.T) {
	root := setupRollbackLayout(t, `{
  "version_previous": "2.0",
  "version": "3.0",
  "version_status": "applying",
  "rollback_previous": "2.0",
  "rollback_target": "1.0"
}`)
	writePkgFile(t, root, "ApplicationFolder/v2.txt", "v2")     // 回滚前活动版本内容
	writePkgFile(t, root, "ApplicationFolder_1.0/v1.txt", "v1") // 回滚目标
	// 存在历史旁移残留（崩溃点遗留快照）与旧备份目录
	writePkgFile(t, root, "ApplicationFolder_2.0/stale.txt", "stale")
	writePkgFile(t, root, "ApplicationFolder_2.0.old/snapshot.txt", "snapshot")

	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(root, "ApplicationFolder"),
	}
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if err := resumeRollback(fc, vi, 5*time.Second, ""); err != nil {
		t.Fatalf("带历史 .old 残留时分支 2 应仍能完成: %v", err)
	}
	// 回滚完成：主目录 = 1.0 内容；1.0 目录被消耗；2.0 备份 = 回滚前内容
	assertPkgExists(t, root, "ApplicationFolder/v1.txt")
	assertPkgExists(t, root, "ApplicationFolder_2.0/v2.txt")
	assertPkgMissing(t, root, "ApplicationFolder_1.0")
	// 成功路径清理旁移残留
	assertPkgMissing(t, root, "ApplicationFolder_2.0.old")

	persisted, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if persisted.Version != "1.0" || persisted.VersionStatus != config.VersionStatusApplied {
		t.Errorf("应为 applied/1.0，实际 %+v", persisted)
	}
	if persisted.RollbackPrevious != "" || persisted.RollbackTarget != "" {
		t.Errorf("回滚标记应被清空，实际 %+v", persisted)
	}
}

// TestRollbackRecoveryFailureHelperLogsCircuit 熔断阈值边界：助手函数在达到阈值时
// 仍保持 applying 且计数为阈值（check_update 侧据此停止上报）。
func TestRollbackRecoveryFailureHelperLogsCircuit(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applying",
  "rollback_previous": "1.0",
  "rollback_target": "0.9",
  "apply_fail_count": 2
}`)
	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(pkg, "ApplicationFolder"),
	}
	_ = fc
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	err = rollbackRecoveryFailure(vi, fmt.Errorf("occupied"))
	if err == nil {
		t.Fatal("rollbackRecoveryFailure 应原样返回失败")
	}
	persisted, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if persisted.VersionStatus != config.VersionStatusApplying || persisted.ApplyFailCount != 3 {
		t.Errorf("应保持 applying 且计数达阈值 3，实际 %+v", persisted)
	}
}
