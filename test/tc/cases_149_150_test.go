package tc

// 用例 149–150：更新检查的快速失败与隐藏 UI 进程回收。
//
// 149 在真实 aly-client 子进程上钉住：项目接口已经 503 后，不得再调用
// 会拖延的日志接口；150 用真实 winui -mode auto 复现“没有界面但后台
// 进程还在等”的现场，要求核心短预算结束后 UI 自行退出。

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func setClientServerURL(t *testing.T, c *Client, serverURL string) {
	t.Helper()
	WriteJSON(t, c.Shared(), map[string]interface{}{
		"server_url":      serverURL,
		"project_name":    DefaultProject,
		"ignore_folders":  []string{},
		"ignore_files":    []string{},
		"un_copy_folders": []string{},
		"un_copy_files":   []string{},
	})
}

// TestCase149：首次服务不可用必须直接返回，不得再等待第二个接口。
func TestCase149(t *testing.T) {
	e := NewEnv(t, true, 0)
	var logRequests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "get_project_change_logs") {
			atomic.AddInt32(&logRequests, 1)
			// 旧实现会在首次 503 后进入这里，额外等待 5 秒；正确实现不会调用。
			time.Sleep(5 * time.Second)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: DefaultProject, MainFile: "app-100",
	})
	setClientServerURL(t, c, srv.URL)
	versionPath := filepath.Join(c.Up(), "version.json")
	before := ReadText(versionPath)

	start := time.Now()
	out := e.RunLast(t, c, "check_update")
	elapsed := time.Since(start)
	True(t, out.IsSuccess, "服务不可用须保持离线协议 isSuccess=true: %s", out.ErrorMsg)
	d := Data[CheckUpdateData](t, out)
	Eq(t, "has_update", d.HasUpdate, false)
	Eq(t, "current_version", d.CurrentVersion, "1.0.0")
	Eq(t, "首次 503 后日志接口调用次数", atomic.LoadInt32(&logRequests), int32(0))
	True(t, elapsed < 3*time.Second, "首次 503 后须立即返回，实际耗时 %s", elapsed)
	Eq(t, "version.json 逐字节不变", ReadText(versionPath), before)
	t.Logf("首次 503 后 check_update 耗时 %s，日志接口调用 %d 次", elapsed, atomic.LoadInt32(&logRequests))
}

// TestCase150：隐藏 auto UI 遇到接受连接但不及时响应的服务时，核心应在
// 短预算内返回，UI 随后自行退出，而不是留下无界面的后台进程。
func TestCase150(t *testing.T) {
	e := NewEnv(t, true, 0)
	var projectRequests int32
	var logRequests int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "get_project_by_name") {
			atomic.AddInt32(&projectRequests, 1)
		}
		if strings.Contains(r.URL.Path, "get_project_change_logs") {
			atomic.AddInt32(&logRequests, 1)
		}
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: DefaultProject, MainFile: "app-100",
	})
	setClientServerURL(t, c, srv.URL)
	uiExe := e.BuildUI(t)
	uiInClient := filepath.Join(c.Up(), "aly-client-ui.exe")
	CopyFile(t, uiExe, uiInClient)
	before := ReadText(filepath.Join(c.Up(), "version.json"))

	res := e.RunUIModeAuto(t, uiInClient, c, "auto", "", 0, 8*time.Second)
	Eq(t, "隐藏 UI 必须自行退出，不得等到外层 8 秒看门狗", res.ExitCode, 0)
	True(t, res.Elapsed < 4*time.Second,
		"慢服务下 UI 应在核心短预算加调度余量内退出，实际 %s", res.Elapsed)
	NotContains(t, res.Output, "panic:", "UI 不得 panic")
	Eq(t, "version.json 逐字节不变", ReadText(filepath.Join(c.Up(), "version.json")), before)
	Eq(t, "UI 必须真实请求一次项目接口，排除提前静默退出假绿", atomic.LoadInt32(&projectRequests), int32(1))
	Eq(t, "首次超时后不得再请求日志接口", atomic.LoadInt32(&logRequests), int32(0))
	t.Logf("慢服务下隐藏 UI 退出码 %d，UI 自身耗时 %s，项目/日志请求 %d/%d",
		res.ExitCode, res.Elapsed, atomic.LoadInt32(&projectRequests), atomic.LoadInt32(&logRequests))
}
