package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aly/client/aly-client/config"
	"aly/client/aly-client/model"
	"aly/client/aly-client/util"
)

// Clean 独立清理命令（精简计划条目 7）。
//
// pruneVersionSnapshots 只在 apply/rollback 入口机会式触发：客户端不再更新时
// 历史快照永久占盘。clean 提供显式入口：
//
//	aly-client clean [--keep N] [--all]
//
//	--keep N   保留最近 N 个版本快照（默认 3，与 prune 语义一致），清理更旧的；
//	--all      激进模式：只保留 MainFolder 当前版本相关的保护集，清掉其余全部快照；
//
// 两种模式都顺带清理：.old/.old.N 旁移残留、孤儿无版本号备份目录、
// .part/.tmp 下载临时文件、超过 30 天的 update_*_fail.log。
func Clean() {
	fs := flag.NewFlagSet("clean", flag.ContinueOnError)
	keepFlag := fs.Int("keep", 0, "保留最近 N 个版本快照（默认 3）")
	allFlag := fs.Bool("all", false, "激进清理：只保留当前版本保护集")
	// A6：ContinueOnError + JSON 信封，参数解析失败 exit 2；-h 打印 usage 后 exit 0
	parseCommandFlags(fs, os.Args[2:])

	// 复用全局更新锁：clean 与 apply/rollback 一样会动 PackageFolder，
	// 不能与在途更新并发。
	releaseLock, lockErr := AcquireUpdateLock("clean")
	if lockErr != nil {
		printOutput(false, lockErr.Error(), nil)
		return
	}
	defer releaseLock()

	fc, err := loadFullConfig("", "", "")
	if err != nil {
		printOutput(false, err.Error(), nil)
		return
	}

	if *allFlag {
		cleanAll(fc)
		return
	}

	keep := *keepFlag
	if keep < 1 {
		keep = defaultSnapshotKeep
	}
	pruneVersionSnapshots(fc, keep)
	if err := cleanTempFiles(fc); err != nil {
		printOutput(false, fmt.Sprintf("clean temp files: %v", err), nil)
		return
	}
	printOutput(true, "", &model.CleanData{Mode: "keep", Keep: keep})
}

// cleanAll 激进清理：--keep 1 的 prune（仅保留最新一个可回滚快照）+ 临时文件清理。
// 保护集语义由 pruneVersionSnapshots 内部保证（MainFolder/Version/Previous/RollbackTarget
// 永不删除），因此 --all 不会破坏当前运行与崩溃恢复能力。
func cleanAll(fc *FullConfig) {
	pruneVersionSnapshots(fc, 1)
	if err := cleanTempFiles(fc); err != nil {
		printOutput(false, fmt.Sprintf("clean temp files: %v", err), nil)
		return
	}
	printOutput(true, "", &model.CleanData{Mode: "all"})
}

// cleanTempFiles 清理 UpdateFolder 与 PackageFolder 下的临时产物：
//   - 版本目录旁的 .old / .old.N（removeAsideVariants 的入口式调用）；
//   - 孤儿无版本号备份目录（{MainFolder}_，同 prune 内部触发的 removeOrphanBackupDir）；
//   - 版本快照目录与 .updator 内的 .part / .tmp 下载残留（#11：不递归 MainFolder）；
//   - 超过 30 天的 update_*.log / update_*_fail.log。
func cleanTempFiles(fc *FullConfig) error {
	pkgDir, err := config.PackageDir()
	if err != nil {
		return err
	}
	exeDir, err := config.ExeDir()
	if err != nil {
		return err
	}

	// 1) 旁移残留：遍历 PackageFolder 下 {folderName}_{version}.old* 家族，
	//    以及 MainFolder 本体被隐式旁移产生的 {folderName}.old / {folderName}.old.N
	//    （M9：该名字不匹配 {folderName}_ 前缀规则，clean/prune 原有规则都收不到 → 永久泄漏）。
	if folderName, fErr := fc.ExeCfg.MainExeFolderName(); fErr == nil {
		prefix := folderName + "_"
		mainAside := folderName + ".old"
		if entries, rErr := readDirNames(pkgDir); rErr == nil {
			for _, name := range entries {
				full := filepath.Join(pkgDir, name)
				if name == mainAside || strings.HasPrefix(name, mainAside+".") {
					if rmErr := os.RemoveAll(full); rmErr == nil {
						util.AppendToLog(exeDir, "update.log", fmt.Sprintf("clean: removed aside variant %s", full))
					}
					continue
				}
				if !strings.HasPrefix(name, prefix) {
					continue
				}
				if strings.Contains(name, ".old") {
					if rmErr := os.RemoveAll(full); rmErr == nil {
						util.AppendToLog(exeDir, "update.log", fmt.Sprintf("clean: removed aside variant %s", full))
					}
				}
			}
		}
		// 2) 孤儿无版本号备份目录（AppVersionDir("")）
		removeOrphanBackupDir(fc)
	}

	// 3) .part/.tmp 下载残留（#11 收窄）：只清两处——
	//   a. 版本快照目录（ApplicationFolder_{version}，下载目标目录，可递归）；
	//   b. .updator 内的运行时临时文件（version.json 原子写 .tmp 等，不递归子目录）；
	//   c. pkgDir 顶层非递归清一遍（孤儿 .part 兜底）。
	// 不再对 MainFolder 递归：业务程序（尤其老 Win32/办公软件）会持久持有
	// .tmp 数据文件，全量递归会无差别误删；MainFolder 内若有下载中断残留，
	// 由 download 流程自清理（重下前计数/续传逻辑）。
	folderName, fnErr := fc.ExeCfg.MainExeFolderName()
	if fnErr == nil {
		prefix := folderName + "_"
		if entries, rErr := readDirNames(pkgDir); rErr == nil {
			for _, name := range entries {
				if !strings.HasPrefix(name, prefix) {
					continue
				}
				full := filepath.Join(pkgDir, name)
				if info, sErr := os.Stat(full); sErr == nil && info.IsDir() {
					removeTempSuffixFiles(full)
				}
			}
		}
	}
	if updatorDir := config.UpdatorDir(fc.MainFolder); updatorDir != "" {
		removeTempFilesNonRecursive(updatorDir)
	}
	removeTempFilesNonRecursive(pkgDir)

	// 4) 30 天前的更新失败日志与常驻日志（update.log / download.log / check_diff.log，
	// 含 AppendToLog 轮转出的 .1 代）：只保留近期排障价值。
	// 轻微项：原实现只清 update_*.log——常驻日志（update. vs update_）全部漏网，
	// 无轮转时无限增长；现已纳入统一 cutoff（轮转代同样按修改时间回收）。
	cutoff := time.Now().AddDate(0, 0, -30)
	steadyLogs := map[string]bool{
		"update.log":     true,
		"download.log":   true,
		"check_diff.log": true,
	}
	if entries, rErr := readDirNames(exeDir); rErr == nil {
		for _, name := range entries {
			// 去掉轮转代数后缀（update.log.1 → update.log）后判断是否属于清理范围
			baseName := strings.TrimSuffix(name, ".1")
			steadyLog := steadyLogs[baseName]
			failureLog := strings.HasPrefix(name, "update_") && strings.HasSuffix(name, ".log")
			if !steadyLog && !failureLog {
				continue
			}
			full := filepath.Join(exeDir, name)
			if info, sErr := os.Stat(full); sErr == nil && !info.IsDir() && info.ModTime().Before(cutoff) {
				if rmErr := os.Remove(full); rmErr == nil {
					util.AppendToLog(exeDir, "update.log", fmt.Sprintf("clean: removed stale log %s", full))
				}
			}
		}
	}
	return nil
}

// removeTempFilesNonRecursive 仅删除 dir 顶层的 .part/.tmp 文件（不进入子目录）。
// 用于 .updator / pkgDir 顶层等只需要浅清理的位置（#11）。
func removeTempFilesNonRecursive(dir string) {
	entries, err := readDirNames(dir)
	if err != nil {
		return // 尽力而为
	}
	for _, name := range entries {
		if !strings.HasSuffix(name, ".part") && !strings.HasSuffix(name, ".tmp") {
			continue
		}
		full := filepath.Join(dir, name)
		if info, sErr := os.Stat(full); sErr != nil || info.IsDir() {
			continue
		}
		if rmErr := os.Remove(full); rmErr == nil {
			exeDir, _ := config.ExeDir()
			util.AppendToLog(exeDir, "update.log", fmt.Sprintf("clean: removed temp file %s", full))
		}
	}
}

// removeTempSuffixFiles 递归删除 dir 下所有 .part/.tmp 文件（下载残留）。
// 只用于版本快照目录；MainFolder 等业务目录禁止调用（#11）。
// 遍历失败静默跳过（清理是尽力而为，不因单个目录 ACL 失败整体报错）。
func removeTempSuffixFiles(dir string) {
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 尽力而为
		}
		if info.IsDir() {
			return nil
		}
		name := info.Name()
		if strings.HasSuffix(name, ".part") || strings.HasSuffix(name, ".tmp") {
			if rmErr := os.Remove(path); rmErr == nil {
				exeDir, _ := config.ExeDir()
				util.AppendToLog(exeDir, "update.log", fmt.Sprintf("clean: removed temp file %s", path))
			}
		}
		return nil
	})
}

// readDirNames 读目录名列表（出错返回 err，供调用方跳过）。
func readDirNames(dir string) ([]string, error) {
	d, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer d.Close()
	return d.Readdirnames(0)
}
