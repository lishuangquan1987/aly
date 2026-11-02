package tc

// 用例 9–17：第四批新特性专项
//
// 覆盖 PROGRESS 进度协议、完成行 data:null 契约、差量删除（delete-sync）、
// 运行时数据保护、首周期保守、清单缓存固化/清除/损坏降级、续传基准。
//
// 独立执行：
//
//	go test -tags purego -run 'TestCase09$' -v -timeout 900s ./test/tc

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────
// 用例 9 — PROGRESS 行：大文件下载期间持续输出进度
//
// 重点：3s 节流；BEGIN 行带 total_bytes；PROGRESS 字段齐全；
//       末行 data=null；快照文件大小正确。
// ────────────────────────────────────────────────────────────────
func TestCase09(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-progress"
	e.EnsureProject(t, proj, false, nil, nil)
	BigFile(t, filepath.Join(e.DataDir(proj), "big.bin"), 30)
	True(t, e.Publish(t, proj, "2.0.0", nil, "", "").IsSuccess, "发布失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, NoMain: true, // 主目录空，确保必须下载
	})

	// 计时：PROGRESS 行是 3s 节流的（download_update.go 的 onBytes 里
	// `now.Sub(lastEmit) < 3*time.Second → return`），所以"整个下载耗时 < 3s"
	// 必然一行 PROGRESS 都没有——这属于环境过快，不是缺陷。用**耗时**而不是
	// 行数来区分"环境太快"与"输出回归"，才不会在更快的机器上误报。
	start := time.Now()
	outs := e.Run(t, c, "download_update")
	elapsed := time.Since(start)
	var progress []Progress
	for _, o := range outs {
		if o.Progress != nil && o.Progress.Status == "PROGRESS" {
			progress = append(progress, *o.Progress)
		}
	}
	last := outs[len(outs)-1]
	True(t, last.IsSuccess, "download_update 失败: %s", last.ErrorMsg)

	if len(progress) == 0 {
		if elapsed < 3*time.Second {
			t.Skipf("整体耗时 %s < 3s 节流窗口，本机根本没机会产出 PROGRESS 行（环境所致，非缺陷）", elapsed)
		}
		Fatalf(t, "下载耗时 %s（>3s 节流窗口）却无任何 PROGRESS 行：UI/SDK 会因长时间无输出误判卡死", elapsed)
	}
	p := progress[0]
	Eq(t, "file", p.File, "big.bin")
	Eq(t, "total_bytes", p.TotalBytes, int64(31457280))
	True(t, p.DownloadedBytes > 0, "downloaded_bytes 应 > 0")

	// speed_bps 的语义要说清（download_update.go 的 speedBpsLocked / sampleRate）：
	//   speedBpsLocked 只统计 3s 滑动窗口内「文件**完成**时」的采样；
	//   sampleRate 对首样本（prevAt 为零值）直接回退 windowBps（此时窗口为空 → 0）。
	// 所以**首条** PROGRESS 的 speed_bps 必然是 0，从第二条起才有出数。
	// 用「任一 PROGRESS 行 speed_bps > 0」来验证速度字段有出数，而不是死抠首条。
	var maxSpeed int64
	for _, x := range progress {
		if x.SpeedBps > maxSpeed {
			maxSpeed = x.SpeedBps
		}
	}
	if maxSpeed == 0 {
		// 只有拿到 ≥2 条 PROGRESS 才可能算出速度（第一条没有前一样本）。
		// 单条 → 环境在 3~6s 之间传完，属环境过快；≥2 条仍全 0 → 速度计算真失效。
		if len(progress) == 1 {
			t.Skipf("只有 1 条 PROGRESS（耗时 %s，落在单个节流窗口内），无第二样本可比，"+
				"speed_bps 恒为 0（环境所致，非缺陷）", elapsed)
		}
		Fatalf(t, "有 %d 条 PROGRESS 但 speed_bps 全为 0（耗时 %s）：第二条起应能算出速度，计算可能失效",
			len(progress), elapsed)
	}
	True(t, maxSpeed > 0, "speed_bps 应有 >0 的样本")

	Eq(t, "快照文件大小",
		SizeOf(filepath.Join(c.VerDir("2.0.0"), "big.bin")), int64(31457280))
	Eq(t, "完成行 data 应为 null", string(last.Data) == "null" || len(last.Data) == 0, true)

	// 节流校验：3s 节流下不允许刷屏
	True(t, len(progress) < 100, "PROGRESS 行过多(%d)，3s 节流可能失效", len(progress))
}

// ────────────────────────────────────────────────────────────────
// 用例 10 — 协议兼容：完成行的 data 必须为 null
// ────────────────────────────────────────────────────────────────
func TestCase10(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-proto"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-200")
	True(t, e.Publish(t, proj, "2.0.0", nil, "", "").IsSuccess, "发布失败")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Project: proj, MainFile: "app-100",
	})

	last := e.RunLast(t, c, "download_update")
	Eq(t, "isSuccess", last.IsSuccess, true)
	Eq(t, "data 应为 null", string(last.Data) == "null" || len(last.Data) == 0, true)
}

// ────────────────────────────────────────────────────────────────
// 用例 11 — 差量删除：服务端删除的文件在客户端 apply 后被删除
//
// 前置要点：项目创建即 V1.0.0，所以**不能**指派 1.0.0 的基线版本，
// 否则 download 会报 `already at latest version`（无更高版本可下）。
// 这里让客户端停在 1.0.0、服务端先发 1.0.1 建立清单缓存，再删文件发 1.0.2。
// ────────────────────────────────────────────────────────────────
func TestCase11(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-del"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-v1")
	e.PutFile(t, proj, "old.txt", "will-be-deleted")
	e.PutFile(t, proj, "keep.txt", "keep-me")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-v1",
	})
	WriteText(t, filepath.Join(c.Main(), "old.txt"), "will-be-deleted")
	WriteText(t, filepath.Join(c.Main(), "keep.txt"), "keep-me")

	// 服务端先发 1.0.1（内容不变，仅推进版本），客户端跑一轮建立清单缓存
	True(t, e.Publish(t, proj, "1.0.1", nil, "", "").IsSuccess, "发布 1.0.1")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.0.1")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.0.1")
	True(t, Exists(filepath.Join(c.Up(), "release_manifest.json")), "前置：清单缓存应存在")
	Contains(t, ReadText(filepath.Join(c.Up(), "release_manifest.json")), "old.txt",
		"前置：首轮清单缓存应含 old.txt")

	// 服务端删文件后发版
	e.RemoveFile(t, proj, "old.txt")
	True(t, e.Publish(t, proj, "1.0.2", nil, "", "").IsSuccess, "发布 1.0.2")

	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.0.2")
	log := c.Log(t, "download.log", 200)
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.0.2")

	Contains(t, log, "delete-sync removed 1 file(s) deleted by server", "download.log")
	Eq(t, "old.txt 应被删除", Exists(filepath.Join(c.Main(), "old.txt")), false)
	Eq(t, "keep.txt 应保留",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "keep.txt"))), "keep-me")
	Eq(t, "app.exe 应保留",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-v1")
}

// ────────────────────────────────────────────────────────────────
// 用例 12 — 运行时数据保护：不在清单内的文件不被删除
//
// 同一约束：客户端 1.0.0 时服务端也必须是 1.0.0 以上才有东西可下。
// 先发 1.0.1 建缓存，再发 1.1.0 验证差量删除不碰运行时数据。
// ────────────────────────────────────────────────────────────────
func TestCase12(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-rt"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})

	// 服务端推进到 1.0.1，客户端跑一轮 → 建立清单缓存
	True(t, e.Publish(t, proj, "1.0.1", nil, "", "").IsSuccess, "发布 1.0.1")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.0.1")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.0.1")

	// 主目录里放"应用自己写的"运行时数据（从不在服务端清单里）
	WriteText(t, filepath.Join(c.Main(), "runtime", "user.cfg"), "user-data-must-survive")

	// 服务端发新版
	e.PutFile(t, proj, "app.exe", "app-110")
	e.PutFile(t, proj, "added.txt", "in-110")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.1.0")
	True(t, Exists(filepath.Join(c.Main(), "runtime", "user.cfg")),
		"download 不应改动主目录的运行时数据")

	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply 1.1.0")

	Eq(t, "运行时数据必须存活",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "runtime", "user.cfg"))),
		"user-data-must-survive")
	Eq(t, "新版文件已落地", Exists(filepath.Join(c.Main(), "added.txt")), true)
	Eq(t, "app.exe 内容",
		strings.TrimSpace(ReadText(filepath.Join(c.Main(), "app.exe"))), "app-110")
}

// ────────────────────────────────────────────────────────────────
// 用例 13 — 首周期保守：无清单缓存时不执行删除
// ────────────────────────────────────────────────────────────────
func TestCase13(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-first"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "legacy.txt", "old-legacy")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	// 主目录里有 legacy.txt（旧版本带来的）
	WriteText(t, filepath.Join(c.Main(), "legacy.txt"), "old-legacy")
	True(t, !Exists(filepath.Join(c.Up(), "release_manifest.json")), "前置：不应有清单缓存")

	e.RemoveFile(t, proj, "legacy.txt")
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download 1.1.0")
	log := c.Log(t, "download.log", 120)

	Contains(t, log, "no cached manifest (first cycle)", "download.log")
	NotContains(t, log, "delete-sync removed", "download.log")
	True(t, Exists(filepath.Join(c.Up(), "release_manifest.json")),
		"首周期也应固化清单，供下轮做差量基线")
}

// ────────────────────────────────────────────────────────────────
// 用例 14 — 清单缓存固化：内容正确性与原子写
// ────────────────────────────────────────────────────────────────
func TestCase14(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-cache"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "lib.dll", "lib-100")
	e.PutFile(t, proj, "sub/res.txt", "res-100")
	// 客户端 1.0.0 → 服务端必须发更高的版本才有得下（项目创建即 V1.0.0）
	True(t, e.Publish(t, proj, "1.0.1", nil, "", "").IsSuccess, "发布 1.0.1")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, NoMain: true,
	})

	dl := e.RunLast(t, c, "download_update")
	True(t, dl.IsSuccess, "download 失败: %s", dl.ErrorMsg)

	cachePath := filepath.Join(c.Up(), "release_manifest.json")
	True(t, Exists(cachePath), "缓存文件应生成")

	// 缓存格式是**纯字符串数组**（不是 {"files":[...]} 对象）：
	//   ["app.exe","lib.dll","sub/res.txt"]
	// 依据：客户端 writeManifestCache 直接 json.Marshal(paths)。
	var cache []string
	True(t, json.Unmarshal([]byte(ReadText(cachePath)), &cache) == nil,
		"缓存应是合法 JSON 字符串数组，实际: %s", truncate(ReadText(cachePath), 300))

	srv := e.GetManifest(t, proj)
	Eq(t, "缓存条数与服务端一致", len(cache), len(srv.Files))

	// 原子写：不应有 .tmp 残留
	Eq(t, "release_manifest.json.tmp 不应残留",
		Exists(filepath.Join(c.Up(), "release_manifest.json.tmp")), false)

	// 归一化：缓存内路径应为 / 分隔
	for _, s := range cache {
		Eq(t, "缓存路径应用 / 分隔: "+s, strings.Contains(s, "\\"), false)
	}

	Contains(t, c.Log(t, "download.log", 120), "materialized unchanged files", "download.log")
}

// ────────────────────────────────────────────────────────────────
// 用例 15 — 回滚后清单缓存被清除
// ────────────────────────────────────────────────────────────────
func TestCase15(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-cache-roll"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-110")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.1.0", Status: StatusApplied, Previous: "1.1.0",
		Project: proj, MainFile: "app-110",
	})
	// 造 1.0.0 备份（含 shared.json：真实发布包里 .updator/ 随版本下发）
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), "app.exe"), "app-100")
	WriteText(t, filepath.Join(c.VerDir("1.0.0"), ".updator", "shared.json"),
		ReadText(filepath.Join(c.Main(), ".updator", "shared.json")))

	WriteText(t, filepath.Join(c.Up(), "release_manifest.json"), `["app.exe"]`)
	True(t, Exists(filepath.Join(c.Up(), "release_manifest.json")), "前置：缓存应存在")

	out := e.RunLast(t, c, "rollback", "--version", "1.0.0")
	True(t, out.IsSuccess, "rollback 失败: %s", out.ErrorMsg)

	Eq(t, "回滚后清单缓存应被删除",
		Exists(filepath.Join(c.Up(), "release_manifest.json")), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 16 — 缓存损坏：禁用删除同步但不阻断下载
// ────────────────────────────────────────────────────────────────
func TestCase16(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-badcache"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	e.PutFile(t, proj, "gone.txt", "to-be-removed")
	e.PutFile(t, proj, "keep.txt", "keep")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	WriteText(t, filepath.Join(c.Main(), "gone.txt"), "to-be-removed")
	WriteText(t, filepath.Join(c.Main(), "keep.txt"), "keep")

	// 服务端先推进到 1.1.0：正常走一轮，产生缓存（项目创建即 V1.0.0）
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")
	True(t, e.RunLast(t, c, "download_update").IsSuccess, "download")
	True(t, e.RunLast(t, c, "apply_update").IsSuccess, "apply")

	// 写坏缓存（模拟断电写坏；真实格式是字符串数组，半截即 ["app.exe",）
	WriteText(t, filepath.Join(c.Up(), "release_manifest.json"), `["app.exe",`)

	// 服务端删文件并发版
	e.RemoveFile(t, proj, "gone.txt")
	e.PutFile(t, proj, "app.exe", "app-120")
	True(t, e.Publish(t, proj, "1.2.0", nil, "", "").IsSuccess, "发布 1.2.0")

	dl := e.RunLast(t, c, "download_update")
	log := c.Log(t, "download.log", 120)

	True(t, dl.IsSuccess, "缓存损坏不应阻断下载: %s", dl.ErrorMsg)
	Contains(t, log, "read cached manifest failed (delete-sync disabled)", "download.log")
	NotContains(t, log, "delete-sync removed", "download.log")

	var m []string
	True(t, json.Unmarshal([]byte(ReadText(filepath.Join(c.Up(), "release_manifest.json"))), &m) == nil,
		"坏缓存应被好数据覆盖")
}

// ────────────────────────────────────────────────────────────────
// 用例 17 — 续传进度基准：.part 存在时从已有字节继续
// ────────────────────────────────────────────────────────────────
func TestCase17(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-resume"
	e.EnsureProject(t, proj, false, nil, nil)
	bigSrc := filepath.Join(e.DataDir(proj), "big.bin")
	BigFile(t, bigSrc, 20)
	True(t, e.Publish(t, proj, "1.1.0", nil, "", "").IsSuccess, "发布 1.1.0")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Project: proj, NoMain: true,
	})

	// 造 .part：拷前 5MB
	snap := c.VerDir("1.1.0")
	part := filepath.Join(snap, "big.bin.part")
	WriteText(t, filepath.Join(snap, ".keep"), "") // 保证目录存在
	CopyHead(t, bigSrc, part, 5<<20)

	dl := e.RunLast(t, c, "download_update")
	True(t, dl.IsSuccess, "续传下载失败: %s", dl.ErrorMsg)

	final := filepath.Join(snap, "big.bin")
	Eq(t, "最终文件大小", SizeOf(final), int64(20<<20))
	Eq(t, ".part 应被清理", Exists(part), false)
	Eq(t, "内容哈希应与源一致", MD5Of(final), MD5Of(bigSrc))
}