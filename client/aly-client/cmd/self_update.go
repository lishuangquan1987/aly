package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
	"aly/client/aly-client/selfupdate"
	"aly/client/aly-client/util"
)

// SelfUpdate 用主程序目录里随版本分发的更新器（aly-client.exe / aly-client）
// 替换 UpdateFolder 中正在运行的副本（self_update 命令）。
//
// 背景（缺口闭环）：此前自更新的"检测"（check_self_update）与"执行"
// （C# SDK UpdateSelf / UI UpdateUpdater）分离，且执行方只有 SDK/UI——
// 裸控制台部署（宿主直接 spawn aly-client）时 UpdateFolder 副本会永久
// 漂移在旧版本。self_update 把执行能力下沉到更新器本体，三方口径统一：
//
//	新版本来源   ：主程序目录的更新器（aly-client.exe / aly-client，随版本包
//	              分发，完整性已由 download_update 的服务端 MD5+SHA256 校验保证）
//	无差异       ：SHA256 相同 → 跳过并顺带清理 .new/.old 残留（updated=false）
//	源缺失       ：主程序目录没有更新器 → 跳过，不算错误（updated=false）
//	替换舞步     ：先复制源到 .new 并校验（此阶段 self 始终存在）→ 运行中的
//	              exe 改名到 .old（两端 OS 都允许改名运行中文件）→ .new 原子
//	              落位；落位失败回滚 .old。慢操作不暴露目标缺失窗口，失败也
//	              优先保证 self 存在。
//
// 与 check_self_update 的关系：check 只检测（输出 need_update），self_update
// 只执行；宿主可以 check→(need_update)→self_update 两段式调用，也可以直接调
// self_update（内部自带一致性检查，重复调用幂等）。
//
// --include-ui（可选，图形前端补齐）：顺带把 <主程序目录>/aly-client-ui(.exe)
// 落到 UpdateFolder —— **只在目标缺失/零字节时**落位（首次部署、被杀软吃掉、
// 崩溃现场）。已存在的 UI 副本一律不动（ui_reason=existing_kepped）：它可能
// 正在运行，而"已存在但落后"的情况由 UI 自己的启动自更新（UpdateSelfUI +
// -finish-self-update 收尾）收敛。因此带该旗标的调用是安全的：不会替换运行中
// 的 UI 镜像，也不会与在途的收尾进程插队（<dst>.selfupdate.json 的 pending
// 标记为在途时会跳过，ui_reason=pending_inflight）。UI 复制的硬失败上报为
// 命令失败（调用方据此兜底或报错），不把"两件套不齐"报成成功。
//
// 并发：与 download_update / apply_update / rollback / clean 共用 UpdateFolder
// 下的 .aly.lock；另一更新进行中时直接返回错误，避免边换更新器边读半换位的
// 主目录。
func SelfUpdate() {
	fs := flag.NewFlagSet("self_update", flag.ContinueOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	includeUiFlag := fs.Bool("include-ui", false, "同时补齐 aly-client-ui 副本（仅在缺失时落位）")
	// A6：ContinueOnError + JSON 信封，参数解析失败 exit 2；-h 打印 usage 后 exit 0
	parseCommandFlags(fs, os.Args[2:])

	fc, err := loadFullConfig("", "", *mainExePathFlag)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	data, err := runSelfUpdateWithUI(fc, *includeUiFlag)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}
	printOutput(true, "", data)
}

// selfExePathFn 注入点（测试替换；默认 os.Executable）。
var selfExePathFn = func() (string, error) { return os.Executable() }

// runSelfUpdate 自更新执行编排（不带图形前端补齐；既有调用方与单测用）。
func runSelfUpdate(fc *FullConfig) (*model.SelfUpdateData, error) {
	return runSelfUpdateWithUI(fc, false)
}

// runSelfUpdateWithUI 自更新执行编排（从命令入口拆出，便于单测断言 data 结构）。
// includeUi=true 时顺带图形前端副本补齐（语义见文件头 --include-ui 段）。
func runSelfUpdateWithUI(fc *FullConfig, includeUi bool) (*model.SelfUpdateData, error) {
	// 与其它更新命令共用全局锁：apply/download/rollback 正在跑时拒绝，
	// 避免"自更新读主目录"与"apply 换主目录"并发。
	release, lockErr := AcquireUpdateLock("self_update")
	if lockErr != nil {
		return nil, lockErr
	}
	defer release()

	srcPath, err := fc.ExeCfg.CheckUpdaterPath()
	if err != nil {
		return nil, err
	}

	selfPath, err := selfExePathFn()
	if err != nil {
		return nil, fmt.Errorf("get executable path failed: %v", err)
	}
	// 与 config.ExeDir 同款：先解析符号链接再取真实路径
	if resolved, rErr := filepath.EvalSymlinks(selfPath); rErr == nil {
		selfPath = resolved
	}

	// 更新器本体：判定顺序与 data 口径（updated/reason/old_path）与本旗标引入前一致。
	// 正在运行的更新器副本缺失属异常现场，报错而不是静默落位（见 SyncPolicy）。
	res, err := selfupdate.SyncFile(srcPath, selfPath,
		selfupdate.SyncPolicy{BootstrapOnly: false, DstMissingIsError: true}, selfupdate.Options{})
	if err != nil {
		return nil, err
	}
	data := &model.SelfUpdateData{
		Updated: res.Updated,
		Reason:  res.Reason,
		OldPath: res.OldPath,
	}
	if res.Updated {
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("self_update replaced: old=%s new=%s", res.OldPath, srcPath))
	}

	if includeUi {
		// 图形前端：源按"主程序目录与 UpdateFolder 同名副本"约定推导，无平台分支。
		uiSrc := filepath.Join(fc.MainFolder, config.UIFileName())
		uiDst := filepath.Join(filepath.Dir(selfPath), config.UIFileName())
		uiRes, uiErr := selfupdate.SyncFile(uiSrc, uiDst,
			selfupdate.SyncPolicy{BootstrapOnly: true}, selfupdate.Options{})
		if uiErr != nil {
			return nil, fmt.Errorf("sync ui %s: %v", uiDst, uiErr)
		}
		data.UIUpdated = uiRes.Updated
		data.UIReason = uiRes.Reason
		data.UIOldPath = uiRes.OldPath
		if uiRes.Updated {
			util.AppendToLog(logDir(), "update.log",
				fmt.Sprintf("self_update --include-ui deployed ui: new=%s", uiSrc))
		}
	}

	return data, nil
}
