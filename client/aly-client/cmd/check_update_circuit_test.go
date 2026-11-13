package cmd

import (
	"os"
	"testing"

	"aly/client/aly-client/model"
)

// TestCheckUpdateApplyingCircuitBroken M6：applying 早退分支必须受 ApplyFailCount 熔断约束——
// 此前该分支在 :236 的熔断检查之前无条件返回 has_update=true，恢复失败的机器会被
// 无限"杀掉主程序→apply→再 check"。熔断后返回 has_update=false（不再自动 apply）。
func TestCheckUpdateApplyingCircuitBroken(t *testing.T) {
	serverURL := logsServer(t, "2.0")
	root := setupCheckUpdateLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applying",
  "apply_fail_count": 3
}`, serverURL)
	_ = root

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.IsSuccess {
		t.Fatalf("check_update 应成功输出信封")
	}
	if got.Data.HasUpdate {
		t.Errorf("applying 熔断后不应再上报 has_update=true，实际 %+v", got.Data)
	}
	if got.Data.NeedDownloadUpdate {
		t.Errorf("熔断后不应要求下载，实际 %+v", got.Data)
	}
}

// TestCheckUpdateRollbackResumeCircuitBroken M6：回滚续跑早退分支同样必须受熔断约束，
// 否则 resumeRollback 的连续失败永远无法让宿主停止调用 apply。
func TestCheckUpdateRollbackResumeCircuitBroken(t *testing.T) {
	serverURL := logsServer(t, "2.0")
	root := setupCheckUpdateLayout(t, `{
  "version_previous": "2.0",
  "version": "3.0",
  "version_status": "applying",
  "rollback_previous": "2.0",
  "rollback_target": "1.0",
  "apply_fail_count": 3
}`, serverURL)
	_ = root

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if got.Data.HasUpdate {
		t.Errorf("回滚续跑熔断后不应再上报 has_update=true，实际 %+v", got.Data)
	}
}

// TestCheckUpdateApplyingBelowThresholdStillReported 熔断阈值以下仍应上报续跑（避免误熔断）。
func TestCheckUpdateApplyingBelowThresholdStillReported(t *testing.T) {
	serverURL := unreachableServer(t)
	root := setupCheckUpdateLayout(t, `{
  "version_previous": "1.0",
  "version": "2.0",
  "version_status": "applying",
  "apply_fail_count": 2
}`, serverURL)
	_ = root

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.Data.HasUpdate || got.Data.NeedDownloadUpdate {
		t.Errorf("applying 未达阈值（且断网）仍应上报 (true,false)，实际 %+v", got.Data)
	}
	if got.Data.NewVersion != "2.0" {
		t.Errorf("NewVersion 应为待应用版本 2.0，实际 %q", got.Data.NewVersion)
	}
}

// TestFindLatestLogEmptyGuard 轻微项：空 logs 切片不得 panic（findLatestLog 原实现直接 logs[0]）。
func TestFindLatestLogEmptyGuard(t *testing.T) {
	if got := findLatestLog(nil); got.Version != "" || got.ID != 0 {
		t.Errorf("nil logs 应返回零值，实际 %+v", got)
	}
	if got := findLatestLog([]model.ProjectChangeLog{}); got.Version != "" || got.ID != 0 {
		t.Errorf("空 logs 应返回零值，实际 %+v", got)
	}
}
