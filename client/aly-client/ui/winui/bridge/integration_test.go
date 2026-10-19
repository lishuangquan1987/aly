// +build windows

package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// realAlyClient 返回仓库内 aly-client.exe 的路径（不存在则跳过用例）。
//
// 路径推导：ui/winui/bridge -> ui/winui -> ui -> client/aly-client
func realAlyClient(t *testing.T) string {
	p := filepath.Join("..", "..", "..", "aly-client.exe")
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Skipf("abs path failed: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Skipf("aly-client.exe not found at %s, skip integration test", abs)
	}
	return abs
}

// TestIntegrationCheckSelfUpdateJSON 用真实 aly-client.exe 验证：
// 执行器能拿到合法 JSON、能解析成 KindResult、且 errorMsg 原样带回。
//
// 仓库目录不是真实部署布局（缺 ../win-x64/.updator/shared.json），
// 因此预期 isSuccess=false 且 errorMsg 非空 —— 这正是要验证的失败路径。
func TestIntegrationCheckSelfUpdateJSON(t *testing.T) {
	exe := realAlyClient(t)
	c := New(exe)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	p, err := c.CheckSelfUpdate(ctx)
	if err != nil {
		t.Fatalf("Run returned transport error: %v", err)
	}
	if p.Kind != KindResult {
		t.Fatalf("want KindResult, got %v", p.Kind)
	}
	if p.Success {
		// 若哪天仓库被布置成真实布局，也应能解析成功；两种都算通过解析
		t.Logf("check_self_update succeeded (repo laid out as deployment)")
	}
	if !p.Success && p.ErrMsg == "" {
		t.Fatal("failed result must carry a non-empty errorMsg")
	}
	if p.Self != nil {
		t.Fatal("check_self_update must not be parsed as CheckUpdateData")
	}
	t.Logf("kind=%v success=%v errMsg=%q", p.Kind, p.Success, p.ErrMsg)
}

// TestIntegrationUsageGoesToStdoutNoJSON 用真实 aly-client.exe 验证：
// 无参数时它打印用法（非 JSON）并 exit 1 —— 执行器必须报错而不是 hẚng。
func TestIntegrationUsageNoJSON(t *testing.T) {
	exe := realAlyClient(t)
	c := New(exe)

	start := time.Now()
	_, err := c.Run(context.Background(), nil, Options{TotalTimeout: 15 * time.Second}, nil)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error: usage output is not JSON and exit code is 1")
	}
	if elapsed > 10*time.Second {
		t.Fatalf("did not terminate promptly, elapsed=%v", elapsed)
	}
	t.Logf("err=%v elapsed=%v", err, elapsed)
}

// TestIntegrationWorkingDirectory 验证子进程的 CWD 是 aly-client.exe 所在目录
// （对齐 C# SDK，且保证 client.json / shared.json 的相对路径解析正确）。
func TestIntegrationWorkingDirectory(t *testing.T) {
	exe := realAlyClient(t)
	c := New(exe)
	if got, want := c.ExeDir(), filepath.Dir(exe); got != want {
		t.Fatalf("ExeDir=%q want %q", got, want)
	}
	if !filepath.IsAbs(c.ExeDir()) {
		t.Fatalf("ExeDir should be absolute, got %q", c.ExeDir())
	}
}
