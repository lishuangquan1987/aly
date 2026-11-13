package tc

// 用例 139–148：2026-11-12《全仓库审查报告-20261112》问题修复的 E2E 回归锚。
//
// 覆盖原则：每条用例独立复现一个"审查发现 → 修复"的现场，断言修复后的行为；
// 边界条件优先（空值、负数、超大值、穿越路径、幂等、软删除、大小写）。
//
//	139  B1  上传接口 projectName 路径穿越必须被拒（. / .. / ../.. / ..\..\ / ../sub）
//	140  B3  分片合并 totalChunks<=0 / 超上界 / 全零字节 不得清空目标文件；缺片如实上报
//	141  M2  软删除项目不再对外服务（文件列表/清单/项目查询全部拒绝，磁盘目录保留）
//	142  B6  delete_file 删除同步：删除生效、幂等、拒绝穿越
//	143  B5  push 的 force_update 三态：不传不动 / --set 打开 / --clear 关闭
//	144  M13/14 CLI 参数校验：log --limit 负数不 panic、config init 缺项目名报错
//	145  B6  删除同步全链路：本地删 → add --all → push → 服务端文件消失且清单排除
//	146  B9  后置脚本失败不写 marker（崩溃恢复会重跑，不再"一次失败永久标记完成"）
//	147  M19 ignore 规则边界：嵌套目录 / 大小写 / 文件名 glob
//	148  M21 add 路径归一：反斜杠与 ./ 写法不得造成"同一文件既 staged 又 unstaged"

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────
// 小工具（仅本文件使用）
// ────────────────────────────────────────────────────────────────

// tcPostForm 发 application/x-www-form-urlencoded 请求（complete/delete_file 等）
func tcPostForm(t *testing.T, e *Env, path string, fields map[string]string) Resp {
	t.Helper()
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	req, err := http.NewRequest("POST", e.BaseURL()+path, strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("构造请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := e.base.Do(req)
	if err != nil {
		t.Fatalf("POST %s 失败: %v", path, err)
	}
	defer resp.Body.Close()
	return decodeResp(t, resp.Body, path)
}

// tcUploadChunk 上传单分片
func tcUploadChunk(t *testing.T, e *Env, project, rel string, idx, total int, content []byte) Resp {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("projectName", project)
	_ = mw.WriteField("relativeFileName", rel)
	_ = mw.WriteField("chunkIndex", itoa(idx))
	_ = mw.WriteField("totalChunks", itoa(total))
	fw, err := mw.CreateFormFile("file", "chunk")
	if err != nil {
		t.Fatalf("构造分片失败: %v", err)
	}
	_, _ = fw.Write(content)
	mw.Close()

	req, _ := http.NewRequest("POST", e.BaseURL()+"/api/file/upload_chunk", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := e.base.Do(req)
	if err != nil {
		t.Fatalf("上传分片失败: %v", err)
	}
	defer resp.Body.Close()
	return decodeResp(t, resp.Body, "/api/file/upload_chunk")
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

// ────────────────────────────────────────────────────────────────
// 139 B1：项目名路径穿越
// ────────────────────────────────────────────────────────────────

func TestCase139(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-b1-traversal"
	e.EnsureProject(t, proj, false, nil, nil)

	local := filepath.Join(e.Root, "src", "evil.bin")
	WriteText(t, local, "pwn")

	// 旧实现放行 . 与 ..（正则 ^[^<>:"/\\|?*]{1,64}$），Clean 后逃出 data 目录
	for _, name := range []string{".", "..", "../..", `..\..\`, "../sub", "a/../.."} {
		r := e.Upload(t, name, "evil.bin", local)
		True(t, !r.IsSuccess, "项目名 %q 必须被拒绝，实际成功（路径穿越）", name)
	}

	// 断言没有任何穿越落盘：exe 根、临时根、data 同级都不该出现 evil.bin
	True(t, !Exists(filepath.Join(e.Bin, "evil.bin")), "不得写入 exe 目录")
	True(t, !Exists(filepath.Join(e.Root, "evil.bin")), "不得写入临时根目录")
	True(t, !Exists(filepath.Join(e.Bin, "data", "evil.bin")), "不得写入 data 根")

	// 合法项目名仍可正常上传（不得误伤）
	r := e.Upload(t, proj, "ok.bin", local)
	True(t, r.IsSuccess, "合法项目名上传应成功: %s", r.ErrorMsg)
	for _, f := range e.GetAllFiles(t, proj) {
		if f.FileRelativePath == "ok.bin" {
			return
		}
	}
	t.Fatal("合法上传的文件未出现在服务端文件列表")
}

// ────────────────────────────────────────────────────────────────
// 140 B3 + N1：分片合并的 totalChunks 边界
// ────────────────────────────────────────────────────────────────

func TestCase140(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-b3-chunk"
	const rel = "target.bin"
	e.EnsureProject(t, proj, false, nil, nil)

	// 预置目标文件：旧实现 totalChunks=0 会把它原子覆盖成 0 字节
	local := filepath.Join(e.Root, "orig.bin")
	WriteText(t, local, "IMPORTANT-ORIGINAL")
	r := e.Upload(t, proj, rel, local)
	True(t, r.IsSuccess, "预置目标文件失败: %s", r.ErrorMsg)
	target := filepath.Join(e.DataDir(proj), rel)

	// B3：0 / 负数必须拒绝，且目标文件字节不变
	for _, total := range []int{0, -1, -100} {
		r := tcPostForm(t, e, "/api/file/upload_chunk_complete", map[string]string{
			"projectName": proj, "relativeFileName": rel, "totalChunks": itoa(total),
		})
		True(t, !r.IsSuccess, "totalChunks=%d 应被拒绝（否则清空目标文件）", total)
		Eq(t, "目标文件不得被清空", ReadText(target), "IMPORTANT-ORIGINAL")
	}

	// N1：上界必须拒绝（否则单请求可打成 OOM/CPU 自旋）
	r = tcPostForm(t, e, "/api/file/upload_chunk_complete", map[string]string{
		"projectName": proj, "relativeFileName": rel, "totalChunks": "65537",
	})
	True(t, !r.IsSuccess, "totalChunks 超上界应被拒绝")

	// 全零字节分片（分片都在但内容为空）也必须拒绝
	r = tcUploadChunk(t, e, proj, rel, 0, 2, []byte(""))
	True(t, r.IsSuccess, "上传空分片 0 失败: %s", r.ErrorMsg)
	r = tcUploadChunk(t, e, proj, rel, 1, 2, []byte(""))
	True(t, r.IsSuccess, "上传空分片 1 失败: %s", r.ErrorMsg)
	r = tcPostForm(t, e, "/api/file/upload_chunk_complete", map[string]string{
		"projectName": proj, "relativeFileName": rel, "totalChunks": "2",
	})
	True(t, !r.IsSuccess, "全零字节合并应被拒绝")
	Eq(t, "目标文件不得被空分片覆盖", ReadText(target), "IMPORTANT-ORIGINAL")

	// 缺片：如实上报 chunksComplete=false + missingChunks
	os.RemoveAll(target + ".chunks")
	r = tcUploadChunk(t, e, proj, rel, 0, 3, []byte("AAA"))
	True(t, r.IsSuccess, "上传分片失败: %s", r.ErrorMsg)
	r = tcPostForm(t, e, "/api/file/upload_chunk_complete", map[string]string{
		"projectName": proj, "relativeFileName": rel, "totalChunks": "3",
	})
	True(t, r.IsSuccess, "缺片应返回成功信封（业务状态）")
	var cc struct {
		ChunksComplete bool  `json:"chunksComplete"`
		MissingChunks  []int `json:"missingChunks"`
	}
	if err := json.Unmarshal(r.Data, &cc); err != nil {
		t.Fatalf("解析 complete 响应失败: %v (data=%s)", err, string(r.Data))
	}
	True(t, !cc.ChunksComplete, "缺片时 chunksComplete 必须为 false")
	Eq(t, "缺失分片数", len(cc.MissingChunks), 2)
}

// ────────────────────────────────────────────────────────────────
// 141 M2：软删除项目不再对外服务
// ────────────────────────────────────────────────────────────────

func TestCase141(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-m2-softdel"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "a.txt", "a")
	e.Publish(t, proj, "1.1.0", []string{"m2"}, "", "")

	// 软删除
	r := e.Post(t, "/api/project/delete_project/"+proj, struct{}{})
	True(t, r.IsSuccess, "软删除失败: %s", r.ErrorMsg)

	// 三个接口都必须拒绝（旧实现：已删项目仍被 get_all_files / manifest / 查询服务）
	if r := e.Get(t, "/api/file/get_all_files/"+proj); r.IsSuccess {
		_ = t
		t.Fatal("软删除项目不应再被 get_all_files 服务")
	}
	if r := e.Get(t, "/api/file/get_release_manifest/"+proj); r.IsSuccess {
		t.Fatal("软删除项目不应再被 get_release_manifest 服务")
	}
	if r := e.Get(t, "/api/project/get_project_by_name/"+proj); r.IsSuccess {
		t.Fatal("软删除项目不应再被 get_project_by_name 返回")
	}

	// 语义=仅从 API 隐藏，磁盘目录保留（便于恢复/审计）
	True(t, Exists(e.DataDir(proj)), "软删除不应直接删盘")
}

// ────────────────────────────────────────────────────────────────
// 142 B6：delete_file
// ────────────────────────────────────────────────────────────────

func TestCase142(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-b6-delete-api"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "keep.txt", "keep")
	e.PutFile(t, proj, "sub/drop.txt", "drop")

	r := tcPostForm(t, e, "/api/file/delete_file", map[string]string{
		"projectName": proj, "relativeFileName": "sub/drop.txt",
	})
	True(t, r.IsSuccess, "删除失败: %s", r.ErrorMsg)
	True(t, !Exists(filepath.Join(e.DataDir(proj), "sub", "drop.txt")), "删除后文件仍存在")

	// 幂等：再删一次仍成功
	r = tcPostForm(t, e, "/api/file/delete_file", map[string]string{
		"projectName": proj, "relativeFileName": "sub/drop.txt",
	})
	True(t, r.IsSuccess, "重复删除应幂等成功: %s", r.ErrorMsg)

	// 同项目其它文件不受影响
	Eq(t, "不应误删同项目其它文件", ReadText(filepath.Join(e.DataDir(proj), "keep.txt")), "keep")

	// 穿越必须拒绝
	for _, rel := range []string{"../../aly.db", "../other/x", `..\..\win.ini`} {
		r = tcPostForm(t, e, "/api/file/delete_file", map[string]string{
			"projectName": proj, "relativeFileName": rel,
		})
		True(t, !r.IsSuccess, "delete_file 应拒绝穿越路径 %q", rel)
	}
}

// ────────────────────────────────────────────────────────────────
// 143 B5：force_update 三态
// ────────────────────────────────────────────────────────────────

func TestCase143(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-b5-force"
	// 初始为强制更新
	e.EnsureProject(t, proj, true, nil, nil)

	ws := e.NewPublishWS(t, proj, "b5ws")
	WriteText(t, filepath.Join(ws, "app.txt"), "v1")
	if r := e.PublishCLI(t, ws, "add", "--all"); !r.IsSuccess {
		t.Fatalf("add 失败: %s", r.ErrorMsg)
	}
	// ① 不传开关：不得改动服务端现值（旧实现会静默重置为 false）
	r := e.PublishCLI(t, ws, "push", "--version", "1.1.0", "--message", "no-flag")
	True(t, r.IsSuccess, "push 1.1.0 失败: %s", r.ErrorMsg)
	Eq(t, "不传开关不得改动 force_update", e.GetProject(t, proj).ForceUpdate, true)

	// ② --clear-force-update：明确关闭
	WriteText(t, filepath.Join(ws, "app.txt"), "v2")
	if r := e.PublishCLI(t, ws, "add", "--all"); !r.IsSuccess {
		t.Fatalf("add 失败: %s", r.ErrorMsg)
	}
	r = e.PublishCLI(t, ws, "push", "--version", "1.2.0", "--message", "clear", "--clear-force-update")
	True(t, r.IsSuccess, "push 1.2.0 失败: %s", r.ErrorMsg)
	Eq(t, "--clear-force-update 应关闭", e.GetProject(t, proj).ForceUpdate, false)

	// ③ --set-force-update：明确打开
	WriteText(t, filepath.Join(ws, "app.txt"), "v3")
	if r := e.PublishCLI(t, ws, "add", "--all"); !r.IsSuccess {
		t.Fatalf("add 失败: %s", r.ErrorMsg)
	}
	r = e.PublishCLI(t, ws, "push", "--version", "1.3.0", "--message", "set", "--set-force-update")
	True(t, r.IsSuccess, "push 1.3.0 失败: %s", r.ErrorMsg)
	Eq(t, "--set-force-update 应打开", e.GetProject(t, proj).ForceUpdate, true)

	// ④ 互斥开关必须报错
	r = e.PublishCLI(t, ws, "push", "--version", "1.4.0", "--message", "both",
		"--set-force-update", "--clear-force-update")
	True(t, !r.IsSuccess, "同时传 --set 与 --clear 应报错")
}

// ────────────────────────────────────────────────────────────────
// 144 M13/M14：CLI 参数校验
// ────────────────────────────────────────────────────────────────

func TestCase144(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-cli-args"
	e.EnsureProject(t, proj, false, nil, nil)
	ws := e.NewPublishWS(t, proj, "args-ws")

	// M13：--limit 负数直接 panic（make([]T,-1)）→ 修复后必须是可读错误
	for _, limit := range []string{"-1", "0"} {
		r := e.PublishCLI(t, ws, "log", "--limit", limit)
		True(t, !r.IsSuccess, "--limit %s 应报错", limit)
		Contains(t, r.ErrorMsg, "limit", "--limit 错误信息应指明参数")
	}
	// 合法值不得报错
	r := e.PublishCLI(t, ws, "log", "--limit", "5")
	True(t, r.IsSuccess, "--limit 5 应成功: %s", r.ErrorMsg)

	// M14：空目录里 config init 不带 --project 必须报错（旧实现静默写全空配置）
	empty := filepath.Join(e.Root, "empty-ws")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatalf("建空目录失败: %v", err)
	}
	r = e.PublishCLI(t, empty, "config", "init")
	True(t, !r.IsSuccess, "缺 --project 时 config init 应报错")
	Contains(t, r.ErrorMsg, "project", "错误信息应指明 --project")
	True(t, !Exists(filepath.Join(empty, ".updator", "shared.json")),
		"校验失败不得写出配置文件")
}

// ────────────────────────────────────────────────────────────────
// 145 B6：删除同步全链路
// ────────────────────────────────────────────────────────────────

func TestCase145(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-b6-fullchain"
	e.EnsureProject(t, proj, false, nil, nil)

	ws := e.NewPublishWS(t, proj, "b6ws")
	WriteText(t, filepath.Join(ws, "a.txt"), "a")
	WriteText(t, filepath.Join(ws, "b.txt"), "b")
	if r := e.PublishCLI(t, ws, "add", "--all"); !r.IsSuccess {
		t.Fatalf("add 失败: %s", r.ErrorMsg)
	}
	if r := e.PublishCLI(t, ws, "push", "--version", "1.1.0", "--message", "init"); !r.IsSuccess {
		t.Fatalf("push 1.1.0 失败: %s", r.ErrorMsg)
	}
	// 首发前两个文件都在服务端
	if len(e.GetAllFiles(t, proj)) < 2 {
		t.Fatal("首发后服务端应至少有 2 个文件")
	}

	// 本地删除 a.txt
	if err := os.Remove(filepath.Join(ws, "a.txt")); err != nil {
		t.Fatalf("删除本地文件失败: %v", err)
	}

	// add --all 必须把 deleted 纳入（旧实现跳过 deleted，删除永远同步不过去）
	r := e.PublishCLI(t, ws, "add", "--all")
	True(t, r.IsSuccess, "add --all 失败: %s", r.ErrorMsg)
	var sd StatusData
	st := e.PublishCLI(t, ws, "status")
	True(t, st.IsSuccess, "status 失败: %s", st.ErrorMsg)
	if err := json.Unmarshal(st.Data, &sd); err != nil {
		t.Fatalf("解析 status 失败: %v", err)
	}
	foundDeleted := false
	for _, f := range sd.Staged {
		if f.RelativePath == "a.txt" && f.Status == "deleted" {
			foundDeleted = true
		}
	}
	True(t, foundDeleted, "a.txt 应以 deleted 状态进入暂存区: %+v", sd.Staged)

	// push 会调 delete_file 而不是上传
	r = e.PublishCLI(t, ws, "push", "--version", "1.2.0", "--message", "drop-a")
	True(t, r.IsSuccess, "push 1.2.0 失败: %s", r.ErrorMsg)

	True(t, !Exists(filepath.Join(e.DataDir(proj), "a.txt")), "服务端 a.txt 应被删除")
	True(t, Exists(filepath.Join(e.DataDir(proj), "b.txt")), "服务端 b.txt 必须保留")
	m := e.GetManifest(t, proj)
	for _, f := range m.Files {
		True(t, f != "a.txt", "发布清单不得再包含已删除文件 a.txt")
	}
}

// ────────────────────────────────────────────────────────────────
// 146 B9：后置脚本失败不写 marker
// ────────────────────────────────────────────────────────────────

func TestCase146(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-b9-script"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "app.exe", "app-100")
	// 失败脚本：exit /b 1（Windows）
	e.PutFile(t, proj, "fail.bat", "@echo off\r\nexit /b 1\r\n")

	c := e.NewClient(t, NewClientOpts{
		Version: "1.0.0", Status: StatusApplied, Previous: "1.0.0",
		Project: proj, MainFile: "app-100",
	})
	e.PutFile(t, proj, "app.exe", "app-110")
	e.Publish(t, proj, "1.1.0", []string{"b9"}, "", "fail.bat")

	// 下载 + 应用：脚本失败不应导致 apply 失败，但绝不能写 marker
	o := e.RunLast(t, c, "download_update")
	True(t, o.IsSuccess, "download_update 失败: %s", o.ErrorMsg)
	o = e.RunLast(t, c, "apply_update")
	// apply 本身可能仍成功（脚本失败只记录）；这里不强制成功，但要检查 marker
	marker := filepath.Join(c.Main(), ".updator", "after_apply_1.1.0.done")
	True(t, !Exists(marker), "脚本失败时不得写 marker（否则崩溃恢复永不重跑）: apply=%+v", o)
}

// ────────────────────────────────────────────────────────────────
// 147 M19：ignore 规则边界
// ────────────────────────────────────────────────────────────────

func TestCase147(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-m19-ignore"
	e.EnsureProject(t, proj, false, nil, nil)
	ws := e.NewPublishWS(t, proj, "m19ws")

	// 现场：根级 build、嵌套 a/build、大小写不同的 BUILD、普通文件、日志文件
	WriteText(t, filepath.Join(ws, "build", "root.txt"), "x")
	WriteText(t, filepath.Join(ws, "a", "build", "nested.txt"), "x")
	WriteText(t, filepath.Join(ws, "BUILD", "upper.txt"), "x")
	WriteText(t, filepath.Join(ws, "keep.txt"), "keep")
	WriteText(t, filepath.Join(ws, "debug.log"), "log")

	// 写入 ignore 规则：目录 build（应命中任意层级 + 大小写不敏感）、文件 *.log
	WriteJSON(t, filepath.Join(ws, ".updator", "shared.json"), map[string]interface{}{
		"server_url":      e.BaseURL(),
		"project_name":    proj,
		"ignore_folders":  []string{"build"},
		"ignore_files":    []string{"*.log"},
		"un_copy_folders": []string{},
		"un_copy_files":   []string{},
	})

	r := e.PublishCLI(t, ws, "status")
	True(t, r.IsSuccess, "status 失败: %s", r.ErrorMsg)
	var sd StatusData
	if err := json.Unmarshal(r.Data, &sd); err != nil {
		t.Fatalf("解析 status 失败: %v", err)
	}
	rels := map[string]bool{}
	for _, f := range sd.Unstaged {
		rels[f.RelativePath] = true
	}
	// 三个 build 变体都必须被忽略（旧实现只认根级、且大小写敏感）
	True(t, !rels["build/root.txt"], "根级 build 应被忽略")
	True(t, !rels["a/build/nested.txt"], "嵌套 a/build 应被忽略（旧实现失效）")
	True(t, !rels["BUILD/upper.txt"], "BUILD 应被忽略（大小写不敏感）")
	True(t, !rels["debug.log"], "*.log 应被忽略")
	True(t, rels["keep.txt"], "keep.txt 不应被忽略")
}

// ────────────────────────────────────────────────────────────────
// 148 M21：add 路径归一
// ────────────────────────────────────────────────────────────────

func TestCase148(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-m21-addpath"
	e.EnsureProject(t, proj, false, nil, nil)
	ws := e.NewPublishWS(t, proj, "m21ws")
	WriteText(t, filepath.Join(ws, "sub", "f.txt"), "hello")

	// 用反斜杠 + ./ 写法添加（CLI 手工输入的典型形态）
	if r := e.PublishCLI(t, ws, "add", `.\sub\f.txt`); !r.IsSuccess {
		t.Fatalf("add 反斜杠路径失败: %s", r.ErrorMsg)
	}
	// 再用正斜杠写法添加同一文件：不得重复
	if r := e.PublishCLI(t, ws, "add", "./sub/f.txt"); !r.IsSuccess {
		t.Fatalf("add 正斜杠路径失败: %s", r.ErrorMsg)
	}

	r := e.PublishCLI(t, ws, "status")
	True(t, r.IsSuccess, "status 失败: %s", r.ErrorMsg)
	var sd StatusData
	if err := json.Unmarshal(r.Data, &sd); err != nil {
		t.Fatalf("解析 status 失败: %v", err)
	}
	Eq(t, "同一文件只能暂存一次", len(sd.Staged), 1)
	Eq(t, "暂存路径必须归一化为正斜杠", sd.Staged[0].RelativePath, "sub/f.txt")
	for _, f := range sd.Unstaged {
		True(t, f.RelativePath != "sub/f.txt",
			"已暂存文件不得同时出现在 unstaged（旧实现双行 bug）")
	}
}
