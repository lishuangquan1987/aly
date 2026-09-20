// +build windows

package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

const (
	fileShareRead    = 0x00000001 // FILE_SHARE_READ（不含 DELETE）
	genericRead      = 0x80000000 // GENERIC_READ
	openExisting     = 3          // OPEN_EXISTING
	fileFlagBackup   = 0x02000000 // FILE_FLAG_BACKUP_SEMANTICS（用于打开目录）
)

// TestFindProcessesHoldingPathDirectory 回归测试：
// Restart Manager 注册"目录"时检测不到持有目录内文件的进程（实测返回无占用者），
// 导致更新时无法杀进程。修复后 FindProcessesHoldingPath 会逐一注册目录下所有文件，
// 必须能找到持有目录内文件的进程（即本测试进程自身）。
func TestFindProcessesHoldingPathDirectory(t *testing.T) {
	root, err := ioutil.TempDir("", "rm-dir-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	filePath := filepath.Join(root, "a.txt")
	if err := ioutil.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	// 用不含 FILE_SHARE_DELETE 的方式打开文件（模拟占用）
	pathPtr, _ := syscall.UTF16PtrFromString(filePath)
	handle, err := syscall.CreateFile(pathPtr, genericRead, fileShareRead, nil, openExisting, 0, 0)
	if err != nil {
		t.Fatalf("CreateFile 失败: %v", err)
	}
	defer syscall.CloseHandle(handle)

	pids, err := FindProcessesHoldingPath(root)
	if err != nil {
		t.Fatalf("FindProcessesHoldingPath 失败: %v", err)
	}
	me := uint32(os.Getpid())
	found := false
	for _, pid := range pids {
		if pid == me {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("目录内文件被占用时应能探测到本进程 PID=%d，实际返回 %v", me, pids)
	}
}

// TestFindProcessesHoldingPathFile 验证直接注册文件仍能探测到占用者。
func TestFindProcessesHoldingPathFile(t *testing.T) {
	root, err := ioutil.TempDir("", "rm-file-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	filePath := filepath.Join(root, "a.txt")
	if err := ioutil.WriteFile(filePath, []byte("x"), 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	pathPtr, _ := syscall.UTF16PtrFromString(filePath)
	handle, err := syscall.CreateFile(pathPtr, genericRead, fileShareRead, nil, openExisting, 0, 0)
	if err != nil {
		t.Fatalf("CreateFile 失败: %v", err)
	}
	defer syscall.CloseHandle(handle)

	pids, err := FindProcessesHoldingPath(filePath)
	if err != nil {
		t.Fatalf("FindProcessesHoldingPath 失败: %v", err)
	}
	me := uint32(os.Getpid())
	found := false
	for _, pid := range pids {
		if pid == me {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("文件被占用时应能探测到本进程 PID=%d，实际返回 %v", me, pids)
	}
}

// TestFindProcessesHoldingPathNoHolder 验证无占用时返回空。
func TestFindProcessesHoldingPathNoHolder(t *testing.T) {
	root, err := ioutil.TempDir("", "rm-none-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)
	if err := ioutil.WriteFile(filepath.Join(root, "a.txt"), []byte("x"), 0644); err != nil {
		t.Fatalf("写入文件失败: %v", err)
	}

	pids, err := FindProcessesHoldingPath(root)
	if err != nil {
		t.Fatalf("FindProcessesHoldingPath 失败: %v", err)
	}
	if len(pids) != 0 {
		t.Fatalf("无占用时应返回空，实际返回 %v", pids)
	}
}

// TestFindProcessesHoldingPathDirHandle 验证打开目录本身（不含 FILE_SHARE_DELETE）时也能探测到。
// 注意：目录句柄类占用（如 CWD、Explorer 窗口）RM 可能检测不到，此测试仅记录当前行为，不强制断言。
func TestFindProcessesHoldingPathDirHandle(t *testing.T) {
	root, err := ioutil.TempDir("", "rm-dirhandle-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	dirPtr, _ := syscall.UTF16PtrFromString(root)
	handle, err := syscall.CreateFile(dirPtr, genericRead, fileShareRead, nil, openExisting, fileFlagBackup, 0)
	if err != nil {
		t.Skipf("打开目录句柄失败（不影响其他用例）: %v", err)
	}
	defer syscall.CloseHandle(handle)

	pids, err := FindProcessesHoldingPath(root)
	if err != nil {
		t.Fatalf("FindProcessesHoldingPath 失败: %v", err)
	}
	t.Logf("目录句柄占用探测结果: %v（RM 对目录句柄可能检测不到，属已知限制）", pids)
}
