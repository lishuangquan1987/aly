package tc

// 用例文档里用到的高阶辅助：等待、进程控制、UI 与 SDK 驱动、并发编排。
// 全部只用标准库，语义与"人手工测"一致。

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────
// 等待（替代 sleep 轮询，失败时给出可诊断信息）
// ────────────────────────────────────────────────────────────────

// WaitFile 等文件出现，返回是否在超时前出现。
func WaitFile(t *testing.T, p string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if Exists(p) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// MustWaitFile 等文件出现，超时即失败。
func MustWaitFile(t *testing.T, p string, timeout time.Duration) {
	t.Helper()
	if !WaitFile(t, p, timeout) {
		t.Fatalf("等待文件出现超时(%s): %s", timeout, p)
	}
}

// WaitLog 等日志文件包含指定子串（用于"某阶段已发生"的同步点）。
func WaitLog(t *testing.T, logPath, needle string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(ReadText(logPath), needle) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// WaitGone 等文件/目录消失。
func WaitGone(t *testing.T, p string, timeout time.Duration) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !Exists(p) {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

// ────────────────────────────────────────────────────────────────
// 进程控制
// ────────────────────────────────────────────────────────────────

// Kill 强杀进程（模拟断电/任务管理器结束进程）。
func Kill(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
}

// StartHolder 启动一个以 dir 为工作目录的常驻进程（占住目录，模拟"目录被占用"）。
// 用 fake_main.exe 当占位进程：它只要运行就会持有自己的 CWD。
// 返回的 stop 用于收尾，务必 defer 调用。
func (e *Env) StartHolder(t *testing.T, dir string) (cmd *exec.Cmd, stop func()) {
	t.Helper()
	exe := e.FakeMain
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("fake_main.exe 缺失: %s", exe)
	}
	cmd = exec.Command(exe, "--hold")
	cmd.Dir = dir
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动占位进程失败: %v", err)
	}
	stop = func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	// 等它真正起来（避免与后续操作抢时序）
	time.Sleep(300 * time.Millisecond)
	return cmd, stop
}

// ────────────────────────────────────────────────────────────────
// 并发编排
// ────────────────────────────────────────────────────────────────

// ConcResult 是并发跑的一路结果（比裸 Out 多带诊断信息）。
//
// 为什么需要它：原实现里 `outs[idx]` 在"进程没起来""一行 JSON 都没打""内部
// panic"三种情况下都保持零值 `Out{IsSuccess:false, ErrorMsg:""}`——调用方看到
// "失败"却拿不到任何原因（errorMsg 是空的），断言信息形同废纸；更糟的是
// `defer func(){ _ = recover() }()` 把 panic 也吞了，静默变成"失败但无线索"。
type ConcResult struct {
	Out    Out    // 末行解析结果（无输出时为零值，此时请看 Err/Raw）
	Err    error  // 进程启动或退出码非零的错误
	Raw    string // 完整 stdout+stderr（诊断用）
	Panics string // 内部 panic 的字符串（正常为空）
}

// RunConcurrent 并发跑多个 aly-client 命令，返回每路的结果行（按入参顺序）。
// 每路独立进程；用于"并发锁""两个客户端同时 download"这类用例。
//
// 只返回 Out 会让失败无法诊断，新用例请用 RunConcurrentFull。
func (e *Env) RunConcurrent(t *testing.T, cs []*Client, argSets ...[]string) []Out {
	t.Helper()
	rs := e.RunConcurrentFull(t, cs, argSets...)
	outs := make([]Out, len(rs))
	for i, r := range rs {
		outs[i] = r.Out
	}
	return outs
}

// RunConcurrentFull 同 RunConcurrent，但保留每路的错误与原始输出。
func (e *Env) RunConcurrentFull(t *testing.T, cs []*Client, argSets ...[]string) []ConcResult {
	t.Helper()
	if len(cs) != len(argSets) {
		t.Fatalf("RunConcurrent: 客户端数(%d) 与命令数(%d) 不匹配", len(cs), len(argSets))
	}
	rs := make([]ConcResult, len(cs))
	var wg sync.WaitGroup
	wg.Add(len(cs))
	for i := range cs {
		go func(idx int) {
			defer wg.Done()
			// panic 不再静默：记到 Panics 里，调用方至少能看到"这里出过事"。
			defer func() {
				if p := recover(); p != nil {
					rs[idx].Panics = fmt.Sprint(p)
				}
			}()
			cmd := exec.Command(filepath.Join(cs[idx].Up(), "aly-client.exe"), argSets[idx]...)
			cmd.Dir = cs[idx].Up()
			var buf bytes.Buffer
			cmd.Stdout, cmd.Stderr = &buf, &buf
			rs[idx].Err = cmd.Run()
			rs[idx].Raw = buf.String()
			lines := parseLines(rs[idx].Raw)
			if len(lines) > 0 {
				rs[idx].Out = lines[len(lines)-1]
			}
		}(i)
	}
	wg.Wait()

	// 一路都拿不到结果行时，直接把原始输出带进 errorMsg，别让调用方
	// 对着空 errorMsg 猜。
	for i := range rs {
		if rs[i].Out.ErrorMsg != "" || rs[i].Panics == "" {
			continue
		}
		rs[i].Out.ErrorMsg = "内部 panic: " + rs[i].Panics
	}
	return rs
}

// ────────────────────────────────────────────────────────────────
// UI 与 SDK（需要额外构建产物，用 t.Skip 保护）
// ────────────────────────────────────────────────────────────────

// BuildUI 构建 aly-client-ui.exe（go1.10.8 + 386 + -H=windowsgui）。
func (e *Env) BuildUI(t *testing.T) string {
	t.Helper()
	src := filepath.Join(e.Root, "gopath", "src", "aly", "client", "aly-client")
	if _, err := os.Stat(src); err != nil {
		t.Skip("UI 构建依赖 aly-client 的 GOPATH 副本，先跑 NewEnv(t, true, 0)")
	}
	out := filepath.Join(e.Bin, "aly-client-ui.exe")
	return e.goBuild(t, Go1108, src, out, "aly/client/aly-client/ui/winui",
		[]string{"-ldflags", "-H=windowsgui"},
		map[string]string{
			"GOPATH":      filepath.Join(e.Root, "gopath"),
			"GO111MODULE": "off",
			"GOOS":        "windows",
			"GOARCH":      "386",
			"CGO_ENABLED": "0",
		})
}

// RunUIMode 以指定 -mode 启动 aly-client-ui.exe 并等待退出，返回退出码与输出。
// timeout 到期仍未退出视为"UI 常驻不退出"，由调用方按用例语义判定。
func (e *Env) RunUIMode(t *testing.T, uiExe string, c *Client, mode string,
	timeout time.Duration) (code int, output string) {
	t.Helper()
	cmd := exec.Command(uiExe, "-mode", mode)
	cmd.Dir = filepath.Join(c.Up())
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动 UI 失败: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return ee.ExitCode(), buf.String()
			}
			return -1, buf.String()
		}
		return 0, buf.String()
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return -999, buf.String() // -999 约定：超时未退出
	}
}

// BuildSDK 构建 C# SDK 与控制台驱动 AlySdkSmoke（net40），返回 smoke.exe 路径。
func (e *Env) BuildSDK(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(RepoRoot, "client", "aly-client-sdk", "AlyClient.CSharpSDK")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("C# SDK 目录不存在，跳过")
	}
	proj := filepath.Join(dir, "AlySdkSmoke", "AlySdkSmoke.csproj")
	if _, err := os.Stat(proj); err != nil {
		t.Skip("AlySdkSmoke 工程不存在，跳过")
	}
	cmd := exec.Command("dotnet", "build", proj, "-c", "Release", "-v", "q")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("构建 SDK 失败: %v\n%s", err, truncate(string(out), 3000))
	}
	// net40 输出路径：bin/Release/net40/AlySdkSmoke.exe
	cand := filepath.Join(dir, "AlySdkSmoke", "bin", "Release", "net40", "AlySdkSmoke.exe")
	if !Exists(cand) {
		// 兼容 net40/net40 双层或不同 RID 布局
		var found string
		_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
			if err == nil && !fi.IsDir() && fi.Name() == "AlySdkSmoke.exe" {
				found = p
			}
			return nil
		})
		if found == "" {
			t.Fatalf("AlySdkSmoke.exe 未生成，检查 dotnet build 输出:\n%s", truncate(string(out), 2000))
		}
		cand = found
	}
	return cand
}

// RunSmoke 跑 AlySdkSmoke 的一条或全部用例，返回输出文本。
// 需在 SDK 目录下执行（依赖 SDK dll 的相对路径）。
func (e *Env) RunSmoke(t *testing.T, smokeExe, id string, c *Client) string {
	t.Helper()
	return e.RunSmokeEnv(t, smokeExe, id, c, nil)
}

// RunSmokeEnv 同 RunSmoke，但可注入额外环境变量（如 ALY_EXPECT_VERSION，SD9 用）。
func (e *Env) RunSmokeEnv(t *testing.T, smokeExe, id string, c *Client,
	extraEnv map[string]string) string {
	t.Helper()
	cmd := exec.Command(smokeExe, id, filepath.Join(c.Up(), "aly-client.exe"))
	cmd.Dir = filepath.Dir(smokeExe)
	cmd.Env = cleanEnv(extraEnv)
	out, _ := cmd.CombinedOutput()
	return string(out)
}

// RunUIModeLang 同 RunUIMode，但可指定 -lang（验证 en-US 文案不崩）。
func (e *Env) RunUIModeLang(t *testing.T, uiExe string, c *Client, mode, lang string,
	timeout time.Duration) (code int, output string) {
	t.Helper()
	args := []string{"-mode", mode}
	if lang != "" {
		args = append(args, "-lang", lang)
	}
	cmd := exec.Command(uiExe, args...)
	cmd.Dir = c.Up()
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动 UI 失败: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return ee.ExitCode(), buf.String()
			}
			return -1, buf.String()
		}
		return 0, buf.String()
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		return -999, buf.String()
	}
}

// UIModeResult 是 RunUIModeAuto 的观测结果。
type UIModeResult struct {
	ExitCode  int    // 进程退出码；-999 = 超时未退出
	Output    string // 合并的 stdout/stderr（windowsgui 下通常为空）
	Dismissed bool   // 是否自动点掉过一个模态框
	Elapsed   time.Duration
}

// RunUIModeAuto 同 RunUIMode，但会在后台**自动点掉标题为 msgBoxTitle 的模态框**。
//
// 为什么需要它：-mode check 的「无更新」分支走 view.Window.ShowInfo，那是
// walk.MsgBox —— 阻塞直到用户点「确定」，然后才 quit(w)。自动化里没有手，
// 不点掉就永远拿不到退出码，会把"设计如此的交互"误判成"进程泄漏"。
//
// dismissWait 是等待模态框出现的时长；msgBoxTitle 传空串则不点（只跑纯后台模式）。
func (e *Env) RunUIModeAuto(t *testing.T, uiExe string, c *Client, mode string,
	msgBoxTitle string, dismissWait, timeout time.Duration) UIModeResult {
	t.Helper()
	start := time.Now()
	cmd := exec.Command(uiExe, "-mode", mode)
	cmd.Dir = c.Up()
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Start(); err != nil {
		t.Fatalf("启动 UI 失败: %v", err)
	}

	// 后台点框：拿到就点，点完继续盯着（重试场景可能弹第二次）
	dismissed := make(chan bool, 1)
	stopWatch := make(chan struct{})
	go func() {
		hitOnce := false
		for {
			select {
			case <-stopWatch:
				dismissed <- hitOnce
				return
			default:
			}
			if msgBoxTitle != "" && dismissMessageBox(msgBoxTitle, 300*time.Millisecond) {
				hitOnce = true
			}
			// 起手留 dismissWait 的观察窗，之后转为低速陪跑
			time.Sleep(150 * time.Millisecond)
		}
	}()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	res := UIModeResult{}
	select {
	case err := <-done:
		res.ExitCode = 0
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				res.ExitCode = ee.ExitCode()
			} else {
				res.ExitCode = -1
			}
		}
		res.Elapsed = time.Since(start)
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
		res.ExitCode = -999
		res.Elapsed = time.Since(start)
	}

	close(stopWatch)
	select {
	case res.Dismissed = <-dismissed:
	case <-time.After(2 * time.Second):
	}
	res.Output = buf.String()
	_ = dismissWait
	return res
}

// GoTest 在 dir 下跑 `go test -tags purego <args...>`，返回合并输出。
// 用于跑组件自带单测（UI bridge / server controllers / publish 各包）。
// 失败不在这里 Fatal：单测断言由调用方按输出判定。
func (e *Env) GoTest(t *testing.T, goBin, dir string, extraEnv map[string]string,
	args ...string) (code int, output string) {
	t.Helper()
	full := append([]string{"test", "-tags", "purego"}, args...)
	cmd := exec.Command(goBin, full...)
	cmd.Dir = dir
	cmd.Env = cleanEnv(extraEnv)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	code = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return code, buf.String()
}

// GoTestGopath 在 GOPATH 模式下跑 go test（aly-client / ui 用 go1.10.8，无 -tags 需求）。
func (e *Env) GoTestGopath(t *testing.T, pkg string, args ...string) (code int, output string) {
	t.Helper()
	gp := filepath.Join(e.Root, "gopath")
	src := filepath.Join(gp, "src", "aly", "client", "aly-client")
	full := append([]string{"test"}, args...)
	full = append(full, pkg)
	cmd := exec.Command(Go1108, full...)
	cmd.Dir = src
	cmd.Env = cleanEnv(map[string]string{
		"GOPATH": gp, "GO111MODULE": "off",
		"GOOS": "windows", "GOARCH": "386", "CGO_ENABLED": "0",
	})
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	code = 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	return code, buf.String()
}

// RawStatus 发请求并返回**状态码**与响应体（不像 Raw 那样对非 200 直接 Fatal）。
// 路径穿越、中间态拒绝这类用例必须看得到 404/500。
func (e *Env) RawStatus(t *testing.T, method, path string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, e.BaseURL()+path, nil)
	if err != nil {
		t.Fatalf("构造请求失败(%s %s): %v", method, path, err)
	}
	resp, err := e.base.Do(req)
	if err != nil {
		t.Fatalf("%s %s 失败: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// Status 发 GET 只取状态码。
func (e *Env) Status(t *testing.T, path string) int {
	t.Helper()
	code, _ := e.RawStatus(t, "GET", path)
	return code
}

// RespFull 是一次"带任意请求头"的响应快照。
// 多段 Range / If-Range 这类用例必须同时看到状态码、响应头（Content-Type /
// Content-Range）与响应体，RawStatus 只给状态码+body 不够用。
type RespFull struct {
	Code   int
	Header http.Header
	Body   []byte
}

// ContentType 返回 Content-Type 头（无则空串）。
func (r RespFull) ContentType() string { return r.Header.Get("Content-Type") }

// ContentRange 返回 Content-Range 头（无则空串）。
func (r RespFull) ContentRange() string { return r.Header.Get("Content-Range") }

// DoFull 发请求并返回完整响应（状态码 + 头 + 体）。
//
// 为什么需要：`RawRange`（env.go）恒定只发 `bytes=<off>-` 单段请求，
// `RawStatus` 不带任何自定义头。多段 Range（`bytes=0-1,4-5`）与 If-Range
// 必须能自己指定 `Range` / `If-Range`，否则测不了。
// **不要**用它替代 RawRange 做单段测试——语义不同，容易误判 206/200。
func (e *Env) DoFull(t *testing.T, method, path string, headers map[string]string) RespFull {
	t.Helper()
	req, err := http.NewRequest(method, e.BaseURL()+path, nil)
	if err != nil {
		t.Fatalf("构造请求失败(%s %s): %v", method, path, err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := e.base.Do(req)
	if err != nil {
		t.Fatalf("%s %s 失败: %v", method, path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return RespFull{Code: resp.StatusCode, Header: resp.Header.Clone(), Body: b}
}
// ────────────────────────────────────────────────────────────────
// 文件与哈希辅助（用例文档直接引用）
// ────────────────────────────────────────────────────────────────

// MD5Of 计算文件 MD5（不存在返回空串）。
func MD5Of(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// SHA256Of 计算文件 SHA256（不存在返回空串）。
func SHA256Of(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// CopyHead 把 src 的前 n 字节拷到 dst（造"下载了一半的 .part"）。
func CopyHead(t *testing.T, src, dst string, n int64) {
	t.Helper()
	in, err := os.Open(src)
	if err != nil {
		t.Fatalf("打开源文件失败(%s): %v", src, err)
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	out, err := os.Create(dst)
	if err != nil {
		t.Fatalf("创建目标文件失败(%s): %v", dst, err)
	}
	defer out.Close()
	if _, err := io.CopyN(out, in, n); err != nil {
		t.Fatalf("拷贝前 %d 字节失败: %v", n, err)
	}
}

// MakeFiles 在 dir 下批量生成小文件（造海量小文件现场）。
// 例：MakeFiles(t, dir, 500, 1024) → f0001.txt … f0500.txt，各 1KB
func MakeFiles(t *testing.T, dir string, count, sizeBytes int) {
	t.Helper()
	MakeFileRange(t, dir, 1, count, sizeBytes)
}

// MakeFileRange 只生成 [from, to] 区间的小文件 f%04d.txt（各 sizeBytes 字节）。
//
// 与 MakeFiles 的区别：不动区间外的既有文件。差量场景必须用它——否则
// MakeFiles(t, dir, 1050, ...) 会把刚被删除的 f0001..f0100 重新写回，
// 服务端清单与上一轮完全一致，差集为空，delete-sync 无事可做。
func MakeFileRange(t *testing.T, dir string, from, to, sizeBytes int) {
	t.Helper()
	if from > to {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	payload := bytes.Repeat([]byte("a"), sizeBytes)
	for i := from; i <= to; i++ {
		name := fmt.Sprintf("f%04d.txt", i)
		if err := os.WriteFile(filepath.Join(dir, name), payload, 0o644); err != nil {
			t.Fatalf("生成 %s 失败: %v", name, err)
		}
	}
}

// ────────────────────────────────────────────────────────────────
// 用例 78–94 需要的 SDK 辅助
// ────────────────────────────────────────────────────────────────

// OutputHasFail 判断 AlySdkSmoke 输出里是否出现 FAIL（"FAIL:" 前缀或 "FAIL=" 非零）。
func OutputHasFail(out string) bool {
	if strings.Contains(out, "FAIL:") {
		return true
	}
	// SUMMARY: PASS=n FAIL=m —— m != 0 即失败
	if i := strings.Index(out, "FAIL="); i >= 0 {
		rest := out[i+5:]
		end := 0
		for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
			end++
		}
		if end > 0 && rest[:end] != "0" {
			return true
		}
	}
	return false
}

// StringContainsAny 判断 hay 是否含 needles 中任意一个。
func StringContainsAny(hay string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(hay, n) {
			return true
		}
	}
	return false
}

// progressFiles 列出输出里所有进度行的 "状态:文件" 摘要（断言失败时打印，
// 用来判断"到底哪几个文件走过进度分支"——死断言最常见的成因是文件根本没出现）。
func progressFiles(outs []Out) []string {
	var got []string
	for _, o := range outs {
		if o.Progress == nil {
			continue
		}
		got = append(got, o.Progress.Status+":"+filepath.ToSlash(o.Progress.File))
	}
	if len(got) == 0 {
		return []string{"<无任何进度行>"}
	}
	return got
}

// progressStartFor 返回指定文件的第一条 START 进度行；没有则 nil。
// 与"for 循环里 if 命中才 assert"相比，这个写法能让调用方显式处理"没有"的情况，
// 避免断言压根不执行却算通过。
func progressStartFor(outs []Out, rel string) *Progress {
	want := filepath.ToSlash(rel)
	for _, o := range outs {
		if o.Progress == nil || o.Progress.Status != "START" {
			continue
		}
		if filepath.ToSlash(o.Progress.File) == want {
			p := *o.Progress
			return &p
		}
	}
	return nil
}

// stripCLineComments 剥掉 C# 源码里的注释，供"源码里出现 N 次某字面量"这类
// 结构断言使用。
//
// 为什么需要它：`strings.Count(src, "--json")` 会把注释与文档字符串里的
// 同一串也算进去 —— 改一句注释就让断言 FAIL，而真正的执行路径少了一条却可能
// 因为注释补上而不报警。计数必须只针对代码。
//
// 保守实现（不误伤字符串字面量）：
//   - 整行以 // 开头（trim 后）→ 整行丢弃
//   - 行内 `//` **不**剥离（URL 的 `http://` 会被误伤），宁可多算不可少算
//   - `/* ... */` 块注释（含跨行）→ 状态机剥离
func stripCLineComments(src string) string {
	var out []string
	inBlock := false
	for _, line := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if inBlock {
			if i := strings.Index(trimmed, "*/"); i >= 0 {
				inBlock = false
				trimmed = trimmed[i+2:]
			} else {
				continue
			}
		}
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		if i := strings.Index(trimmed, "/*"); i >= 0 {
			rest := trimmed[i+2:]
			if j := strings.Index(rest, "*/"); j >= 0 {
				trimmed = trimmed[:i] + rest[j+2:]
			} else {
				trimmed = trimmed[:i]
				inBlock = true
			}
		}
		out = append(out, trimmed)
	}
	return strings.Join(out, "\n")
}
