package alyclient

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

var (
	fakeOnce sync.Once
	fakePath string
	fakeErr  error
)

func repoRoot() string {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		panic(err)
	}
	return root
}

// fakeClient 构建并缓存 tools/fake-aly-client。
func fakeClient(t *testing.T) string {
	t.Helper()
	fakeOnce.Do(func() {
		dir, err := os.MkdirTemp("", "aly-go-fake-")
		if err != nil {
			fakeErr = err
			return
		}
		name := "aly-client"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		out := filepath.Join(dir, name)
		cmd := exec.Command("go", "build", "-o", out, ".")
		cmd.Dir = filepath.Join(repoRoot(), "tools", "fake-aly-client")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			fakeErr = fmt.Errorf("build fake client: %v\n%s", err, stderr.String())
			return
		}
		fakePath = out
	})
	if fakeErr != nil {
		t.Fatal(fakeErr)
	}
	return fakePath
}

func useScenario(t *testing.T, name string) {
	t.Helper()
	t.Setenv("ALY_FAKE_SCENARIO", filepath.Join(repoRoot(), "spec", "fixtures", "scenarios", name+".json"))
}

const testTimeout = 10 * time.Second

func waitFor(t *testing.T, predicate func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("waitFor timeout after %s", timeout)
}
