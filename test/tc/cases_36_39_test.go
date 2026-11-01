package tc

// 用例 36–39：极限条件
//
// 不硬断言性能数字，只断言**正确性**：量大/大文件/慢速都不能把机制撑破。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// countFiles 递归统计目录下的常规文件数（对应 downloaded_file_count 的口径）。
func countFiles(root string) int {
	n := 0
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil {
			return nil
		}
		if !fi.IsDir() {
			n++
		}
		return nil
	})
	return n
}

// ────────────────────────────────────────────────────────────────
// 用例 36 — 超大文件（120MB）下载与校验
// ────────────────────────────────────────────────────────────────
func TestCase36(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-120"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	hugeSrc := filepath.Join(e.DataDir(proj), "huge.bin")
	BigFile(t, hugeSrc, 120)
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Project: proj, NoMain: true,
	})

	outs := e.Run(t, c, "download_update")
	last := outs[len(outs)-1]
	True(t, last.IsSuccess, "120MB 下载失败: %s", last.ErrorMsg)

	// PROGRESS 必须持续（单文件大也不能长时间无输出）
	progressCount := 0
	for _, o := range outs {
		if o.Progress != nil && o.Progress.Status == "PROGRESS" {
			progressCount++
		}
	}
	True(t, progressCount >= 1, "120MB 下载全程无 PROGRESS 行（UI 90s idle 会误杀）")

	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 失败")

	final := filepath.Join(c.Main(), "huge.bin")
	Eq(t, "大小", SizeOf(final), int64(120<<20))
	Eq(t, "MD5 与源一致", MD5Of(final), MD5Of(hugeSrc))
	t.Logf("120MB 下载 PROGRESS 行数: %d", progressCount)
}

// ────────────────────────────────────────────────────────────────
// 用例 37 — 慢下载不被误杀（单文件 >90 秒）
//
// 说明：本机回环无法真正造出 >90s 的单文件下载（除非限速代理）。
// 这里走"可验证的代理指标"：PROGRESS 行必须存在且数量足够，
// 使任意相邻输出的间隔理论上 < 90s。真正的 90s idle 判定由
// 用例 40–45 的 UI 侧覆盖。做不到限速时按 SKIP 记录原因。
// ────────────────────────────────────────────────────────────────
func TestCase37(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-slow"
	e.EnsureProject(t, proj, false, nil, nil)
	bigSrc := filepath.Join(e.DataDir(proj), "big.bin")
	BigFile(t, bigSrc, 200) // 200MB，拉长耗时
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Project: proj, NoMain: true,
	})

	start := time.Now()
	outs := e.Run(t, c, "download_update")
	elapsed := time.Since(start)
	last := outs[len(outs)-1]
	True(t, last.IsSuccess, "下载失败: %s", last.ErrorMsg)

	progressCount := 0
	for _, o := range outs {
		if o.Progress != nil && o.Progress.Status == "PROGRESS" {
			progressCount++
		}
	}
	Eq(t, "最终大小", SizeOf(filepath.Join(c.VerDir("1.1.0"), "big.bin")), int64(200<<20))
	Eq(t, "MD5 与源一致", MD5Of(filepath.Join(c.VerDir("1.1.0"), "big.bin")), MD5Of(bigSrc))

	// 不得出现停流误杀类错误
	for _, bad := range []string{"download stalled", "no output for 90s", "idle timeout"} {
		NotContains(t, last.ErrorMsg, bad, "不应出现停流误杀")
	}

	t.Logf("200MB 耗时 %s，PROGRESS 行 %d 条", elapsed, progressCount)
	if elapsed < 90*time.Second {
		t.Skipf("本机 %s 就传完 200MB，未进入 >90s 慢下载区间；"+
			"本用例的核心（90s idle 不误杀）由用例 44/45 的 UI 侧覆盖", elapsed)
	}
	// 真跑到 >90s 时，PROGRESS 必须存在，否则就是停流
	True(t, progressCount >= 1, "慢下载(%s)全程无 PROGRESS，UI 必然在 90s 处误杀", elapsed)
}

// ────────────────────────────────────────────────────────────────
// 用例 38 — 海量小文件（500 × 1KB）
// ────────────────────────────────────────────────────────────────
func TestCase38(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-many"
	e.EnsureProject(t, proj, false, nil, nil)
	MakeFiles(t, e.DataDir(proj), 500, 1024)
	e.PutFile(t, proj, "app.exe", "app-100")
	// 客户端 1.0.0 → 服务端发更高版本
	True(t, e.Publish(t, proj, "1.0.1", nil, "", "").IsSuccess, "发布失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Project: proj, NoMain: true,
	})

	start := time.Now()
	dl := e.RunLast(t, c, "download_update")
	ap := e.RunLast(t, c, "apply_update")
	t.Logf("500 文件全链路耗时: %s", time.Since(start))

	True(t, dl.IsSuccess && ap.IsSuccess, "500 文件链路失败: dl=%s ap=%s",
		dl.ErrorMsg, ap.ErrorMsg)

	// ListDirMatch 只扫顶层文件：主目录顶层是 500 小文件 + app.exe = 501；
	// `.updator/shared.json` 在子目录里，故 downloaded_file_count 是 502（递归计数）。
	Eq(t, "主目录顶层文件数", ListDirMatch(c.Main(), "*"), 501)
	Eq(t, "主目录递归文件数（含 .updator/shared.json）", countFiles(c.Main()), 502)
	t.Logf("主目录顶层 %d / 递归 %d", ListDirMatch(c.Main(), "*"), countFiles(c.Main()))

	Eq(t, "抽查 f0001.txt",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "f0001.txt"))), strings.Repeat("a", 1024))
	Eq(t, "抽查 f0500.txt",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "f0500.txt"))), strings.Repeat("a", 1024))
	Eq(t, "downloaded_file_count（501 业务 + shared.json）",
		c.ReadVersion(t).DownloadedFileCount, 502)
}

// ────────────────────────────────────────────────────────────────
// 用例 39 — 清单 1000+ 条目的差量计算
// ────────────────────────────────────────────────────────────────
func TestCase39(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-1000"
	e.EnsureProject(t, proj, false, nil, nil)
	MakeFiles(t, e.DataDir(proj), 1000, 1024)
	e.PutFile(t, proj, "app.exe", "app-100")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, NoMain: true,
	})

	// 关键：差量删除要有**基线缓存**才生效。首轮（1.0.1）必然只是
	// "no cached manifest (first cycle), skip delete-sync"，把缓存建立起来；
	// 第二轮（1.0.2）才是真正做 1000 条差量的那一轮。
	True(t, e.Publish(t, proj, "1.0.1", nil, "", "").IsSuccess, "发布 1.0.1")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.0.1（建缓存）")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.0.1")
	True(t, Exists(filepath.Join(c.Up(), "release_manifest.json")), "前置：首轮后应有缓存")
	t.Logf("首轮主目录条目: %d", ListDirMatch(c.Main(), "*"))

	// 服务端改造：删 100、增 50、改 10。
	// 注意：这里必须用 MakeFileRange(1001,1050) 而不是 MakeFiles(1050)——后者会
	// 把刚删掉的 f0001..f0100 重新写回，服务端清单与上轮一致，差集为空。
	for i := 1; i <= 100; i++ {
		e.RemoveFile(t, proj, fmt.Sprintf("f%04d.txt", i))
	}
	MakeFileRange(t, e.DataDir(proj), 1001, 1050, 1024) // 只增 50 个新文件
	for i := 200; i <= 209; i++ {
		e.PutFile(t, proj, fmt.Sprintf("f%04d.txt", i), fmt.Sprintf("modified-%d", i))
	}
	True(t, e.Publish(t, proj, "1.0.2", nil, "", "").IsSuccess, "发布 1.0.2")

	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.0.2")
	downloadLog := c.Log(t, "download.log", 300)
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.0.2")

	// 差量基线：上一轮缓存应被读到，且删掉的正好是 100 个。
	// download.log 是累积文件：第一轮 1.0.1 必然是 "first cycle"（缓存尚不存在），
	// 第二轮 1.0.2 若仍读到 nil 就会再写一条 → 因此断言"该提示只出现 1 次"。
	Eq(t, "\"no cached manifest\" 只应出现在首轮",
		strings.Count(downloadLog, "no cached manifest (first cycle), skip delete-sync"), 1)
	Contains(t, downloadLog, "delete-sync removed 100 file(s) deleted by server", "download.log")
	if i := strings.Index(downloadLog, "delete-sync removed"); i >= 0 {
		end := strings.Index(downloadLog[i:], "\n")
		if end > 0 {
			t.Logf("差量日志: %s", downloadLog[i:i+end])
		}
	}

	// 三类抽查
	Eq(t, "被删的 f0001.txt 不应存在",
		Exists(filepath.Join(c.Main(), "f0001.txt")), false)
	Eq(t, "被删的 f0100.txt 不应存在",
		Exists(filepath.Join(c.Main(), "f0100.txt")), false)
	Eq(t, "新增的 f1050.txt 应存在",
		Exists(filepath.Join(c.Main(), "f1050.txt")), true)
	Eq(t, "修改的 f0200.txt 内容",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "f0200.txt"))), "modified-200")
	Eq(t, "未变的 f0600.txt 应保留",
		Exists(filepath.Join(c.Main(), "f0600.txt")), true)

	// 缓存条目数 = 服务端清单条数
	var cache []string
	True(t, json.Unmarshal([]byte(ReadText(filepath.Join(c.Up(), "release_manifest.json"))), &cache) == nil,
		"缓存应是合法 JSON 数组")
	srv := e.GetManifest(t, proj)
	Eq(t, "缓存条数与服务端一致", len(cache), len(srv.Files))
	t.Logf("最终主目录条目数: %d", ListDirMatch(c.Main(), "*"))
}