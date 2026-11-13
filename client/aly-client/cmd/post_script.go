package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"aly/client/aly-client/util"
)

// runPostApplyScript 幂等执行后置脚本（加固②）：
// 脚本**成功完成后**写 marker（at-most-once 语义），崩溃恢复分支据此跳过"已完成"的脚本，
// 修复"崩溃恢复重跑已完成脚本导致迁移/初始化执行两次"的问题。
//
// 契约：脚本必须可幂等执行 —— marker 在脚本完成后才写入（B9：Start 失败 / 非零退出 /
// 超时被 Kill 都不写 marker），脚本执行中被杀不会留下 marker，崩溃恢复会重跑该脚本
// （依赖幂等性）；写 applied 之后、脚本完成之前被杀则不重试（残余窗口，见实现注记）。
// 两处残余行为均属设计取舍，脚本应按幂等编写。
//
// 路径安全（M12）：scriptRelPath 来自服务端（after_apply_update_script），Join 之前必须
// 过 safeRelPath 白名单，防止 `..\..\` 逃逸主目录后被 cmd /c / sh 执行（与 M11 组合
// 即任意命令执行）。
func runPostApplyScript(fc *FullConfig, scriptRelPath, version string) {
	if scriptRelPath == "" {
		return
	}
	if !safeRelPath(scriptRelPath) {
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("skip post script: 非法相对路径 %q（M12 白名单拒绝）", scriptRelPath))
		return
	}
	scriptPath := filepath.Join(fc.MainFolder, filepathFromSlash(normalizePath(scriptRelPath)))
	markerDir := filepath.Join(fc.MainFolder, ".updator")
	marker := filepath.Join(markerDir, "after_apply_"+stripVPrefix(version)+".done")

	if _, err := os.Stat(marker); err == nil {
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("skip post script %s: marker %s exists", scriptPath, marker))
		return
	}

	if err := runScript(scriptPath, fc.MainFolder); err != nil {
		// B9：脚本失败不写 marker —— 下一次崩溃恢复/下次更新会重跑，避免"失败的迁移
		// 被标记为已完成、应用带着未完成的迁移上线"。
		util.AppendToLog(logDir(), "update.log",
			fmt.Sprintf("post script not marked as done (will retry on recovery): %s: %v", scriptPath, err))
		return
	}

	// 脚本确实存在才写 marker（脚本文件缺失时 runScript 直接返回 nil，无需标记）
	if _, err := os.Stat(scriptPath); err == nil {
		if mkErr := os.MkdirAll(markerDir, 0755); mkErr == nil {
			if f, err := os.Create(marker); err == nil {
				f.WriteString("done\n")
				f.Close()
			}
		}
	}
}
