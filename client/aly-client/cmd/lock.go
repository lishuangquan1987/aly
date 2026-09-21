package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"aly/client/aly-client/config"
	"aly/client/aly-client/util"
)

// 全局更新锁：借鉴 Velopack 的 .velopack_lock，保证同一时刻只有一个
// download_update / apply_update / rollback 在运行，避免并发写状态导致混乱。
//
// 锁文件位于 UpdateFolder/.aly.lock，内容含持有者 PID、时间戳与随机 token；
// 陈旧检测：持有者进程已退出（崩溃残留）自动清理；超过 lockTTL 无条件抢占
// （覆盖"持有者崩溃 + PID 被复用"导致锁永久有效的场景，#9）。

// lockTTL 锁持有超时：超过该时长无条件抢占
const lockTTL = 30 * time.Minute

type updateLockInfo struct {
	PID   int    `json:"pid"`
	TS    string `json:"ts"`
	Cmd   string `json:"cmd"`
	Token string `json:"token"`
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
		// 锁被占用：若持有者已退出（崩溃残留）或超时（陈旧）则清理并重试一次
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
	token := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	info := updateLockInfo{
		PID:   os.Getpid(),
		TS:    strconv.FormatInt(time.Now().Unix(), 10),
		Cmd:   cmdName,
		Token: token,
	}
	if data, mErr := json.Marshal(info); mErr == nil {
		f.Write(data)
	}
	f.Close()
	return func() {
		releaseLock(path, token)
	}, nil
}

// releaseLock 释放锁：仅当锁文件仍是本次持有者（token 匹配）时删除，
// 避免误删已被 TTL 抢占或新持有者的锁。
func releaseLock(path, token string) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		// 锁文件已不存在（可能已被清理）：无需处理
		return
	}
	var info updateLockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		// 内容损坏：无法确认持有者，保守不删除
		return
	}
	if info.Token != "" && info.Token == token {
		os.Remove(path)
		return
	}
	// token 为空（旧格式锁）或不匹配（已被他人抢占）：不删除，避免误删他人锁
	util.AppendToLog(logDir(), "update.log",
		fmt.Sprintf("release lock skipped: token mismatch (pid=%d)", info.PID))
}

// cleanStaleLock 判断锁是否陈旧并清理：
//   - 读取/解析失败：视为残留，删除后返回 true；
//   - 持有超过 lockTTL（30 分钟）：无条件删除抢占（覆盖 PID 复用场景）；
//   - 持有者进程已退出：删除；
//   - 持有者仍存活且在 TTL 内：返回 false（锁有效，不应抢占）。
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
	ts, ok := parseLockTS(info.TS)
	if !ok {
		// TS 缺失或无法解析：视为陈旧
		os.Remove(path)
		return true
	}
	// TTL 抢占：锁持有超过 lockTTL 无条件删除
	if time.Now().Unix()-ts > int64(lockTTL/time.Second) {
		os.Remove(path)
		return true
	}
	if util.IsProcessAlive(uint32(info.PID)) {
		return false // 持有者存活
	}
	os.Remove(path)
	return true
}

// parseLockTS 解析锁时间戳：优先 Unix 秒（新格式），兼容旧格式 "2006-01-02 15:04:05"。
// TS 为空或格式未知返回 (0, false)；TS 在未来（时钟偏差）时返回其值（不会触发 TTL 抢占）。
func parseLockTS(ts string) (int64, bool) {
	if ts == "" {
		return 0, false
	}
	if unix, err := strconv.ParseInt(ts, 10, 64); err == nil {
		return unix, true
	}
	if t, err := time.Parse("2006-01-02 15:04:05", ts); err == nil {
		return t.Unix(), true
	}
	return 0, false
}
