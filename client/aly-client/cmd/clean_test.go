package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"aly/client/aly-client/config"
)

// newCleanTestEnv 搭建 clean 测试环境：
// root/UpdateFolder（ExeDir）+ root/ApplicationFolder（MainFolder）
// + 若干版本快照目录。返回 root 与 fc。
func newCleanTestEnv(t *testing.T, name string) (string, *FullConfig) {
	root, err := ioutil.TempDir("", name)
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	update := filepath.Join(root, "UpdateFolder")
	if err := os.MkdirAll(update, 0755); err != nil {
		t.Fatalf("创建 UpdateFolder 失败: %v", err)
	}
	config.SetExeDir(update)
	t.Cleanup(func() { config.SetExeDir(""); os.RemoveAll(root) })

	if err := ioutil.WriteFile(filepath.Join(update, "client.json"),
		[]byte(`{"main_exe_relative_path":"../ApplicationFolder/app.exe","must_close_process_name":[]}`), 0644); err != nil {
		t.Fatalf("写 client.json 失败: %v", err)
	}
	main := filepath.Join(root, "ApplicationFolder")
	if err := os.MkdirAll(main, 0755); err != nil {
		t.Fatalf("创建 MainFolder 失败: %v", err)
	}
	fc := &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: main,
	}
	return root, fc
}

// TestCleanTempFilesNotTouchMainFolder（#11）：clean 不得删除 MainFolder 内的
// .tmp/.part 文件——业务程序（老 Win32/办公软件）会持久持有 .tmp 数据文件，
// 全量递归会误删；快照目录与 .updator 内的残留仍应清理。
func TestCleanTempFilesNotTouchMainFolder(t *testing.T) {
	root, fc := newCleanTestEnv(t, "clean-mainfolder")

	// 1) MainFolder 内的业务 .tmp：必须保留
	businessTmp := filepath.Join(fc.MainFolder, "business_data.tmp")
	if err := ioutil.WriteFile(businessTmp, []byte("x"), 0644); err != nil {
		t.Fatalf("写业务 .tmp 失败: %v", err)
	}
	// 嵌套子目录里的业务 .tmp：同样必须保留（旧实现递归 MainFolder 会删）
	nested := filepath.Join(fc.MainFolder, "sub", "cache.tmp")
	if err := os.MkdirAll(filepath.Dir(nested), 0755); err != nil {
		t.Fatalf("创建嵌套目录失败: %v", err)
	}
	if err := ioutil.WriteFile(nested, []byte("x"), 0644); err != nil {
		t.Fatalf("写嵌套 .tmp 失败: %v", err)
	}

	// 2) 版本快照目录内的下载残留 .part：应删除
	snap := filepath.Join(root, "ApplicationFolder_2.0.0", "bin")
	if err := os.MkdirAll(snap, 0755); err != nil {
		t.Fatalf("创建快照目录失败: %v", err)
	}
	partInSnap := filepath.Join(snap, "half.bin.part")
	if err := ioutil.WriteFile(partInSnap, []byte("x"), 0644); err != nil {
		t.Fatalf("写快照 .part 失败: %v", err)
	}

	// 3) .updator 内的运行时临时文件：应删除
	updatorTmp := filepath.Join(fc.MainFolder, ".updator", "version.json.tmp")
	if err := os.MkdirAll(filepath.Dir(updatorTmp), 0755); err != nil {
		t.Fatalf("创建 .updator 失败: %v", err)
	}
	if err := ioutil.WriteFile(updatorTmp, []byte("{}"), 0644); err != nil {
		t.Fatalf("写 .updator .tmp 失败: %v", err)
	}

	// 4) pkgDir 顶层孤儿 .part：应删除
	orphanPart := filepath.Join(root, "orphan.part")
	if err := ioutil.WriteFile(orphanPart, []byte("x"), 0644); err != nil {
		t.Fatalf("写孤儿 .part 失败: %v", err)
	}

	if err := cleanTempFiles(fc); err != nil {
		t.Fatalf("cleanTempFiles: %v", err)
	}

	if _, err := os.Stat(businessTmp); err != nil {
		t.Errorf("MainFolder 内业务 .tmp 被误删（#11 回归）: %s", businessTmp)
	}
	if _, err := os.Stat(nested); err != nil {
		t.Errorf("MainFolder 嵌套目录内业务 .tmp 被误删（#11 回归）: %s", nested)
	}
	if _, err := os.Stat(partInSnap); err == nil {
		t.Errorf("快照目录内下载残留 .part 未清理: %s", partInSnap)
	}
	if _, err := os.Stat(updatorTmp); err == nil {
		t.Errorf(".updator 内运行时 .tmp 未清理: %s", updatorTmp)
	}
	if _, err := os.Stat(orphanPart); err == nil {
		t.Errorf("pkgDir 顶层孤儿 .part 未清理: %s", orphanPart)
	}
}
