// +build windows

package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const moveFileReplaceExisting = 0x1

var procMoveFileEx = kernel32DLL.NewProc("MoveFileExW")

// spawnDetachedFn 是 spawnDetached 的注入点，便于单测替换。
var spawnDetachedFn = spawnDetached

// UpdateUpdater 与 UpdateSelfUI 的"更新方式"保持一致：
// 把 <主程序目录>/aly-client.exe 复制覆盖 <UpdateFolder>/aly-client.exe。
//
// 与 UI 自更新相同的两条口径：
//  1. 主程序目录没有随包分发的 aly-client.exe → 跳过，不当作错误；
//  2. 两边内容一致（SHA256）→ 不重复写盘，减少杀软扫描与无谓 IO。
//
// 替换采用 .new + 校验 + MoveFileEx 原子替换，失败时保留旧文件
// （不会出现目标被删掉却没有新文件的状态）。
func UpdateUpdater(updateFolder, mainFolder string) error {
	src := filepath.Join(mainFolder, "aly-client.exe")
	dst := filepath.Join(updateFolder, "aly-client.exe")

	if _, err := os.Stat(src); err != nil {
		// 主程序目录没有随包分发的更新器：不是错误，跳过即可（与 UpdateSelfUI 一致）
		return nil
	}
	if same, err := sameFile(src, dst); err == nil && same {
		return nil
	}
	return replaceFromSource(src, dst, 3, time.Second)
}

// UpdateSelfUI 把 <主程序目录>/aly-client-ui.exe 复制为
// <UpdateFolder>/aly-client-ui.exe.new，并派生一个 detached cmd 在自身退出后
// 完成替换与重启（DEVELOPMENT_PLAN §6.3B）。
//
// restartArgs 是重启新实例时透传的命令行参数（Run 传 os.Args[1:]）：
// 自更新会重启进程，若不带上原参数，手动「检查更新」(-mode check) 重启后会变成
// 默认的 auto 模式 —— 静默检查，"没有可用的更新"提示就不会出现。
//
// 返回 replaced=true 表示调用方应立刻退出，把控制权交给延迟脚本。
func UpdateSelfUI(updateFolder, mainFolder string, restartArgs []string) (bool, error) {
	src := filepath.Join(mainFolder, "aly-client-ui.exe")
	self := filepath.Join(updateFolder, "aly-client-ui.exe")

	if _, err := os.Stat(src); err != nil {
		// 主程序目录没有随包分发的 UI：不是错误，跳过即可
		return false, nil
	}

	same, err := sameFile(src, self)
	if err != nil {
		return false, err
	}
	if same {
		return false, nil
	}

	newPath := self + ".new"
	if err := copyFile(src, newPath); err != nil {
		return false, err
	}
	// 复制后校验，避免半截文件被换上
	if same, err := sameFile(src, newPath); err != nil || !same {
		_ = os.Remove(newPath)
		return false, fmt.Errorf("copy %s -> %s: checksum mismatch", src, newPath)
	}

	// 等自身退出后再替换并重启（ping 用作 XP 上可靠的延时）
	cmdLine := fmt.Sprintf(
		`ping -n 3 127.0.0.1 >nul & move /y "%s" "%s" & start "" "%s"%s`,
		newPath, self, self, quoteArgs(restartArgs))
	return true, spawnDetachedFn(cmdLine)
}

// quoteArgs 把参数拼成 cmd 命令行片段（每个参数用双引号包裹），
// 参数里的双引号会被去掉，避免拼接出可注入的命令行。
func quoteArgs(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, a := range args {
		sb.WriteString(` "`)
		sb.WriteString(strings.Replace(a, `"`, "", -1))
		sb.WriteString(`"`)
	}
	return sb.String()
}

// sameFile 比较两个文件的 SHA256 是否相同。
func sameFile(a, b string) (bool, error) {
	ha, err := sha256Of(a)
	if err != nil {
		return false, err
	}
	hb, err := sha256Of(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}

func sha256Of(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(dst, data, 0755)
}

// moveReplace 用 MoveFileEx(MOVEFILE_REPLACE_EXISTING) 替换目标文件。
//
// 不用「先 os.Remove 再 os.Rename」：那中间存在一个目标文件不存在的窗口，
// 一旦此时进程被杀/断电，aly-client.exe 就彻底没了，更新链也断了。
// MoveFileEx 在同一卷上是原子的，失败时目标保持原样。
func moveReplace(src, dst string) error {
	s, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	d, err := syscall.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	r, _, callErr := procMoveFileEx.Call(
		uintptr(unsafe.Pointer(s)),
		uintptr(unsafe.Pointer(d)),
		uintptr(moveFileReplaceExisting),
	)
	if r == 0 {
		if callErr == nil {
			callErr = syscall.EINVAL
		}
		return callErr
	}
	return nil
}

// replaceFromSource 复制 src → dst.new → 校验 → 原子替换；失败重试 n 次。
func replaceFromSource(src, dst string, retries int, delay time.Duration) error {
	var lastErr error
	tmp := dst + ".new"

	for i := 0; i < retries; i++ {
		if err := copyFile(src, tmp); err != nil {
			lastErr = err
			time.Sleep(delay)
			continue
		}
		same, err := sameFile(src, tmp)
		if err != nil || !same {
			lastErr = fmt.Errorf("checksum mismatch after copying %s", src)
			_ = os.Remove(tmp)
			time.Sleep(delay)
			continue
		}
		if err := moveReplace(tmp, dst); err != nil {
			lastErr = err
			_ = os.Remove(tmp)
			time.Sleep(delay)
			continue
		}
		return nil
	}
	return fmt.Errorf("replace %s: %v", dst, lastErr)
}
