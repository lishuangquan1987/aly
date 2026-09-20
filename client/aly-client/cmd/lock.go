package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"aly/client/aly-client/config"
	"aly/client/aly-client/util"
)

// 全局更新锁：借鉴 Velopack 的 .velopack_lock，保证同一时刻只有一个
// download_update / apply_update / rollback 在运行，避免并发写状态导致混乱。
//
// 锁文件位于 UpdateFolder/.aly.lock，内容含持有者 PID 与时间戳；
// 持有者进程已退出（崩溃残留）时自动清理并重试获取。

type updateLockInfo struct {
	PID int    `json:"pid"`
	TS  string `json:"ts"`
	Cmd string `json:"cmd"`
}

// updateLockPath 返回锁文件路径（UpdateFolder/.aly.lock）
func updateLockPath() string {
	dir, err := config.ExeDir()
	if err != nil || dir == "" {
		return filepath.Join(".", ".aly.lock")
	}
	return filepath.Join(dir, ".aly.lock")
}

// AcquireUpdateLock 获取全局更新锁。成功返回释放函数（调用方 defer 调用）；
// 失败返回 error（锁被另一存活的更新进程持有）。
func AcquireUpdateLock(cmdName string) (func(), error) {
	path := updateLockPath()
	for attempt := 0; attempt < 2; attempt++ {
		release, err := tryAcquireLock(path, cmdName)
		if err == nil {
			return release, nil
		}
		// 锁被占用：若持有者已退出（崩溃残留）则清理并重试一次
		if !cleanStaleLock(path) {
			return nil, fmt.Errorf("另一更新正在进行中，请稍后重试")
		}
	}
	return nil, fmt.Errorf("另一更新正在进行中，请稍后重试")
}

// tryAcquireLock 尝试独占创建锁文件
func tryAcquireLock(path, cmdName string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	info := updateLockInfo{PID: os.Getpid(), TS: time.Now().Format("2006-01-02 15:04:05"), Cmd: cmdName}
	if data, mErr := json.Marshal(info); mErr == nil {
		f.Write(data)
	}
	f.Close()
	return func() {
		os.Remove(path)
	}, nil
}

// cleanStaleLock 若锁文件存在且持有者进程已退出（崩溃残留），删除并返回 true；
// 持有者仍存活返回 false（锁有效，不应抢占）。
func cleanStaleLock(path string) bool {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		// 读取失败（如刚被释放/被删）：清理后重试
		os.Remove(path)
		return true
	}
	var info updateLockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		// 内容损坏：视为残留
		os.Remove(path)
		return true
	}
	if util.IsProcessAlive(uint32(info.PID)) {
		return false // 持有者存活
	}
	os.Remove(path)
	return true
}
