package api

// 上传重试口径的单元测试（2026-11-06）。
//
// 背景：远程服务端 ReadTimeout = 60s，链路上 1MiB 分片要 43–66s 才传完，
// 一旦越过 60s 服务端就断链，客户端往往只看到一个光光的 "EOF" / "wsarecv"。
// 而 isTransientUploadError 不含这些字样 → 判为"非临时错误" → 第一片就放弃，
// 5 次重试形同虚设；且 uploadFileDirect 原本**完全没有重试**。
//
// 这里用进程内 TCP 服务端精确注入"先断链、后成功"，钉死两条契约：
//  1. 对端直接断链（EOF）必须被判为可重试；
//  2. 直传（小文件）必须真的重试，且最终成功。
//
// 顺带覆盖"非临时错误不得重试"——否则会把业务拒绝也拖成 5 次退避。

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ────────────────────────────────────────────────────────────────
// isTransientUploadError：对端断链类错误必须可重试
// ────────────────────────────────────────────────────────────────
func TestIsTransientUploadError(t *testing.T) {
	// 真实观测到的断链形态（Go / Windows）：
	//   - Post "...": EOF                          ← 服务端读超时后直接关连接
	//   - read tcp ...: wsarecv: An existing connection was forcibly closed by the remote host.
	transient := []string{
		`upload chunk: Post "http://h/api/file/upload_chunk": EOF`,
		"read tcp 10.0.0.1:1234->10.0.0.2:7000: wsarecv: An existing connection was forcibly closed by the remote host.",
		"write tcp: wsasend: An existing connection was forcibly closed by the remote host.",
		"EOF",
		"io: read/write on closed pipe",
		"read tcp: connection reset by peer",
		"read tcp: connection abort",
		"context deadline exceeded: timeout",
	}
	for _, msg := range transient {
		if !isTransientUploadError(fmt.Errorf("%s", msg)) {
			t.Fatalf("应判为可重试（对端断链/超时）却判成了不可重试: %q", msg)
		}
	}

	// 非临时：业务拒绝、参数错——重试无意义，必须立刻失败
	notTransient := []string{
		"文件过大: 1073741824 字节，上限 1073741824 字节",
		"项目名称不能为空",
		"HTTP 404: 404 page not found", // 路由不存在，重试也没用
		"非法的文件路径",
	}
	for _, msg := range notTransient {
		if isTransientUploadError(fmt.Errorf("%s", msg)) {
			t.Fatalf("应判为不可重试（业务错误）却判成了可重试: %q", msg)
		}
	}
}

// ────────────────────────────────────────────────────────────────
// 注入保真性：dropRespFirst 必须产出**纯 EOF**（不含 "closed"/"wsarecv"）
//
// 这个用例是上一个用例的前提。若 stub 实际产生的是
// `wsarecv: ... forcibly closed`，而旧判定器本就含 "forcibly closed"，
// 那么"删掉 EOF 分支"的负向对照依然 PASS —— 重试用例就失去了区分力
// （2026-11-06 踩过：failFirst 形态正是如此）。
// ────────────────────────────────────────────────────────────────
func TestStubDropRespProducesPureEOF(t *testing.T) {
	stub := &uploadStub{dropRespFirst: 1}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	defer ln.Close()
	go stub.serve(ln)

	// 裸 post，不经过 uploadFileDirect 的重试，以观测单次错误的原始文案
	resp, err := http.Post("http://"+ln.Addr().String()+"/api/file/upload_file",
		"text/plain", strings.NewReader("hello"))
	if err == nil {
		resp.Body.Close()
		t.Fatal("stub 不给响应就关闭，客户端应报错，实际成功")
	}

	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "eof") {
		t.Fatalf("注入应产生 EOF，实际: %v", err)
	}
	// 关键断言：不得夹带任何"连接被强制关闭"类字样，否则无法区分 EOF 判定
	for _, leak := range []string{"forcibly closed", "wsarecv", "wsasend", "connection reset", "closed pipe"} {
		if strings.Contains(msg, leak) {
			t.Fatalf("注入不纯：出现了 %q，该形态会被旧判定器覆盖，用例失去区分力。实际: %v", leak, err)
		}
	}
}

// ────────────────────────────────────────────────────────────────
// uploadFileDirect：对端先断链 N 次，必须重试到成功
//
// 两种断链形态分别验一遍：
//   - 纯 EOF：请求读完、响应一个字节不给就关闭（干净 FIN）
//   - 半途断链：读到一半就关闭（服务端 ReadTimeout 的真实形态）
//
// 断言三点：
//  1. 最终返回 nil（重试生效）；
//  2. 请求次数 = 失败次数 + 1（确实重试了，且没有多余重试）；
//  3. 成功那次的 body 是**完整** multipart（缓冲被正确重建，没被读空）。
//
// ────────────────────────────────────────────────────────────────
func TestUploadFileDirectRetries(t *testing.T) {
	const failFirst = 2

	cases := []struct {
		name string
		stub *uploadStub
	}{
		{"纯EOF", &uploadStub{dropRespFirst: failFirst}},
		{"半途断链", &uploadStub{failFirst: failFirst}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stub := tc.stub
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatalf("监听失败: %v", err)
			}
			defer ln.Close()
			go stub.serve(ln)

			c := NewClient("http://" + ln.Addr().String())

			// 造一个"小文件"（<= chunkSize，走直传分支）
			dir := t.TempDir()
			path := filepath.Join(dir, "small.bin")
			payload := bytes.Repeat([]byte("aly"), 777)
			if err := os.WriteFile(path, payload, 0o644); err != nil {
				t.Fatalf("写测试文件失败: %v", err)
			}

			if err := c.UploadFile(path, "proj", "dir/small.bin"); err != nil {
				t.Fatalf("断链 %d 次后应重试成功，实际失败: %v", failFirst, err)
			}

			stub.mu.Lock()
			defer stub.mu.Unlock()
			if stub.n != failFirst+1 {
				t.Fatalf("请求次数应 = %d（失败 %d + 成功 1），实际 %d——重试未生效或次数不对",
					failFirst+1, failFirst, stub.n)
			}
			// 只断言"最后一次请求的 body 完整"：两种断链形态下 stub 能记到的
			// 请求数不同（纯 EOF 会读完每一次，半途断链读不完前几次），
			// 而真正要钉的是——重试时缓冲被重建过，内容没被上一次读空。
			if len(stub.bodies) == 0 {
				t.Fatal("未记录到任何完整请求 body")
			}
			last := stub.bodies[len(stub.bodies)-1]
			if !bytes.Contains(last, payload) {
				t.Fatalf("成功那次请求的 body 不完整（长度 %d，未包含原始 payload %d 字节）",
					len(last), len(payload))
			}
			if !strings.Contains(stub.contentType, "multipart/form-data") {
				t.Fatalf("Content-Type 应为 multipart/form-data，实际 %q", stub.contentType)
			}
		})
	}
}

// ────────────────────────────────────────────────────────────────
// uploadFileDirect：非临时错误不得重试（立刻失败）
// ────────────────────────────────────────────────────────────────
func TestUploadFileDirectNoRetryOnBusinessError(t *testing.T) {
	stub := &uploadStub{rejectAll: true}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("监听失败: %v", err)
	}
	defer ln.Close()
	go stub.serve(ln)

	c := NewClient("http://" + ln.Addr().String())
	dir := t.TempDir()
	path := filepath.Join(dir, "small.bin")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("写测试文件失败: %v", err)
	}

	err = c.UploadFile(path, "proj", "small.bin")
	if err == nil {
		t.Fatal("业务拒绝应返回错误")
	}
	if !strings.Contains(err.Error(), "项目名称不能为空") {
		t.Fatalf("错误应透传服务端 errorMsg，实际: %v", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.n != 1 {
		t.Fatalf("业务错误不得重试，请求次数应为 1，实际 %d", stub.n)
	}
}

// ────────────────────────────────────────────────────────────────
// uploadStub：可注入两种"断链"形态，之后正常应答；rejectAll 时始终业务失败。
//
//   - failFirst：读到一半就断开（模拟服务端 ReadTimeout 到点主动断链）。
//     客户端此时仍在写，Windows 上观测为
//     `wsarecv: An existing connection was forcibly closed`。
//   - dropRespFirst：**完整读完请求**、一个字节响应都不给就关闭。
//     客户端已写完，读到的是干净的 FIN → 纯 `EOF`，不含任何 "closed" 字样。
//
// 为什么必须区分：旧版判定器自带 "forcibly closed"，用 failFirst 做负向对照
// 时,"删掉 EOF 分支"仍然是 PASS——测不出 EOF 判定是否存在（2026-11-06 实测）。
// 只有 dropRespFirst 产生的纯 EOF 才有区分力。
// ────────────────────────────────────────────────────────────────
type uploadStub struct {
	mu            sync.Mutex
	failFirst     int
	dropRespFirst int
	rejectAll     bool
	n             int
	bodies        [][]byte
	contentType   string
}

func (s *uploadStub) serve(ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go s.handle(conn)
	}
}

func (s *uploadStub) handle(conn net.Conn) {
	defer conn.Close()

	s.mu.Lock()
	s.n++
	idx := s.n
	s.mu.Unlock()

	if idx <= s.failFirst {
		// 只读一点点就主动断开：正是"服务端读超时/链路中断"的观测形态，
		// 客户端会收到 EOF（或 wsarecv），且拿不到任何 HTTP 响应。
		buf := make([]byte, 1024)
		_, _ = conn.Read(buf)
		return
	}

	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return
	}
	defer req.Body.Close()
	body := readAll(req.Body)

	s.mu.Lock()
	s.bodies = append(s.bodies, body)
	s.contentType = req.Header.Get("Content-Type")
	s.mu.Unlock()

	// 请求已完整收下，但一个字节响应都不给就关闭 —— 客户端读到干净 FIN，得纯 EOF
	if idx <= s.failFirst+s.dropRespFirst {
		return
	}

	var resp string
	if s.rejectAll {
		resp = `{"isSuccess":false,"errorMsg":"项目名称不能为空","data":null}`
	} else {
		resp = `{"isSuccess":true,"errorMsg":"","data":null}`
	}
	_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\n" +
		"Content-Type: application/json\r\n" +
		fmt.Sprintf("Content-Length: %d\r\n", len(resp)) +
		"\r\n" + resp))
}

// readAll 读完 body（io.ReadAll 的直接包装，便于此处单点控制）
func readAll(r interface{ Read([]byte) (int, error) }) []byte {
	var buf bytes.Buffer
	b := make([]byte, 32*1024)
	for {
		n, err := r.Read(b)
		if n > 0 {
			buf.Write(b[:n])
		}
		if err != nil {
			return buf.Bytes()
		}
	}
}
