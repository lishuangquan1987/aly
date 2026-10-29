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
	fs := flag.NewFlagSet("clean", flag.ExitOnError)
	keepFlag := fs.Int("keep", 0, "保留最近 N 个版本快照（默认 3）")
	allFlag := fs.Bool("all", false, "激进清理：只保留当前版本保护集")
	fs.Parse(os.Args[2:])

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
//   - 所有版本目录与 MainFolder 内的 .part / .tmp 下载残留；
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

	// 1) 旁移残留：遍历 PackageFolder 下 {folderName}_{version}.old* 家族
	if folderName, fErr := fc.ExeCfg.MainExeFolderName(); fErr == nil {
		prefix := folderName + "_"
		if entries, rErr := readDirNames(pkgDir); rErr == nil {
			for _, name := range entries {
				if !strings.HasPrefix(name, prefix) {
					continue
				}
				if strings.Contains(name, ".old") {
					full := filepath.Join(pkgDir, name)
					if rmErr := os.RemoveAll(full); rmErr == nil {
						util.AppendToLog(exeDir, "update.log", fmt.Sprintf("clean: removed aside variant %s", full))
					}
				}
			}
		}
		// 2) 孤儿无版本号备份目录（AppVersionDir("")）
		removeOrphanBackupDir(fc)
	}

	// 3) .part/.tmp：版本目录 + MainFolder 内（.updator 内运行时文件不动）
	dirs := []string{fc.MainFolder, pkgDir}
	if entries, rErr := readDirNames(pkgDir); rErr == nil {
		for _, name := range entries {
			full := filepath.Join(pkgDir, name)
			if info, sErr := os.Stat(full); sErr == nil && info.IsDir() {
				dirs = append(dirs, full)
			}
		}
	}
	for _, dir := range dirs {
		removeTempSuffixFiles(dir)
	}

	// 4) 30 天前的更新失败日志（update_*.log 只保留近期排障价值）
	cutoff := time.Now().AddDate(0, 0, -30)
	if entries, rErr := readDirNames(exeDir); rErr == nil {
		for _, name := range entries {
			if !strings.HasPrefix(name, "update_") || !strings.HasSuffix(name, ".log") {
				continue
			}
			full := filepath.Join(exeDir, name)
			if info, sErr := os.Stat(full); sErr == nil && info.ModTime().Before(cutoff) {
				if rmErr := os.Remove(full); rmErr == nil {
					util.AppendToLog(exeDir, "update.log", fmt.Sprintf("clean: removed stale log %s", full))
				}
			}
		}
	}
	return nil
}

// removeTempSuffixFiles 递归删除 dir 下所有 .part/.tmp 文件（下载残留）。
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
