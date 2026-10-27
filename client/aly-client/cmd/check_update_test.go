package cmd

import (
	"encoding/json"
	"fmt"
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
		HasUpdate          bool     `json:"has_update"`
		NeedDownloadUpdate bool     `json:"need_download_update"`
		CurrentVersion     string   `json:"current_version"`
		NewVersion         string   `json:"new_version"`
		ReleaseNotes       []string `json:"release_notes"`
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

// logsServer 返回一个"服务器可达且含变更日志"的 server_url：GetProjectChangeLogs 返回给定版本列表。
func logsServer(t *testing.T, versions ...string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		items := make([]string, 0, len(versions))
		for i, v := range versions {
			items = append(items, fmt.Sprintf(`{"id":%d,"version":%q}`, i+1, v))
		}
		w.Write([]byte(`{"isSuccess":true,"errorMsg":"","data":[` + strings.Join(items, ",") + `]}`))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// notesServer 返回一个"服务器有新版本且带变更内容"的 server_url：
// 变更日志里除 version 外还带 logs 数组，用于验证 check_update 是否把发布内容透传出去。
// 注意 applied 分支要求 project 接口也成功（serverReachable 需要 projectErr == nil），
// 因此这里对 get_project_by_name 返回一个合法项目对象。
func notesServer(t *testing.T, version string, notes []string) string {
	t.Helper()
	logsJSON, err := json.Marshal(notes)
	if err != nil {
		t.Fatalf("序列化 logs 失败: %v", err)
	}
	logsBody := `{"isSuccess":true,"errorMsg":"","data":[{"id":1,"version":"` + version + `","logs":` + string(logsJSON) + `}]}`
	projectBody := `{"isSuccess":true,"errorMsg":"","data":{"id":1,"name":"test","title":"test","version":"` +
		version + `","force_update":false}}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "change_logs") {
			w.Write([]byte(logsBody))
			return
		}
		w.Write([]byte(projectBody))
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

// TestCheckUpdateInvalidEqualVersionsNotReported 验证 F4：downloaded 且
// Version == VersionPrevious（异常/遗留状态，apply 会拒绝替换）时，check 不再上报
// "待应用更新"，避免 SDK 反复 apply → 拒绝 → 兜底回 downloaded 的反复杀进程循环。
func TestCheckUpdateInvalidEqualVersionsNotReported(t *testing.T) {
	pkg := setupCheckUpdateLayout(t,
		`{"version_previous":"2.0","version":"2.0","version_status":"downloaded"}`,
		logsServer(t, "2.0"))
	// 待应用目录存在且非空 → 走 F4 非法状态判定（而不是 F3"产物丢失→重下"）
	writePkgFile(t, pkg, "ApplicationFolder_2.0/x.txt", "x")

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.IsSuccess {
		t.Errorf("非法状态应安全返回 isSuccess=true，实际 false")
	}
	if got.Data.HasUpdate {
		t.Errorf("Version == VersionPrevious 时不应上报待应用更新，实际 has_update=true")
	}
	if got.Data.NeedDownloadUpdate {
		t.Errorf("服务器无更高版本时不应要求下载，实际 need_download_update=true")
	}
	if got.Data.CurrentVersion != "2.0" {
		t.Errorf("current_version 应为 2.0，实际 %q", got.Data.CurrentVersion)
	}
}

// TestCheckUpdatePendingDirMissingRequestsRedownload 验证 F3（check 侧）：downloaded 状态但
// 待应用版本目录缺失/为空（下载产物丢失），且服务器版本正好是该待应用版本 →
// 要求重新下载（download_update 允许同版本重下补齐）。
func TestCheckUpdatePendingDirMissingRequestsRedownload(t *testing.T) {
	setupCheckUpdateLayout(t,
		`{"version_previous":"1.0","version":"2.0","version_status":"downloaded"}`,
		logsServer(t, "2.0"))

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.Data.HasUpdate || !got.Data.NeedDownloadUpdate {
		t.Errorf("待应用目录丢失且服务器同版本时应要求重新下载，实际 has=%v need=%v",
			got.Data.HasUpdate, got.Data.NeedDownloadUpdate)
	}
	if got.Data.NewVersion != "2.0" {
		t.Errorf("new_version 应为待应用版本 2.0，实际 %q", got.Data.NewVersion)
	}
	if got.Data.CurrentVersion != "1.0" {
		t.Errorf("current_version 应为真实内容版本 1.0，实际 %q", got.Data.CurrentVersion)
	}
}

// TestCheckUpdatePendingDirMissingServerLowerFallsThroughToApply 验证 F3 的边界（B3）：
// 待应用目录丢失但服务器版本**低于**待应用版本（例如回滚发布后该版本的变更日志被删除）时，
// 不能要求重下（download 会因"already at latest version"拒绝，永远补不回来），
// 必须落到"继续 apply"，由 apply_update 的 F3 守卫拒绝并自愈为 applied + Previous。
func TestCheckUpdatePendingDirMissingServerLowerFallsThroughToApply(t *testing.T) {
	setupCheckUpdateLayout(t,
		`{"version_previous":"1.0","version":"2.0","version_status":"downloaded"}`,
		logsServer(t, "1.5"))

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.Data.HasUpdate || got.Data.NeedDownloadUpdate {
		t.Errorf("服务器低于待应用版本时应交给 apply 自愈（true/false），实际 has=%v need=%v",
			got.Data.HasUpdate, got.Data.NeedDownloadUpdate)
	}
	if got.Data.NewVersion != "2.0" {
		t.Errorf("new_version 应为待应用版本 2.0，实际 %q", got.Data.NewVersion)
	}
}

// TestDownloadNormalizesInvalidEqualVersions 验证 F4 的修复路径：downloaded 且
// Version == VersionPrevious 时，按 downloaded 不变式（主目录内容 == VersionPrevious）状态等价于
// applied → download_update 直接归一化为 applied（不下载、不空转），并输出标准完成行。
func TestDownloadNormalizesInvalidEqualVersions(t *testing.T) {
	pkg := setupCheckUpdateLayout(t,
		`{"version_previous":"2.0","version":"2.0","version_status":"downloaded"}`,
		logsServer(t, "2.0"))

	os.Args = []string{"aly-client", "download_update"}
	out := captureStdout(t, DownloadUpdate)

	if !strings.Contains(out, `"data":null`) {
		t.Errorf("归一化路径应以标准完成行收尾，实际输出: %s", out)
	}
	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi.VersionStatus != config.VersionStatusApplied || vi.Version != "2.0" {
		t.Errorf("应归一化为 applied/2.0，实际 %+v", vi)
	}
	if vi.VersionPrevious != "2.0" {
		t.Errorf("归一化不得改写 VersionPrevious，实际 %q", vi.VersionPrevious)
	}
	// 未发生下载：不应凭空创建版本目录
	assertPkgMissing(t, pkg, "ApplicationFolder_2.0")
}

// TestDownloadDoesNotNormalizeWhenServerHasNewerVersion 验证 B1：非法状态（downloaded 且
// Version == VersionPrevious）只在"服务器没有更高版本"时才归一化为 applied；服务器已发新版时
// 必须照常下载更高版本 —— 否则本次下载会变成空操作，紧随其后的 apply 只会报"无待应用更新"。
func TestDownloadDoesNotNormalizeWhenServerHasNewerVersion(t *testing.T) {
	setupCheckUpdateLayout(t,
		`{"version_previous":"2.0","version":"2.0","version_status":"downloaded"}`,
		logsServer(t, "3.0"))

	os.Args = []string{"aly-client", "download_update"}
	captureStdout(t, DownloadUpdate)

	vi, err := config.ReadVersion()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi.VersionStatus == config.VersionStatusApplied {
		t.Errorf("服务器有更高版本时不得归一化为 applied（会使本次下载变成空操作），实际 %+v", vi)
	}
}

// TestCheckUpdateInvalidEqualVersionsRedownloadsWhenServerNewer 验证 F4 的自愈出路：
// 同一非法状态在服务器有更高版本时仍走重新下载（download_update 会把 Version 改成新版本，
// 从而修好 Version == VersionPrevious）。
func TestCheckUpdateInvalidEqualVersionsRedownloadsWhenServerNewer(t *testing.T) {
	setupCheckUpdateLayout(t,
		`{"version_previous":"2.0","version":"2.0","version_status":"downloaded"}`,
		logsServer(t, "3.0"))

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.Data.HasUpdate || !got.Data.NeedDownloadUpdate {
		t.Errorf("服务器更高时应上报 need_download_update=true，实际 has=%v need=%v",
			got.Data.HasUpdate, got.Data.NeedDownloadUpdate)
	}
	if got.Data.NewVersion != "3.0" {
		t.Errorf("new_version 应为 3.0，实际 %q", got.Data.NewVersion)
	}
}

// TestCheckUpdateDownloadedCurrentVersionIsPrevious 验证 F10：联网分支的 current_version
// 与断网分支统一为"MainFolder 真实内容版本（VersionPrevious）"。
func TestCheckUpdateDownloadedCurrentVersionIsPrevious(t *testing.T) {
	pkg := setupCheckUpdateLayout(t,
		`{"version_previous":"1.0","version":"2.0","version_status":"downloaded"}`,
		logsServer(t, "1.0"))
	// 待应用目录存在且非空（否则会走 F3"产物丢失→重下"分支，测不到本口径）
	writePkgFile(t, pkg, "ApplicationFolder_2.0/x.txt", "x")

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.Data.HasUpdate || got.Data.NeedDownloadUpdate {
		t.Errorf("服务器未超过待应用版本时应继续 apply（true/false），实际 has=%v need=%v",
			got.Data.HasUpdate, got.Data.NeedDownloadUpdate)
	}
	if got.Data.CurrentVersion != "1.0" {
		t.Errorf("current_version 应为真实内容版本 1.0，实际 %q", got.Data.CurrentVersion)
	}
	if got.Data.NewVersion != "2.0" {
		t.Errorf("new_version 应为待应用版本 2.0，实际 %q", got.Data.NewVersion)
	}
}

// TestCheckUpdateDownloadedEmptyPreviousReportsEmptyCurrent 验证 F10 的另一半：
// VersionPrevious 为空（首次部署，内容版本未知）时输出空 current_version，
// 不再回退成待应用版本（旧实现会输出 current == new 的自相矛盾报文）。
func TestCheckUpdateDownloadedEmptyPreviousReportsEmptyCurrent(t *testing.T) {
	pkg := setupCheckUpdateLayout(t,
		`{"version_previous":"","version":"2.0","version_status":"downloaded"}`,
		logsServer(t, "1.0"))
	// 待应用目录存在且非空（否则会走 F3"产物丢失→重下"分支）
	writePkgFile(t, pkg, "ApplicationFolder_2.0/x.txt", "x")

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.Data.HasUpdate {
		t.Errorf("服务器未超过待应用版本时应上报继续 apply，实际 has_update=false")
	}
	if got.Data.CurrentVersion != "" {
		t.Errorf("内容版本未知时应输出空 current_version，实际 %q", got.Data.CurrentVersion)
	}
	if got.Data.NewVersion != "2.0" {
		t.Errorf("new_version 应为 2.0，实际 %q", got.Data.NewVersion)
	}
}

// ============================================================
// check_update：新版本发布内容（release_notes）
// ============================================================

// TestCheckUpdateReportsReleaseNotes 验证上报新版本时一并透传发布内容：
// release_notes ← 服务端 project_change_logs.logs，供宿主/UI 展示"这次更新了什么"。
func TestCheckUpdateReportsReleaseNotes(t *testing.T) {
	notes := []string{"修复：测长偶发偏差", "新增：点间距支持 2.5m"}
	setupCheckUpdateLayout(t,
		`{"version_previous":"","version":"1.0","version_status":"applied"}`,
		notesServer(t, "2.0", notes))

	os.Args = []string{"aly-client", "check_update"}
	got := runCheckUpdate(t)

	if !got.Data.HasUpdate || !got.Data.NeedDownloadUpdate {
		t.Fatalf("服务器更高时应上报 need_download_update=true，实际 has=%v need=%v",
			got.Data.HasUpdate, got.Data.NeedDownloadUpdate)
	}
	if len(got.Data.ReleaseNotes) != len(notes) {
		t.Fatalf("release_notes 应透传 %d 条，实际 %d 条（%v）",
			len(notes), len(got.Data.ReleaseNotes), got.Data.ReleaseNotes)
	}
	for i := range notes {
		if got.Data.ReleaseNotes[i] != notes[i] {
			t.Errorf("release_notes[%d] 应为 %q，实际 %q", i, notes[i], got.Data.ReleaseNotes[i])
		}
	}
}

// TestCheckUpdateOmitsReleaseNotesWhenEmpty 验证服务端没有变更内容时不输出 release_notes
// （omitempty）：宿主据此不会弹出空的"更新内容"面板。
func TestCheckUpdateOmitsReleaseNotesWhenEmpty(t *testing.T) {
	setupCheckUpdateLayout(t,
		`{"version_previous":"","version":"1.0","version_status":"applied"}`,
		logsServer(t, "2.0"))

	os.Args = []string{"aly-client", "check_update"}
	out := captureStdout(t, CheckUpdate)

	if strings.Contains(out, "release_notes") {
		t.Errorf("没有变更内容时不应输出 release_notes，实际 %q", out)
	}
}
