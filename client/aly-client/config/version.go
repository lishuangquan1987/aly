package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

const (
	VersionStatusDownloaded = "downloaded"
	VersionStatusApplying   = "applying"
	VersionStatusApplied    = "applied"
)

// VersionInfo represents the version.json structure.
type VersionInfo struct {
	VersionPrevious        string `json:"version_previous"`
	Version                string `json:"version"`
	VersionStatus          string `json:"version_status"`
	AfterApplyUpdateScript string `json:"after_apply_update_script,omitempty"`
	// RollbackPrevious 记录回滚开始时 MainFolder 的真实内容版本（pre-rollback active version）。
	// 仅 rollback 在 status=applying 期间写入，用于崩溃恢复时正确还原备份目录对应的版本，
	// 避免 downloaded 状态下回滚导致的"目录名与实际内容错配"（#5）。
	RollbackPrevious string `json:"rollback_previous,omitempty"`
	// RollbackTarget 记录本次回滚的目标版本。仅 rollback 在 status=applying 期间写入，
	// 供"回滚中断"后的崩溃恢复使用（apply_update 委托 resumeRollback 时据此重建回滚意图，
	// 避免被当成升级处理，Bug#1/#5）。恢复完成后清空。
	RollbackTarget string `json:"rollback_target,omitempty"`
}

func versionPath() (string, error) {
	dir, err := ExeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "version.json"), nil
}

// ReadVersion reads version.json; returns an empty struct if the file does not exist (first deploy).
func ReadVersion() (*VersionInfo, error) {
	path, err := versionPath()
	if err != nil {
		return nil, err
	}

	data, err := ioutilReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &VersionInfo{}, nil
		}
		return nil, fmt.Errorf("读取 version.json 失败: %v", err)
	}

	var info VersionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		// 自愈链（Bug#4）：解析失败时先回退上次成功写入的 .bak；仍失败则隔离损坏文件
		// 并重置为首次部署语义（status="" → checkUpdateApplied → 重新下载应用，可自愈）。
		// 绝不静默瘫痪：check/download/apply/rollback 四命令此前都会因解析失败直接 return false。
		if bak, bErr := ioutilReadFile(path + ".bak"); bErr == nil {
			var bakInfo VersionInfo
			if json.Unmarshal(bak, &bakInfo) == nil {
				appendLog(fmt.Sprintf("version.json 损坏，已从 .bak 恢复: %v", err))
				// WriteVersion 内部不调用 ReadVersion，无递归风险
				WriteVersion(&bakInfo)
				return &bakInfo, nil
			}
		}
		os.Remove(path + ".corrupt") // 清理上一次隔离残留（Windows rename 目标已存在会失败）
		os.Rename(path, path+".corrupt")
		appendLog(fmt.Sprintf("version.json 损坏且无可用备份，已隔离为 .corrupt 并重置: %v", err))
		return &VersionInfo{}, nil
	}

	return &info, nil
}

// WriteVersion writes version.json.
func WriteVersion(info *VersionInfo) error {
	path, err := versionPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 version.json 失败: %v", err)
	}

	// 先保留上一版（last-known-good），供 ReadVersion 损坏自愈回退（尽力而为，失败不阻断）。
	// 仅当当前文件内容可解析时才覆盖 .bak：避免把损坏内容写进 last-known-good，
	// 保证自愈链"version.json 损坏 → 从 .bak 恢复"在恢复过程中不被破坏。
	if _, statErr := os.Stat(path); statErr == nil {
		if bakData, rErr := ioutilReadFile(path); rErr == nil {
			var cur VersionInfo
			if json.Unmarshal(bakData, &cur) == nil {
				ioutil.WriteFile(path+".bak", bakData, 0644)
			}
		}
	}

	// 原子写 + 落盘加固（Bug#4）：写 tmp → Sync（Windows 下即 FlushFileBuffers）→ rename。
	// 顺序必须是"先落盘后发布"：断电时不会出现"rename 已持久化、数据未落盘"的半截文件。
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("写入 version.json 失败: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("写入 version.json 失败: %v", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("同步 version.json 失败: %v", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("关闭 version.json 失败: %v", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("原子替换 version.json 失败: %v", err)
	}

	return nil
}

// appendLog 追加一行到 UpdateFolder/update.log（config 包内本地实现，避免与 util 包耦合）。
func appendLog(msg string) {
	dir, err := ExeDir()
	if err != nil || dir == "" {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "update.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", msg)
}
