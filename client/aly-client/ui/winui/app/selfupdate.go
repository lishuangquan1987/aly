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

	"aly/client/aly-client/selfupdate"
)

// 自更新防循环参数：
//   - maxSelfUpdateAttempts 观察窗口内允许的连续失败上限，达到后本次启动跳过
//     自更新（否则"替换失败 → 重启 → 再替换失败"会无限循环）；
//   - selfUpdateRetryWindow 观察窗口，窗口过后计数重置、允许再试——给磁盘满 /
//     杀软占用 / 目标被上一进程短暂持有这类瞬时故障恢复的机会。
const (
	maxSelfUpdateAttempts = 3
	selfUpdateRetryWindow = 30 * time.Minute
)

// markSelfUpdateJournal 记录自更新结果到 target 对应的 journal 文件：
// ok=true 清零（收敛信号），ok=false 计数 +1 并记错误原因（诊断线索）。
// journal 落盘失败静默：它只影响重试节奏，不应让自更新本身失败。
func markSelfUpdateJournal(target string, ok bool, err error) {
	p := selfupdate.JournalPath(target)
	j := selfupdate.LoadJournal(p)
	if ok {
		j.MarkSuccess()
	} else {
		msg := "unknown"
		if err != nil {
			msg = err.Error()
		}
		j.MarkFailure(msg)
	}
	selfupdate.SaveJournal(p, j)
}

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
	// 清理上次替换可能留下的 .new/.old 残留：dst（aly-client.exe）不在运行，
	// 清理无竞态（UI 自身运行的是 aly-client-ui.exe，不受影响）。
	selfupdate.CleanArtifacts(dst)

	same, err := sameFile(src, dst)
	if err == nil && same {
		// 收敛信号：副本已是新版，清零失败计数
		markSelfUpdateJournal(dst, true, nil)
		return nil
	}
	err = replaceFromSource(src, dst, 3, time.Second)
	// 失败不阻断启动，但必须留痕（journal 带错误原因，连续失败触发跳过保护）
	markSelfUpdateJournal(dst, err == nil, err)
	return err
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

	// 无限重启保护（2026-11-09 优化）：观察窗口内连续失败达到上限时跳过本次
	// 自更新——此前替换失败时本函数只返回错误、进程继续正常启动，但下次启动
	// 会原样重试，若故障持续（如磁盘满、目标被永久占用）就形成
	// "启动→替换失败→重启→替换失败"的无限循环。窗口过后自动恢复重试。
	jp := selfupdate.JournalPath(self)
	j := selfupdate.LoadJournal(jp)
	if j.ShouldSkip(maxSelfUpdateAttempts, selfUpdateRetryWindow) {
		return false, nil
	}

	same, err := sameFile(src, self)
	if err != nil {
		markSelfUpdateJournal(self, false, err)
		return false, err
	}
	if same {
		// 收敛信号：当前副本已是新版——清零计数，并顺带清理历史残留
		//（.new / .old）。此路径说明没有在途的延迟替换脚本，清理无竞态。
		selfupdate.CleanArtifacts(self)
		markSelfUpdateJournal(self, true, nil)
		return false, nil
	}

	newPath := self + ".new"
	if err := copyFile(src, newPath); err != nil {
		markSelfUpdateJournal(self, false, err)
		return false, err
	}
	// 复制后校验，避免半截文件被换上
	if same, err := sameFile(src, newPath); err != nil || !same {
		_ = os.Remove(newPath)
		err = fmt.Errorf("copy %s -> %s: checksum mismatch", src, newPath)
		markSelfUpdateJournal(self, false, err)
		return false, err
	}

	// 等自身退出后再替换并重启（ping 用作 XP 上可靠的延时）
	cmdLine := fmt.Sprintf(
		`ping -n 3 127.0.0.1 >nul & move /y "%s" "%s" & start "" "%s"%s`,
		newPath, self, self, quoteArgs(restartArgs))
	if err := spawnDetachedFn(cmdLine); err != nil {
		// 派生延迟脚本失败：本次替换没有成功，也不算收敛——计数 +1，
		// 下次启动重试；连续超限后由 ShouldSkip 保护。
		markSelfUpdateJournal(self, false, err)
		return false, err
	}
	// 此处不 MarkSuccess：真正的收敛信号是重启后 same → noop 清零；
	// 若延迟脚本执行失败，下次启动走 same!=same 分支继续计数。
	return true, nil
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
