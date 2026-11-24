package tc

// 用例 66–77：aly-publish（发布端命令行）
//
// 工作目录约定：在哪个目录执行就用哪个目录的 `.updator/`（对标 git）。
// 所有调用统一走 e.PublishCLI（自动附加 --server <url> --json），
// 保证成功与失败都是 stdout 上一行 {"isSuccess":...}，断言口径统一。
//
// 共同前置：项目「创建即 V1.0.0」，所以首版发布必须是 >V1.0.0 的版本号。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────
// 用例 66 — config init 生成两个配置文件（幂等）
// ────────────────────────────────────────────────────────────────
func TestCase66(t *testing.T) {
	e := NewEnv(t, true, 0)
	e.EnsureProject(t, "tc-app", false, nil, nil)

	ws := filepath.Join(e.Root, "pub-init")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("建空工作区失败: %v", err)
	}

	r := e.PublishCLI(t, ws, "config", "init", "--project", "tc-app",
		"--ignore-folders", "logs,tmp", "--ignore-files", "*.log,*.tmp")
	True(t, r.IsSuccess, "config init 失败: %s", r.ErrorMsg)

	sharedPath := filepath.Join(ws, ".updator", "shared.json")
	pubPath := filepath.Join(ws, ".updator", "publish.json")
	Eq(t, "shared.json 已创建", Exists(sharedPath), true)
	Eq(t, "publish.json 已创建", Exists(pubPath), true)

	shared := map[string]interface{}{}
	True(t, jsonUnmarshalText(t, sharedPath, &shared), "shared.json 应是合法 JSON")
	Eq(t, "project_name", fmt.Sprint(shared["project_name"]), "tc-app")
	Eq(t, "ignore_folders", fmt.Sprint(shared["ignore_folders"]), "[logs tmp]")
	Eq(t, "ignore_files", fmt.Sprint(shared["ignore_files"]), "[*.log *.tmp]")
	// 未指定的两个数组必须是 []，不能是 null
	Eq(t, "un_copy_folders 应为 []", fmt.Sprint(shared["un_copy_folders"]), "[]")
	Eq(t, "un_copy_files 应为 []", fmt.Sprint(shared["un_copy_files"]), "[]")

	// 幂等：再 init 一次不报错，且不清空已有 ignore 规则
	True(t, e.PublishCLI(t, ws, "config", "init", "--project", "tc-app").IsSuccess, "重复 init 应幂等")
	after := map[string]interface{}{}
	True(t, jsonUnmarshalText(t, sharedPath, &after), "重复 init 后 shared.json 仍合法")
	Eq(t, "重复 init 不清空 ignore_files", fmt.Sprint(after["ignore_files"]), "[*.log *.tmp]")
}

// jsonUnmarshalText 读文件并解析 JSON。
func jsonUnmarshalText(t *testing.T, path string, v interface{}) bool {
	t.Helper()
	return jsonUnmarshalBytes([]byte(ReadText(path)), v)
}

// jsonUnmarshalBytes 解析 JSON 字节，失败返回 false（不 Fatal，调用方自行断言）。
func jsonUnmarshalBytes(b []byte, v interface{}) bool {
	return json.Unmarshal(b, v) == nil
}

// ────────────────────────────────────────────────────────────────
// 用例 67 — status 分类（new / modified / unchanged）
//
// 重要口径一（易踩）：`.updator/shared.json` **必须参与扫描并上传**——
// 它是随版本下发的客户端配置（scanner.go 明确只排除 `.updator/publish.json`
// 与 `.updator/staging/`）。所以工作区里这个文件永远算一个 `new`。
// 下面的断言把业务文件与 .updator/shared.json 分开计算。
//
// 重要口径二（2026-11-23 契约变更，d17cc69）：**删除同步已移除**。
// 「服务端有、本地没有」的文件不再标 `deleted`，diff 直接忽略——
// 服务端文件永久保留，status 里也不会再出现 deleted 分类。
// ────────────────────────────────────────────────────────────────
func TestCase67(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-pub"
	e.EnsureProject(t, proj, false, nil, nil)
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		e.PutFile(t, proj, n, n+"-v1")
	}
	True(t, e.Publish(t, proj, "1.0.1", []string{"首版"}, "", "").IsSuccess, "发布 1.0.1")

	ws := e.NewPublishWS(t, proj, "pub-status")
	WriteText(t, filepath.Join(ws, "a.txt"), "a.txt-v1") // 相同
	WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v2") // 修改
	// c.txt 不建 → 本地缺失（服务端独有，新契约下不算差异）
	WriteText(t, filepath.Join(ws, "d.txt"), "d.txt-new") // 新增

	r := e.PublishCLI(t, ws, "status")
	True(t, r.IsSuccess, "status 失败: %s", r.ErrorMsg)
	sd := RespData[StatusData](t, r)

	byPath := map[string]FileStatusItem{}
	for _, it := range sd.Unstaged {
		byPath[it.RelativePath] = it
	}

	_, aIn := byPath["a.txt"]
	Eq(t, "a.txt 无差异不应出现", aIn, false)
	Eq(t, "b.txt 状态", byPath["b.txt"].Status, "modified")
	// 2026-11-23 契约：服务端独有文件被 diff 忽略，不再标 deleted
	_, cIn := byPath["c.txt"]
	Eq(t, "c.txt（服务端独有）不再出现在差异里", cIn, false)
	Eq(t, "d.txt 状态", byPath["d.txt"].Status, "new")
	// 2 个业务差异（b/d）+ .updator/shared.json（new）；c.txt 已被忽略
	Eq(t, "unstaged 条数（2 个业务差异 + .updator/shared.json）", len(sd.Unstaged), 3)
	Eq(t, ".updator/shared.json 也在差异里（随版本下发的客户端配置）",
		byPath[".updator/shared.json"].Status, "new")

	// b.txt 带两侧 md5 且不同；d.txt 的 serverMd5 为空
	True(t, byPath["b.txt"].ServerMd5 != byPath["b.txt"].LocalMd5, "modified 两侧 md5 应不同")
	Eq(t, "new 的 serverMd5 应为空", byPath["d.txt"].ServerMd5, "")

	// a.txt 应出现在 unchanged 里
	foundUnchanged := false
	for _, it := range sd.Unchanged {
		if it.RelativePath == "a.txt" {
			foundUnchanged = true
		}
	}
	True(t, foundUnchanged, "a.txt 应归入 unchanged")
}

// ────────────────────────────────────────────────────────────────
// 用例 68 — add --all 暂存【全部变更（new + modified）】
//
// 2026-11-23 契约变更（d17cc69）：删除同步已移除，diff 不再产生 deleted
// 分类，add --all 只暂存 new + modified + `.updator/shared.json`。
// 本例：b.txt(modified) + d.txt(new) + .updator/shared.json(new) = 3 项，
// 暂存后 unstaged 清空；服务端独有的 c.txt 不参与。
// ────────────────────────────────────────────────────────────────
func TestCase68(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-pub-add"
	e.EnsureProject(t, proj, false, nil, nil)
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		e.PutFile(t, proj, n, n+"-v1")
	}
	True(t, e.Publish(t, proj, "1.0.1", []string{"首版"}, "", "").IsSuccess, "发布")

	ws := e.NewPublishWS(t, proj, "pub-add")
	WriteText(t, filepath.Join(ws, "a.txt"), "a.txt-v1")
	WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v2")
	WriteText(t, filepath.Join(ws, "d.txt"), "d.txt-new")

	r := e.PublishCLI(t, ws, "add", "--all")
	True(t, r.IsSuccess, "add --all 失败: %s", r.ErrorMsg)
	// 3 项（b.txt modified、d.txt new、.updator/shared.json new）；c.txt 服务端独有被忽略
	Eq(t, "暂存数量（b + d + .updator/shared.json）", RespData[AddData](t, r).Added, 3)

	st := e.PublishCLI(t, ws, "staged")
	True(t, st.IsSuccess, "staged 失败: %s", st.ErrorMsg)
	staged := RespData[[]StagedItem](t, st)
	names := make([]string, 0, len(staged))
	for _, s := range staged {
		names = append(names, s.RelativePath)
	}
	sort.Strings(names)
	Eq(t, "暂存内容（不含服务端独有的 c.txt）",
		strings.Join(names, ","), ".updator/shared.json,b.txt,d.txt")

	byPath := map[string]StagedItem{}
	for _, s := range staged {
		byPath[s.RelativePath] = s
	}
	Eq(t, "b.txt 暂存 status 记录加入时的分类", byPath["b.txt"].Status, "modified")
	Eq(t, "d.txt 暂存 status 记录加入时的分类", byPath["d.txt"].Status, "new")
	True(t, byPath["b.txt"].LocalMd5 != "", "暂存项必须带 md5（push 前复核的基线）")

	// 暂存后 status：staged 为 3，unstaged 清空
	sd := RespData[StatusData](t, e.PublishCLI(t, ws, "status"))
	Eq(t, "staged 为 3", len(sd.Staged), 3)
	Eq(t, "unstaged 清空", len(sd.Unstaged), 0)
}

// ────────────────────────────────────────────────────────────────
// 用例 69 — push 发布（上传 + 建版本 + 清单固化）
//
// 2026-11-23 契约变更（d17cc69）：push 只上传暂存条目并建版本，不再调用
// file/delete_file（该接口已随删除同步一并移除）。服务端独有文件永久保留。
// 本例断言：上传的 b/d 落地、c.txt 保留、清单按发布后磁盘内容固化。
// ────────────────────────────────────────────────────────────────
func TestCase69(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-pub-push"
	e.EnsureProject(t, proj, false, nil, nil)
	for _, n := range []string{"a.txt", "b.txt", "c.txt"} {
		e.PutFile(t, proj, n, n+"-v1")
	}
	True(t, e.Publish(t, proj, "1.0.1", []string{"首版"}, "", "").IsSuccess, "发布 1.0.1")

	ws := e.NewPublishWS(t, proj, "pub-push")
	WriteText(t, filepath.Join(ws, "a.txt"), "a.txt-v1")
	WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v2")
	WriteText(t, filepath.Join(ws, "d.txt"), "d.txt-new")
	True(t, e.PublishCLI(t, ws, "add", "--all").IsSuccess, "add --all")

	r := e.PublishCLI(t, ws, "push", "--version", "1.1.0",
		"--message", "更新说明", "--message", "第二条")
	True(t, r.IsSuccess, "push 失败: %s", r.ErrorMsg)
	pd := RespData[PushData](t, r)
	Eq(t, "push 回报的版本", pd.Version, "1.1.0")
	// staged 3 项（b.txt / d.txt / .updator/shared.json）
	Eq(t, "push 回报的文件数", pd.Files, "3")

	Eq(t, "服务端版本", e.GetProject(t, proj).Version, "1.1.0")
	logs := e.ChangeLogs(t, proj)
	Eq(t, "日志条数（V1.0.0 + 1.0.1 + 1.1.0）", len(logs), 3)
	Eq(t, "最新日志版本（倒序首项）", logs[0].Version, "1.1.0")
	Eq(t, "两条 --message 都落库", len(logs[0].Logs), 2)
	Eq(t, "第一条 message", logs[0].Logs[0], "更新说明")
	Eq(t, "第二条 message", logs[0].Logs[1], "第二条")

	// 服务端文件集合：b 更新、d 新增；c.txt 服务端独有，**保留**
	want := ".updator/shared.json,a.txt,b.txt,c.txt,d.txt"
	Eq(t, "服务端文件集合", strings.Join(relsOf(e.GetAllFiles(t, proj)), ","), want)
	Eq(t, "c.txt 保留（删除同步已移除，服务端文件不再被删）",
		Exists(filepath.Join(e.DataDir(proj), "c.txt")), true)
	Eq(t, "b.txt 已更新",
		strings.TrimSpace(ReadText(filepath.Join(e.DataDir(proj), "b.txt"))), "b.txt-v2")
	Eq(t, "d.txt 已落地",
		strings.TrimSpace(ReadText(filepath.Join(e.DataDir(proj), "d.txt"))), "d.txt-new")

	// 清单与 get_all_files 同口径（发布时按磁盘实际内容固化）
	m := e.GetManifest(t, proj)
	sort.Strings(m.Files)
	Eq(t, "清单", strings.Join(m.Files, ","), want)

	// 暂存区被清空（push 成功才清）
	staged := RespData[[]StagedItem](t, e.PublishCLI(t, ws, "staged"))
	Eq(t, "推送成功后暂存区应清空", len(staged), 0)
}

// ────────────────────────────────────────────────────────────────
// 用例 70 — push 后 status 干净 + 本地删除不再同步（新契约）
//
// 2026-11-23 契约变更（d17cc69）：push 后 status 收敛为干净；本地删除文件
// **不再**产生 deleted 差异——服务端文件保留，status 保持干净，不产生
// 可暂存/可推送的删除条目（防误删只存在于服务端的资源）。
// ────────────────────────────────────────────────────────────────
func TestCase70(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-pub-clean"
	e.EnsureProject(t, proj, false, nil, nil)
	for _, n := range []string{"a.txt", "b.txt"} {
		e.PutFile(t, proj, n, n+"-v1")
	}
	True(t, e.Publish(t, proj, "1.0.1", []string{"首版"}, "", "").IsSuccess, "发布")

	ws := e.NewPublishWS(t, proj, "pub-clean")
	WriteText(t, filepath.Join(ws, "a.txt"), "a.txt-v1")
	WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v2")
	WriteText(t, filepath.Join(ws, "d.txt"), "d.txt-new")

	True(t, e.PublishCLI(t, ws, "add", "--all").IsSuccess, "add --all")
	True(t, e.PublishCLI(t, ws, "push", "--version", "1.1.0", "--message", "更新").IsSuccess, "push")

	// 推完立刻 status：所有参与发布的文件都应与服务端一致
	sd := RespData[StatusData](t, e.PublishCLI(t, ws, "status"))
	Eq(t, "unstaged 应为空", len(sd.Unstaged), 0)
	Eq(t, "staged 应为空", len(sd.Staged), 0)

	// unchanged 里应能看到 a.txt / b.txt / d.txt / .updator/shared.json
	unchanged := make([]string, 0, len(sd.Unchanged))
	for _, it := range sd.Unchanged {
		unchanged = append(unchanged, it.RelativePath)
	}
	sort.Strings(unchanged)
	Eq(t, "已上传文件全部归入 unchanged",
		strings.Join(unchanged, ","), ".updator/shared.json,a.txt,b.txt,d.txt")

	// ── 本地删了文件：新契约下不再同步删除 ──
	if err := os.Remove(filepath.Join(ws, "d.txt")); err != nil {
		t.Fatalf("删本地 d.txt 失败: %v", err)
	}
	sd2 := RespData[StatusData](t, e.PublishCLI(t, ws, "status"))
	// d.txt 服务端独有 → diff 忽略，status 仍干净；服务端文件保留
	Eq(t, "本地删除后 status 仍干净（服务端独有文件不算差异）", len(sd2.Unstaged), 0)
	Eq(t, "staged 仍为空", len(sd2.Staged), 0)
	Eq(t, "add --all 无可暂存项", RespData[AddData](t, e.PublishCLI(t, ws, "add", "--all")).Added, 0)
	Eq(t, "服务端 d.txt 保留", Exists(filepath.Join(e.DataDir(proj), "d.txt")), true)

	// 服务端 b.txt 照常可更新（确认忽略逻辑没有误伤 modified 链路）
	WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v3")
	sd3 := RespData[StatusData](t, e.PublishCLI(t, ws, "status"))
	Eq(t, "modified 仍正常出现", sd3.Unstaged[0].RelativePath+"/"+sd3.Unstaged[0].Status, "b.txt/modified")
	True(t, e.PublishCLI(t, ws, "add", "b.txt").IsSuccess, "add b.txt")
	True(t, e.PublishCLI(t, ws, "push", "--version", "1.2.0", "--message", "b3").IsSuccess, "push b3")
	Eq(t, "服务端 b.txt 已更新",
		strings.TrimSpace(ReadText(filepath.Join(e.DataDir(proj), "b.txt"))), "b.txt-v3")
}

// ────────────────────────────────────────────────────────────────
// 用例 71 — push --dry-run 不实际推送
//
// 2026-11-23 修复（审查轻微①）：`--dry-run` 在 `--json` 下也输出一行
// 成功信封（data.dryRun="true"），GUI 的 parsePushLine 依赖看到结果行，
// 否则误报 "publish.noResult"。人读文本 [DRY RUN] 预告保留。
// 用"服务端零变化"证明真的没推。
// ────────────────────────────────────────────────────────────────
func TestCase71(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-pub-dry"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "b.txt", "b.txt-v2")
	True(t, e.Publish(t, proj, "1.1.0", []string{"首版"}, "", "").IsSuccess, "发布 1.1.0")

	ws := e.NewPublishWS(t, proj, "pub-dry")
	WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v3")
	True(t, e.PublishCLI(t, ws, "add", "b.txt").IsSuccess, "add b.txt")

	before := e.GetProject(t, proj).Version
	beforeLogs := len(e.ChangeLogs(t, proj))

	raw := e.PublishCLIRaw(t, ws, "push", "--version", "1.2.0", "--message", "x", "--dry-run")
	Contains(t, raw.Stdout, "[DRY RUN]", "dry-run 应打印人读预告")
	Contains(t, raw.Stdout, "b.txt", "预告应列出待上传文件")
	Contains(t, raw.Stdout, "1.2.0", "预告应列出待创建版本")
	// 修复后：--json 下也有一行结果信封，且 data.dryRun=true
	Contains(t, raw.Stdout, `"dryRun":"true"`, "json 模式必须输出 dryRun 信封（GUI parsePushLine 依赖）")
	dry := RespData[PushDryData](t, e.PublishCLI(t, ws, "push", "--version", "1.2.0", "--message", "x", "--dry-run"))
	Eq(t, "dry-run 信封的版本", dry.Version, "1.2.0")
	Eq(t, "dry-run 信封的文件数", dry.Files, "1")

	// 服务端零变化
	Eq(t, "服务端版本未变", e.GetProject(t, proj).Version, before)
	Eq(t, "服务端 b.txt 未变",
		strings.TrimSpace(ReadText(filepath.Join(e.DataDir(proj), "b.txt"))), "b.txt-v2")
	Eq(t, "不得新增 changelog", len(e.ChangeLogs(t, proj)), beforeLogs)
	// 暂存区保留（dry-run 后可直接重试，不必重新 add）
	staged := RespData[[]StagedItem](t, e.PublishCLI(t, ws, "staged"))
	Eq(t, "暂存区应保留", len(staged), 1)
	Eq(t, "保留的是 b.txt", staged[0].RelativePath, "b.txt")

	// 反证：不加 --dry-run 立刻能推上去（说明上面的"零变化"不是别的原因导致）
	True(t, e.PublishCLI(t, ws, "push", "--version", "1.2.0", "--message", "x").IsSuccess, "真推应成功")
	Eq(t, "真推后版本推进", e.GetProject(t, proj).Version, "1.2.0")
}

// ────────────────────────────────────────────────────────────────
// 用例 72 — 缺 --message 被拒（且校验在上传之前）
// ────────────────────────────────────────────────────────────────
func TestCase72(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-pub-nomsg"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "b.txt", "b.txt-v2")
	True(t, e.Publish(t, proj, "1.1.0", []string{"首版"}, "", "").IsSuccess, "发布")

	ws := e.NewPublishWS(t, proj, "pub-nomsg")
	WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v3")
	True(t, e.PublishCLI(t, ws, "add", "b.txt").IsSuccess, "add")

	before := e.GetProject(t, proj).Version
	r := e.PublishCLI(t, ws, "push", "--version", "1.2.0")
	Eq(t, "缺 message 应失败", r.IsSuccess, false)
	Contains(t, strings.ToLower(r.ErrorMsg), "message", "errorMsg")
	Eq(t, "服务端版本未变", e.GetProject(t, proj).Version, before)
	Eq(t, "不得新增 changelog", len(e.ChangeLogs(t, proj)), 2)

	// 校验发生在上传之前：服务端 b.txt 内容未被改动（证明没先传一堆文件再报错）
	Eq(t, "服务端 b.txt 未被上传",
		strings.TrimSpace(ReadText(filepath.Join(e.DataDir(proj), "b.txt"))), "b.txt-v2")
}

// ────────────────────────────────────────────────────────────────
// 用例 73 — 项目不存在：报错不崩溃、不上传
//
// 注意 push 的实际报错路径：`add --all` 已经先因为"项目不存在"失败了，
// 暂存区是空的，于是 push 在**发请求之前**就被本地前置检查拦下，
// 报的是"暂存区为空，请先 add 文件"，而不是服务端的"项目不存在"。
// 所以这里只断"明确失败 + 有 errorMsg"，不锁定文案。
// ────────────────────────────────────────────────────────────────
func TestCase73(t *testing.T) {
	e := NewEnv(t, true, 0)
	const ghost = "tc-not-exist-here"

	ws2 := filepath.Join(e.Root, "pub-missing")
	if err := os.MkdirAll(filepath.Join(ws2, ".updator"), 0o755); err != nil {
		t.Fatalf("建工作区失败: %v", err)
	}
	WriteJSON(t, filepath.Join(ws2, ".updator", "shared.json"), map[string]interface{}{
		"server_url": e.BaseURL(), "project_name": ghost,
		"ignore_folders": []string{}, "ignore_files": []string{},
		"un_copy_folders": []string{}, "un_copy_files": []string{},
	})
	WriteText(t, filepath.Join(ws2, "x.txt"), "x")

	r1 := e.PublishCLI(t, ws2, "status")
	Eq(t, "status 应失败", r1.IsSuccess, false)
	True(t, r1.ErrorMsg != "", "status errorMsg 不应为空")
	// status 直接打服务端，能拿到服务端的原始文案 "ent: project not found"
	Contains(t, strings.ToLower(r1.ErrorMsg), "project not found", "status errorMsg 应指出项目不存在")

	// add --all 需要服务端文件列表来算差异，拿不到就失败
	r2 := e.PublishCLI(t, ws2, "add", "--all")
	Eq(t, "add --all 应失败（算不出差异）", r2.IsSuccess, false)
	True(t, r2.ErrorMsg != "", "add errorMsg 不应为空")

	// push：暂存区为空 → 本地就被拦下（没发请求）
	r3 := e.PublishCLI(t, ws2, "push", "--version", "9.9.9", "--message", "x")
	Eq(t, "push 应失败", r3.IsSuccess, false)
	True(t, r3.ErrorMsg != "", "push errorMsg 不应为空")
	t.Logf("push errorMsg = %q（真实路径：add 失败 → 暂存区空 → push 本地拦下）", r3.ErrorMsg)

	// 三条都不得在任何地方创建数据目录 / 上传文件
	Eq(t, "不得为不存在项目创建数据目录", Exists(e.DataDir(ghost)), false)
}

// ────────────────────────────────────────────────────────────────
// 用例 74 — 重复/倒退版本号被服务端拒绝，暂存区保留
//
// ⚠ 这里依赖 `parseCLIResult` 的"跳过进度行与 printProgressDone 尾巴行"逻辑。
// 真实缺陷：push 的失败路径是
//     outputResult(false, "创建版本失败: ...")   ← 真结果
//     printProgressDone()                       ← 又打一行 {"isSuccess":true,"data":null}
// 若按"取最后一行"解析，会把失败读成成功（CI 误判为发布成功）。
// ────────────────────────────────────────────────────────────────
func TestCase74(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-pub-ver"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "b.txt", "b.txt-v2")
	True(t, e.Publish(t, proj, "1.1.0", []string{"首版"}, "", "").IsSuccess, "发布 1.1.0")

	ws := e.NewPublishWS(t, proj, "pub-ver")
	WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v3")
	True(t, e.PublishCLI(t, ws, "add", "b.txt").IsSuccess, "add")

	r1 := e.PublishCLI(t, ws, "push", "--version", "1.1.0", "--message", "重发")
	Eq(t, "重发同版本应失败", r1.IsSuccess, false)
	Contains(t, r1.ErrorMsg, "1.1.0", "errorMsg 应带上被拒的版本号")
	Contains(t, r1.ErrorMsg, "防倒退", "errorMsg 应透传服务端的防倒退说明")

	r2 := e.PublishCLI(t, ws, "push", "--version", "1.0.0", "--message", "倒退")
	Eq(t, "倒退版本应失败", r2.IsSuccess, false)
	Contains(t, r2.ErrorMsg, "防倒退", "倒退同样被防倒退拦下")

	// 服务端版本不变、无新日志
	Eq(t, "服务端版本未变", e.GetProject(t, proj).Version, "1.1.0")
	Eq(t, "不得新增 changelog", len(e.ChangeLogs(t, proj)), 2) // V1.0.0 + 1.1.0

	// 暂存区保留（用户修正版本号后可直接重试）
	staged := RespData[[]StagedItem](t, e.PublishCLI(t, ws, "staged"))
	Eq(t, "暂存区应保留", len(staged), 1)

	// 已知行为（文档已记录）：push 先传文件后建版本，所以版本被拒时
	// 文件其实已经落到服务端了。断言把它钉死，避免以后误以为是新问题。
	Eq(t, "已知行为：被拒时文件已上传（先传文件后建版本）",
		strings.TrimSpace(ReadText(filepath.Join(e.DataDir(proj), "b.txt"))), "b.txt-v3")

	// 改对版本号后能直接重试成功（暂存区还在）
	True(t, e.PublishCLI(t, ws, "push", "--version", "1.1.1", "--message", "修正").IsSuccess,
		"修正版本号后应能直接重试")
	Eq(t, "重试后版本推进", e.GetProject(t, proj).Version, "1.1.1")
}

// ────────────────────────────────────────────────────────────────
// 用例 75 — 服务器不可达：快速失败不挂死
// ────────────────────────────────────────────────────────────────
func TestCase75(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-pub-dead"
	e.EnsureProject(t, proj, false, nil, nil)
	ws := e.NewPublishWS(t, proj, "pub-dead")
	WriteText(t, filepath.Join(ws, "x.txt"), "x")

	dead := "http://127.0.0.1:1" // 必然不可达
	start := time.Now()
	r := e.PublishCLIAt(t, ws, dead, "server", "info")
	elapsed := time.Since(start)

	Eq(t, "应失败", r.IsSuccess, false)
	True(t, elapsed < 60*time.Second, "应快速失败，实际耗时 %s", elapsed)
	True(t, r.ErrorMsg != "", "errorMsg 不应为空")
	t.Logf("不可达耗时 %s，errorMsg=%s", elapsed, r.ErrorMsg)

	// status 同样应快速失败
	start2 := time.Now()
	r2 := e.PublishCLIAt(t, ws, dead, "status")
	Eq(t, "status 应失败", r2.IsSuccess, false)
	True(t, time.Since(start2) < 60*time.Second, "status 也应快速失败")
}

// ────────────────────────────────────────────────────────────────
// 用例 76 — 中文 / 空格 / 深路径端到端
// ────────────────────────────────────────────────────────────────
func TestCase76(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-path"
	e.EnsureProject(t, proj, false, nil, nil)
	ws := e.NewPublishWS(t, proj, "pub-path")

	deep := "一级 目录/二级 目录/三级/deep file 名.txt"
	WriteText(t, filepath.Join(ws, filepath.FromSlash(deep)), "中文内容")
	WriteText(t, filepath.Join(ws, "根 目录", "sub", "x.txt"), "x")

	True(t, e.PublishCLI(t, ws, "add", "--all").IsSuccess, "add 失败")
	r := e.PublishCLI(t, ws, "push", "--version", "1.0.1", "--message", "中文路径")
	True(t, r.IsSuccess, "push 失败: %s", r.ErrorMsg)

	dst := filepath.Join(e.DataDir(proj), filepath.FromSlash(deep))
	Eq(t, "深路径落地", Exists(dst), true)
	Eq(t, "内容一致", strings.TrimSpace(ReadText(dst)), "中文内容")

	rels := relsOf(e.GetAllFiles(t, proj))
	joined := strings.Join(rels, ",")
	Contains(t, joined, deep, "相对路径应为 / 分隔且无乱码")
	Contains(t, joined, "根 目录/sub/x.txt", "根目录下路径")

	// 清单里也是 / 分隔的中文路径
	m := e.GetManifest(t, proj)
	sort.Strings(m.Files)
	Contains(t, strings.Join(m.Files, ","), deep, "清单路径")

	// status 事后干净
	sd := RespData[StatusData](t, e.PublishCLI(t, ws, "status"))
	Eq(t, "push 后 unstaged 应为空", len(sd.Unstaged), 0)
}

// ────────────────────────────────────────────────────────────────
// 用例 77 — 暂存后文件被改：MD5 复核拒绝；--force 可跳过
// ────────────────────────────────────────────────────────────────
func TestCase77(t *testing.T) {
	e := NewEnv(t, true, 0)
	const proj = "tc-pub-verify"
	e.EnsureProject(t, proj, false, nil, nil)
	e.PutFile(t, proj, "b.txt", "b.txt-v2")
	True(t, e.Publish(t, proj, "1.1.0", []string{"首版"}, "", "").IsSuccess, "发布")

	ws := e.NewPublishWS(t, proj, "pub-verify")
	WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-v3")
	True(t, e.PublishCLI(t, ws, "add", "b.txt").IsSuccess, "add")

	// 暂存后改文件：暂存 md5 与磁盘不再一致
	WriteText(t, filepath.Join(ws, "b.txt"), "b.txt-CHANGED-AFTER-ADD")

	r := e.PublishCLI(t, ws, "push", "--version", "1.2.0", "--message", "x")
	Eq(t, "被改文件应拒绝 push", r.IsSuccess, false)
	Contains(t, r.ErrorMsg, "b.txt", "errorMsg 应列出冲突文件")
	Contains(t, strings.ToLower(r.ErrorMsg), "add", "errorMsg 应提示重新 add")
	Eq(t, "服务端版本未变", e.GetProject(t, proj).Version, "1.1.0")
	Eq(t, "服务端 b.txt 未被推上去",
		strings.TrimSpace(ReadText(filepath.Join(e.DataDir(proj), "b.txt"))), "b.txt-v2")

	// --force 跳过复核，上传当前内容
	r2 := e.PublishCLI(t, ws, "push", "--version", "1.2.0", "--message", "x", "--force")
	True(t, r2.IsSuccess, "--force 应跳过复核: %s", r2.ErrorMsg)
	Eq(t, "force 后上传的是新内容",
		strings.TrimSpace(ReadText(filepath.Join(e.DataDir(proj), "b.txt"))), "b.txt-CHANGED-AFTER-ADD")
	Eq(t, "force 后版本推进", e.GetProject(t, proj).Version, "1.2.0")
}