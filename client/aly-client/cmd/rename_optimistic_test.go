package cmd

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestIsRenameRetryableErr 验证乐观重命名的错误码分类（问题 3 / §3.3 ①）：
// 只有"占用类"错误（32/33/1224/5）值得探测+击杀+重试，
// 跨卷/目标已存在/源不存在等错误重试无意义，必须立即失败。
func TestIsRenameRetryableErr(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"ERROR_SHARING_VIOLATION(32)", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.Errno(32)}, true},
		{"ERROR_LOCK_VIOLATION(33)", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.Errno(33)}, true},
		{"ERROR_USER_MAPPED_FILE(1224)", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.Errno(1224)}, true},
		{"ERROR_ACCESS_DENIED(5)", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.Errno(5)}, true},
		{"ERROR_NOT_SAME_DEVICE(17)", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.Errno(17)}, false},
		{"ERROR_DIR_NOT_EMPTY(145)", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.Errno(145)}, false},
		{"ERROR_ALREADY_EXISTS(183)", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.Errno(183)}, false},
		{"ERROR_PATH_NOT_FOUND(3)", &os.LinkError{Op: "rename", Old: "a", New: "b", Err: syscall.Errno(3)}, false},
		{"非 LinkError（无法分类）保守重试", errors.New("plain error"), true},
		{"nil 不出现（防御）", nil, true},
	}
	for _, tt := range tests {
		if got := isRenameRetryableErr(tt.err); got != tt.want {
			t.Errorf("%s: isRenameRetryableErr = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestProbeHoldersStateNoOccupant 验证探测状态基本行为：
// 无占用者时返回空、只探测存在的路径、不 panic。
// 注意：不传 attempt>=3（深扫是全系统句柄枚举，10s-60s 级，测试环境不宜触发），
// 深扫去重逻辑由 TestDeepScanCandidates 单独覆盖。
func TestProbeHoldersStateNoOccupant(t *testing.T) {
	st := newRenameProbeState()
	// 伪造一个已击杀的 PID
	st.killedPids[9999] = true

	root, err := ioutil.TempDir("", "probe-state-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "from")
	to := filepath.Join(root, "to")
	if err := os.MkdirAll(from, 0755); err != nil {
		t.Fatalf("创建 from 失败: %v", err)
	}

	// 浅扫（attempt<3）：无真实占用者时返回空
	pids := probeHoldersState(from, to, 1, st)
	if len(pids) != 0 {
		t.Errorf("无占用者时应返回空，实际 %v", pids)
	}
	// to 不存在时不应触发探测错误，仍返回空
	pids2 := probeHoldersState(from, to, 2, st)
	if len(pids2) != 0 {
		t.Errorf("目标不存在时也应返回空，实际 %v", pids2)
	}
}

// TestDeepScanCandidates 验证深扫路径去重（#23）：
// 同一路径在本次 apply/rollback 内只被深扫一次，避免 3 次 rename 重复全量扫描。
func TestDeepScanCandidates(t *testing.T) {
	st := newRenameProbeState()
	paths := []string{`C:\pkg\ApplicationFolder`, `C:\pkg\ApplicationFolder_1.0`}

	first := deepScanCandidates(paths, st)
	if len(first) != 2 {
		t.Fatalf("首次深扫应返回全部未深扫路径，实际 %v", first)
	}
	for _, p := range first {
		st.deepPaths[p] = true
	}
	second := deepScanCandidates(paths, st)
	if len(second) != 0 {
		t.Errorf("深扫过后不应再返回重复路径，实际 %v", second)
	}
	// 新出现的路径仍应被返回
	st.deepPaths[`C:\pkg\ApplicationFolder_2.0`] = true
	third := deepScanCandidates([]string{`C:\pkg\ApplicationFolder_2.0`, `C:\pkg\ApplicationFolder_3.0`}, st)
	if len(third) != 1 || third[0] != `C:\pkg\ApplicationFolder_3.0` {
		t.Errorf("应只返回未深扫过的新路径，实际 %v", third)
	}
}

// TestRenameDirWithKillOptimisticNoOccupant 验证乐观重命名：
// 无占用者场景先直接 rename 成功，不产生旁移目录（问题 3 核心路径）。
func TestRenameDirWithKillOptimisticNoOccupant(t *testing.T) {
	root, err := ioutil.TempDir("", "rename-opt-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "win-x64")
	to := filepath.Join(root, "win-x64_3.0")
	mustMkdirFile(t, from, "app.exe", "new app")

	start := time.Now()
	if err := renameDirWithKill(from, to, 5*time.Second); err != nil {
		t.Fatalf("无占用者时 rename 应直接成功: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 3*time.Second {
		t.Errorf("无占用者场景应快速完成（乐观路径零探测），实际耗时 %v", elapsed)
	}
	if _, err := os.Stat(filepath.Join(to, "app.exe")); err != nil {
		t.Errorf("目标 %s 应包含 app.exe: %v", to, err)
	}
	if _, err := os.Stat(to + ".old"); !os.IsNotExist(err) {
		t.Errorf("目标不存在时不应产生 %s.old", to)
	}
}
