package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"aly/client/aly-client/model"
	"aly/client/aly-client/selfupdate"
	"aly/client/aly-client/util"
)

// SelfUpdate 用主程序目录里随版本分发的 aly-client.exe 替换 UpdateFolder
// 中正在运行的副本（self_update 命令）。
//
// 背景（缺口闭环）：此前自更新的"检测"（check_self_update）与"执行"
// （C# SDK UpdateSelf / UI UpdateUpdater）分离，且执行方只有 SDK/UI——
// 裸控制台部署（宿主直接 spawn aly-client）时 UpdateFolder 副本会永久
// 漂移在旧版本。self_update 把执行能力下沉到更新器本体，三方口径统一：
//
//	新版本来源   ：主程序目录 aly-client.exe（随版本包分发，
//	              完整性已由 download_update 的服务端 MD5+SHA256 校验保证）
//	无差异       ：SHA256 相同 → 跳过并顺带清理 .new/.old 残留（updated=false）
//	源缺失       ：主程序目录没有更新器 → 跳过，不算错误（updated=false）
//	替换舞步     ：运行中的 exe 先改名到 .old（两端 OS 都允许改名运行中文件），
//	              再 .new 中转 + 校验 + os.Rename 原子落位；落位失败回滚 .old，
//	              保证 self 始终存在（宁可失败不能消失）
//
// 与 check_self_update 的关系：check 只检测（输出 need_update），self_update
// 只执行；宿主可以 check→(need_update)→self_update 两段式调用，也可以直接调
// self_update（内部自带一致性检查，重复调用幂等）。
func SelfUpdate() {
	fs := flag.NewFlagSet("self_update", flag.ExitOnError)
	mainExePathFlag := fs.String("main-exe-path", "", "main exe relative path")
	fs.Parse(os.Args[2:])

	fc, err := loadFullConfig("", "", *mainExePathFlag)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	data, err := runSelfUpdate(fc)
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}
	printOutput(true, "", data)
}

// selfExePathFn 注入点（测试替换；默认 os.Executable）。
var selfExePathFn = func() (string, error) { return os.Executable() }

// runSelfUpdate 自更新执行编排（从命令入口拆出，便于单测断言 data 结构）。
func runSelfUpdate(fc *FullConfig) (*model.SelfUpdateData, error) {
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

	// 1) 主程序目录没有随包分发的更新器：不是错误，跳过（对齐 UI 口径）
	if _, statErr := os.Stat(srcPath); os.IsNotExist(statErr) {
		return &model.SelfUpdateData{Updated: false, Reason: "source_missing"}, nil
	}

	// 2) 内容一致：跳过 + 顺带清理历史残留（.new / .old*）
	same, err := selfupdate.SameFile(selfPath, srcPath)
	if err != nil {
		return nil, fmt.Errorf("compare self vs source: %v", err)
	}
	if same {
		selfupdate.CleanArtifacts(selfPath)
		return &model.SelfUpdateData{Updated: false, Reason: "already_up_to_date"}, nil
	}

	// 3) 内容不同：执行替换舞步（.old 备份 + .new 中转 + 校验 + 原子落位）
	oldPath, err := selfupdate.ReplaceRunningSelf(selfPath, srcPath, selfupdate.Options{})
	if err != nil {
		return nil, fmt.Errorf("replace self: %v", err)
	}
	util.AppendToLog(logDir(), "update.log",
		fmt.Sprintf("self_update replaced: old=%s new=%s", oldPath, srcPath))
	return &model.SelfUpdateData{Updated: true, Reason: "replaced", OldPath: oldPath}, nil
}
