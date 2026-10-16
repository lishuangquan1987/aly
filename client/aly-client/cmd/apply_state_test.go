package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aly/client/aly-client/config"
)

// 本文件覆盖 apply_update 的"状态守卫 / 现场校验"（F1–F6）与 prune 的历史残留清理（F2）。
// 复用 rollback_test.go 的布局 helper：setupRollbackLayout / writePkgFile / assertPkgExists /
// assertPkgMissing / captureStdout，以及 check_update_test.go 的 readFileOrFail。

// applyOnce 执行一次 apply_update 并返回 (stdout, version.json)。
func applyOnce(t *testing.T) (string, *config.VersionInfo) {
	t.Helper()
	os.Args = []string{"aly-client", "apply_update"}
	out := captureStdout(t, ApplyUpdate)
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	return out, vi
}

func versionFilePath(pkg string) string {
	return filepath.Join(pkg, "UpdateFolder", "version.json")
}

// TestApplyRejectsEmptyStatus 验证 F1：空状态（version.json 缺失 / 内容为 {} / 损坏自愈重置）
// 与 applied 同义 → 拒绝替换，不再出现"把 MainFolder 改名到自己"的假成功。
func TestApplyRejectsEmptyStatus(t *testing.T) {
	cases := []struct {
		name       string
		removeFile bool
	}{
		{"version.json 为 {}", false},
		{"version.json 不存在", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pkg := setupRollbackLayout(t, `{}`)
			if c.removeFile {
				if err := os.Remove(versionFilePath(pkg)); err != nil {
					t.Fatalf("删除 version.json 失败: %v", err)
				}
			}
			writePkgFile(t, pkg, "ApplicationFolder/app.txt", "old")
			writePkgFile(t, pkg, "ApplicationFolder_2.0/new.txt", "new")

			out, vi := applyOnce(t)

			if !strings.Contains(out, `"isSuccess":false`) {
				t.Errorf("空状态应拒绝替换，实际输出: %s", out)
			}
			if !strings.Contains(out, "no pending update to apply") {
				t.Errorf("应提示 no pending update to apply，实际输出: %s", out)
			}
			if vi.VersionStatus != "" && vi.VersionStatus != config.VersionStatusApplied {
				t.Errorf("空状态不得被改写成其它状态，实际 %q", vi.VersionStatus)
			}
			// 磁盘未被破坏：主目录仍是旧内容、版本目录未被消耗、未产生无名备份目录
			assertPkgExists(t, pkg, "ApplicationFolder/app.txt")
			assertPkgExists(t, pkg, "ApplicationFolder_2.0/new.txt")
			assertPkgMissing(t, pkg, "ApplicationFolder_")
		})
	}
}

// TestApplyRejectsUnknownStatus 验证 F6：未知 version_status 不再 fall through 到替换流程，
// 也不猜测、不改写状态（统一口径：未知状态只允许 download_update 重写成 downloaded 修复）。
func TestApplyRejectsUnknownStatus(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "Downloaded"
}`)
	writePkgFile(t, pkg, "ApplicationFolder/old.txt", "old")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/new.txt", "new")
	before := readFileOrFail(t, versionFilePath(pkg))

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":false`) || !strings.Contains(out, "unknown version_status") {
		t.Errorf("未知状态应被拒绝并明确报错，实际输出: %s", out)
	}
	// 状态与磁盘都不得被改动
	if vi.VersionStatus != "Downloaded" {
		t.Errorf("未知状态不得被改写，实际 %q", vi.VersionStatus)
	}
	if after := readFileOrFail(t, versionFilePath(pkg)); after != before {
		t.Errorf("未知状态不得改写 version.json，改前=%q 改后=%q", before, after)
	}
	assertPkgExists(t, pkg, "ApplicationFolder/old.txt")
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/new.txt")
}

// TestApplyRejectsEqualVersionsBeforeTouchingDisk 验证 F4 的应用层守卫：
// downloaded 且 Version == VersionPrevious（异常/遗留状态）时，在写 applying / 关进程 /
// 改名之前就拒绝，避免"拒绝 → 兜底回 downloaded → check 仍要求 apply"的反复杀进程循环。
func TestApplyRejectsEqualVersionsBeforeTouchingDisk(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "2.0",
  "version": "2.0",
  "version_status": "downloaded"
}`)
	writePkgFile(t, pkg, "ApplicationFolder/old.txt", "old")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/new.txt", "new")
	before := readFileOrFail(t, versionFilePath(pkg))

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":false`) || !strings.Contains(out, "VersionPrevious") {
		t.Errorf("应拒绝 Version == VersionPrevious 的替换，实际输出: %s", out)
	}
	if vi.VersionStatus != config.VersionStatusDownloaded {
		t.Errorf("拒绝后状态应保持 downloaded，实际 %q", vi.VersionStatus)
	}
	if after := readFileOrFail(t, versionFilePath(pkg)); after != before {
		t.Errorf("拒绝路径不得改写 version.json，改前=%q 改后=%q", before, after)
	}
	assertPkgExists(t, pkg, "ApplicationFolder/old.txt")
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/new.txt")
}

// TestApplyRepairsMissingPendingDir 验证 F3：待应用版本目录缺失或为空时拒绝替换（否则会把
// MainFolder 自拷贝进版本目录后改名，报"更新成功"但内容没变 = 假成功），并把状态自愈为
// applied + 真实内容版本 Previous，让下一轮 check 重新下载。
func TestApplyRepairsMissingPendingDir(t *testing.T) {
	cases := []struct {
		name      string
		createDir bool
	}{
		{"目录不存在", false},
		{"目录存在但为空", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "downloaded"
}`)
			writePkgFile(t, pkg, "ApplicationFolder/v1.txt", "v1")
			if c.createDir {
				if err := os.MkdirAll(filepath.Join(pkg, "ApplicationFolder_2.0"), 0755); err != nil {
					t.Fatalf("创建待应用目录失败: %v", err)
				}
			}

			out, vi := applyOnce(t)

			if !strings.Contains(out, `"isSuccess":false`) || !strings.Contains(out, "待应用版本目录缺失") {
				t.Errorf("应拒绝并说明原因，实际输出: %s", out)
			}
			// 自愈：applied + Version 回退为真实内容版本 1.0
			if vi.VersionStatus != config.VersionStatusApplied || vi.Version != "1.0" {
				t.Errorf("应自愈为 applied/1.0，实际 %+v", vi)
			}
			// 主目录内容未被改动（没有发生"自拷贝改名"）
			assertPkgExists(t, pkg, "ApplicationFolder/v1.txt")
			assertPkgMissing(t, pkg, "ApplicationFolder_")
		})
	}
}

// TestApplyRejectsEmptyVersionDownloaded 验证 A1：downloaded 但 Version 为空属异常/遗留状态。
// AppVersionDir("") 会退化成无版本号备份目录（ApplicationFolder_），把它当待应用版本会装入
// 陈旧内容或做一次"改名到自己"的假成功 → 必须拒绝并自愈为 applied + 真实内容版本。
func TestApplyRejectsEmptyVersionDownloaded(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "",
  "version_status": "downloaded"
}`)
	writePkgFile(t, pkg, "ApplicationFolder/cur.txt", "CUR")
	// 遗留的无名备份目录（有内容）——绝不能被当成待应用版本激活
	writePkgFile(t, pkg, "ApplicationFolder_/stale.txt", "STALE")

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":false`) || !strings.Contains(out, "Version 为空") {
		t.Errorf("Version 为空时应拒绝替换，实际输出: %s", out)
	}
	if vi.VersionStatus != config.VersionStatusApplied || vi.Version != "1.0" {
		t.Errorf("应自愈为 applied/1.0，实际 %+v", vi)
	}
	// 主目录内容未被替换（没有激活陈旧备份目录）
	assertPkgExists(t, pkg, "ApplicationFolder/cur.txt")
	assertPkgMissing(t, pkg, "ApplicationFolder/stale.txt")
	assertPkgExists(t, pkg, "ApplicationFolder_/stale.txt")
}

// TestApplyRejectsIncompleteDownloadedDir 验证 B2：待应用目录文件数少于下载完成时记录的数量
// （目录还在但文件被删 / 同版本重下中途被杀）时拒绝替换，避免混装旧/新文件的半成品。
func TestApplyRejectsIncompleteDownloadedDir(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "downloaded",
  "downloaded_file_count": 3
}`)
	writePkgFile(t, pkg, "ApplicationFolder/v1.txt", "v1")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/only-one.txt", "one") // 记录 3 个文件，实际只有 1 个

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":false`) || !strings.Contains(out, "不完整") {
		t.Errorf("残缺产物应拒绝替换，实际输出: %s", out)
	}
	if vi.VersionStatus != config.VersionStatusApplied || vi.Version != "1.0" {
		t.Errorf("应自愈为 applied/1.0，实际 %+v", vi)
	}
	assertPkgExists(t, pkg, "ApplicationFolder/v1.txt")
}

// TestApplyAcceptsCountedDownloadedDir 验证 B2 的正向：文件数达到记录值（.part/.tmp 不计入）
// 时正常替换。
func TestApplyAcceptsCountedDownloadedDir(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "downloaded",
  "downloaded_file_count": 2
}`)
	writePkgFile(t, pkg, "ApplicationFolder/v1.txt", "v1")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/a.txt", "a")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/b.txt", "b")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/inflight.bin.part", "half") // 临时文件不计入

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":true`) {
		t.Fatalf("文件数达标时 apply 应成功，实际输出: %s", out)
	}
	if vi.VersionStatus != config.VersionStatusApplied || vi.Version != "2.0" {
		t.Errorf("应为 applied/2.0，实际 %+v", vi)
	}
	assertPkgExists(t, pkg, "ApplicationFolder/a.txt")
	assertPkgExists(t, pkg, "ApplicationFolder/b.txt")
	// 上一版本备份以 VersionPrevious 命名
	assertPkgExists(t, pkg, "ApplicationFolder_1.0/v1.txt")
}

// TestApplyFirstUpdateCleansOrphanBackup 验证 F2：Previous 为空的首次更新完成后，
// 不残留"无版本号备份目录"（ApplicationFolder_）——它不可回滚、不会被 prune 清理，
// 却占一份完整应用体积。
func TestApplyFirstUpdateCleansOrphanBackup(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "",
  "version": "1.0",
  "version_status": "downloaded"
}`)
	writePkgFile(t, pkg, "ApplicationFolder/marker.txt", "OLD")
	writePkgFile(t, pkg, "ApplicationFolder_1.0/marker.txt", "NEW")

	out, vi := applyOnce(t)

	if !strings.Contains(out, `"isSuccess":true`) {
		t.Fatalf("正常 apply 应成功，实际输出: %s", out)
	}
	if vi.VersionStatus != config.VersionStatusApplied || vi.Version != "1.0" {
		t.Errorf("应为 applied/1.0，实际 %+v", vi)
	}
	if got := readFileOrFail(t, filepath.Join(pkg, "ApplicationFolder", "marker.txt")); got != "NEW" {
		t.Errorf("主目录应为 NEW，实际 %q", got)
	}
	assertPkgMissing(t, pkg, "ApplicationFolder_1.0")
	assertPkgMissing(t, pkg, "ApplicationFolder_")
}

// TestApplyCrashRecoveryUsesApplyStageEvidence 验证 F5：applying + 主目录在 + 版本目录不在
// 时，带 apply_stage=activating 证据（改名确实发生过）→ 补写 applied；
// 老现场（无该字段）→ 仍按历史启发式判定，但必须在 update.log 留痕。
func TestApplyCrashRecoveryUsesApplyStageEvidence(t *testing.T) {
	cases := []struct {
		name      string
		versionJS string
		wantLog   bool
	}{
		{
			"带 activating 证据",
			`{"version_previous":"1.0","version":"2.0","version_status":"applying","apply_stage":"activating"}`,
			false,
		},
		{
			"老现场无证据（历史启发式）",
			`{"version_previous":"1.0","version":"2.0","version_status":"applying"}`,
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pkg := setupRollbackLayout(t, c.versionJS)
			// 现场：主目录存在（已是新版本内容），版本目录不存在（改名已完成）
			writePkgFile(t, pkg, "ApplicationFolder/new.txt", "new")

			out, vi := applyOnce(t)

			if !strings.Contains(out, `"isSuccess":true`) {
				t.Errorf("崩溃恢复应成功，实际输出: %s", out)
			}
			if vi.VersionStatus != config.VersionStatusApplied || vi.Version != "2.0" {
				t.Errorf("应补写 applied/2.0，实际 %+v", vi)
			}
			if vi.ApplyStage != "" {
				t.Errorf("终态必须清空 apply_stage，实际 %q", vi.ApplyStage)
			}
			logPath := filepath.Join(pkg, "UpdateFolder", "update.log")
			hasLog := false
			if data, err := ioutil.ReadFile(logPath); err == nil {
				hasLog = strings.Contains(string(data), "apply_stage=activating")
			}
			if c.wantLog && !hasLog {
				t.Errorf("缺少证据时应写 update.log 留痕，实际未见（%s）", logPath)
			}
			if !c.wantLog && hasLog {
				t.Errorf("有证据时不应再写“缺少证据”告警")
			}
		})
	}
}

// TestPruneRemovesOrphanBackupDir 验证 F2 的历史残留清理：
// Version 与 VersionPrevious 都非空时，无版本号备份目录不可能被引用 → prune 入口清掉。
func TestPruneRemovesOrphanBackupDir(t *testing.T) {
	pkg := setupRollbackLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applied"
}`)
	writePkgFile(t, pkg, "ApplicationFolder/m.txt", "cur")
	writePkgFile(t, pkg, "ApplicationFolder_1.0/m.txt", "prev")
	writePkgFile(t, pkg, "ApplicationFolder_/m.txt", "orphan")
	writePkgFile(t, pkg, "ApplicationFolder_.old/m.txt", "orphan-aside")

	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(pkg, "ApplicationFolder"),
	}
	pruneVersionSnapshots(fc, defaultSnapshotKeep)

	assertPkgMissing(t, pkg, "ApplicationFolder_")
	assertPkgMissing(t, pkg, "ApplicationFolder_.old")
	// 受保护/正常快照不受影响
	assertPkgExists(t, pkg, "ApplicationFolder_1.0/m.txt")
	assertPkgExists(t, pkg, "ApplicationFolder/m.txt")
}

// TestPruneKeepsOrphanBackupWhenVersionOrPreviousEmpty 验证 F2 的保护条件：
// Version 或 VersionPrevious 为空时，无版本号目录仍可能被当作版本目录引用 → 绝不删除。
func TestPruneKeepsOrphanBackupWhenVersionOrPreviousEmpty(t *testing.T) {
	for _, versionJSON := range []string{
		`{"version_previous":"","version":"1.0","version_status":"applied"}`,
		`{"version_previous":"1.0","version":"","version_status":"applied"}`,
	} {
		pkg := setupRollbackLayout(t, versionJSON)
		writePkgFile(t, pkg, "ApplicationFolder_/m.txt", "orphan")

		fc := &FullConfig{
			ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
			MainFolder: filepath.Join(pkg, "ApplicationFolder"),
		}
		pruneVersionSnapshots(fc, defaultSnapshotKeep)

		assertPkgExists(t, pkg, "ApplicationFolder_/m.txt")
	}
}
