// Package selfupdate 提供 aly-client / aly-client-ui 的自更新核心编排：
//
//	.new 中转复制 → SHA256 校验 → 运行中 exe 改名 .old → .new 原子落位
//	→ 失败回滚 → 残留（.new/.old）清理 → 失败计数（Journal，防止无限重启循环）。
//
// 运行中可执行文件的替换遵循"先备后换"：慢操作（复制 + 校验）全部在 self
// 仍然存在时完成，最后只剩两次 rename，目标缺失窗口极短。
//
// 设计约束（AGENTS.md）：Go 1.10 兼容（XP 轨）、仅标准库、**无平台分支**——
//   - Windows 的 os.Rename 底层为 MoveFileEx(MOVEFILE_REPLACE_EXISTING)，
//     覆盖已存在目标是原子的，失败时目标保持原样；
//   - "运行中的可执行文件"在 Windows / Unix 上都允许被**改名**（不允许的
//     是删除与写打开），因此"self 改名到 .old → 新文件落位"的舞步在两端
//     语义一致，单一路径即可覆盖全平台。
//
// 版本来源约定（与 check_self_update / self_update / UI UpdateUpdater /
// SDK UpdateSelf 一致）：主程序目录里随版本分发的 aly-client(.exe) 是唯一
// 可信源——其完整性已由主更新链路（download_update 的服务端 MD5+SHA256）
// 保证，本包只做"复制后二次校验"，不重复实现下载侧校验。
package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// ────────────────────────────────────────────────────────────────
// 基础工具：SHA256 / 相等比较 / 带校验复制
// ────────────────────────────────────────────────────────────────

// SHA256Of 返回文件内容的 SHA256 十六进制串（小写）。
func SHA256Of(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SameFile 判断两个文件内容是否一致（SHA256；与用例 92 的口径一致：
// 不用 mtime/size——mtime 在拷贝后会变，会误判"永远需要更新"）。
// 任一文件读取失败时返回错误（调用方决定缺文件时的跳过策略）。
func SameFile(a, b string) (bool, error) {
	ha, err := SHA256Of(a)
	if err != nil {
		return false, err
	}
	hb, err := SHA256Of(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}

// CopyAndVerify 把 src 复制到 tmp，并校验副本与源逐字节一致；
// 校验失败时删除 tmp（避免半截文件被换上）。
func CopyAndVerify(src, tmp string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	// 落盘后再 rename：否则断电可能出现“名字已换、内容截断”的新文件。
	// Windows 下 FlushFileBuffers、Unix 下 fsync，Go 1.10 的 File.Sync 已覆盖两端。
	if err = out.Sync(); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err = out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	// 可执行位：源是可执行文件（Windows 下无意义但无害；Unix 下必须保留）
	if fi, statErr := os.Stat(src); statErr == nil {
		if chmodErr := os.Chmod(tmp, fi.Mode()); chmodErr != nil {
			os.Remove(tmp)
			return chmodErr
		}
	}

	same, err := SameFile(src, tmp)
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if !same {
		os.Remove(tmp)
		return fmt.Errorf("copy %s -> %s: checksum mismatch", src, tmp)
	}
	return nil
}

// ────────────────────────────────────────────────────────────────
// 替换编排
// ────────────────────────────────────────────────────────────────

// Options 控制替换行为；零值可用（Retries=3、Delay=1s、Move=os.Rename）。
// Move 是测试注入点：默认 os.Rename（Windows 下等价 MoveFileEx(REPLACE_EXISTING)）。
type Options struct {
	Retries int
	Delay   time.Duration
	Move    func(src, dst string) error
}

func (o *Options) normalized() Options {
	n := *o
	if n.Retries <= 0 {
		n.Retries = 3
	}
	if n.Delay <= 0 {
		n.Delay = time.Second
	}
	if n.Move == nil {
		n.Move = os.Rename
	}
	return n
}

// ReplaceFromFile 用 src 替换 dst（编排：dst.new 中转 + 校验 + 原子替换）。
//
// 不用「先 os.Remove 再 os.Rename」：那中间存在目标不存在的窗口，一旦此时
// 进程被杀/断电，dst 就彻底没了。os.Rename 在 Windows 上带 REPLACE_EXISTING
// 语义，失败时目标保持原样；.new 残留会在下次 CleanArtifacts 时清理。
//
// src 不存在视为不可恢复错误（调用方应在进入本函数前做 source_missing 跳过）。
func ReplaceFromFile(src, dst string, opts Options) error {
	o := opts.normalized()
	tmp := dst + ".new"

	var lastErr error
	for i := 0; i < o.Retries; i++ {
		if err := CopyAndVerify(src, tmp); err != nil {
			lastErr = err
			time.Sleep(o.Delay)
			continue
		}
		if err := o.Move(tmp, dst); err != nil {
			lastErr = err
			os.Remove(tmp)
			time.Sleep(o.Delay)
			continue
		}
		return nil
	}
	return fmt.Errorf("replace %s: %v", dst, lastErr)
}

// ReplaceRunningSelf 用 src 替换**正在运行**的 selfPath（self_update 舞步）：
//
//  1. **先备**：在 selfPath.new 上复制并校验源。此阶段 selfPath 始终存在，
//     磁盘慢/杀软扫描/重试都不会暴露“目标缺失”窗口；
//  2. **换位**：把运行中的 self 改名到不冲突的 .old 侧路（运行中的文件允许
//     改名，原进程从旧 inode 继续运行），再把校验过的 .new 原子改名到原路径；
//  3. **回滚**：落位失败则把 .old 改名回去；回滚也失败时保留 .old/.new 现场
//     并以错误明确暴露（self 缺失比替换失败更严重）。
//
// 与“先改名再复制”相比，缺失窗口从整个复制时长（最长 3 次重试）缩到两次
// rename 之间。即使在这极短窗口内进程被杀，UI/SDK 的兜底落位或手工把 .old
// 改回仍可恢复。
//
// 成功时返回 .old 路径（留给下一次 CleanArtifacts 清理）；失败时 self 保持
// 原样，或（回滚也失败时）由错误信息明确指出现场异常。
func ReplaceRunningSelf(selfPath, srcPath string, opts Options) (oldPath string, err error) {
	o := opts.normalized()
	tmp := selfPath + ".new"

	// 1) 先备：复制 + 校验（慢操作），失败不动 self。
	var lastErr error
	prepared := false
	for i := 0; i < o.Retries; i++ {
		if copyErr := CopyAndVerify(srcPath, tmp); copyErr != nil {
			lastErr = copyErr
			time.Sleep(o.Delay)
			continue
		}
		prepared = true
		break
	}
	if !prepared {
		return "", fmt.Errorf("prepare replace %s: %v", selfPath, lastErr)
	}

	// 2) 换位：self -> .old（原子），.new -> self（原子）。
	oldPath = NextAsidePath(selfPath, ".old")
	if renameErr := os.Rename(selfPath, oldPath); renameErr != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("rename running self %s -> %s: %v", selfPath, oldPath, renameErr)
	}

	var moveErr error
	for i := 0; i < o.Retries; i++ {
		if moveErr = o.Move(tmp, selfPath); moveErr == nil {
			return oldPath, nil
		}
		time.Sleep(o.Delay)
	}

	// 3) 回滚：把 .old 改名回原路径；回滚失败时优先报回滚错误。
	if rbErr := os.Rename(oldPath, selfPath); rbErr != nil {
		return oldPath, fmt.Errorf("replace failed (%v) AND rollback failed (%v): %s is missing", moveErr, rbErr, selfPath)
	}
	os.Remove(tmp)
	return "", fmt.Errorf("replace %s: %v", selfPath, moveErr)
}

// ────────────────────────────────────────────────────────────────
// 残留清理
// ────────────────────────────────────────────────────────────────

// NextAsidePath 返回一个不冲突的侧路名：dst.old / dst.old.1 / dst.old.2 …
// （与 cmd 包 nextAsideName 的命名约定一致，供日志对账）。
func NextAsidePath(dst, suffix string) string {
	aside := dst + suffix
	for i := 1; ; i++ {
		if _, statErr := os.Stat(aside); os.IsNotExist(statErr) {
			return aside
		}
		aside = dst + suffix + "." + strconv.Itoa(i)
	}
}

// CleanArtifacts 清理 self-update 过程中的临时/备份残留（尽力而为，
// 任何清理失败都不报错——残留无害，只是多占一份磁盘）：
//   - dst+".new"：复制/替换中断的临时物，无条件可删；
//   - dst+".old*"：舞步留下的旧版本备份，只在 dst 本体存在时才删
//     （dst 缺失说明上次替换失败且回滚也失败，.old 是最后的现场，不能动）。
func CleanArtifacts(dst string) {
	os.Remove(dst + ".new")
	if _, err := os.Stat(dst); err == nil {
		matches, globErr := filepath.Glob(dst + ".old*")
		if globErr != nil {
			return
		}
		for _, m := range matches {
			os.Remove(m)
		}
	}
}

// ────────────────────────────────────────────────────────────────
// 失败计数（Journal）
// ────────────────────────────────────────────────────────────────

// journalSuffix 追加在自更新目标文件名后的 journal 文件后缀，
// 如 aly-client-ui.exe.selfupdate.json。
const journalSuffix = ".selfupdate.json"

// Journal 记录自更新连续失败次数与最近一次失败时间，用于阻断
// "替换失败 → 重启 → 再替换失败"的无限循环（UI 自更新会自动重启进程）。
//
// Pending 表示"已经把替换交给 UI 收尾进程（-finish-self-update），但尚未
// 观测到收敛"：收尾进程的等待/改名/重启失败无法被派生方捕获，只能在下一次
// 启动时按 Pending 计一次未收敛失败。旧 journal 没有该字段时读出 false，
// 向后兼容。
type Journal struct {
	Count     int    `json:"count"`
	LastTS    int64  `json:"last_ts"`
	LastError string `json:"last_error"`
	Pending   bool   `json:"pending,omitempty"`
}

// JournalPath 返回 target 对应的 journal 文件路径。
func JournalPath(target string) string {
	return target + journalSuffix
}

// LoadJournal 读取 journal 文件；文件缺失或损坏时返回零值
//（损坏视为无历史，允许重试——保守方向是"多试一次"而不是"拒绝服务"）。
func LoadJournal(path string) Journal {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return Journal{}
	}
	var j Journal
	if json.Unmarshal(data, &j) != nil {
		return Journal{}
	}
	return j
}

// SaveJournal 原子落盘 journal（tmp + rename）；失败静默——journal 只影响
// 重试节奏，写失败不应让自更新本身失败。
func SaveJournal(path string, j Journal) {
	data, err := json.Marshal(j)
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

// MarkSuccess 清零计数（成功或"无差异 noop"都算收敛信号）。
func (j *Journal) MarkSuccess() {
	j.Count = 0
	j.LastTS = 0
	j.LastError = ""
	j.Pending = false
}

// MarkAttempt 记录"已把替换交给 UI 收尾进程，结果未知"。它不增加失败计数，
// 也不改 LastTS——真正的失败由下一次启动发现 Pending 仍为 true 时计入
//（收尾进程的等待/改名/重启失败无法被派生方直接观测）。
func (j *Journal) MarkAttempt() {
	j.Pending = true
}

// MarkFailure 记一次失败：最近失败在窗口外时重置为 1（旧失败已过观察期），
// 窗口内则累加。失败一旦被观测到，Pending 即清除。
func (j *Journal) MarkFailure(msg string) {
	now := time.Now().Unix()
	if j.LastTS > 0 && now-j.LastTS >= failureWindowForReset {
		j.Count = 0
	}
	j.Count++
	j.LastTS = now
	j.LastError = msg
	j.Pending = false
}

// ShouldSkip 判断是否应跳过本次自更新：连续失败 ≥ maxAttempts 且最近一次
// 失败仍在 window 内。窗口过后允许再试（给磁盘满/杀软占用这类瞬时故障
// 恢复的机会）。
func (j Journal) ShouldSkip(maxAttempts int, window time.Duration) bool {
	if j.Count < maxAttempts || j.LastTS <= 0 {
		return false
	}
	return time.Now().Unix()-j.LastTS < int64(window/time.Second)
}

// failureWindowForReset：MarkFailure 里判断"是否重新计数"的最小间隔（秒）。
// 与 UI 侧 selfUpdateRetryWindow 同量级（30 分钟），避免长间隔的孤立失败
// 被累计成"连续失败"。
const failureWindowForReset = 30 * 60
