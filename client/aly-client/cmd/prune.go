package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"aly/client/aly-client/config"
	"aly/client/aly-client/util"
)

// defaultSnapshotKeep 保留的最近版本快照数量（Bug#6 配额）。
const defaultSnapshotKeep = 3

// pruneVersionSnapshots 保留最近 keep 个版本快照，删除更旧的（修复 Bug#6：磁盘无上限增长）。
// 在 apply_update / rollback 入口（ReadVersion 后）机会式调用，属安全清理：
//
//	保护集合（绝不删除）：MainFolder、AppVersionDir(Version)（当前/待应用版本目录）、
//	AppVersionDir(VersionPrevious)（最新备份，回滚目标）、AppVersionDir(RollbackTarget)
//	（回滚中断现场的回滚目标）、extraVersions（调用方显式指定的额外保护版本，如 rollback 的
//	CLI 目标 —— 避免 rollback 入口清理时把用户正要回滚到的旧快照删掉）。
//	候选集合：其余 {MainFolder}_{version} 目录（isLikelyVersion 过滤，排除 .old 变体）。
//	按版本号降序保留最新 keep 个，删除更旧的，删除动作写 update.log 留痕。
func pruneVersionSnapshots(fc *FullConfig, keep int, extraVersions ...string) {
	if keep < 1 {
		keep = 1
	}
	pkgDir, err := config.PackageDir()
	if err != nil {
		return
	}
	folderName, err := fc.ExeCfg.MainExeFolderName()
	if err != nil {
		return
	}
	prefix := folderName + "_"

	dir, err := os.Open(pkgDir)
	if err != nil {
		return
	}
	names, err := dir.Readdirnames(0)
	dir.Close()
	if err != nil {
		return
	}

	protected := map[string]bool{fc.MainFolder: true}
	if vi, vErr := config.ReadVersion(); vErr == nil {
		for _, v := range []string{vi.Version, vi.VersionPrevious, vi.RollbackTarget} {
			if v == "" {
				continue
			}
			if d, dErr := fc.ExeCfg.AppVersionDir(v); dErr == nil {
				protected[d] = true
			}
		}
	}
	for _, v := range extraVersions {
		if v == "" {
			continue
		}
		if d, dErr := fc.ExeCfg.AppVersionDir(v); dErr == nil {
			protected[d] = true
		}
	}

	var candidates []string
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if !isLikelyVersion(strings.TrimPrefix(name, prefix)) {
			continue
		}
		full := filepath.Join(pkgDir, name)
		if protected[full] {
			continue
		}
		if info, statErr := os.Stat(full); statErr == nil && info.IsDir() {
			candidates = append(candidates, full)
		}
	}
	if len(candidates) <= keep {
		return
	}

	// 按版本号降序（保留最新），删除最老的 len(candidates)-keep 个。
	// 注意：比较的是去掉前缀后的版本号，不能直接用目录名（目录名含非数字前缀，
	// compareVersion 会把非数字段当 0，导致排序错误）。
	sort.Slice(candidates, func(i, j int) bool {
		vi := strings.TrimPrefix(filepath.Base(candidates[i]), prefix)
		vj := strings.TrimPrefix(filepath.Base(candidates[j]), prefix)
		return compareVersion(vi, vj) > 0
	})
	for _, c := range candidates[keep:] {
		if err := os.RemoveAll(c); err != nil {
			util.AppendToLog(logDir(), "update.log", fmt.Sprintf("prune version snapshot %s: %v", c, err))
		} else {
			util.AppendToLog(logDir(), "update.log", fmt.Sprintf("pruned old version snapshot %s", c))
		}
	}
}
