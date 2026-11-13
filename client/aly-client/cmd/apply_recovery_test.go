package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aly/client/aly-client/config"
)

// 本文件覆盖 B8（applying 崩溃恢复两个子路径接入 F3/A1 校验）、M6（恢复失败路径状态迁移 +
// 熔断计数）、M9（stat 错误 fail-safe）与 F5 弱校验留痕。
// 复用 rollback_test.go 的 setupRollbackLayout / writePkgFile / assertPkg* / captureStdout。

// sharedJSON 版本目录回退读取的共享配置（主目录缺失时 loadFullConfig 依赖它）。
const sharedJSON = `{"server_url":"http://127.0.0.1:2000","project_name":"test"}`

// TestApplyRecoveryFallThroughRejectsIncompletePending B8 子路径①：
// applying + 主目录在 + 版本目录在（崩溃在替换前）→ 重做替换前必须复核 F3/A1，
// 不完整时走与 downloaded 相同的自愈（applied + Previous），绝不激活残缺目录。
func TestApplyRecoveryFallThroughRejectsIncompletePending(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applying",
  "downloaded_file_count": 2
}`)
	// MainFolder 仍是上一版本内容（崩溃未发生替换）
	writePkgFile(t, pkg, "ApplicationFolder/old.txt", "old")
	// 版本目录残缺：记录 2 个文件，实际只有 1 个
	writePkgFile(t, pkg, "ApplicationFolder_2.0/only.txt", "only")

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":false`) {
		t.Fatalf("残缺产物应拒绝激活，实际输出: %s", out)
	}
	if vi.VersionStatus != config.VersionStatusApplied || vi.Version != "1.0" {
		t.Errorf("应自愈为 applied/1.0，实际 %+v", vi)
	}
	// 主目录未被替换，版本目录未被消耗
	assertPkgExists(t, pkg, "ApplicationFolder/old.txt")
	assertPkgMissing(t, pkg, "ApplicationFolder/only.txt")
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/only.txt")
}

// TestApplyRecoveryDirectActivateRestoresBackup B8 子路径②：
// applying + 主目录缺失 + 版本目录残缺 → 不得直改名激活；真崩溃窗内存在
// prevVersionDir 备份，应先恢复备份再自愈为 applied/Previous（不得凭空写 applied）。
func TestApplyRecoveryDirectActivateRestoresBackup(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applying",
  "downloaded_file_count": 5
}`)
	// 现场：MainFolder 已被改名成备份 App_1.0（含配置，供 loadFullConfig 回退）
	if err := os.RemoveAll(filepath.Join(pkg, "ApplicationFolder")); err != nil {
		t.Fatalf("删除 MainFolder 失败: %v", err)
	}
	writePkgFile(t, pkg, "ApplicationFolder_1.0/v1.txt", "v1")
	writePkgFile(t, pkg, "ApplicationFolder_1.0/.updator/shared.json", sharedJSON)
	// 待应用版本目录残缺（1 < 5）
	writePkgFile(t, pkg, "ApplicationFolder_2.0/only.txt", "only")

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":false`) {
		t.Fatalf("残缺产物不得激活，实际输出: %s", out)
	}
	if !strings.Contains(out, "恢复上一版本") {
		t.Errorf("应说明从备份恢复，实际输出: %s", out)
	}
	// 备份恢复到主目录，且自愈为 applied/1.0（内容版本）
	assertPkgExists(t, pkg, "ApplicationFolder/v1.txt")
	assertPkgMissing(t, pkg, "ApplicationFolder_1.0")
	if vi.VersionStatus != config.VersionStatusApplied || vi.Version != "1.0" {
		t.Errorf("应自愈为 applied/1.0，实际 %+v", vi)
	}
	// 残缺的待应用目录保留（下次重下会覆盖/补齐），未被激活进主目录
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/only.txt")
	assertPkgMissing(t, pkg, "ApplicationFolder/only.txt")
}

// TestApplyRecoveryDirectActivateWithoutBackupDegrades M6/B8 子路径②的兜底：
// 主目录缺失、版本目录残缺、上一版本备份也不在 → 无内容可恢复，不得写 applied；
// 降级 downloaded + ApplyFailCount++，让 check 重下补齐。
func TestApplyRecoveryDirectActivateWithoutBackupDegrades(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applying",
  "downloaded_file_count": 5
}`)
	if err := os.RemoveAll(filepath.Join(pkg, "ApplicationFolder")); err != nil {
		t.Fatalf("删除 MainFolder 失败: %v", err)
	}
	// 供 loadFullConfig 回退的无关快照
	writePkgFile(t, pkg, "ApplicationFolder_0.9/x.txt", "x")
	writePkgFile(t, pkg, "ApplicationFolder_0.9/.updator/shared.json", sharedJSON)
	writePkgFile(t, pkg, "ApplicationFolder_2.0/only.txt", "only")

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":false`) {
		t.Fatalf("应拒绝（无可恢复内容），实际输出: %s", out)
	}
	if vi.VersionStatus != config.VersionStatusDownloaded {
		t.Errorf("无可挽救现场应降级 downloaded，实际 %+v", vi)
	}
	if vi.ApplyFailCount != 1 {
		t.Errorf("降级时必须递增 ApplyFailCount（M6），实际 %d", vi.ApplyFailCount)
	}
	assertPkgMissing(t, pkg, "ApplicationFolder")
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/only.txt")
}

// TestApplyRecoveryBothMissingDegrades M6：主目录与版本目录都不存在 → 降级 downloaded +
// ApplyFailCount++（此前只报错不写状态，check 持续要求 apply，主程序永不启动 = 活锁）。
func TestApplyRecoveryBothMissingDegrades(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applying"
}`)
	if err := os.RemoveAll(filepath.Join(pkg, "ApplicationFolder")); err != nil {
		t.Fatalf("删除 MainFolder 失败: %v", err)
	}
	// 无关快照提供 shared.json 回退
	writePkgFile(t, pkg, "ApplicationFolder_0.9/x.txt", "x")
	writePkgFile(t, pkg, "ApplicationFolder_0.9/.updator/shared.json", sharedJSON)

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":false`) || !strings.Contains(out, "neither main folder nor version folder exists") {
		t.Fatalf("应报告两边都缺失，实际输出: %s", out)
	}
	if vi.VersionStatus != config.VersionStatusDownloaded || vi.ApplyFailCount != 1 {
		t.Errorf("应降级 downloaded 且 ApplyFailCount=1，实际 %+v", vi)
	}
}

// TestApplyRecoveryVersionDirStatErrorFailsSafe M9：版本目录 stat 的错误不是"不存在"
// （ACL/网络盘瞬断/非法名字）时，必须 fail-safe 报错返回、不改状态、不做任何磁盘动作，
// 绝不能当成"缺失"继续走恢复（可能把真实主目录旁移成 {folderName}.old）。
func TestApplyRecoveryVersionDirStatErrorFailsSafe(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "bad*name",
  "version_status": "applying"
}`)
	writePkgFile(t, pkg, "ApplicationFolder/old.txt", "old")
	before := readFileOrFail(t, versionFilePath(pkg))

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":false`) || !strings.Contains(out, "版本目录状态不可读") {
		t.Fatalf("stat 错误应 fail-safe 报错，实际输出: %s", out)
	}
	if vi.VersionStatus != config.VersionStatusApplying {
		t.Errorf("fail-safe 路径不得改状态，实际 %q", vi.VersionStatus)
	}
	if after := readFileOrFail(t, versionFilePath(pkg)); after != before {
		t.Errorf("fail-safe 路径不得改写 version.json，改前=%q 改后=%q", before, after)
	}
	assertPkgExists(t, pkg, "ApplicationFolder/old.txt")
}

// TestApplyRecoveryCompletedBranchKeepsF5Heuristic B8 子路径③（109-127）：
// 主目录在 + 版本目录不在 = 激活已完成；保留 F5 历史启发式（不拒绝），
// 但弱校验"文件数 < 下载记录"必须留痕。
func TestApplyRecoveryCompletedBranchKeepsF5Heuristic(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applying",
  "downloaded_file_count": 3
}`)
	// 激活已完成：主目录存在（新版本内容），版本目录已被消耗
	writePkgFile(t, pkg, "ApplicationFolder/new.txt", "new")

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":true`) {
		t.Fatalf("F5 启发式下应判定为已完成，实际输出: %s", out)
	}
	if vi.VersionStatus != config.VersionStatusApplied || vi.Version != "2.0" {
		t.Errorf("应补写 applied/2.0，实际 %+v", vi)
	}
	logData, err := ioutil.ReadFile(filepath.Join(pkg, "UpdateFolder", "update.log"))
	if err != nil {
		t.Fatalf("读 update.log 失败: %v", err)
	}
	if !strings.Contains(string(logData), "少于下载记录") {
		t.Errorf("弱校验未通过时必须留痕，实际日志: %s", logData)
	}
}

// TestKeepApplyingAfterRecoveryFailureCounts M6：恢复失败但现场可重试时，保持 applying
// 并递增 ApplyFailCount（check_update 的 applying 早退分支据此熔断）。
func TestKeepApplyingAfterRecoveryFailureCounts(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applying"
}`)
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	keepApplyingAfterRecoveryFailure(vi, fmt.Errorf("rename occupied"))
	persisted, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if persisted.VersionStatus != config.VersionStatusApplying {
		t.Errorf("应保持 applying，实际 %q", persisted.VersionStatus)
	}
	if persisted.ApplyFailCount != 1 {
		t.Errorf("ApplyFailCount 应递增为 1，实际 %d", persisted.ApplyFailCount)
	}
	// 版本字段语义不被破坏
	if persisted.Version != "2.0" || persisted.VersionPrevious != "1.0" {
		t.Errorf("Version/VersionPrevious 不得被改写，实际 %+v", persisted)
	}
	_ = pkg
}
