// +build windows

package bridge

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeBat 在临时目录写一个 .bat 并返回路径。
func writeBat(t *testing.T, body string) (string, func()) {
	dir, err := ioutil.TempDir("", "alybridge")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	p := filepath.Join(dir, "t.bat")
	if err := ioutil.WriteFile(p, []byte(body), 0644); err != nil {
		os.RemoveAll(dir)
		t.Fatalf("WriteFile: %v", err)
	}
	return p, func() { os.RemoveAll(dir) }
}

func TestRunMissingExe(t *testing.T) {
	c := New(filepath.Join(os.TempDir(), "definitely-not-here-12345.exe"))
	_, err := c.Run(context.Background(), []string{"x"}, Options{}, nil)
	if err == nil {
		t.Fatal("expected error for missing exe")
	}
}

func TestRunNonZeroExitNoStdout(t *testing.T) {
	bat, cleanup := writeBat(t, "@exit /b 3\r\n")
	defer cleanup()

	c := New("cmd.exe")
	_, err := c.Run(context.Background(), []string{"/c", bat}, Options{}, nil)
	if err == nil {
		t.Fatal("expected error for exit code 3")
	}
	if !strings.Contains(err.Error(), "3") {
		t.Fatalf("error should mention exit code 3, got: %v", err)
	}
}

// 关键回归：子进程输出非 JSON 且退出时，必须以 EOF 正常结束，
// 不能像 C# SDK 那样在 ReadLine 返回 null 后 100% CPU 忙等。
func TestRunNonJSONOutputTerminates(t *testing.T) {
	bat, cleanup := writeBat(t, "@echo Usage of apply_update:\r\n@echo   -close-timeout int\r\n")
	defer cleanup()

	c := New("cmd.exe")
	done := make(chan struct{})
	var err error
	go func() {
		_, err = c.Run(context.Background(), []string{"/c", bat}, Options{}, nil)
		close(done)
	}()

	select {
	case <-done:
		// 正常结束
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not terminate within 10s (busy-loop regression)")
	}
	if err == nil {
		t.Fatal("expected error: no JSON result was produced")
	}
}

func TestRunParsesDoneLine(t *testing.T) {
	bat, cleanup := writeBat(t, "@echo {\"isSuccess\":true,\"data\":null}\r\n")
	defer cleanup()

	c := New("cmd.exe")
	p, err := c.Run(context.Background(), []string{"/c", bat}, Options{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Kind != KindDone {
		t.Fatalf("want KindDone, got %v", p.Kind)
	}
}

func TestRunParsesResultAndSkipsNoise(t *testing.T) {
	// 混入非 JSON 行 + 最终结果行：应跳过噪声、取到结果
	bat, cleanup := writeBat(t,
		"@echo some diagnostic noise\r\n"+
			"@echo {\"isSuccess\":true,\"data\":{\"need_update\":true}}\r\n")
	defer cleanup()

	c := New("cmd.exe")
	p, err := c.Run(context.Background(), []string{"/c", bat}, Options{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Self == nil || !p.Self.NeedUpdate {
		t.Fatalf("want CheckSelfUpdateData need_update=true, got %+v", p)
	}
}

func TestRunProgressCallback(t *testing.T) {
	bat, cleanup := writeBat(t,
		"@echo {\"isSuccess\":true,\"data\":{\"index\":1,\"total\":2,\"file\":\"a.dll\",\"status\":\"START\",\"file_size\":10}}\r\n"+
			"@echo {\"isSuccess\":true,\"data\":{\"index\":1,\"total\":2,\"file\":\"a.dll\",\"status\":\"DONE\",\"file_size\":10}}\r\n"+
			"@echo {\"isSuccess\":true,\"data\":null}\r\n")
	defer cleanup()

	var got []ProgressInfo
	c := New("cmd.exe")
	p, err := c.Run(context.Background(), []string{"/c", bat}, Options{}, func(pi ProgressInfo) {
		got = append(got, pi)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Kind != KindDone {
		t.Fatalf("want KindDone, got %v", p.Kind)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 progress callbacks, got %d: %+v", len(got), got)
	}
	if got[0].Status != "START" || got[1].Status != "DONE" || got[1].Total != 2 {
		t.Fatalf("bad progress payload: %+v", got)
	}
}

func TestRunTotalTimeout(t *testing.T) {
	// ping -n 20 约 19 秒；总超时 500ms 必须提前结束。
	// 注意：这里直接用 ping.exe（单层进程），不用 `cmd /c ping`。
	// Run 只杀**直接子进程**——这是有意的：apply_update 会拉起主程序，
	// 若按进程树杀，会把刚启动的主程序一起杀掉。
	c := New("ping.exe")
	start := time.Now()
	_, err := c.Run(context.Background(),
		[]string{"-n", "20", "127.0.0.1"},
		Options{TotalTimeout: 500 * time.Millisecond}, nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if elapsed > 8*time.Second {
		t.Fatalf("timeout did not take effect, elapsed=%v", elapsed)
	}
}

func TestRunContextCancel(t *testing.T) {
	c := New("ping.exe")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := c.Run(ctx, []string{"-n", "20", "127.0.0.1"}, Options{}, nil)
	if err == nil {
		t.Fatal("expected error after cancel")
	}
	if time.Since(start) > 8*time.Second {
		t.Fatal("cancel did not take effect promptly")
	}
}

func TestRunIdleTimeout(t *testing.T) {
	// 无任何 stdout 行时，IdleTimeout 应触发并结束进程
	c := New("ping.exe")
	start := time.Now()
	_, err := c.Run(context.Background(),
		[]string{"-n", "20", "127.0.0.1"},
		Options{IdleTimeout: 500 * time.Millisecond}, nil)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("expected idle-timeout error")
	}
	if elapsed > 15*time.Second {
		t.Fatalf("idle timeout did not take effect, elapsed=%v", elapsed)
	}
}

func TestExitCodeHelper(t *testing.T) {
	if got := exitCode(nil); got != 0 {
		t.Fatalf("nil should be 0, got %d", got)
	}
}
