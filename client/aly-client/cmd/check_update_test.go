package cmd

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aly/client/aly-client/config"
)

// TestNeedUpdate 验证统一版本判断口径（#6）：
// 仅当服务器版本严格高于本地版本时才需要更新，与 download_update 的
// compareVersion(newVersion, currentVersion) > 0 语义保持一致。
func TestNeedUpdate(t *testing.T) {
	tests := []struct {
		name    string
		server  string
		local   string
		want    bool
	}{
		{"服务器更高（补丁）", "2.0.1", "2.0.0", true},
		{"服务器更高（主版本）", "3.0.0", "2.9.9", true},
		{"相同版本", "2.0.0", "2.0.0", false},
		{"服务器更低（回滚发布）", "1.9.0", "2.0.0", false},
		{"服务器更低（主版本）", "1.0.0", "2.0.0", false},
		{"本地位数更多且更高", "2.0", "2.0.1", false},
	}
	for _, tt := range tests {
		got := needUpdate(tt.server, tt.local)
		if got != tt.want {
			t.Errorf("needUpdate(%q, %q) = %v, want %v", tt.server, tt.local, got, tt.want)
		}
	}
}

// ============================================================
// check_update：断网（服务器不可达）语义
// ============================================================

// checkUpdateResult 是 check_update 标准输出的最小解析模型。
type checkUpdateResult struct {
	IsSuccess bool `json:"isSuccess"`
	Data      struct {
		HasUpdate          bool   `json:"has_update"`
		NeedDownloadUpdate bool   `json:"need_download_update"`
		CurrentVersion     string `json:"current_version"`
		NewVersion         string `json:"new_version"`
	} `json:"data"`
}

// unreachableServer 返回一个"服务器不可达"的 server_url：变更日志请求返回 500，
// GetProjectChangeLogs 报错 → check_update 走 !serverReachable 分支。
// 用 httptest 而非关闭端口，保证失败判定确定且快速（不依赖系统端口行为）。
func unreachableServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// emptyLogsServer 返回一个"服务器可达但没有任何变更日志"的 server_url：
// GetProjectChangeLogs 成功但返回空列表 → serverReachable=false，与断网同一条分支。
func emptyLogsServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"isSuccess":true,"errorMsg":"","data":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// setupCheckUpdateLayout 构造 PackageFolder 布局（UpdateFolder/client.json + version.json、
// ApplicationFolder/.updator/shared.json），server_url 由调用方指定。
func setupCheckUpdateLayout(t *testing.T, versionJSON, serverURL string) string {
	t.Helper()
	root, err := ioutil.TempDir("", "check-update-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	writePkgFile(t, root, "UpdateFolder/client.json",
		`{"main_exe_relative_path":"../ApplicationFolder/app.exe","must_close_process_name":[]}`)
	writePkgFile(t, root, "UpdateFolder/version.json", versionJSON)
	writePkgFile(t, root, "ApplicationFolder/.updator/shared.json",
		`{"server_url":"`+serverURL+`","project_name":"test"}`)
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })
	return root
}

// runCheckUpdate 执行 CheckUpdate（os.Args 由调用方设置）并解析其 stdout 输出。
func runCheckUpdate(t *testing.T) checkUpdateResult {
	t.Helper()
	out := captureStdout(t, CheckUpdate)
	var got checkUpdateResult
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &got); err != nil {
		t.Fatalf("解析 check_update 输出失败: %v (输出=%q)", err, out)
	}
	return got
}

func readFileOrFail(t *testing.T, path string) string {
	t.Helper()
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return string(data)
}

// TestCheckUpdateOfflineDownloadedNotReported 验证断网 + downloaded 不再上报"待应用更新"：
// 已下载内容与 version.json 必须原样保留（check_update 只读），服务器可达后再继续 apply。
func TestCheckUpdateOfflineDownloadedNotReported(t *testing.T) {
	pkg := setupCheckUpdateLayout(t,
		`{"version_previous":"1.0","version":"2.0","version_status":"downloaded"}`,
		unreachableServer(t))
	writePkgFile(t, pkg, "ApplicationFolder/v1.txt", "v1")
	writePkgFile(t, pkg, "ApplicationFolder_2.0/v2.pending", "v2pend")

	versionPath := filepath.Join(pkg, "UpdateFolder", "version.json")
	before := readFileOrFail(t, versionPath)

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.IsSuccess {
		t.Errorf("断网 + downloaded 应安全返回 isSuccess=true，实际 false")
	}
	if got.Data.HasUpdate {
		t.Errorf("断网 + downloaded 不应上报待应用更新，实际 has_update=true")
	}
	if got.Data.NeedDownloadUpdate {
		t.Errorf("断网时不应要求下载，实际 need_download_update=true")
	}
	if got.Data.CurrentVersion != "1.0" {
		t.Errorf("current_version 应为 MainFolder 真实内容版本 1.0，实际 %q", got.Data.CurrentVersion)
	}
	if got.Data.NewVersion != "" {
		t.Errorf("不上报更新时不应输出 new_version，实际 %q", got.Data.NewVersion)
	}
	if after := readFileOrFail(t, versionPath); after != before {
		t.Errorf("check_update 不得改写 version.json，改前=%q 改后=%q", before, after)
	}
	assertPkgExists(t, pkg, "ApplicationFolder_2.0/v2.pending")
}

// TestCheckUpdateOfflineDownloadedEmptyPreviousVersion 复现现场：
// version.json = {version_previous:"", version:"1.0", version_status:"downloaded"} + 断网。
// 期望不上报待应用更新，且不再把待应用版本回退成 current_version
// （旧实现输出 current_version=1.0/new_version=1.0，自相矛盾）。
func TestCheckUpdateOfflineDownloadedEmptyPreviousVersion(t *testing.T) {
	pkg := setupCheckUpdateLayout(t,
		`{"version_previous":"","version":"1.0","version_status":"downloaded"}`,
		unreachableServer(t))
	writePkgFile(t, pkg, "ApplicationFolder_1.0/app.exe", "v1")

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.IsSuccess {
		t.Errorf("断网 + downloaded（previous 为空）应安全返回 isSuccess=true，实际 false")
	}
	if got.Data.HasUpdate {
		t.Errorf("断网 + downloaded（previous 为空）不应上报待应用更新，实际 has_update=true")
	}
	if got.Data.CurrentVersion != "" {
		t.Errorf("内容版本未知时应输出空 current_version，实际 %q", got.Data.CurrentVersion)
	}
	if got.Data.NewVersion != "" {
		t.Errorf("不上报更新时不应输出 new_version，实际 %q", got.Data.NewVersion)
	}
}

// TestCheckUpdateOfflineApplyingStillReported 验证 applying（崩溃恢复）在断网时照旧上报：
// 若被抑制，主目录已缺失的机器将永远无法恢复（apply_update 不会被 SDK 调用）。
func TestCheckUpdateOfflineApplyingStillReported(t *testing.T) {
	setupCheckUpdateLayout(t,
		`{"version_previous":"1.0","version":"2.0","version_status":"applying"}`,
		unreachableServer(t))

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.Data.HasUpdate {
		t.Errorf("applying 崩溃恢复在断网时必须上报 has_update=true，实际 false")
	}
	if got.Data.NeedDownloadUpdate {
		t.Errorf("applying 崩溃恢复不应要求下载（断网也下载不了），实际 need_download_update=true")
	}
	if got.Data.NewVersion != "2.0" {
		t.Errorf("new_version 应为待完成应用的 2.0，实际 %q", got.Data.NewVersion)
	}
}

// TestCheckUpdateOfflineRollbackResumeStillReported 验证回滚中断（applying + rollback_previous）
// 在断网时照旧上报：必须续跑回滚，绝不能因断网转向下载/升级，也绝不能不发命令。
func TestCheckUpdateOfflineRollbackResumeStillReported(t *testing.T) {
	setupCheckUpdateLayout(t,
		`{"version_previous":"2.0","version":"2.0","version_status":"applying","rollback_previous":"2.0","rollback_target":"1.0"}`,
		unreachableServer(t))

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.Data.HasUpdate {
		t.Errorf("回滚中断在断网时必须上报 has_update=true（否则回滚永远续不上），实际 false")
	}
	if got.Data.NeedDownloadUpdate {
		t.Errorf("回滚中断不应要求下载，实际 need_download_update=true")
	}
	if got.Data.CurrentVersion != "2.0" {
		t.Errorf("回滚前版本应为 2.0，实际 %q", got.Data.CurrentVersion)
	}
	if got.Data.NewVersion != "1.0" {
		t.Errorf("回滚目标应为 1.0，实际 %q", got.Data.NewVersion)
	}
}

// TestCheckUpdateEmptyChangeLogsNotReported 验证"服务器可达但没有任何变更日志"也按"无法核对"
// 处理（serverReachable = logsErr == nil && len(logs) > 0）：不上报待应用更新，不做版本比较。
func TestCheckUpdateEmptyChangeLogsNotReported(t *testing.T) {
	setupCheckUpdateLayout(t,
		`{"version_previous":"1.0","version":"2.0","version_status":"downloaded"}`,
		emptyLogsServer(t))

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.IsSuccess {
		t.Errorf("服务器可达但无变更日志时应安全返回 isSuccess=true，实际 false")
	}
	if got.Data.HasUpdate {
		t.Errorf("无可用变更日志时不应上报待应用更新，实际 has_update=true")
	}
	if got.Data.CurrentVersion != "1.0" {
		t.Errorf("current_version 应为实际内容版本 1.0，实际 %q", got.Data.CurrentVersion)
	}
}
