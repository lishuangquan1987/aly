package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aly/client/aly-client/config"
	"aly/client/aly-client/selfupdate"
)

// ioutilWrite 写测试现场文件（与既有用例同款：ioutil.WriteFile + 0755）。
func ioutilWrite(t *testing.T, path, content string) error {
	t.Helper()
	return ioutil.WriteFile(path, []byte(content), 0755)
}

// 本文件钉 self_update --include-ui 的图形前端补齐语义（与
// selfupdate.SyncFile 的 SyncPolicy 一一对应）：
//
//	目标缺失/零字节 → 从主程序目录落位（ui_reason=replaced）；
//	目标已存在     → 一律不动（ui_reason=existing_kepped）——不替换可能正在
//	                 运行的 UI 镜像，落后的副本由 UI 自己的启动自更新收敛；
//	收尾在途       → ui_reason=pending_inflight；
//	源缺失         → ui_reason=source_missing（不算错误）；
//	落位硬失败     → 命令报错（不把"两件套不齐"报成成功）；
//	不带旗标       → ui_* 三个字段必须缺省（既有调用方行为逐字节不变）。

// uiPathOf 返回 UpdateFolder 下同名的图形前端副本路径。
func uiPathOf(fc *FullConfig) string {
	return filepath.Clean(filepath.Join(fc.MainFolder, "..", "UpdateFolder", config.UIFileName()))
}

// uiSrcPathOf 返回主程序目录下的图形前端副本路径（随版本分发的权威源）。
func uiSrcPathOf(fc *FullConfig) string {
	return filepath.Join(fc.MainFolder, config.UIFileName())
}

// selfCopyPathOf 返回正在运行的更新器副本路径（UpdateFolder 下）。
func selfCopyPathOf(fc *FullConfig) string {
	return filepath.Clean(filepath.Join(fc.MainFolder, "..", "UpdateFolder", config.UpdaterFileName()))
}

// updaterSrcPathOf 返回主程序目录下的更新器副本路径。
func updaterSrcPathOf(fc *FullConfig) string {
	return filepath.Join(fc.MainFolder, config.UpdaterFileName())
}

// TestRunSelfUpdateIncludeUiBootstrapsUi UI 副本缺失时从主目录落位。
func TestRunSelfUpdateIncludeUiBootstrapsUi(t *testing.T) {
	t.Helper()
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-ui-bootstrap")
	withSelfPath(t, selfCopyPathOf(fc))

	// 更新器两边一致 → 本体跳过（本用例只关注 UI 部分）
	if err := ioutilWrite(t, selfCopyPathOf(fc), "V1"); err != nil {
		t.Fatalf("写 self: %v", err)
	}
	if err := ioutilWrite(t, updaterSrcPathOf(fc), "V1"); err != nil {
		t.Fatalf("写更新器源: %v", err)
	}
	// UI：主目录有、UpdateFolder 没有（现场故障形态）
	if err := ioutilWrite(t, uiSrcPathOf(fc), "UI-V2"); err != nil {
		t.Fatalf("写 UI 源: %v", err)
	}

	data, err := runSelfUpdateWithUI(fc, true)
	if err != nil {
		t.Fatalf("runSelfUpdateWithUI: %v", err)
	}
	if data.Updated || data.Reason != "already_up_to_date" {
		t.Fatalf("更新器本体应一致跳过, data=%+v", data)
	}
	if !data.UIUpdated || data.UIReason != "replaced" {
		t.Fatalf("UI 应被补齐, data=%+v", data)
	}
	if got := readFileOrFail(t, uiPathOf(fc)); got != "UI-V2" {
		t.Fatalf("UI 副本内容 = %q; want UI-V2", got)
	}
}

// TestRunSelfUpdateIncludeUiExistingKepped UI 副本已存在（含落后）时一律不动。
func TestRunSelfUpdateIncludeUiExistingKepped(t *testing.T) {
	t.Helper()
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-ui-kepped")
	withSelfPath(t, selfCopyPathOf(fc))

	if err := ioutilWrite(t, selfCopyPathOf(fc), "V1"); err != nil {
		t.Fatalf("写 self: %v", err)
	}
	if err := ioutilWrite(t, updaterSrcPathOf(fc), "V1"); err != nil {
		t.Fatalf("写更新器源: %v", err)
	}
	// 主目录 UI 比副本新，但副本存在（可能正在运行）→ 不得替换
	if err := ioutilWrite(t, uiSrcPathOf(fc), "UI-V2"); err != nil {
		t.Fatalf("写 UI 源: %v", err)
	}
	if err := ioutilWrite(t, uiPathOf(fc), "UI-V1"); err != nil {
		t.Fatalf("写 UI 副本: %v", err)
	}

	data, err := runSelfUpdateWithUI(fc, true)
	if err != nil {
		t.Fatalf("runSelfUpdateWithUI: %v", err)
	}
	if data.UIUpdated || data.UIReason != "existing_kepped" {
		t.Fatalf("已存在的 UI 副本必须原样不动, data=%+v", data)
	}
	if got := readFileOrFail(t, uiPathOf(fc)); got != "UI-V1" {
		t.Fatalf("UI 副本内容 = %q; want 未改动（可能正在运行的镜像不得被替换）", got)
	}
}

// TestRunSelfUpdateIncludeUiSourceMissing 两件套在主目录都缺失：两个 source_missing，命令成功。
func TestRunSelfUpdateIncludeUiSourceMissing(t *testing.T) {
	t.Helper()
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-ui-nosrc")
	withSelfPath(t, selfCopyPathOf(fc))
	if err := ioutilWrite(t, selfCopyPathOf(fc), "V1"); err != nil {
		t.Fatalf("写 self: %v", err)
	}

	data, err := runSelfUpdateWithUI(fc, true)
	if err != nil {
		t.Fatalf("源缺失不是错误: %v", err)
	}
	if data.Reason != "source_missing" {
		t.Fatalf("更新器 reason = %q; want source_missing", data.Reason)
	}
	if data.UIUpdated || data.UIReason != "source_missing" {
		t.Fatalf("UI 源缺失口径不对, data=%+v", data)
	}
	if got := readFileOrFail(t, selfCopyPathOf(fc)); got != "V1" {
		t.Fatalf("self 必须原样, got %q", got)
	}
}

// TestRunSelfUpdateIncludeUiPendingSkips 收尾进程在途（journal pending）时让位。
func TestRunSelfUpdateIncludeUiPendingSkips(t *testing.T) {
	t.Helper()
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-ui-pending")
	withSelfPath(t, selfCopyPathOf(fc))
	if err := ioutilWrite(t, selfCopyPathOf(fc), "V1"); err != nil {
		t.Fatalf("写 self: %v", err)
	}
	if err := ioutilWrite(t, updaterSrcPathOf(fc), "V1"); err != nil {
		t.Fatalf("写更新器源: %v", err)
	}
	if err := ioutilWrite(t, uiSrcPathOf(fc), "UI-V2"); err != nil {
		t.Fatalf("写 UI 源: %v", err)
	}
	if err := ioutilWrite(t, uiPathOf(fc), "UI-V1"); err != nil {
		t.Fatalf("写 UI 副本: %v", err)
	}
	selfupdate.SaveJournal(selfupdate.JournalPath(uiPathOf(fc)),
		selfupdate.Journal{Pending: true, Count: 1})

	data, err := runSelfUpdateWithUI(fc, true)
	if err != nil {
		t.Fatalf("runSelfUpdateWithUI: %v", err)
	}
	if data.UIReason != "pending_inflight" {
		t.Fatalf("在途收尾必须被让位, data=%+v", data)
	}
	if got := readFileOrFail(t, uiPathOf(fc)); got != "UI-V1" {
		t.Fatalf("UI 副本内容 = %q; want 未改动", got)
	}
}

// TestRunSelfUpdateWithoutFlagKeepsUiFieldsEmpty 不带旗标时 ui_* 缺省（回归）。
func TestRunSelfUpdateWithoutFlagKeepsUiFieldsEmpty(t *testing.T) {
	t.Helper()
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-ui-flag-off")
	withSelfPath(t, selfCopyPathOf(fc))
	if err := ioutilWrite(t, selfCopyPathOf(fc), "V1"); err != nil {
		t.Fatalf("写 self: %v", err)
	}
	if err := ioutilWrite(t, uiSrcPathOf(fc), "UI-V2"); err != nil {
		t.Fatalf("写 UI 源: %v", err)
	}

	data, err := runSelfUpdate(fc)
	if err != nil {
		t.Fatalf("runSelfUpdate: %v", err)
	}
	if data.UIUpdated || data.UIReason != "" || data.UIOldPath != "" {
		t.Fatalf("不带 --include-ui 时 ui_* 必须缺省, data=%+v", data)
	}
	if _, err := os.Stat(uiPathOf(fc)); !os.IsNotExist(err) {
		t.Fatalf("不带旗标时不得动 UI 副本 (err=%v)", err)
	}
}

// TestRunSelfUpdateIncludeUiHardFailureIsReported UI 落位硬失败必须上报为命令失败。
func TestRunSelfUpdateIncludeUiHardFailureIsReported(t *testing.T) {
	t.Helper()
	_, fc := newSelfUpdateTestEnv(t, "selfupdate-ui-hardfail")
	withSelfPath(t, selfCopyPathOf(fc))
	if err := ioutilWrite(t, selfCopyPathOf(fc), "V1"); err != nil {
		t.Fatalf("写 self: %v", err)
	}
	if err := ioutilWrite(t, updaterSrcPathOf(fc), "V1"); err != nil {
		t.Fatalf("写更新器源: %v", err)
	}
	// 用"同名目录"冒充 UI 源：复制必然失败 → 命令必须报错（不静默成功）
	if err := os.MkdirAll(uiSrcPathOf(fc), 0755); err != nil {
		t.Fatalf("建同名目录: %v", err)
	}

	_, err := runSelfUpdateWithUI(fc, true)
	if err == nil {
		t.Fatal("UI 落位硬失败必须上报（否则调用方会把两件套不齐当成功）")
	}
	if !strings.Contains(err.Error(), "sync ui") {
		t.Fatalf("error = %v; want a sync ui prefixed failure", err)
	}
}
