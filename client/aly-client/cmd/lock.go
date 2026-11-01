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
// 成功后启动 touch 协程：持有期间每 lockTouchInterval 重写时间戳（原子落盘），
// 保证"慢网络大更新拖过 TTL"时 TTL 不会误判陈旧（#9）。
func AcquireUpdateLock(cmdName string) (func(), error) {
	path := updateLockPath()
	for attempt := 0; attempt < 2; attempt++ {
		release, err := tryAcquireLock(path, cmdName)
		if err == nil {
			// touch 协程：周期刷新锁时间戳（write-tmp + rename 原子落盘）。
			// 释放函数先等待协程退出再删锁，避免 touch 与 release 并发
			//（协程在 release 删锁后把锁文件写回，造成锁泄漏）。
			stop := make(chan struct{})
			done := make(chan struct{})
			go func() {
				defer close(done)
				touchLockLoop(path, cmdName, stop)
			}()
			inner := release
			return func() {
				close(stop)
				<-done
				inner()
			}, nil
		}
		// 锁被占用：若持有者已退出（崩溃残留）或超时（陈旧）则清理并重试一次
		if !cleanStaleLock(path) {
			return nil, fmt.Errorf("另一更新正在进行中，请稍后重试")
		}
	}
	return nil, fmt.Errorf("另一更新正在进行中，请稍后重试")
}

// lockTouchInterval 锁心跳周期（#9）：远小于 lockTTL，保证 TTL 判定基于
// "最后一次 touch" 而非"获取时刻"——持有者活着且在工作就不被抢占。
const lockTouchInterval = 5 * time.Minute

// touchLockLoop 周期性重写锁时间戳（仅当锁仍归本持有者时）。
// 用 write-tmp + rename 原子落盘：读取方要么看到旧完整内容、要么看到新完整内容，
// 不会读到写了一半的 JSON。
func touchLockLoop(path, cmdName string, stop <-chan struct{}) {
	ticker := time.NewTicker(lockTouchInterval)
	defer ticker.Stop()
	token := lockTokenOf(path, cmdName)
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			// 锁已被他人抢占（TTL 判定或崩溃清理）：停止 touch（token 不匹配写不进去，
			// 但显式退出更干净）。
			if !isLockHeldBy(path, token) {
				return
			}
			writeLockFile(path, cmdName, token)
		}
	}
}

// lockTokenOf 生成持有者 token（与 tryAcquireLock 一致格式）。
// 注意：touch 用独立 token 而非重新生成——同一持有者跨多次写入需保持 token 不变，
// 否则 releaseLock 的 token 匹配删除会失效。token 从锁文件读回（见 tryAcquireLock 的返回）。
func lockTokenOf(path, cmdName string) string {
	// 从当前锁文件读回本持有者 token（tryAcquireLock 已写入）。
	if data, err := ioutil.ReadFile(path); err == nil {
		var info updateLockInfo
		if json.Unmarshal(data, &info) == nil && info.Token != "" {
			return info.Token
		}
	}
	return "" // 读不回（异常）：touchLoop 侧靠 isLockHeldBy("") 判 false 退出
}

// isLockHeldBy 判断锁文件当前持有者 token 是否匹配。
func isLockHeldBy(path, token string) bool {
	if token == "" {
		return false
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return false
	}
	var info updateLockInfo
	if json.Unmarshal(data, &info) != nil {
		return false
	}
	return info.Token == token
}

// writeLockFile 原子写入锁内容（write-tmp + rename，#9 竞态修复）。
func writeLockFile(path, cmdName, token string) {
	info := updateLockInfo{
		PID:   os.Getpid(),
		TS:    strconv.FormatInt(time.Now().Unix(), 10),
		Cmd:   cmdName,
		Token: token,
	}
	data, err := json.Marshal(info)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if wErr := ioutil.WriteFile(tmp, data, 0644); wErr != nil {
		return
	}
	if rErr := os.Rename(tmp, path); rErr != nil {
		os.Remove(tmp)
	}
}

// tryAcquireLock 尝试独占创建锁文件。
// 写入采用 write-tmp + rename 原子落盘（#9）：旧实现 O_EXCL 创建后直接写锁文件，
// "创建成功到写入完成"之间有空窗——另一等待者读到空/不完整内容会按"残留"删掉
// 刚创建的新锁，导致双实例并发更新。
func tryAcquireLock(path, cmdName string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	f.Close() // 立即关闭：占位已由 O_EXCL 建立，内容走原子写入
	token := fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())
	writeLockFile(path, cmdName, token)
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
//   - 读取/解析失败：结合 mtime 二次确认（#9）——1 分钟内的空/坏锁视为"在途写入"
//     （O_EXCL 占位与原子写入之间的正常窗口），不删；超过 1 分钟才按残留清理；
//   - 持有者进程已退出：删除；
//   - 持有超过 lockTTL 且最后 touch 无进展：删除（真正的陈旧锁——
//     touch 机制上线后，存活持有者的 TS 会持续刷新，只有"持有者活着但卡死/无 touch"
//     才会超 TTL。注意 PID 复用场景：TTL 抢占作为最后手段保留）；
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
		// 内容空/损坏：区分"在途写入"与"真残留"（#9）。
		// 原子写入（write-tmp + rename）下窗口极短，但防御 O_EXCL 占位刚建立、
		// rename 尚未发生的瞬间。
		if fi, statErr := os.Stat(path); statErr == nil && time.Since(fi.ModTime()) < time.Minute {
			return false // 可能是刚创建的在途锁：不删，稍后重试
		}
		os.Remove(path)
		return true
	}
	ts, ok := parseLockTS(info.TS)
	if !ok {
		// TS 缺失或无法解析：视为陈旧
		os.Remove(path)
		return true
	}
	// TTL 抢占：持有超过 lockTTL 无 touch 进展（touchLoop 每 5 分钟刷新 TS，
	// 存活且工作的持有者不会触发本分支）。
	if time.Now().Unix()-ts > int64(lockTTL/time.Second) {
		// 存活二次确认（#9）：持有者活着时不抢占——接受"锁泄漏"风险
		//（极端场景：进程卡死不退出也不释放），clean 命令可手工清理。
		// PID 复用导致的"锁永久有效"由下方"超 TTL 且进程不存活"兜住。
		if util.IsProcessAlive(uint32(info.PID)) {
			return false
		}
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
