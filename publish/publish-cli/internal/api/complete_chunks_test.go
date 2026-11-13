package api

// complete 分片合并的响应语义测试（审查 M15）。
//
// 关键缺陷：服务端缺分片时返回 HTTP 200 + isSuccess=true + chunksComplete=false，
// 旧实现只看 isSuccess 就把"未合并"当成功 → 文件静默缺失。
// 这里用 httptest 服务端精确注入三种响应：缺片 / 成功 / 5xx 瞬时错误。

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
)

func newCompleteTestServer(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := NewClient(srv.URL)
	return c, srv
}

func TestCompleteChunksParsesMissingChunks(t *testing.T) {
	var calls int32
	c, srv := newCompleteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		// 第一次：缺片；第二次：成功
		if atomic.LoadInt32(&calls) == 1 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"isSuccess": true,
				"errorMsg":  "",
				"data": map[string]interface{}{
					"chunksComplete": false,
					"missingChunks":  []int{1, 3},
				},
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"isSuccess": true,
			"errorMsg":  "",
			"data": map[string]interface{}{
				"chunksComplete": true,
				"totalSize":      42,
			},
		})
	})
	defer srv.Close()

	missing, err := c.completeChunksOnce("p", "a.bin", 4, "MD5")
	if err != nil {
		t.Fatalf("首次 complete 不应报错（缺片是业务状态）: %v", err)
	}
	if len(missing) != 2 || missing[0] != 1 || missing[1] != 3 {
		t.Fatalf("missingChunks 解析错误: %v", missing)
	}

	missing, err = c.completeChunksOnce("p", "a.bin", 4, "MD5")
	if err != nil {
		t.Fatalf("第二次 complete 报错: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("合并成功后不应返回缺失分片: %v", missing)
	}
}

func TestCompleteChunksNotSuccessIsError(t *testing.T) {
	c, srv := newCompleteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"isSuccess": false,
			"errorMsg":  "合并结果 MD5 不符: 期望 X，实际 Y",
			"data":      nil,
		})
	})
	defer srv.Close()

	if _, err := c.completeChunksOnce("p", "a.bin", 2, "X"); err == nil {
		t.Fatal("isSuccess=false 必须作为错误返回")
	}
}

func TestCompleteChunksRetriesOnTransient(t *testing.T) {
	var calls int32
	c, srv := newCompleteTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 2 {
			// 前一次瞬时失败（5xx）
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprint(w, "bad gateway")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"isSuccess": true,
			"errorMsg":  "",
			"data":      map[string]interface{}{"chunksComplete": true},
		})
	})
	defer srv.Close()

	missing, err := c.completeChunksWithRetry("p", "a.bin", 2, "")
	if err != nil {
		t.Fatalf("瞬时错误应被重试后成功: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("应无缺失分片，实际 %v", missing)
	}
	if atomic.LoadInt32(&calls) < 2 {
		t.Fatalf("未发生重试（调用次数 %d）", calls)
	}
}

// TestFileMD5 校验分块读取实现的正确性（空文件/多块边界）
func TestFileMD5(t *testing.T) {
	cases := []struct {
		name    string
		content []byte
	}{
		{"empty", []byte{}},
		{"small", []byte("hello world")},
		{"multi-megabyte", make([]byte, 3<<20)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := t.TempDir() + "/f.bin"
			if err := os.WriteFile(p, c.content, 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			got, err := fileMD5(p)
			if err != nil {
				t.Fatalf("md5: %v", err)
			}
			sum := md5.Sum(c.content)
			want := hex.EncodeToString(sum[:])
			if got != want {
				t.Fatalf("MD5 不一致: got %s want %s", got, want)
			}
		})
	}
}
