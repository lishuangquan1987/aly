package controllers_test

// 服务端 HTTP 契约测试（审查 A4：server 是全体系自动化测试最薄弱的一环）。
//
// 覆盖范围（重点为边界条件）：
//   - B1  项目名路径穿越：".", "..", "../..", "..\\..\\", "a/../..", "../sub"
//   - B3  分片合并：totalChunks<=0 / 超上界 / 全零字节 / MD5 不符 / 缺片
//   - N1  totalChunks 上界（OOM/CPU 自旋 DoS）
//   - M1  Content-Disposition 的 RFC 6266/5987 编码
//   - M2  软删除项目不再对外服务
//   - M3  并发 complete 的互斥与合并结果校验
//   - M5  上传要求项目存在
//   - B6  delete_file 删除同步（含幂等与穿越防护）
//   - B11 Range 断点续传 206/416
//   - 轻微 get_project_change_logs 条数上限
//
// 全部用例走真实 gin 引擎 + httptest，数据目录由 pathcfg 注入 t.TempDir 等价目录。

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"aly/server/ent/project"
	"aly/server/internal/db"
	"aly/server/internal/pathcfg"
	"aly/server/models"
	"aly/server/routers"

	"github.com/gin-gonic/gin"
)

var testBaseDir string

func TestMain(m *testing.M) {
	gin.SetMode(gin.ReleaseMode)
	dir, err := os.MkdirTemp("", "aly-server-apitest")
	if err != nil {
		panic(err)
	}
	testBaseDir = dir
	pathcfg.SetBaseDir(dir)
	db.DBPath = filepath.Join(dir, "test.db")
	db.InitDB()

	code := m.Run()

	if db.Client != nil {
		db.Client.Close()
	}
	os.RemoveAll(dir)
	os.Exit(code)
}

func newRouter() *gin.Engine {
	r := gin.New()
	routers.InitRouter(r)
	return r
}

// doJSON 发送 JSON 请求并解析统一信封
func doJSON(t *testing.T, r *gin.Engine, method, path string, body interface{}) models.CommonResponse {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp models.CommonResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("%s %s: 响应不是合法 JSON 信封: %v; body=%s", method, path, err, w.Body.String())
	}
	return resp
}

// mustCreateProject 建立测试项目（已存在时视为成功）
func mustCreateProject(t *testing.T, r *gin.Engine, name string) {
	t.Helper()
	resp := doJSON(t, r, http.MethodPost, "/api/project/create_project", map[string]interface{}{
		"name":  name,
		"title": "test " + name,
	})
	if !resp.IsSuccess && !strings.Contains(resp.ErrorMsg, "已存在") {
		t.Fatalf("创建项目 %s 失败: %s", name, resp.ErrorMsg)
	}
}

// postMultipart 发送 multipart 表单
func postMultipart(t *testing.T, r *gin.Engine, path string, fields map[string]string, fileField, fileName string, content []byte) models.CommonResponse {
	t.Helper()
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	fw, err := mw.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp models.CommonResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("%s: 响应不是合法 JSON 信封: %v; body=%s", path, err, w.Body.String())
	}
	return resp
}

// uploadFile 直接上传整文件
func uploadFile(t *testing.T, r *gin.Engine, project, rel string, content []byte) models.CommonResponse {
	t.Helper()
	return postMultipart(t, r, "/api/file/upload_file", map[string]string{
		"projectName":      project,
		"relativeFileName": rel,
	}, "file", filepath.Base(strings.Replace(rel, "\\", "/", -1)), content)
}

// dataDirOf 返回某个项目的数据目录
func dataDirOf(t *testing.T, project string) string {
	t.Helper()
	d, err := pathcfg.DataDir()
	if err != nil {
		t.Fatalf("resolve data dir: %v", err)
	}
	return filepath.Join(d, project)
}

func md5Hex(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

// ────────────────────────────────────────────────────────────────
// B1 项目名路径穿越
// ────────────────────────────────────────────────────────────────

func TestUploadRejectsProjectNameTraversal(t *testing.T) {
	r := newRouter()
	// 这些名字在旧实现里能通过正则校验（放行 . 与 ..），
	// 经 filepath.Join Clean 后逃出 data 目录。
	badNames := []string{".", "..", "../..", `..\..\`, "a/../..", "../sub", "..%5Csub"}
	for _, name := range badNames {
		t.Run(name, func(t *testing.T) {
			resp := uploadFile(t, r, name, "evil.bin", []byte("pwn"))
			if resp.IsSuccess {
				t.Fatalf("项目名 %q 应被拒绝，实际成功（路径穿越漏洞）", name)
			}
			// 断言服务端基础目录没有被写入 evil.bin（防止穿越落盘）
			leak := filepath.Join(testBaseDir, "evil.bin")
			if _, err := os.Stat(leak); err == nil {
				os.Remove(leak)
				t.Fatalf("项目名 %q 造成穿越落盘: %s", name, leak)
			}
		})
	}
}

func TestProjectNameValidationBoundaries(t *testing.T) {
	// 直接对校验规则做边界表驱动（service.ValidateProjectName 经控制器入口验证）
	r := newRouter()
	cases := []struct {
		name string
		ok   bool
	}{
		{".", false},
		{"..", false},
		{"...", false}, // 尾部点是 Windows 无法稳定创建的形态（被静默剥离）
		{"a.b", true},  // 含点在中间合法
		{"abc.", false},// 尾部点 Windows 会被剥掉
		{"abc ", false},// 尾部空格同上
		{"CON", false}, // Windows 保留名
		{"nul", false}, // 大小写不敏感
		{"normal-name", true},
		{"中文项目", true},
		{strings.Repeat("a", 64), true},
		{strings.Repeat("a", 65), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp := doJSON(t, r, http.MethodPost, "/api/project/create_project", map[string]interface{}{
				"name":  c.name,
				"title": "t",
			})
			created := resp.IsSuccess || strings.Contains(resp.ErrorMsg, "已存在")
			if created != c.ok {
				t.Fatalf("项目名 %q 期望可创建=%v，实际=%v（err=%s）", c.name, c.ok, created, resp.ErrorMsg)
			}
			if created {
				// 清理，避免影响其它子用例
				doJSON(t, r, http.MethodPost, "/api/project/delete_project/"+c.name, nil)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────
// M5 上传要求项目存在
// ────────────────────────────────────────────────────────────────

func TestUploadRequiresExistingProject(t *testing.T) {
	r := newRouter()
	resp := uploadFile(t, r, "no-such-project-xyz", "a.bin", []byte("x"))
	if resp.IsSuccess {
		t.Fatal("向不存在的项目上传应被拒绝")
	}
	// 不得自动创建目录
	if _, err := os.Stat(dataDirOf(t, "no-such-project-xyz")); err == nil {
		t.Fatal("上传不得为不存在的项目自动创建目录")
	}
}

// ────────────────────────────────────────────────────────────────
// B3 / N1 分片合并的 totalChunks 边界
// ────────────────────────────────────────────────────────────────

// prepareChunks 先上传若干分片
func prepareChunks(t *testing.T, r *gin.Engine, project, rel string, chunks [][]byte) {
	t.Helper()
	mustCreateProject(t, r, project)
	total := len(chunks)
	for i, c := range chunks {
		resp := postMultipart(t, r, "/api/file/upload_chunk", map[string]string{
			"projectName":      project,
			"relativeFileName": rel,
			"chunkIndex":       fmt.Sprint(i),
			"totalChunks":      fmt.Sprint(total),
		}, "file", fmt.Sprintf("c%d", i), c)
		if !resp.IsSuccess {
			t.Fatalf("上传分片 %d 失败: %s", i, resp.ErrorMsg)
		}
	}
}

func completeChunks(t *testing.T, r *gin.Engine, project, rel string, total int, md5Value string) models.CommonResponse {
	t.Helper()
	fields := map[string]string{
		"projectName":      project,
		"relativeFileName": rel,
		"totalChunks":      fmt.Sprint(total),
	}
	if md5Value != "" {
		fields["md5"] = md5Value
	}
	return postForm(t, r, "/api/file/upload_chunk_complete", fields)
}

// postForm 发送 application/x-www-form-urlencoded 表单（complete/delete 等无文件接口）
func postForm(t *testing.T, r *gin.Engine, path string, fields map[string]string) models.CommonResponse {
	t.Helper()
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp models.CommonResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("%s: 响应不是合法 JSON 信封: %v; body=%s", path, err, w.Body.String())
	}
	return resp
}

func TestChunkCompleteRejectsInvalidTotalChunks(t *testing.T) {
	r := newRouter()
	const proj = "tc-b3-proj"
	const rel = "target.bin"
	mustCreateProject(t, r, proj)

	// 目标文件先存在且有内容——旧实现 totalChunks=0 会把它清空成 0 字节
	original := []byte("IMPORTANT-ORIGINAL-CONTENT")
	if resp := uploadFile(t, r, proj, rel, original); !resp.IsSuccess {
		t.Fatalf("预置目标文件失败: %s", resp.ErrorMsg)
	}

	// B3：0 与负数必须拒绝
	for _, total := range []int{0, -1, -100} {
		resp := completeChunks(t, r, proj, rel, total, "")
		if resp.IsSuccess {
			t.Fatalf("totalChunks=%d 应被拒绝，实际成功（B3 清空文件漏洞）", total)
		}
		// 断言目标文件未被清空
		got, err := os.ReadFile(filepath.Join(dataDirOf(t, proj), rel))
		if err != nil || !bytes.Equal(got, original) {
			t.Fatalf("totalChunks=%d 后目标文件被破坏: content=%q err=%v", total, got, err)
		}
	}

	// N1：上界必须拒绝（65537 片）
	resp := completeChunks(t, r, proj, rel, 65537, "")
	if resp.IsSuccess {
		t.Fatal("totalChunks=65537 超上界应被拒绝")
	}
}

func TestChunkCompleteRejectsZeroSizeMerge(t *testing.T) {
	r := newRouter()
	const proj = "tc-b3-zero"
	const rel = "zero.bin"
	// 两个空分片：分片都在但总大小为 0
	prepareChunks(t, r, proj, rel, [][]byte{{}, {}})

	if resp := uploadFile(t, r, proj, rel, []byte("keep-me")); !resp.IsSuccess {
		t.Fatalf("预置目标文件失败: %s", resp.ErrorMsg)
	}
	resp := completeChunks(t, r, proj, rel, 2, "")
	if resp.IsSuccess {
		t.Fatal("全零字节合并应被拒绝（否则会把目标文件清空）")
	}
	got, _ := os.ReadFile(filepath.Join(dataDirOf(t, proj), rel))
	if string(got) != "keep-me" {
		t.Fatalf("目标文件被破坏: %q", got)
	}
}

func TestChunkCompleteMissingChunks(t *testing.T) {
	r := newRouter()
	const proj = "tc-b3-missing"
	const rel = "part.bin"
	// 上传 1/3 分片
	prepareChunks(t, r, proj, rel, [][]byte{[]byte("AAA")})
	resp := completeChunks(t, r, proj, rel, 3, "")
	if !resp.IsSuccess {
		t.Fatalf("缺片应返回成功信封+chunksComplete=false，实际 err=%s", resp.ErrorMsg)
	}
	m, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("响应 data 结构异常: %#v", resp.Data)
	}
	if m["chunksComplete"] != false {
		t.Fatalf("chunksComplete 应为 false，实际 %#v", m["chunksComplete"])
	}
}

// ────────────────────────────────────────────────────────────────
// M3 合并结果 MD5 校验 + 并发幂等
// ────────────────────────────────────────────────────────────────

func TestChunkCompleteMD5MismatchKeepsChunks(t *testing.T) {
	r := newRouter()
	const proj = "tc-m3-md5"
	const rel = "big.bin"
	chunks := [][]byte{[]byte("hello "), []byte("world")}
	prepareChunks(t, r, proj, rel, chunks)

	// 错误 MD5：必须拒绝，且 .chunks 保留供重试
	resp := completeChunks(t, r, proj, rel, len(chunks), md5Hex([]byte("wrong")))
	if resp.IsSuccess {
		t.Fatal("MD5 不符应被拒绝")
	}
	chunksDir := filepath.Join(dataDirOf(t, proj), rel) + ".chunks"
	if _, err := os.Stat(chunksDir); err != nil {
		t.Fatalf("MD5 不符后应保留分片目录: %v", err)
	}

	// 正确 MD5：成功
	want := md5Hex([]byte("hello world"))
	resp = completeChunks(t, r, proj, rel, len(chunks), want)
	if !resp.IsSuccess {
		t.Fatalf("MD5 正确应合并成功，实际: %s", resp.ErrorMsg)
	}
	got, err := os.ReadFile(filepath.Join(dataDirOf(t, proj), rel))
	if err != nil {
		t.Fatalf("读取合并结果失败: %v", err)
	}
	if string(got) != "hello world" {
		t.Fatalf("合并内容错误: %q", got)
	}
	// 成功后分片目录应被清理
	if _, err := os.Stat(chunksDir); err == nil {
		t.Fatal("合并成功后应清理分片目录")
	}
}

func TestChunkCompleteConcurrentIsSafe(t *testing.T) {
	r := newRouter()
	const proj = "tc-m3-concurrent"
	const rel = "race.bin"
	chunks := [][]byte{[]byte("AAAA"), []byte("BBBB"), []byte("CCCC")}
	prepareChunks(t, r, proj, rel, chunks)
	want := md5Hex([]byte("AAAABBBBCCCC"))

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = completeChunks(t, r, proj, rel, len(chunks), want)
		}()
	}
	wg.Wait()

	// 无论谁先完成，最终落盘内容必须完整正确（不得互相清分片产出损坏文件）
	got, err := os.ReadFile(filepath.Join(dataDirOf(t, proj), rel))
	if err != nil {
		t.Fatalf("并发合并后文件缺失: %v", err)
	}
	if md5Hex(got) != want {
		t.Fatalf("并发合并产出损坏文件: %q", got)
	}
}

// ────────────────────────────────────────────────────────────────
// B11 / M1 下载：Range、Content-Disposition
// ────────────────────────────────────────────────────────────────

// publishedFile 上传一个文件并返回其绝对路径
func publishedFile(t *testing.T, r *gin.Engine, project, rel string, content []byte) string {
	t.Helper()
	mustCreateProject(t, r, project)
	if resp := uploadFile(t, r, project, rel, content); !resp.IsSuccess {
		t.Fatalf("上传 %s 失败: %s", rel, resp.ErrorMsg)
	}
	return filepath.Join(dataDirOf(t, project), filepath.FromSlash(rel))
}

func TestDownloadRejectsPathTraversal(t *testing.T) {
	r := newRouter()
	mustCreateProject(t, r, "tc-dl-proj")
	// 在 data 之外放一个"敏感文件"
	secret := filepath.Join(testBaseDir, "aly.db")
	if err := os.WriteFile(secret, []byte("secret"), 0600); err != nil {
		t.Fatalf("写入敏感文件失败: %v", err)
	}

	dataDir, _ := pathcfg.DataDir()
	badPaths := []string{
		secret,
		filepath.Join(dataDir, "..", "aly.db"),
		filepath.Join(dataDir, "..", "..", "windows", "win.ini"),
		dataDir + string(filepath.Separator) + ".." + string(filepath.Separator) + "aly.db",
	}
	for _, p := range badPaths {
		req := httptest.NewRequest(http.MethodGet, "/api/file/download_file?path="+urlEscape(p), nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Fatalf("穿越路径 %q 应返回 404，实际 %d", p, w.Code)
		}
	}
}

func TestDownloadRangeAndContentDisposition(t *testing.T) {
	r := newRouter()
	content := []byte("0123456789abcdefghij")
	abs := publishedFile(t, r, "tc-dl-rfc", "目录/中文 名称.bin", content)

	// Range 请求 → 206 + 片段内容
	req := httptest.NewRequest(http.MethodGet, "/api/file/download_file?path="+urlEscape(abs), nil)
	req.Header.Set("Range", "bytes=2-5")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusPartialContent {
		t.Fatalf("Range 请求应返回 206，实际 %d", w.Code)
	}
	if got := w.Body.String(); got != "2345" {
		t.Fatalf("Range 内容错误: %q", got)
	}
	// M1：Content-Disposition 必须能被标准库解析，且 filename 还原为中文原名
	cd := w.Header().Get("Content-Disposition")
	mediatype, params, err := mime.ParseMediaType(cd)
	if err != nil {
		t.Fatalf("Content-Disposition 不合 RFC: %q err=%v", cd, err)
	}
	if mediatype != "attachment" {
		t.Fatalf("Content-Disposition type 错误: %q", mediatype)
	}
	if params["filename"] != "中文 名称.bin" {
		t.Fatalf("filename 解析错误: %q（原始头 %q）", params["filename"], cd)
	}
	// 不得再出现历史遗留头
	if v := w.Header().Get("Content-Transfer-Encoding"); v != "" {
		t.Fatalf("不应再输出 Content-Transfer-Encoding: %q", v)
	}

	// 越界 Range → 416
	req = httptest.NewRequest(http.MethodGet, "/api/file/download_file?path="+urlEscape(abs), nil)
	req.Header.Set("Range", "bytes=100000-")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("越界 Range 应返回 416，实际 %d", w.Code)
	}
}

// ────────────────────────────────────────────────────────────────
// M2 软删除语义
// ────────────────────────────────────────────────────────────────

func TestSoftDeletedProjectStopsServing(t *testing.T) {
	r := newRouter()
	const proj = "tc-m2-softdel"
	publishedFile(t, r, proj, "a.bin", []byte("data"))

	// 软删除
	if resp := doJSON(t, r, http.MethodPost, "/api/project/delete_project/"+proj, nil); !resp.IsSuccess {
		t.Fatalf("软删除失败: %s", resp.ErrorMsg)
	}

	// get_project_by_name 不应再返回
	if resp := doJSON(t, r, http.MethodGet, "/api/project/get_project_by_name/"+proj, nil); resp.IsSuccess {
		t.Fatal("软删除项目不应再被 get_project_by_name 返回")
	}
	// get_all_files 不应再服务
	if resp := doJSON(t, r, http.MethodGet, "/api/file/get_all_files/"+proj, nil); resp.IsSuccess {
		t.Fatal("软删除项目不应再被 get_all_files 服务")
	}
	// get_release_manifest 不应再服务（N2）
	if resp := doJSON(t, r, http.MethodGet, "/api/file/get_release_manifest/"+proj, nil); resp.IsSuccess {
		t.Fatal("软删除项目不应再被 get_release_manifest 服务")
	}
	// 磁盘目录保留（软删=仅从 API 隐藏，便于恢复/审计）
	if _, err := os.Stat(dataDirOf(t, proj)); err != nil {
		t.Fatalf("软删除不应直接删盘: %v", err)
	}
}

// ────────────────────────────────────────────────────────────────
// B6 delete_file
// ────────────────────────────────────────────────────────────────

func TestDeleteFile(t *testing.T) {
	r := newRouter()
	const proj = "tc-b6-delete"
	publishedFile(t, r, proj, "keep.txt", []byte("keep"))
	target := publishedFile(t, r, proj, "sub/drop.txt", []byte("drop"))

	resp := doJSON(t, r, http.MethodPost, "/api/file/delete_file", map[string]string{
		"projectName":      proj,
		"relativeFileName": "sub/drop.txt",
	})
	if !resp.IsSuccess {
		t.Fatalf("删除文件失败: %s", resp.ErrorMsg)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("删除后文件仍存在")
	}
	// 幂等：再删一次仍成功
	resp = doJSON(t, r, http.MethodPost, "/api/file/delete_file", map[string]string{
		"projectName":      proj,
		"relativeFileName": "sub/drop.txt",
	})
	if !resp.IsSuccess {
		t.Fatalf("重复删除应幂等成功，实际: %s", resp.ErrorMsg)
	}
	// 其它文件不受影响
	if _, err := os.Stat(filepath.Join(dataDirOf(t, proj), "keep.txt")); err != nil {
		t.Fatalf("删除不应影响同项目其它文件: %v", err)
	}
	// 穿越防护
	resp = doJSON(t, r, http.MethodPost, "/api/file/delete_file", map[string]string{
		"projectName":      proj,
		"relativeFileName": "../../aly.db",
	})
	if resp.IsSuccess {
		t.Fatal("delete_file 应拒绝路径穿越")
	}
}

// ────────────────────────────────────────────────────────────────
// 轻微：changelog 条数上限
// ────────────────────────────────────────────────────────────────

func TestChangeLogsAreLimited(t *testing.T) {
	r := newRouter()
	const proj = "tc-changelog-limit"
	mustCreateProject(t, r, proj)

	// 直接插 250 条变更日志（绕过发布流程，只验证返回条数上限）
	ctx := context.Background()
	p, err := db.Client.Project.Query().Where(project.NameEQ(proj)).First(ctx)
	if err != nil {
		t.Fatalf("查询项目失败: %v", err)
	}
	for i := 0; i < 250; i++ {
		if _, err := db.Client.ProjectChangeLog.Create().
			SetProject(p).
			SetVersion(fmt.Sprintf("1.%d.0", i)).
			SetLogs([]string{"x"}).
			SetTime("2026-01-01 00:00:00").
			Save(ctx); err != nil {
			t.Fatalf("插入变更日志失败: %v", err)
		}
	}

	resp := doJSON(t, r, http.MethodGet, "/api/project/get_project_change_logs/"+proj, nil)
	if !resp.IsSuccess {
		t.Fatalf("查询变更日志失败: %s", resp.ErrorMsg)
	}
	arr, ok := resp.Data.([]interface{})
	if !ok {
		t.Fatalf("变更日志数据不是数组: %#v", resp.Data)
	}
	const maxRows = 200
	if len(arr) > maxRows {
		t.Fatalf("变更日志返回 %d 条，超过上限 %d", len(arr), maxRows)
	}
}

// ────────────────────────────────────────────────────────────────
// 工具
// ────────────────────────────────────────────────────────────────

func urlEscape(s string) string {
	return url.QueryEscape(s)
}
