package cmd

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aly/client/aly-client/config"
)

// dlFile 是假服务端上的一个文件。
type dlFile struct {
	rel     string // fileRelativePath
	abs     string // fileAbsolutePath（客户端下载时原样回传）
	content string
}

func (f dlFile) md5Hex() string {
	sum := md5.Sum([]byte(f.content))
	return hex.EncodeToString(sum[:])
}

func (f dlFile) sha256Hex() string {
	sum := sha256.Sum256([]byte(f.content))
	return hex.EncodeToString(sum[:])
}

// newDownloadServer 起一个覆盖 download_update 所需接口的假服务端：
// get_project_change_logs / get_all_files / get_release_manifest / download_file。
func newDownloadServer(t *testing.T, version, script string, files []dlFile) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		switch {
		case strings.Contains(path, "get_project_change_logs"):
			scriptJSON, _ := json.Marshal(script)
			fmt.Fprintf(w, `{"isSuccess":true,"errorMsg":"","data":[{"id":1,"version":%q,"after_apply_update_script":%s}]}`,
				version, string(scriptJSON))
		case strings.Contains(path, "get_all_files"):
			items := make([]string, 0, len(files))
			for _, f := range files {
				items = append(items, fmt.Sprintf(
					`{"fileAbsolutePath":%q,"fileRelativePath":%q,"fileSize":%d,"md5":%q,"sha256":%q}`,
					f.abs, f.rel, len(f.content), f.md5Hex(), f.sha256Hex()))
			}
			fmt.Fprintf(w, `{"isSuccess":true,"errorMsg":"","data":[%s]}`, strings.Join(items, ","))
		case strings.Contains(path, "get_release_manifest"):
			rels := make([]string, 0, len(files))
			for _, f := range files {
				rels = append(rels, f.rel)
			}
			body, _ := json.Marshal(rels)
			fmt.Fprintf(w, `{"isSuccess":true,"errorMsg":"","data":{"files":%s}}`, string(body))
		case strings.Contains(path, "download_file"):
			want := r.URL.Query().Get("path")
			for _, f := range files {
				if strings.HasSuffix(want, filepath.Base(f.abs)) {
					w.Write([]byte(f.content))
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestDownloadUpdateToleratesMissingMainFolder M6：主目录缺失（applying 崩溃等待重下补齐）时，
// materialize 无源可拷 → 必须跳过复制并留痕，而不是让整轮下载失败（否则 check 的
// "重下补齐"出口被堵死，形成"重下→失败→清空"永不收敛的活锁）。
func TestDownloadUpdateToleratesMissingMainFolder(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "main", "a.txt")
	files := []dlFile{{rel: "a.txt", abs: abs, content: "hello"}}
	srv := newDownloadServer(t, "2.0", "", files)

	root := setupCheckUpdateLayout(t, `{
  "version_previous": "1.0",
  "version": "1.0",
  "version_status": "applied"
}`, srv.URL)
	// 主目录缺失：但保留一个版本目录的 shared.json 供 loadFullConfig 回退
	if err := os.RemoveAll(filepath.Join(root, "ApplicationFolder")); err != nil {
		t.Fatalf("删除主目录失败: %v", err)
	}
	writePkgFile(t, root, "ApplicationFolder_0.9/.updator/shared.json",
		`{"server_url":"`+srv.URL+`","project_name":"test"}`)

	os.Args = []string{"aly-client", "download_update"}
	out := captureStdout(t, DownloadUpdate)

	if !strings.Contains(out, `"isSuccess":true`) {
		t.Fatalf("主目录缺失时下载应成功（跳过物化），实际输出: %s", out)
	}
	vi, err := readVersionSafe()
	if err != nil {
		t.Fatalf("ReadVersion 失败: %v", err)
	}
	if vi.VersionStatus != "downloaded" || vi.Version != "2.0" {
		t.Errorf("应写入 downloaded/2.0，实际 %+v", vi)
	}
	if got := readFileOrFail(t, filepath.Join(root, "ApplicationFolder_2.0", "a.txt")); got != "hello" {
		t.Errorf("下载产物内容错误: %q", got)
	}
	logData, err := ioutil.ReadFile(filepath.Join(root, "UpdateFolder", "download.log"))
	if err != nil {
		t.Fatalf("读 download.log 失败: %v", err)
	}
	if !strings.Contains(string(logData), "materialize skipped") {
		t.Errorf("跳过物化必须留痕，实际日志: %s", logData)
	}
}

// TestDownloadUpdateRejectsUnsafeRelativePath M12：服务端下发 `../evil.txt` 时必须拒绝并入
// 失败清单，绝不能在目标版本目录之外落盘。
func TestDownloadUpdateRejectsUnsafeRelativePath(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "main", "evil.txt")
	// 合法文件 + 非法相对路径文件
	files := []dlFile{
		{rel: "ok.txt", abs: filepath.Join(t.TempDir(), "main", "ok.txt"), content: "ok"},
		{rel: "../evil.txt", abs: abs, content: "evil"},
	}
	srv := newDownloadServer(t, "2.0", "", files)
	root := setupCheckUpdateLayout(t, `{
  "version_previous": "1.0",
  "version": "1.0",
  "version_status": "applied"
}`, srv.URL)

	os.Args = []string{"aly-client", "download_update"}
	out := captureStdout(t, DownloadUpdate)

	if !strings.Contains(out, `"isSuccess":false`) {
		t.Fatalf("存在非法相对路径时应整体失败，实际输出: %s", out)
	}
	if !strings.Contains(out, "unsafe path") {
		t.Errorf("失败清单应包含非法路径，实际输出: %s", out)
	}
	// 版本目录之外不得出现文件（../evil.txt 逃逸点 = root/evil.txt）
	if _, err := os.Stat(filepath.Join(root, "evil.txt")); err == nil {
		t.Errorf("非法相对路径在版本目录外落盘了（M12 失效）: %s", filepath.Join(root, "evil.txt"))
	}
	// 版本目录内不得出现 evil.txt 的副本
	if _, err := os.Stat(filepath.Join(root, "ApplicationFolder_2.0", "evil.txt")); err == nil {
		t.Errorf("非法相对路径不应被下载")
	}
}

// TestDownloadUpdateFiltersUnsafeScriptPath M12：after_apply_update_script 非法时不得写入
// version.json（apply 阶段会执行它）；合法路径保持原样。
func TestDownloadUpdateFiltersUnsafeScriptPath(t *testing.T) {
	cases := []struct {
		name   string
		script string
		want   string
	}{
		{"合法子目录脚本", "scripts/post.bat", "scripts/post.bat"},
		{"父目录穿越脚本", "../evil.bat", ""},
		{"绝对路径脚本", "C:\\evil.bat", ""},
		{"空脚本", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			abs := filepath.Join(t.TempDir(), "main", "a.txt")
			files := []dlFile{{rel: "a.txt", abs: abs, content: "hello"}}
			srv := newDownloadServer(t, "2.0", c.script, files)
			setupCheckUpdateLayout(t, `{
  "version_previous": "1.0",
  "version": "1.0",
  "version_status": "applied"
}`, srv.URL)

			os.Args = []string{"aly-client", "download_update"}
			out := captureStdout(t, DownloadUpdate)
			if !strings.Contains(out, `"isSuccess":true`) {
				t.Fatalf("下载应成功，实际输出: %s", out)
			}
			vi, err := readVersionSafe()
			if err != nil {
				t.Fatalf("ReadVersion 失败: %v", err)
			}
			if vi.AfterApplyUpdateScript != c.want {
				t.Errorf("after_apply_update_script = %q, want %q", vi.AfterApplyUpdateScript, c.want)
			}
		})
	}
}

// readVersionSafe 读取 version.json（包一层便于测试断言）。
func readVersionSafe() (*config.VersionInfo, error) {
	return config.ReadVersion()
}
