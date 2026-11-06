package tc

// 用例 128–129：客户端真实分片上传路径（publish-cli → upload_chunk）
//
// 背景（2026-11-06 真机缺陷）：GUI 发布 262MB 文件报
//     upload chunk 0/263: HTTP 404: 404 page not found
// 根因是**运行中的服务端是陈旧构建**（二进制内没有 upload_chunk 路由），
// 但这条缺陷之所以没被 CI 拦住，是测试侧有漏网：
//
//   - 服务端分片逻辑已被用例 114 覆盖（手工 postForm 直打 upload_chunk /
//     upload_chunk_complete），护得住"服务端算法对不对"；
//   - **客户端真实分片路径无人覆盖**：publish-cli 的 UploadFile 只对
//     > chunkSize（1MiB）的文件走 uploadFileChunked；而用例 66–77（publish-cli）
//     全部用文本小文件，永远走 uploadFileDirect；用例 63/58 的 BigFile 走的是
//     测试框架的 Env.Upload（直打 upload_file），同样不经过客户端分片分支。
//
// 于是"客户端发分片 → 服务端有没有这个路由"这个组合从未被端到端验证过：
// 部署了旧 server.exe 也能全绿。本文件补上这道网。
//
// 值骗性：> chunkSize 的文件若能推成功，必然经过
//   upload_chunk（每个分片）→ upload_chunk_complete（合并）；
// 客户端若走了直传分支就根本不需要这两条路由。所以本用例通过，
// 等价于"客户端分片分支确实执行了 且 服务端两条路由都在"。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// chunckSizeBytes 是 publish-cli 的分片阈值，与
// publish/publish-cli/internal/api/client.go 的 `chunkSize = 1024 * 1024` 逐字对应。
// 客户端逻辑是 `fi.Size() <= chunkSize` 走直传，故 chunkSize+1 起才走分片。
const chunckSizeBytes = 1024 * 1024

// WritePattern 生成 n 字节的**非均匀**内容文件（b[i] = (i*31+7) % 251）。
//
// 不用全零（BigFile 的 Truncate）的原因：全零文件在分片拼接**顺序错位 / 丢片 /
// 重片**时前端字节仍可能是 0，md5 未必变——那就盖住了要抓的缺陷。
// 乘法序列对任何重排/多写/少写都会改变 md5。
func WritePattern(t *testing.T, p string, n int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	buf := make([]byte, n)
	for i := int64(0); i < n; i++ {
		buf[i] = byte((i*31 + 7) % 251)
	}
	if err := os.WriteFile(p, buf, 0o644); err != nil {
		t.Fatalf("写文件失败(%s): %v", p, err)
	}
}

// NoResidue 上传完成后不得残留 .chunks / .merging 中间态。
func (e *Env) NoResidue(t *testing.T, proj, relPath string) {
	t.Helper()
	base := filepath.Join(e.DataDir(proj), filepath.FromSlash(relPath))
	Eq(t, "不得残留 .chunks 目录 ("+relPath+")", Exists(base+".chunks"), false)
	Eq(t, "不得残留 .merging 文件 ("+relPath+")", Exists(base+".merging"), false)
}

// HasRel 断言文件列表里有 relPath（`/` 分隔口径）。
func HasRel(t *testing.T, fs []FileInfo, relPath string) {
	t.Helper()
	for _, f := range fs {
		if filepath.ToSlash(f.FileRelativePath) == relPath {
			return
		}
	}
	t.Fatalf("文件列表应含 %s，实际: %v", relPath, FileRels(t, fs))
}

// ────────────────────────────────────────────────────────────────
// 用例 128 — publish-cli push 大文件必须走通客户端分片通道
//
// 现场：3MiB+ 的非均匀文件（> chunkSize，必然多分片），走真实 aly-publish
// add --all → push。断言：
//  ① push 成功（失败即 404 —— 服务端缺 upload_chunk 路由 / 客户端分片逻辑坏）；
//  ② 落地字节数与 md5 与源文件**逐字节一致**（分片按 chunkIndex 顺序拼接）；
//  ③ 无 .chunks / .merging 残留（合并后清理干净）；
//  ④ get_all_files 能看到它（上传后缓存失效，服务端认可为正式文件）。
// ────────────────────────────────────────────────────────────────
func TestCase128(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-chunk-pub"
	e.EnsureProject(t, proj, false, nil, nil)

	ws := e.NewPublishWS(t, proj, "pub-chunk")

	// 3 个整分片 + 一个尾片：既跨分片阈值，又覆盖"尾片小于 chunkSize"的分支
	size := int64(chunckSizeBytes*3 + 12345)
	rel := "big/bigfile.bin"
	src := filepath.Join(ws, filepath.FromSlash(rel))
	WritePattern(t, src, size)
	Eq(t, "源文件大小应为 3 整片加尾片", SizeOf(src), size)

	True(t, e.PublishCLI(t, ws, "add", "--all").IsSuccess, "add --all 失败")
	r := e.PublishCLI(t, ws, "push", "--version", "1.0.1", "--message", "大文件分片上传")
	True(t, r.IsSuccess,
		"push 大文件失败（分片通道 404/坏）: %s — 服务端若缺 upload_chunk "+
			"路由即报 HTTP 404: 404 page not found", r.ErrorMsg)

	// ② 内容逐字节一致
	dst := filepath.Join(e.DataDir(proj), filepath.FromSlash(rel))
	Eq(t, "落地字节数", SizeOf(dst), size)
	Eq(t, "落地 md5（分片必须按 chunkIndex 顺序拼接）", MD5Of(dst), MD5Of(src))

	// ③ 中间态清理
	e.NoResidue(t, proj, rel)

	// ④ 服务端认可为正式文件，且列表不得混入中间态
	fs := e.GetAllFiles(t, proj)
	HasRel(t, fs, rel)
	for _, x := range FileRels(t, fs) {
		NotContains(t, x, ".chunks", "文件列表不得含中间态")
		NotContains(t, x, ".merging", "文件列表不得含中间态")
	}
}

// ────────────────────────────────────────────────────────────────
// 用例 129 — 分片阈值边界：chunkSize 走直传，chunkSize+1 走分片
//
// 客户端分支配对是 `fi.Size() <= chunkSize` 走直传。这条边界从无覆盖，
// 边界写反（如 <）会让 1MiB 整文件静默改走分片，或 >1MiB 走直传被大文件
// 上限掩盖。两条路径都必须把内容逐字节落地。
// ────────────────────────────────────────────────────────────────
func TestCase129(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-chunk-edge"
	e.EnsureProject(t, proj, false, nil, nil)

	ws := e.NewPublishWS(t, proj, "pub-edge")

	// ① 恰好 chunkSize → 直传分支
	exact := "exact.bin"
	srcExact := filepath.Join(ws, exact)
	WritePattern(t, srcExact, chunckSizeBytes)

	// ② chunkSize+1 → 分片分支（两片，第二片 1 字节）
	over := "over.bin"
	srcOver := filepath.Join(ws, over)
	WritePattern(t, srcOver, chunckSizeBytes+1)

	True(t, e.PublishCLI(t, ws, "add", "--all").IsSuccess, "add --all 失败")
	r := e.PublishCLI(t, ws, "push", "--version", "1.0.1", "--message", "分片阈值边界")
	True(t, r.IsSuccess, "push 失败: %s", r.ErrorMsg)

	for _, c := range []struct {
		rel string
		src string
		n   int64
	}{
		{exact, srcExact, int64(chunckSizeBytes)},
		{over, srcOver, int64(chunckSizeBytes + 1)},
	} {
		dst := filepath.Join(e.DataDir(proj), c.rel)
		Eq(t, "落地字节数 "+c.rel, SizeOf(dst), c.n)
		Eq(t, "落地 md5 "+c.rel, MD5Of(dst), MD5Of(c.src))
		e.NoResidue(t, proj, c.rel)
	}

	// 反证：两文件都在服务端文件列表里
	fs := e.GetAllFiles(t, proj)
	HasRel(t, fs, exact)
	HasRel(t, fs, over)
	got := strings.Join(FileRels(t, fs), ",")
	NotContains(t, got, ".chunks", "文件列表不得含中间态")
}
