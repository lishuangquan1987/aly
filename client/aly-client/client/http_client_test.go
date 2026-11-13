package client

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestDownloadFileWithResumeCompletePartRenamed 轻微项：.part 已完整（大小 == 服务端大小）
// 时应直接 rename，不再整文件重下（旧条件只对"严格小于"续传，完整 .part 反被删掉重下）。
// 用一个必然不可达的 URL 证明没有发起任何下载请求。
func TestDownloadFileWithResumeCompletePartRenamed(t *testing.T) {
	dir, err := ioutil.TempDir("", "resume-complete")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)

	localPath := filepath.Join(dir, "app.exe")
	partPath := localPath + ".part"
	if err := ioutil.WriteFile(partPath, []byte("hello"), 0644); err != nil {
		t.Fatalf("写 .part 失败: %v", err)
	}
	// 不可达地址：只要发出请求就会失败；直接 rename 则返回 nil。
	err = DownloadFileWithResume("http://127.0.0.1:1", "whatever", localPath, int64(len("hello")), 100*1024*1024, nil)
	if err != nil {
		t.Fatalf("完整 .part 应直接 rename，实际报错: %v", err)
	}
	data, err := ioutil.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读目标文件失败: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("目标文件内容应为 hello，实际 %q", data)
	}
	if _, err := os.Stat(partPath); err == nil {
		t.Errorf("完整 .part 已 rename，不应残留")
	}
}

// TestDownloadFileWritesAtomically M10：下载完成后先落盘（Sync）再原子 rename，
// 用 httptest 覆盖成功路径（Sync 错误无法在 Windows 上稳定注入，此处只保证行为与编译）。
func TestDownloadFileWritesAtomically(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("payload"))
	}))
	defer srv.Close()

	dir, err := ioutil.TempDir("", "download-atomic")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)
	localPath := filepath.Join(dir, "sub", "payload.bin")

	if err := DownloadFile(srv.URL, "remote/payload.bin", localPath, nil); err != nil {
		t.Fatalf("DownloadFile 失败: %v", err)
	}
	data, err := ioutil.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读下载文件失败: %v", err)
	}
	if string(data) != "payload" {
		t.Errorf("下载内容错误: %q", data)
	}
	if _, err := os.Stat(localPath + ".part"); err == nil {
		t.Errorf(".part 不应残留")
	}
}

// TestDownloadFileWithResumeFresh httptest 覆盖续传模块的整文件下载路径（含 M10 的 Sync）。
func TestDownloadFileWithResumeFresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("fresh"))
	}))
	defer srv.Close()

	dir, err := ioutil.TempDir("", "resume-fresh")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)
	localPath := filepath.Join(dir, "fresh.bin")

	if err := DownloadFileWithResume(srv.URL, "remote/fresh.bin", localPath, int64(len("fresh")), 100*1024*1024, nil); err != nil {
		t.Fatalf("DownloadFileWithResume 失败: %v", err)
	}
	data, err := ioutil.ReadFile(localPath)
	if err != nil {
		t.Fatalf("读文件失败: %v", err)
	}
	if string(data) != "fresh" {
		t.Errorf("内容错误: %q", data)
	}
}
