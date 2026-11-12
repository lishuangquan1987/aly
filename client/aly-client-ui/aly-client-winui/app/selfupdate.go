// +build windows

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"aly/client/aly-client-ui/aly-client-winui/bridge"
	"aly/client/aly-client/model"
	"aly/client/aly-client/selfupdate"
)

// 自更新防循环参数：
//   - maxSelfUpdateAttempts 观察窗口内允许的连续未收敛次数上限，达到后本次
//     启动跳过自更新（否则"替换失败 → 重启 → 再替换失败"会无限循环）；
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

// spawnFinishFn 是“派生收尾进程”的注入点，便于单测断言参数。
var spawnFinishFn = spawnFinishSelfUpdate

// selfUpdateViaCmd 是 UpdateUpdater 委派 self_update 命令的注入点。
//
// 返回值 (data, commandRan, err)：
//   - commandRan=true：命令已执行并返回了 JSON。err != nil 表示业务失败
//     （典型：.aly.lock 被 apply_update 持有），调用方不得绕过锁去直接复制；
//   - commandRan=false：进程级失败（旧版更新器不认识 self_update / 启动失败 /
//     超时/崩溃），调用方应回退到直接复制，保证升级链不会永久卡住。
var selfUpdateViaCmd = func(exePath string) (model.SelfUpdateData, bool, error) {
	cl := bridge.New(exePath)
	p, runErr := cl.SelfUpdate(context.Background())
	if runErr != nil {
		return model.SelfUpdateData{}, false, runErr
	}
	if !p.Success {
		msg := p.ErrMsg
		if msg == "" {
			msg = "self_update failed"
		}
		return model.SelfUpdateData{}, true, fmt.Errorf("%s", msg)
	}
	if p.SelfUpdate == nil {
		return model.SelfUpdateData{}, true, fmt.Errorf("unexpected response from aly-client self_update")
	}
	return *p.SelfUpdate, true, nil
}

// UpdateUpdater 更新 <UpdateFolder>/aly-client.exe。
//
// 正常路径统一委派给 aly-client 自己的 self_update 命令（与裸控制台、C# SDK
// 共用同一套 SHA 比较 / 先备后换 / 回滚 / 更新锁）。只有两种例外才直接复制：
//  1. 目标缺失（上次替换/apply 的崩溃现场）——命令根本起不来，只能从主目录落位；
//  2. 命令进程级失败——上游更新器还是旧版（不认识 self_update）或进程崩溃/超时。
//
// 命令返回业务失败（JSON isSuccess=false，例如锁被占用）时**不**兜底，避免
// 绕过 .aly.lock 与正在进行的 apply 竞争。
//
// 内容一致时不启动进程（少一次 XP 上的进程创建），清 journal 后返回。
func UpdateUpdater(updateFolder, mainFolder string) error {
	src := filepath.Join(mainFolder, "aly-client.exe")
	dst := filepath.Join(updateFolder, "aly-client.exe")

	if _, err := os.Stat(src); err != nil {
		// 主程序目录没有随包分发的更新器：不是错误，跳过即可（与 UpdateSelfUI 一致）。
		return nil
	}
	// 清理上次替换可能留下的 .new/.old 残留：dst（aly-client.exe）不在运行，
	// 清理无竞态（UI 自身运行的是 aly-client-ui.exe，不受影响）。
	selfupdate.CleanArtifacts(dst)

	// 1) 更新器缺失：命令无法启动，直接由共享原语从主目录落位（恢复路径）。
	if _, err := os.Stat(dst); err != nil {
		repErr := selfupdate.ReplaceFromFile(src, dst, selfupdate.Options{})
		markSelfUpdateJournal(dst, repErr == nil, repErr)
		return repErr
	}

	// 2) 内容一致：不启动进程，清 journal。
	same, sameErr := selfupdate.SameFile(src, dst)
	if sameErr == nil && same {
		markSelfUpdateJournal(dst, true, nil)
		return nil
	}

	// 3) 内容不同：委派 self_update。命令内部再做一次 SHA 比较，重复读取的
	//    成本可接受（仅在确有差异时发生）。
	_, commandRan, cmdErr := selfUpdateViaCmd(dst)
	if cmdErr == nil {
		markSelfUpdateJournal(dst, true, nil)
		return nil
	}
	if commandRan {
		// 命令已识别并返回业务失败（锁/复制失败）：不兜底，下次启动再试。
		markSelfUpdateJournal(dst, false, cmdErr)
		return cmdErr
	}

	// 4) 进程级失败（旧版更新器不认识 self_update / 崩溃 / 超时）：兜底直接替换。
	if fbErr := selfupdate.ReplaceFromFile(src, dst, selfupdate.Options{}); fbErr != nil {
		combined := fmt.Errorf("self_update command failed (%v); direct fallback failed: %v", cmdErr, fbErr)
		markSelfUpdateJournal(dst, false, combined)
		return combined
	}
	markSelfUpdateJournal(dst, true, nil)
	return nil
}

// UpdateSelfUI 把 <主程序目录>/aly-client-ui.exe 复制为
// <UpdateFolder>/aly-client-ui.exe.new，并派生 .new 的隐藏收尾模式
//（-finish-self-update）在自身退出后完成替换与重启（DEVELOPMENT_PLAN §6.3B）。
//
// 注意：self_update 命令只能替换 aly-client.exe，管不了 UI 自身；UI 必须
// "准备 .new → 派生 .new 的隐藏收尾模式 -finish-self-update（等旧进程退出后
// 改名并重启）"，因此这段保留独立实现。
//
// restartArgs 是重启新实例时透传的命令行参数（Run 传 os.Args[1:]）：
// 自更新会重启进程，若不带上原参数，手动「检查更新」(-mode check) 重启后会变成
// 默认的 auto 模式 —— 静默检查，"没有可用的更新"提示就不会出现。
//
// 返回 replaced=true 表示调用方应立刻退出，把控制权交给收尾进程。
func UpdateSelfUI(updateFolder, mainFolder string, restartArgs []string) (bool, error) {
	src := filepath.Join(mainFolder, "aly-client-ui.exe")
	self := filepath.Join(updateFolder, "aly-client-ui.exe")

	if _, err := os.Stat(src); err != nil {
		// 主程序目录没有随包分发的 UI：不是错误，跳过即可
		return false, nil
	}

	// 无限重启保护：观察窗口内连续未收敛达到上限时跳过本次自更新。
	// 收尾进程的等待/改名/重启失败无法被本进程观测，因此用 journal.Pending
	// 标记"已交出替换、结果未知"，下一次启动看到 Pending 仍为 true 即计一次
	// 未收敛失败——只统计复制/派生失败会漏掉收尾进程失败导致的循环。
	jp := selfupdate.JournalPath(self)
	j := selfupdate.LoadJournal(jp)
	if j.ShouldSkip(maxSelfUpdateAttempts, selfUpdateRetryWindow) {
		return false, nil
	}

	same, err := selfupdate.SameFile(src, self)
	if err != nil {
		j.MarkFailure(err.Error())
		selfupdate.SaveJournal(jp, j)
		return false, err
	}
	if same {
		// 收敛信号：当前副本已是新版——清零计数，并顺带清理历史残留
		//（.new / .old）。此路径说明没有在途的收尾进程，清理无竞态。
		selfupdate.CleanArtifacts(self)
		j.MarkSuccess()
		selfupdate.SaveJournal(jp, j)
		return false, nil
	}

	// 上一次已交给收尾进程但没收敛（典型：等待/改名失败，或新 UI 没起来）：
	// 先补记一次失败，再判断是否达到跳过阈值。
	if j.Pending {
		j.MarkFailure("previous self-update did not converge")
		selfupdate.SaveJournal(jp, j)
		if j.ShouldSkip(maxSelfUpdateAttempts, selfUpdateRetryWindow) {
			return false, nil
		}
	}

	newPath := self + ".new"
	// 复制 + 校验由共享包完成（保留可执行位、copy 后 SHA256 核对），避免
	// UI 再维护一套 sha256/copy 实现。
	if err := selfupdate.CopyAndVerify(src, newPath); err != nil {
		j.MarkFailure(err.Error())
		selfupdate.SaveJournal(jp, j)
		return false, err
	}

	// 派生新 UI 自身（.new）的隐藏收尾模式：它持有旧进程句柄，等旧 UI 退出
	// （最多 10s，超时强杀）后把 .new 换到目标并拉起新 UI。全程不经过
	// cmd.exe，因此没有 ping 固定延时、PATH、引号或 % 展开问题。
	pid := os.Getpid()
	created := processCreationTime(pid)
	// 把原始宿主 PID 传给收尾链：新 UI 的父进程是收尾进程（拉起后即退出），
	// 若不显式传 PID，新 UI 的宿主跟随会误判"宿主已退出"而自杀（2026-11-12 修复）。
	// 当前 UI 自己若是被收尾进程重启的，则把收到的原始宿主继续往下传。
	hostPID := hostPidFromArgs()
	if hostPID == 0 {
		hostPID = uint32(os.Getppid())
	}
	helperArgs := []string{
		"-finish-self-update",
		"--target", self,
		"--pid", strconv.Itoa(pid),
		"--created", strconv.FormatUint(created, 10),
		"--host-pid", strconv.FormatUint(uint64(hostPID), 10),
		"--new", newPath,
		"--",
	}
	helperArgs = append(helperArgs, restartArgs...)

	// 先落 Pending 再派生：收尾进程的结果本进程无法观测；下一次启动据此
	// 判断"上一次是否收敛"。派生失败则立即记一次失败并清 Pending。
	j.MarkAttempt()
	selfupdate.SaveJournal(jp, j)
	if err := spawnFinishFn(newPath, helperArgs); err != nil {
		j.MarkFailure(err.Error())
		selfupdate.SaveJournal(jp, j)
		return false, err
	}
	// 此处不 MarkSuccess：真正的收敛信号是重启后 same → noop 清零。
	return true, nil
}
