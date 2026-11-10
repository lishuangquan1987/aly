package tc

// 测试框架的核心设施：环境（临时目录 / 服务端进程 / 客户端现场 / publish 工作区）、
// HTTP API 封装、aly-client 与 aly-publish 调用、断言。
//
// 设计约束
//  1. **只用标准库**：任何装了 Go 的机器可直接 go run / go test，不引第三方依赖。
//  2. **每条用例独立**：前置现场由 Env 显式构造，不依赖前一条用例的遗留状态。
//  3. **失败可诊断**：断言失败带"期望 / 实际 / 现场路径"，日志尾部自动附上。
//  4. **工具链口径显式**：server / publish-cli 用 go1.26（go.mod 声明 go1.25）；
//     aly-client 必须 go1.10.8 + GOARCH=386（XP 兼容），见 RepoRoot 与 Go1xx 常量。

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// ────────────────────────────────────────────────────────────────
// 路径常量（执行前按本机实际情况核对，或用环境变量覆盖）
// ────────────────────────────────────────────────────────────────

// RepoRoot 仓库根目录。默认从本文件位置向上推导：<root>/test/tc/env.go
var RepoRoot = func() string {
	_, file, _, _ := runtime.Caller(0) // .../aly/test/tc/env.go
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}()

// Go126 构建 server / publish-cli（go.mod 声明 go 1.25.0，故需 >= 1.25 的工具链）
var Go126 = envOr("ALY_GO126", defaultGo126())

// Go1108 构建 aly-client / aly-client-ui / fake_main（XP 兼容口径：1.10.8 + GOARCH=386）
var Go1108 = envOr("ALY_GO1108", defaultGo1108())

// defaultGo126 返回现代 Go 的默认路径：Windows 用作者机器的安装位置；
// 其他平台回退到 PATH 里的 go（go.mod 声明 1.25+，一般 PATH 即满足）。
func defaultGo126() string {
	if runtime.GOOS == "windows" {
		return `D:\Install\go\bin\go.exe`
	}
	return "go"
}

// defaultGo1108 返回 go1.10.8 的默认路径：Windows 用作者机器的安装位置；
// 其他平台按 golang.org/dl 多版本工具链的官方约定 ~/sdk/go1.10.8/bin/go
// （go install golang.org/dl/go1.10.8@latest && go1.10.8 download 落位处）。
func defaultGo1108() string {
	if runtime.GOOS == "windows" {
		return `C:\Users\5001494\sdk\go1.10.8\bin\go.exe`
	}
	return filepath.Join(os.Getenv("HOME"), "sdk", "go1.10.8", "bin", "go")
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ────────────────────────────────────────────────────────────────
// 断言
// ────────────────────────────────────────────────────────────────

// Fatalf 断言失败：统一的失败格式，测试输出里一眼可辨。
func Fatalf(t *testing.T, format string, a ...interface{}) {
	t.Helper()
	t.Fatalf(format, a...)
}

// Eq 断言相等。
//
// 类型敏感：`fmt.Sprint` 会把 5 与 "5" 都渲染成 "5"，产品把 int 字段改成
// 字符串（或反之）时断言会静默通过。这里对整数/字符串做分类比较：
//   - 两侧都是整数（任意宽度、含无符号）→ 按数值比，跨宽度视为相等（5 == int64(5)）
//   - 一侧整数、另一侧字符串 → 直接失败（类型不同就是回归）
//   - 其余（bool / nil / 切片 / 结构体）→ 回落 fmt.Sprint
func Eq(t *testing.T, field string, got, want interface{}) {
	t.Helper()
	if eqValue(got, want) {
		return
	}
	t.Fatalf("%s: 期望 %v(%T)，实际 %v(%T)", field, want, want, got, got)
}

// eqValue 按类型分类比较，语义见 Eq 注释。
func eqValue(got, want interface{}) bool {
	gi, gok := toInt64(got)
	wi, wok := toInt64(want)
	if gok && wok {
		return gi == wi
	}
	// 一侧整数、另一侧不是：类型不匹配即视为不等（哪怕是 "5" vs 5）
	if gok != wok {
		return false
	}
	// 同为字符串（或同为非整数）：Sprint 足够
	return fmt.Sprint(got) == fmt.Sprint(want)
}

// toInt64 把任意整数类型收敛成 int64；非整数返回 ok=false。
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		return int64(n), true
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		return int64(n), true
	}
	return 0, false
}

// True 断言为真。
func True(t *testing.T, cond bool, format string, a ...interface{}) {
	t.Helper()
	if !cond {
		t.Fatalf(format, a...)
	}
}

// Contains 断言字符串包含。
func Contains(t *testing.T, hay, needle, field string) {
	t.Helper()
	if !strings.Contains(hay, needle) {
		t.Fatalf("%s: 期望包含 %q，实际 %q", field, needle, truncate(hay, 600))
	}
}

// NotContains 断言字符串不包含。
func NotContains(t *testing.T, hay, needle, field string) {
	t.Helper()
	if strings.Contains(hay, needle) {
		t.Fatalf("%s: 期望不包含 %q，实际 %q", field, needle, truncate(hay, 600))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ────────────────────────────────────────────────────────────────
// Env：一次测试会话的全部现场
// ────────────────────────────────────────────────────────────────

type Env struct {
	Root       string // 临时根目录（%TEMP%/aly-tc-xxxx）
	Bin        string // 构建产物：server.exe / aly-publish.exe / aly-client.exe / fake_main.exe
	Port       int
	base       *http.Client
	srv        *exec.Cmd
	srvLog     *os.File
	srvLogPath string // 服务端日志路径（启动失败时打尾部用）

	ServerExe  string
	PublishExe string
	ClientExe  string
	FakeMain   string

	seq int // 现场编号（避免同用例多次建 client 撞目录）
}

// NewEnv 建环境。build=true 时构建全部产物；reusePort>0 时复用端口
//（客户端 shared.json 已固化 URL 的场景，如断网恢复）。
func NewEnv(t *testing.T, build bool, reusePort int) *Env {
	t.Helper()
	root, err := os.MkdirTemp("", "aly-tc-")
	if err != nil {
		t.Fatalf("建临时目录失败: %v", err)
	}
	e := &Env{
		Root: root,
		Bin:  filepath.Join(root, "bin"),
		base: &http.Client{Timeout: 120 * time.Second},
	}
	if err := os.MkdirAll(e.Bin, 0o755); err != nil {
		t.Fatalf("建 bin 目录失败: %v", err)
	}
	if build {
		e.BuildAll(t)
	}
	e.StartServer(t, reusePort)
	t.Cleanup(func() {
		e.StopServer()
		// 复用端口的场景（reusePort>0）不是本进程分配的，别回收别人的。
		if reusePort == 0 {
			releasePort(e.Port)
		}
		// 保留现场便于排查；设 ALY_TC_KEEP=1 时不清理
		if os.Getenv("ALY_TC_KEEP") == "" {
			os.RemoveAll(root)
		} else {
			t.Logf("现场保留: %s", root)
		}
	})
	return e
}

// ── 构建 ──────────────────────────────────────────────────────

// BuildAll 构建 server / publish-cli / aly-client / fake_main。缺一即失败。
//
// 例外：设置 ALY_REUSE_BIN=<目录> 时，该目录下已存在的 server.exe /
// aly-publish.exe / aly-client.exe / fake_main.exe 直接复用，不再从源码构建。
// 用途只有一个——某些受限环境里 go 派生 compile.exe 的进程链被安全软件拦截
//（本机 publish-cli 即如此），而人工执行构建脚本是通的：此时把人工产出的
// 二进制放进来复用，用例语义完全不变（被测对象仍是同一份源码的真实产物）。
// 单条产物仍缺失时，回落到对应源码构建。
func (e *Env) BuildAll(t *testing.T) {
	t.Helper()

	reuseDir := os.Getenv("ALY_REUSE_BIN")
	pick := func(exeName string, build func() string) string {
		if reuseDir != "" {
			p := filepath.Join(reuseDir, exeName)
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				dst := filepath.Join(e.Bin, exeName)
				CopyFile(t, p, dst)
				t.Logf("复用已构建产物: %s", p)
				return dst
			}
			t.Logf("复用目录缺少 %s，回落源码构建", exeName)
		}
		return build()
	}

	e.ServerExe = pick("server.exe", func() string {
		return e.goBuild(t, Go126, filepath.Join(RepoRoot, "server"),
			filepath.Join(e.Bin, "server.exe"), ".", nil, nil)
	})

	// publish-cli：依赖 crypto/tls → chacha20poly1305 有 amd64 汇编，
	// 本机 security 软件环境下会触发 compile.exe 的 fork 拒绝，
	// 加 -tags purego 走纯 Go 实现（2026-11-01 实测有效）。
	e.PublishExe = pick("aly-publish.exe", func() string {
		return e.goBuild(t, Go126, filepath.Join(RepoRoot, "publish", "publish-cli"),
			filepath.Join(e.Bin, "aly-publish.exe"), ".",
			[]string{"-tags", "purego"}, nil)
	})

	// aly-client：go1.10.8 + GOPATH 模式 + GOARCH=386（XP 兼容）
	gp := filepath.Join(e.Root, "gopath")
	srcDst := filepath.Join(gp, "src", "aly", "client", "aly-client")
	if _, err := os.Stat(srcDst); err != nil {
		if err := copyTree(filepath.Join(RepoRoot, "client", "aly-client"), srcDst); err != nil {
			t.Fatalf("复制 aly-client 源码到 GOPATH 失败: %v", err)
		}
	}
	// UI 已迁出 aly-client：winui 源码树单独复制到 GOPATH，供 BuildUI 构建。
	uiSrcDst := filepath.Join(gp, "src", "aly", "client", "aly-client-ui", "aly-client-winui")
	if _, err := os.Stat(uiSrcDst); err != nil {
		if err := copyTree(filepath.Join(RepoRoot, "client", "aly-client-ui", "aly-client-winui"), uiSrcDst); err != nil {
			t.Fatalf("复制 aly-client-winui 源码到 GOPATH 失败: %v", err)
		}
	}
	xgo := map[string]string{
		"GOPATH":      gp,
		"GO111MODULE": "off",
		"GOOS":        "windows",
		"GOARCH":      "386",
		"CGO_ENABLED": "0",
	}
	e.ClientExe = pick("aly-client.exe", func() string {
		return e.goBuild(t, Go1108, srcDst, filepath.Join(e.Bin, "aly-client.exe"),
			"aly/client/aly-client", nil, xgo)
	})

	// fake_main：被更新的主程序替身（386）
	e.FakeMain = pick("fake_main.exe", func() string {
		return e.goBuild(t, Go1108,
			filepath.Join(RepoRoot, "client", "aly-client", "test", "e2e", "fake_main"),
			filepath.Join(e.Bin, "fake_main.exe"), ".", nil, xgo)
	})

	t.Logf("构建完成: %s", e.Bin)
}

// goBuild 统一构建入口。pkg 为 go build 的包路径；gopathPkg 非空时走 GOPATH 模式。
func (e *Env) goBuild(t *testing.T, goBin, srcDir, out, pkg string,
	tags []string, extraEnv map[string]string) string {
	t.Helper()
	if _, err := os.Stat(goBin); err != nil {
		t.Fatalf("Go 工具链缺失: %s（用 ALY_GO126 / ALY_GO1108 环境变量覆盖）", goBin)
	}
	args := []string{"build"}
	args = append(args, tags...)
	args = append(args, "-o", out, pkg)

	cmd := exec.Command(goBin, args...)
	cmd.Dir = srcDir
	cmd.Env = cleanEnv(extraEnv)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	if err := cmd.Run(); err != nil {
		t.Fatalf("构建失败: %s\n命令: %s %s\n输出:\n%s",
			srcDir, goBin, strings.Join(args, " "), truncate(buf.String(), 4000))
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("构建产物缺失: %s", out)
	}
	return out
}

// cleanEnv 继承当前环境，但剔除会干扰子进程的变量：
//   - 代理变量（注册表可能存在大小写两份，子进程环境块构建会冲突）
//   - Go 相关变量（避免继承到错误的 GOOS/GOARCH/GOPATH）
func cleanEnv(extra map[string]string) []string {
	drop := map[string]bool{
		"HTTP_PROXY": true, "HTTPS_PROXY": true, "NO_PROXY": true,
		"http_proxy": true, "https_proxy": true, "no_proxy": true,
		"GOPATH": true, "GO111MODULE": true, "GOOS": true, "GOARCH": true,
		"CGO_ENABLED": true, "GOFLAGS": true,
	}
	var out []string
	for _, kv := range os.Environ() {
		k := kv
		if i := strings.IndexByte(kv, '='); i > 0 {
			k = kv[:i]
		}
		if drop[k] {
			continue
		}
		out = append(out, kv)
	}
	for k, v := range extra {
		out = append(out, k+"="+v)
	}
	return out
}

// copyTree 递归复制目录（跳过 *.exe 与 vendor 由调用方按需处理）。
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if strings.HasSuffix(strings.ToLower(fi.Name()), ".exe") {
			return nil // 产物不入源码树
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, fi.Mode())
	})
}

// ── 服务端 ────────────────────────────────────────────────────

// StartServer 启动 server.exe 并等待就绪。reusePort>0 时复用指定端口
//（客户端 shared.json 已固化 URL 的断网恢复场景必须复用，否则断言必然失败）。
//
// ⚠ 端口竞争防护（跨进程）：
// freePort() 用 "Listen 试探 → Close → 再让 server 去 Listen" 的方式选端口，
// 这个空窗期里**另一个 go test 进程**可能抢到同一端口。此时 server.exe 启动失败，
// 而就绪探测会命中**别人的 server**（状态码同为 200），于是本进程连到错误的
// 数据库——表现为"清单莫名返回 0 项"这种极难排查的偶发失败。
//
// 因此这里做两件事：
//  1. 就绪判据要求 "子进程仍存活 + 接口 200"：端口被别人占住时本进程 server.exe
//     会绑定失败并立即退出，从而进入换端口重试，而不是连到别人的实例；
//  2. 失败时自动换端口重试（最多 3 次），不再干等 30s 超时后直接 Fatal。
func (e *Env) StartServer(t *testing.T, reusePort int) {
	t.Helper()
	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		failedPort := e.startServerOnce(t, reusePort)
		if failedPort == 0 {
			return // 成功
		}
		if reusePort > 0 {
			t.Fatalf("服务端启动失败且指定了复用端口 %d（不可换端口重试）", reusePort)
		}
		t.Logf("端口 %d 启动失败（可能被其他进程抢占），换端口重试（第 %d/%d 次）",
			failedPort, attempt, maxAttempts)
		releasePort(failedPort)
		e.Port = 0
	}
	t.Fatalf("服务端启动失败：连续 %d 次端口被抢占", maxAttempts)
}

// startServerOnce 启动一次。成功返回 0；失败返回**本次用掉的端口号**（供调用方换端口重试）。
func (e *Env) startServerOnce(t *testing.T, reusePort int) int {
	t.Helper()
	e.Port = reusePort
	if e.Port == 0 {
		e.Port = freePort(t)
	}
	logPath := filepath.Join(e.Root, "server.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatalf("打开 server.log 失败: %v", err)
	}
	cmd := exec.Command(e.ServerExe, "-p", fmt.Sprint(e.Port),
		"-db", filepath.Join(e.Root, "e2e.db"))
	cmd.Dir = e.Root
	cmd.Stdout, cmd.Stderr = f, f
	if err := cmd.Start(); err != nil {
		f.Close()
		t.Fatalf("启动 server 失败: %v", err)
	}

	// 就绪判据：**子进程活着** + 接口 200。
	//
	// 为什么"子进程活着"就够区分自己与别人：端口被别的 server 占住时，
	// 本进程的 server.exe 会 Listen 失败并**立即退出**，于是下面 select 的
	// exited 分支先命中 → 换端口重试。只有本进程的 server 顺利绑上端口，
	// 才会持续存活并响应 200。
	//
	// （不要用 "Listen 探测端口是否空闲" 做判据：server 绑 0.0.0.0，
	// Windows 允许 127.0.0.1 与 0.0.0.0 同端口并存，探测结论不可靠。）
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case werr := <-exited:
			f.Close()
			t.Logf("server 启动后立即退出（端口 %d，多半被其他进程占用）: %v\n日志尾部:\n%s",
				e.Port, werr, readTail(logPath, 800))
			return e.Port
		default:
		}

		if resp, err := e.base.Get(e.BaseURL() + "/api/server/info"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				e.srv = cmd
				e.srvLog = f
				e.srvLogPath = logPath
				t.Logf("服务端就绪 :%d（db=%s）", e.Port, filepath.Join(e.Root, "e2e.db"))
				return 0
			}
		}
		time.Sleep(200 * time.Millisecond)
	}

	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	f.Close()
	t.Logf("服务端启动超时（端口 %d），日志见 %s\n尾部:\n%s",
		e.Port, logPath, readTail(logPath, 800))
	return e.Port
}

// readTail 读文件末尾 n 字节（诊断用，失败返回空串）。
func readTail(path string, n int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return string(b)
}

// StopServer 强制结束服务端（kill 语义：不等待优雅退出，等价现场断电）。
func (e *Env) StopServer() {
	if e.srv != nil && e.srv.Process != nil {
		_ = e.srv.Process.Kill()
		_, _ = e.srv.Process.Wait()
		e.srv = nil
	}
	if e.srvLog != nil {
		e.srvLog.Close()
		e.srvLog = nil
	}
}

// BaseURL 服务端根地址。
func (e *Env) BaseURL() string { return fmt.Sprintf("http://127.0.0.1:%d", e.Port) }

// portMu / usedPorts 保证同一测试进程内不复用端口。
//
// freePort 的原始实现是 "Listen 探到空闲 → Close → 返回"，Close 到真正被
// server.exe 绑上之间有窗口；`go test -parallel` 或多个 Env 并发建时两个用例
// 可能拿到同一端口，表现为随机的"服务端启动超时"，或者更糟——两个用例共用一个
// SQLite 库互相干扰。进程内用集合去重即可消除这类自伤。
var (
	portMu    sync.Mutex
	usedPorts = map[int]bool{}
)

func freePort(t *testing.T) int {
	t.Helper()
	portMu.Lock()
	defer portMu.Unlock()
	for p := 23000; p < 32000; p++ {
		if usedPorts[p] {
			continue
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			ln.Close()
			usedPorts[p] = true
			return p
		}
	}
	t.Fatal("23000-32000 无空闲端口")
	return 0
}

// releasePort 归还端口（Env 清理时调用，便于同进程内反复建 Env 的用例）。
func releasePort(p int) {
	if p <= 0 {
		return
	}
	portMu.Lock()
	delete(usedPorts, p)
	portMu.Unlock()
}

// ────────────────────────────────────────────────────────────────
// HTTP API 封装（对应 server/routers/routers.go）
// ────────────────────────────────────────────────────────────────

// Resp 服务端统一响应体。
type Resp struct {
	IsSuccess bool            `json:"isSuccess"`
	ErrorMsg  string          `json:"errorMsg"`
	Data      json.RawMessage `json:"data"`
}

// Get 发 GET 并按统一响应解析。
func (e *Env) Get(t *testing.T, path string) Resp {
	t.Helper()
	resp, err := e.base.Get(e.BaseURL() + path)
	if err != nil {
		t.Fatalf("GET %s 失败: %v", path, err)
	}
	defer resp.Body.Close()
	return decodeResp(t, resp.Body, path)
}

// Post 发 POST JSON 并按统一响应解析。
func (e *Env) Post(t *testing.T, path string, body interface{}) Resp {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("序列化请求体失败: %v", err)
	}
	resp, err := e.base.Post(e.BaseURL()+path, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s 失败: %v", path, err)
	}
	defer resp.Body.Close()
	return decodeResp(t, resp.Body, path)
}

func decodeResp(t *testing.T, r io.Reader, path string) Resp {
	t.Helper()
	raw, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("读响应体失败(%s): %v", path, err)
	}
	var out Resp
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("解析响应失败(%s): %v\n原始: %s", path, err, truncate(string(raw), 800))
	}
	return out
}

// Raw 发任意方法请求，返回原始字节（下载文件用）。
func (e *Env) Raw(t *testing.T, method, path string) []byte {
	t.Helper()
	req, _ := http.NewRequest(method, e.BaseURL()+path, nil)
	resp, err := e.base.Do(req)
	if err != nil {
		t.Fatalf("%s %s 失败: %v", method, path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("%s %s 状态码 %d", method, path, resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读响应失败: %v", err)
	}
	return b
}

// Upload 上传单个文件到服务端项目目录（multipart：projectName / relativeFileName / file）。
func (e *Env) Upload(t *testing.T, project, relPath, localPath string) Resp {
	t.Helper()
	f, err := os.Open(localPath)
	if err != nil {
		t.Fatalf("打开待上传文件失败: %v", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("projectName", project)
	_ = mw.WriteField("relativeFileName", relPath)
	fw, err := mw.CreateFormFile("file", filepath.Base(localPath))
	if err != nil {
		t.Fatalf("构造 multipart 失败: %v", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		t.Fatalf("写入 multipart 失败: %v", err)
	}
	mw.Close()

	req, _ := http.NewRequest("POST", e.BaseURL()+"/api/file/upload_file", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := e.base.Do(req)
	if err != nil {
		t.Fatalf("上传 %s 失败: %v", relPath, err)
	}
	defer resp.Body.Close()
	return decodeResp(t, resp.Body, "/api/file/upload_file")
}

// ── 项目与发布 ────────────────────────────────────────────────

// DataDir 服务端项目数据目录 = server.exe 所在目录\data\{project}（GetProjectWorkPath 实现）。
func (e *Env) DataDir(project string) string {
	return filepath.Join(e.Bin, "data", project)
}

// EnsureProject 创建项目；已存在视为成功（幂等，便于单条用例反复执行）。
func (e *Env) EnsureProject(t *testing.T, name string, force bool, ignoreFolders, ignoreFiles []string) {
	t.Helper()
	r := e.Post(t, "/api/project/create_project", map[string]interface{}{
		"name": name, "title": "测试项目 " + name,
		"isForceUpdate":  force,
		"ignoreFolders":  orEmpty(ignoreFolders),
		"ignoreFiles":    orEmpty(ignoreFiles),
	})
	if !r.IsSuccess && !strings.Contains(r.ErrorMsg, "已存在") {
		t.Fatalf("create_project(%s) 失败: %s", name, r.ErrorMsg)
	}
}

func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// Publish 发布版本。
//
// **字段名是 `time`**（服务端 PublishVersion 的 json tag），不是 timeStr：
// 写成 timeStr 会被 gin 静默忽略 → 服务端 time 落空串 → 时间格式校验被跳过。
func (e *Env) Publish(t *testing.T, project, version string, logs []string,
	timeStr, afterScript string) Resp {
	t.Helper()
	if timeStr == "" {
		timeStr = time.Now().Format("2006-01-02 15:04:05")
	}
	body := map[string]interface{}{
		"projectName": project,
		"version":     version,
		"logs":        orEmpty(logs),
		"time":        timeStr,
	}
	if afterScript != "" {
		body["afterApplyUpdateScript"] = afterScript
	}
	return e.Post(t, "/api/project/publish_version", body)
}

// PublishNoTime 发布但**不传 time 字段**（验证"老客户端不传 time 也能发布"）。
// 与 Publish(t, ..., "", ...) 的区别：后者会兜底填当前时间，前者真的不带该字段。
func (e *Env) PublishNoTime(t *testing.T, project, version string, logs []string) Resp {
	t.Helper()
	return e.Post(t, "/api/project/publish_version", map[string]interface{}{
		"projectName": project,
		"version":     version,
		"logs":        orEmpty(logs),
	})
}

// PutFile 把内容直接写进服务端项目数据目录（等价"发布前改文件"）。
// 服务端 get_all_files 读实时目录，所以必须先写文件再 Publish 才自洽。
func (e *Env) PutFile(t *testing.T, project, relPath, content string) {
	t.Helper()
	p := filepath.Join(e.DataDir(project), filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("写文件失败(%s): %v", p, err)
	}
}

// RemoveFile 从服务端项目数据目录删除文件（模拟"服务端删除了文件"）。
func (e *Env) RemoveFile(t *testing.T, project, relPath string) {
	t.Helper()
	p := filepath.Join(e.DataDir(project), filepath.FromSlash(relPath))
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		t.Fatalf("删除服务端文件失败(%s): %v", p, err)
	}
}

// Manifest 服务端发布清单响应体。
type Manifest struct {
	Files []string `json:"files"`
}

// GetManifest 拉服务端发布清单（.updator_manifest.json 固化的集合）。
func (e *Env) GetManifest(t *testing.T, project string) Manifest {
	t.Helper()
	r := e.Get(t, "/api/file/get_release_manifest/"+project)
	if !r.IsSuccess {
		t.Fatalf("get_release_manifest(%s) 失败: %s", project, r.ErrorMsg)
	}
	return decodeData[Manifest](t, r)
}

// FileInfo 服务端文件列表项（get_all_files，camelCase + 小写 md5/sha256）。
type FileInfo struct {
	FileAbsolutePath string `json:"fileAbsolutePath"`
	FileRelativePath string `json:"fileRelativePath"`
	LastUpdateTime   string `json:"lastUpdateTime"`
	FileSize         int64  `json:"fileSize"`
	MD5              string `json:"md5"`
	SHA256           string `json:"sha256"`
}

// GetAllFiles 拉服务端文件列表（含 md5/sha256，有缓存）。
func (e *Env) GetAllFiles(t *testing.T, project string) []FileInfo {
	t.Helper()
	r := e.Get(t, "/api/file/get_all_files/"+project)
	if !r.IsSuccess {
		t.Fatalf("get_all_files(%s) 失败: %s", project, r.ErrorMsg)
	}
	return decodeData[[]FileInfo](t, r)
}

// ChangeLog 服务端变更日志项。
type ChangeLog struct {
	ID                      int      `json:"id"`
	Version                 string   `json:"version"`
	Logs                    []string `json:"logs"`
	Time                    string   `json:"time"`
	AfterApplyUpdateScript  string   `json:"after_apply_update_script"`
}

// ChangeLogs 拉变更日志。
func (e *Env) ChangeLogs(t *testing.T, project string) []ChangeLog {
	t.Helper()
	r := e.Get(t, "/api/project/get_project_change_logs/"+project)
	if !r.IsSuccess {
		t.Fatalf("get_project_change_logs(%s) 失败: %s", project, r.ErrorMsg)
	}
	return decodeData[[]ChangeLog](t, r)
}

// Project 项目实体。
type Project struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	ForceUpdate bool   `json:"force_update"`
	IsDeleted   bool   `json:"is_deleted"`
}

// GetProject 查项目（用于断言版本号未被改动）。
func (e *Env) GetProject(t *testing.T, name string) Project {
	t.Helper()
	r := e.Get(t, "/api/project/get_project_by_name/"+name)
	if !r.IsSuccess {
		t.Fatalf("get_project_by_name(%s) 失败: %s", name, r.ErrorMsg)
	}
	return decodeData[Project](t, r)
}

func decodeData[T any](t *testing.T, r Resp) T {
	t.Helper()
	var v T
	if len(r.Data) == 0 || string(r.Data) == "null" {
		var zero T
		return zero
	}
	if err := json.Unmarshal(r.Data, &v); err != nil {
		t.Fatalf("解析 data 失败: %v\n原始: %s", err, truncate(string(r.Data), 800))
	}
	return v
}

// ────────────────────────────────────────────────────────────────
// 客户端现场
// ────────────────────────────────────────────────────────────────

// Client 客户端 VersionInfo（version.json 全字段，按需构造）。
type Client struct {
	Pkg      string // PackageFolder：UpdateFolder 与 ApplicationFolder 的父目录
	Version  string // version
	Previous string // version_previous
	Status   string // version_status
}

// VersionInfo version.json 的完整字段。
type VersionInfo struct {
	VersionPrevious         string `json:"version_previous"`
	Version                 string `json:"version"`
	VersionStatus           string `json:"version_status"`
	AfterApplyUpdateScript  string `json:"after_apply_update_script,omitempty"`
	ApplyStage              string `json:"apply_stage,omitempty"`
	DownloadedFileCount     int    `json:"downloaded_file_count,omitempty"`
	RollbackPrevious        string `json:"rollback_previous,omitempty"`
	RollbackTarget          string `json:"rollback_target,omitempty"`
	ApplyFailCount          int    `json:"apply_fail_count,omitempty"`
}

// NewClientOpts 造客户端现场的参数。
type NewClientOpts struct {
	Version  string // 必填：version
	Status   string // 必填：version_status（applied/downloaded/applying/""）
	Previous string // version_previous，缺省 "1.0.0"
	Project  string // shared.json 的 project_name，缺省 DefaultProject
	Extras   *VersionInfo // 额外字段（apply_stage / apply_fail_count 等）
	DirName  string // 目录名，缺省自动编号
	MainFile string // 主目录内主程序内容，缺省 "fake-main-v0"
	// NoMain=true 时**不写** app.exe，用于造"本地缺失"现场（用例 8 的 check_diff）。
	// 仅靠 MainFile="" 无法区分"要默认值"和"故意不要"，故单列一个显式开关。
	NoMain bool
	// AllowEmptyVersion=true 时才允许 Version 为空，用于造"downloaded 但 version 为空"
	// 这类异常/遗留状态现场（用例 28 场景 3、A1 分支）。
	// 默认禁止：忘传 Version 会造出"version 空串"的现场，后续所有断言都在比较
	// "空 vs 期望值"，失败信息毫无指向性——这是最容易犯也最难的查的错。
	AllowEmptyVersion bool
}

const (
	StatusApplied    = "applied"
	StatusDownloaded = "downloaded"
	StatusApplying   = "applying"
	StatusEmpty      = ""
)

// DefaultProject 缺省项目名。
const DefaultProject = "tc-app"

// NewClient 按 §0.3 的目录约定造一套客户端现场：
//
//	<pkg>/UpdateFolder/   aly-client.exe + client.json + version.json
//	<pkg>/ApplicationFolder/  app.exe（fake_main 替身）+ .updator/shared.json
//
// 关键路径规则：
//   - ExeDir    = <pkg>/UpdateFolder
//   - MainFolder= <pkg>/ApplicationFolder（由 client.json 的 main_exe_relative_path 决定）
//   - 版本快照  = <pkg>/ApplicationFolder_{version}
//   - 备份目录  = 同规则（apply 时把 MainFolder 改名成它）
func (e *Env) NewClient(t *testing.T, o NewClientOpts) *Client {
	t.Helper()
	// Version 必填：缺了会造出一个 version 为空串的现场，后续所有断言都在
	// 比较"空 vs 期望值"，失败信息毫无指向性。
	// 只有显式声明 AllowEmptyVersion 的用例（造"空版本号"异常现场）才放行。
	if o.Version == "" && !o.AllowEmptyVersion {
		t.Fatalf("NewClient: Version 必填（Status 允许空串，Version 不允许；" +
			"若确实要造空版本号现场，请显式设 AllowEmptyVersion: true）")
	}
	if o.Previous == "" {
		o.Previous = "1.0.0"
	}
	if o.Project == "" {
		o.Project = DefaultProject
	}
	// MainFile 语义：
	//   MainFile 非空           → 写 app.exe 为该内容（正常现场）
	//   MainFile 为空且 NoMain  → 不写 app.exe（造"本地缺失"现场，用例 8 用）
	//   MainFile 为空且非 NoMain→ 默认写 fake-main-v0（避免误造空主目录）
	//
	// 注意：这里**不再**判断 DirName。原实现多带一个 `o.DirName == ""` 条件，
	// 导致"传了 DirName 又忘了 MainFile"时主目录没有 app.exe，而 client.json
	// 的 main_exe_relative_path 仍指向它 —— apply/自更新会走进完全不同的分支。
	if o.MainFile == "" && !o.NoMain {
		o.MainFile = "fake-main-v0"
	}
	e.seq++
	name := o.DirName
	if name == "" {
		name = fmt.Sprintf("client-%02d", e.seq)
	}
	pkg := filepath.Join(e.Root, name)
	if err := os.RemoveAll(pkg); err != nil {
		t.Fatalf("清理旧现场失败: %v", err)
	}
	up := filepath.Join(pkg, "UpdateFolder")
	app := filepath.Join(pkg, "ApplicationFolder")
	for _, d := range []string{up, filepath.Join(app, ".updator")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("建目录失败(%s): %v", d, err)
		}
	}
	CopyFile(t, e.ClientExe, filepath.Join(up, "aly-client.exe"))

	WriteJSON(t, filepath.Join(up, "client.json"), map[string]interface{}{
		"main_exe_relative_path": "../ApplicationFolder/app.exe",
		"must_close_process_name": []string{"fake_main"},
	})

	vi := VersionInfo{VersionPrevious: o.Previous, Version: o.Version, VersionStatus: o.Status}
	if o.Extras != nil {
		x := *o.Extras
		if x.Version != "" {
			vi.Version = x.Version
		}
		if x.VersionPrevious != "" {
			vi.VersionPrevious = x.VersionPrevious
		}
		if x.VersionStatus != "" {
			vi.VersionStatus = x.VersionStatus
		}
		vi.AfterApplyUpdateScript = x.AfterApplyUpdateScript
		vi.ApplyStage = x.ApplyStage
		vi.DownloadedFileCount = x.DownloadedFileCount
		vi.RollbackPrevious = x.RollbackPrevious
		vi.RollbackTarget = x.RollbackTarget
		vi.ApplyFailCount = x.ApplyFailCount
	}
	WriteJSON(t, filepath.Join(up, "version.json"), vi)

	WriteJSON(t, filepath.Join(app, ".updator", "shared.json"), map[string]interface{}{
		"server_url":       e.BaseURL(),
		"project_name":     o.Project,
		"ignore_folders":   []string{},
		"ignore_files":     []string{},
		"un_copy_folders":  []string{},
		"un_copy_files":    []string{},
	})
	if o.MainFile != "" {
		WriteText(t, filepath.Join(app, "app.exe"), o.MainFile)
	}
	return &Client{Pkg: pkg, Version: o.Version, Previous: o.Previous, Status: o.Status}
}

// Up 返回 UpdateFolder 路径（ExeDir）。
func (c *Client) Up() string { return filepath.Join(c.Pkg, "UpdateFolder") }

// Main 返回 ApplicationFolder 路径（MainFolder）。
func (c *Client) Main() string { return filepath.Join(c.Pkg, "ApplicationFolder") }

// VerDir 返回版本快照目录 <pkg>/ApplicationFolder_{v}；同规则也用于备份目录。
func (c *Client) VerDir(v string) string { return filepath.Join(c.Pkg, "ApplicationFolder_"+v) }

// Shared 返回 shared.json 路径。
func (c *Client) Shared() string { return filepath.Join(c.Main(), ".updator", "shared.json") }

// ReadVersion 读 version.json 全字段。
func (c *Client) ReadVersion(t *testing.T) VersionInfo {
	t.Helper()
	return ReadVersionFile(t, c.Up())
}

// WriteVersion 覆写 version.json（造特定状态现场）。
func (c *Client) WriteVersion(t *testing.T, v VersionInfo) {
	t.Helper()
	WriteJSON(t, filepath.Join(c.Up(), "version.json"), v)
}

// Log 读客户端日志尾部（download.log / update.log）。
func (c *Client) Log(t *testing.T, name string, tailLines int) string {
	t.Helper()
	p := filepath.Join(c.Up(), name)
	b, err := os.ReadFile(p)
	if err != nil {
		return "<" + name + " 不存在>"
	}
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	return strings.Join(lines, "\n")
}

// Exists 判定路径存在。
func Exists(p string) bool { _, err := os.Stat(p); return err == nil }

// ReadText 读文本文件内容（不存在返回空串）。
func ReadText(p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return string(b)
}

// SizeOf 文件大小（不存在返回 -1）。
func SizeOf(p string) int64 {
	fi, err := os.Stat(p)
	if err != nil {
		return -1
	}
	return fi.Size()
}

// ListDir 列目录一级条目名（不存在返回 nil）。
func ListDir(p string) []string {
	es, err := os.ReadDir(p)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}

// ────────────────────────────────────────────────────────────────
// 运行 aly-client
// ────────────────────────────────────────────────────────────────

// Out 是 aly-client 的一行输出（过程行与结果行共用）。
type Out struct {
	IsSuccess bool            `json:"isSuccess"`
	ErrorMsg  string          `json:"errorMsg"`
	Data      json.RawMessage `json:"data"`
	Progress  *Progress       `json:"-"`
}

// Progress 解析后的进度行（download_update 的 START/PROGRESS/DONE/BEGIN/FAIL）。
type Progress struct {
	Index           int    `json:"index"`
	Total           int    `json:"total"`
	File            string `json:"file"`
	Status          string `json:"status"`
	FileSize        int64  `json:"file_size"`
	Error           string `json:"error"`
	DownloadedBytes int64  `json:"downloaded_bytes"`
	TotalBytes      int64  `json:"total_bytes"`
	SpeedBps        int64  `json:"speed_bps"`
}

// Run 执行 aly-client 命令，返回**全部行**（保留 PROGRESS 等过程行）。
// 末行即结果行。args 例如 "check_update"、"rollback","--version","1.0.0"。
func (e *Env) Run(t *testing.T, c *Client, args ...string) []Out {
	t.Helper()
	cmd := exec.Command(filepath.Join(c.Up(), "aly-client.exe"), args...)
	cmd.Dir = c.Up()
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		// aly-client 的失败通过 JSON 表达（isSuccess:false），非零退出码才是异常
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("执行 aly-client %v 失败: %v\nstderr:%s", args, err, stderr.String())
		}
	}
	out := parseLines(stdout.String())
	if len(out) == 0 {
		t.Fatalf("aly-client %v 无任何 JSON 输出\nstdout:%q\nstderr:%s",
			args, truncate(stdout.String(), 500), stderr.String())
	}
	return out
}

// RunLast 执行并只返回末行（结果行）。大多数用例只关心结果。
func (e *Env) RunLast(t *testing.T, c *Client, args ...string) Out {
	t.Helper()
	out := e.Run(t, c, args...)
	return out[len(out)-1]
}

// RunAsync 异步执行（用于"下载中途强杀"这类需要并发干预的用例）。
func (e *Env) RunAsync(c *Client, args ...string) (*exec.Cmd, *bytes.Buffer) {
	cmd := exec.Command(filepath.Join(c.Up(), "aly-client.exe"), args...)
	cmd.Dir = c.Up()
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	_ = cmd.Start()
	return cmd, &buf
}

func parseLines(s string) []Out {
	var out []Out
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var o Out
		if err := json.Unmarshal([]byte(line), &o); err != nil {
			continue
		}
		if string(o.Data) != "null" && len(o.Data) > 0 {
			var p Progress
			if json.Unmarshal(o.Data, &p) == nil && p.Status != "" {
				o.Progress = &p
			}
		}
		out = append(out, o)
	}
	return out
}

// Data 把结果行的 data 解成目标类型。
func Data[T any](t *testing.T, o Out) T {
	t.Helper()
	var v T
	if len(o.Data) == 0 || string(o.Data) == "null" {
		var zero T
		return zero
	}
	if err := json.Unmarshal(o.Data, &v); err != nil {
		t.Fatalf("解析 data 失败: %v\n原始: %s", err, truncate(string(o.Data), 800))
	}
	return v
}

// RespData 同 Data，但作用于 HTTP 统一响应体（server 组件用例用）。
func RespData[T any](t *testing.T, r Resp) T {
	t.Helper()
	var v T
	if len(r.Data) == 0 || string(r.Data) == "null" {
		var zero T
		return zero
	}
	if err := json.Unmarshal(r.Data, &v); err != nil {
		t.Fatalf("解析响应 data 失败: %v\n原始: %s", err, truncate(string(r.Data), 800))
	}
	return v
}

// CheckUpdateData check_update 的 data。
type CheckUpdateData struct {
	HasUpdate          bool     `json:"has_update"`
	NeedDownloadUpdate bool     `json:"need_download_update"`
	CurrentVersion     string   `json:"current_version"`
	NewVersion         string   `json:"new_version"`
	ForceUpdate        *bool    `json:"force_update"`
	ReleaseNotes       []string `json:"release_notes"`
}

// RollbackListData list_rollback_versions 的 data。
type RollbackListData struct {
	CurrentVersion string   `json:"current_version"`
	Versions       []string `json:"versions"`
}

// RollbackData rollback 的 data。
type RollbackData struct {
	Version string `json:"version"`
}

// DiffData check_diff 的 data。
type DiffData struct {
	NewVersion string     `json:"new_version"`
	Files      []DiffItem `json:"files"`
}

// DiffItem check_diff 的单个差异文件。
type DiffItem struct {
	Path         string `json:"path"`
	LocalMD5     string `json:"local_md5"`
	LocalSize    int64  `json:"local_size"`
	LocalSHA256  string `json:"local_sha256"`
	ServerMD5    string `json:"server_md5"`
	ServerSize   int64  `json:"server_size"`
	ServerSHA256 string `json:"server_sha256"`
}

// SelfUpdateData check_self_update 的 data。
type SelfUpdateData struct {
	NeedUpdate bool `json:"need_update"`
}

// SelfUpdateExecData self_update 命令的 data（用例 130–132）。
type SelfUpdateExecData struct {
	Updated bool   `json:"updated"`
	Reason  string `json:"reason"`
	OldPath string `json:"old_path,omitempty"`
}

// ────────────────────────────────────────────────────────────────
// publish-cli（aly-publish）
// ────────────────────────────────────────────────────────────────

// NewPublishWS 新建 publish-cli 工作区（.updator/shared.json）。
func (e *Env) NewPublishWS(t *testing.T, project, dirName string) string {
	t.Helper()
	if dirName == "" {
		dirName = "publish-src"
	}
	ws := filepath.Join(e.Root, dirName)
	if err := os.MkdirAll(filepath.Join(ws, ".updator"), 0o755); err != nil {
		t.Fatalf("建 publish 工作区失败: %v", err)
	}
	WriteJSON(t, filepath.Join(ws, ".updator", "shared.json"), map[string]interface{}{
		"server_url":      e.BaseURL(),
		"project_name":    project,
		"ignore_folders":  []string{},
		"ignore_files":    []string{},
		"un_copy_folders": []string{},
		"un_copy_files":   []string{},
	})
	return ws
}

// PublishCLI 跑 aly-publish 命令，返回末个含 isSuccess 的 JSON 行。
//
// **必须带 --json**：非 json 模式失败信息写 stderr，被调用方当异常；
// json 模式下成功/失败都是 stdout 的一行 {"isSuccess":...}，断言口径统一。
func (e *Env) PublishCLI(t *testing.T, ws string, args ...string) Resp {
	t.Helper()
	full := append([]string{"--server", e.BaseURL(), "--json"}, args...)
	cmd := exec.Command(e.PublishExe, full...)
	cmd.Dir = ws
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	return parseCLIResult(t, stdout.String(), stderr.String(), args, runErr)
}

// parseCLIResult 从 publish-cli 的 stdout 里挑出**真正的结果行**。
//
// 为什么不能"取最后一个 isSuccess 行"：push 失败路径是
//   outputResult(false, "创建版本失败: ...", nil)   ← 真结果
//   printProgressDone()                            ← 又打一行 {"isSuccess":true,"data":null}
// 后者无条件 isSuccess=true，于是"取最后一行"会把失败读成成功（真实缺陷，
// 已记入文档修正项）。这里按 data 形状区分：
//   - 进度行：data 含 index/total/file/status 之一 → 跳过
//   - 尾巴行：data==null 且 errorMsg=="" → 跳过（printProgressDone）
//   - 其余：真结果（成功带 data，失败带 errorMsg）
// 若全部被判为噪声，回落到最后一行并给出 stderr，避免静默吞掉信息。
func parseCLIResult(t *testing.T, stdout, stderr string, args []string, runErr error) Resp {
	t.Helper()
	var results []Resp
	var lastAny *Resp
	for _, line := range strings.Split(strings.ReplaceAll(stdout, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}
		var r Resp
		if json.Unmarshal([]byte(line), &r) != nil {
			continue
		}
		lastAny = &r
		d := strings.TrimSpace(string(r.Data))
		isProgress := strings.Contains(d, `"index"`) || strings.Contains(d, `"total"`) ||
			strings.Contains(d, `"file"`) || strings.Contains(d, `"status"`)
		if isProgress {
			continue
		}
		// printProgressDone：data:null 且无错误信息
		if (d == "null" || d == "") && r.ErrorMsg == "" {
			continue
		}
		results = append(results, r)
	}
	if len(results) > 0 {
		return results[len(results)-1]
	}
	if lastAny != nil {
		return *lastAny
	}
	t.Fatalf("aly-publish %v 无 JSON 输出\nstdout:\n%s\nstderr:\n%s\n退出: %v",
		args, truncate(stdout, 2000), truncate(stderr, 2000), runErr)
	return Resp{}
}

// ── 文件小工具 ───────────────────────────────────────────────

func CopyFile(t *testing.T, src, dst string) {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("读 %s 失败: %v", src, err)
	}
	if err := os.WriteFile(dst, b, 0o755); err != nil {
		t.Fatalf("写 %s 失败: %v", dst, err)
	}
}

func WriteText(t *testing.T, p, s string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatalf("写文件失败(%s): %v", p, err)
	}
}

func WriteJSON(t *testing.T, p string, v interface{}) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("序列化 JSON 失败: %v", err)
	}
	WriteText(t, p, string(b))
}

func ReadVersionFile(t *testing.T, updateFolder string) VersionInfo {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(updateFolder, "version.json"))
	if err != nil {
		t.Fatalf("读 version.json 失败: %v", err)
	}
	var v VersionInfo
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("解析 version.json 失败: %v\n原始: %s", err, string(b))
	}
	return v
}

// BigFile 生成 sizeMB 大小的文件（全零；用 truncate 快速生成稀疏文件）。
func BigFile(t *testing.T, p string, sizeMB int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("创建文件失败: %v", err)
	}
	defer f.Close()
	if err := f.Truncate(int64(sizeMB) << 20); err != nil {
		t.Fatalf("截断失败: %v", err)
	}
}

// Tree 列目录全部文件（失败诊断用）。
func Tree(root string, max int) string {
	var lines []string
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		lines = append(lines, fmt.Sprintf("%s (%dB)", rel, fi.Size()))
		return nil
	})
	if len(lines) > max {
		return strings.Join(lines[:max], "\n") + fmt.Sprintf("\n... 共 %d 项", len(lines))
	}
	return strings.Join(lines, "\n")
}
// ────────────────────────────────────────────────────────────────
// 用例 40–65 需要的辅助（PE 头、服务端信息、目录匹配、Range、原始 MD5）
// ────────────────────────────────────────────────────────────────

// ServerInfo 是 /api/server/info 的 data。
type ServerInfo struct {
	OS              string  `json:"os"`
	Platform        string  `json:"platform"`
	GOARCH          string  `json:"goARCH"`
	Version         string  `json:"version"`
	NumCPU          int     `json:"numCPU"`
	CPUName         string  `json:"cpuName"`
	CPUMhz          float64 `json:"cpuMhz"`
	DiskUsed        float64 `json:"diskUsed"`
	DiskFree        float64 `json:"diskFree"`
	DiskTotal       float64 `json:"diskTotal"`
	DiskUsedPercent float64 `json:"diskUsedPercent"`
}

// AllProjects 拉全部项目（get_all_projects，已排除软删除）。
func (e *Env) AllProjects(t *testing.T) []Project {
	t.Helper()
	r := e.Get(t, "/api/project/get_all_projects")
	if !r.IsSuccess {
		t.Fatalf("get_all_projects 失败: %s", r.ErrorMsg)
	}
	return decodeData[[]Project](t, r)
}

// TryStatus 尝试请求某端口上的路径，返回是否连得上（服务端已被杀时用）。
// 与 Status 的区别：不 Fatal，纯粹"可达性"探测。
func (e *Env) TryStatus(port int, path string) bool {
	cli := &http.Client{Timeout: 2 * time.Second}
	resp, err := cli.Get(fmt.Sprintf("http://127.0.0.1:%d%s", port, path))
	if err != nil {
		return false
	}
	resp.Body.Close()
	return true
}

// TryGetAllFiles 容错版 GetAllFiles：失败时返回 nil 而不是 Fatal
// （并发轮询场景不能让一次瞬时失败打断整个用例）。
func (e *Env) TryGetAllFiles(t *testing.T, project string) []FileInfo {
	t.Helper()
	resp, err := e.base.Get(e.BaseURL() + "/api/file/get_all_files/" + project)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil
	}
	var out Resp
	if json.Unmarshal(raw, &out) != nil || !out.IsSuccess {
		return nil
	}
	var fs []FileInfo
	if json.Unmarshal(out.Data, &fs) != nil {
		return nil
	}
	return fs
}

// RawRange 带 Range 头请求，返回状态码与响应体。
// off 为起始字节（"bytes=<off>-"）。
func (e *Env) RawRange(t *testing.T, path string, off int64) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest("GET", e.BaseURL()+path, nil)
	if err != nil {
		t.Fatalf("构造 Range 请求失败: %v", err)
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-", off))
	resp, err := e.base.Do(req)
	if err != nil {
		t.Fatalf("Range 请求失败: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, b
}

// MD5Bytes 计算内存字节的 MD5（与 MD5Of 口径一致）。
func MD5Bytes(b []byte) string {
	h := md5.New()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// ListDirMatch 统计 dir 下匹配 glob 的**文件**数量（不递归）。
func ListDirMatch(dir, pattern string) int {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, ent := range ents {
		if ent.IsDir() {
			continue
		}
		if ok, _ := filepath.Match(pattern, ent.Name()); ok {
			n++
		}
	}
	return n
}

// ── PE 头解析（用例 40 验 386 + GUI 子系统）──────────────────────

// PE Machine 常量。
const (
	MachineI386        = 0x014c
	MachineAMD64       = 0x8664
	SubsystemWindowsGUI = 2
	SubsystemConsole    = 3
)

// PEMachine 读 PE 头返回 Machine 字段（IMAGE_FILE_HEADER.Machine）。
// 非 PE 文件返回 0。
func PEMachine(path string) uint16 {
	_, _, machine, _, ok := peHeader(path)
	if !ok {
		return 0
	}
	return machine
}

// PESubsystem 读 PE 可选头返回 Subsystem（2=GUI，3=Console）。
func PESubsystem(path string) uint16 {
	_, _, _, sub, ok := peHeader(path)
	if !ok {
		return 0
	}
	return sub
}

// peHeader 解析 DOS 头 → PE 签名 → COFF 头 → 可选头。
// 返回 (e_lfanew, NumberOfSections, Machine, Subsystem, ok)。
func peHeader(path string) (lfanew uint32, sections uint16, machine, subsystem uint16, ok bool) {
	b, err := os.ReadFile(path)
	if err != nil || len(b) < 0x40 {
		return 0, 0, 0, 0, false
	}
	// DOS 头：偏移 0x3C 处是 e_lfanew（4 字节小端）
	lfanew = uint32(b[0x3C]) | uint32(b[0x3D])<<8 | uint32(b[0x3E])<<16 | uint32(b[0x3F])<<24
	if int(lfanew)+24+64 > len(b) {
		return 0, 0, 0, 0, false
	}
	if string(b[lfanew:lfanew+4]) != "PE\x00\x00" {
		return 0, 0, 0, 0, false
	}
	coff := lfanew + 4
	machine = uint16(b[coff]) | uint16(b[coff+1])<<8
	sections = uint16(b[coff+2]) | uint16(b[coff+3])<<8
	optSize := uint16(b[coff+16]) | uint16(b[coff+17])<<8
	opt := coff + 20
	if optSize < 70 || int(opt)+70 > len(b) {
		return lfanew, sections, machine, 0, true
	}
	// 可选头 Subsystem 在偏移 68（PE32 与 PE32+ 相同位置）
	subsystem = uint16(b[opt+68]) | uint16(b[opt+69])<<8
	return lfanew, sections, machine, subsystem, true
}

// ────────────────────────────────────────────────────────────────
// 用例 66–77 需要的辅助（publish-cli 的 data 结构）
// ────────────────────────────────────────────────────────────────

// FileStatusItem 对应 publish-cli 的 models.FileStatusItem。
type FileStatusItem struct {
	RelativePath string `json:"relativePath"`
	Status       string `json:"status"` // new / modified / deleted / unchanged
	LocalMd5     string `json:"localMd5"`
	LocalSize    int64  `json:"localSize"`
	ServerMd5    string `json:"serverMd5,omitempty"`
	ServerSize   int64  `json:"serverSize,omitempty"`
}

// StatusData 是 `status` 命令的 data。
type StatusData struct {
	Staged    []FileStatusItem `json:"staged"`
	Unstaged  []FileStatusItem `json:"unstaged"`
	Unchanged []FileStatusItem `json:"unchanged"`
}

// StagedItem 是 `staged` 命令的 data 元素（与 FileStatusItem 同构）。
type StagedItem = FileStatusItem

// AddData 是 `add --all` 的 data。
type AddData struct {
	Added int `json:"added"`
}

// PushData 是 `push` 成功时的 data：{"version":"1.2.0","files":"3"}。
// 注意 files 是**字符串**不是数字（push.go 用 fmt.Sprintf("%d") 拼的）。
type PushData struct {
	Version string `json:"version"`
	Files   string `json:"files"`
}

// PublishCLIAt 与 PublishCLI 相同，但可指定 server 地址（用例 75 验不可达）。
func (e *Env) PublishCLIAt(t *testing.T, ws, server string, args ...string) Resp {
	t.Helper()
	full := append([]string{"--server", server, "--json"}, args...)
	cmd := exec.Command(e.PublishExe, full...)
	cmd.Dir = ws
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	return parseCLIResult(t, stdout.String(), stderr.String(), args, runErr)
}

// PublishCLIRawOut 是 aly-publish 的原始输出（不解析 JSON），
// 用于那些**本来就不输出 JSON** 的场景。
type PublishCLIRawOut struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// PublishCLIRaw 直接跑 aly-publish 并返回原始输出。
//
// 为什么需要它：`push --dry-run` 在 `--json` 下**只打印人读文本**
// （push.go 的 `printHumanLn("[DRY RUN] ...")` 后直接 `return false`），
// 一行 JSON 都没有。用 PublishCLI 会因"无 JSON 输出"直接 Fatal。
func (e *Env) PublishCLIRaw(t *testing.T, ws string, args ...string) PublishCLIRawOut {
	t.Helper()
	full := append([]string{"--server", e.BaseURL(), "--json"}, args...)
	cmd := exec.Command(e.PublishExe, full...)
	cmd.Dir = ws
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	runErr := cmd.Run()
	code := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			t.Fatalf("执行 aly-publish %v 失败: %v\nstderr:%s", args, runErr, stderr.String())
		}
	}
	return PublishCLIRawOut{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}
}

// FileRels 从 get_all_files 结果里提取相对路径切片（`/` 分隔）。
func FileRels(t *testing.T, fs []FileInfo) []string {
	t.Helper()
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, filepath.ToSlash(f.FileRelativePath))
	}
	return out
}

// SliceContains 判断字符串切片是否含某元素。
func SliceContains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
