package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// TestCleanRemovesMainFolderAsideFamily M9 兜底：MainFolder 本体被隐式旁移产生的
// {folderName}.old / .old.N 不匹配 {folderName}_ 前缀规则，clean 必须单独收走，
// 否则永久泄漏（且 MainFolder 本体/正常快照不得被误删）。
func TestCleanRemovesMainFolderAsideFamily(t *testing.T) {
	root, fc := newCleanTestEnv(t, "clean-main-aside")

	// 旁移残留（MainFolder 本体改名后的 .old 家族）
	aside1 := filepath.Join(root, "ApplicationFolder.old")
	aside2 := filepath.Join(root, "ApplicationFolder.old.1")
	for _, d := range []string{aside1, aside2} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("创建旁移残留失败: %v", err)
		}
		if err := ioutil.WriteFile(filepath.Join(d, "leak.txt"), []byte("x"), 0644); err != nil {
			t.Fatalf("写残留文件失败: %v", err)
		}
	}
	// 正常快照必须保留
	writePkgFile(t, root, "ApplicationFolder_1.0/v1.txt", "v1")

	if err := cleanTempFiles(fc); err != nil {
		t.Fatalf("cleanTempFiles: %v", err)
	}

	for _, d := range []string{aside1, aside2} {
		if _, err := os.Stat(d); err == nil {
			t.Errorf("MainFolder 旁移残留未清理: %s", d)
		}
	}
	if _, err := os.Stat(fc.MainFolder); err != nil {
		t.Errorf("MainFolder 本体被误删: %v", err)
	}
	assertPkgExists(t, root, "ApplicationFolder_1.0/v1.txt")
}

// TestCleanRemovesSteadyLogs 轻微项：常驻日志 update.log / download.log / check_diff.log
// （含轮转出的 .1 代）超过 30 天应被 clean 回收；近期日志保留。
func TestCleanRemovesSteadyLogs(t *testing.T) {
	root, fc := newCleanTestEnv(t, "clean-steady-logs")
	update := filepath.Join(root, "UpdateFolder")

	old := time.Now().AddDate(0, 0, -40)
	// 注：update.log 本身不列入——clean 自身的操作日志会写入 update.log 并刷新 mtime，
	// 无法稳定断言"被回收"；update.log 的轮转由 util 层单测覆盖，clean 对常驻日志的
	// 回收用 update.log.1（轮转代）与 download.log/check_diff.log 验证。
	staleLogs := []string{"update.log.1", "download.log", "check_diff.log"}
	for _, name := range staleLogs {
		p := filepath.Join(update, name)
		if err := ioutil.WriteFile(p, []byte("OLD-"+name+"\n"), 0644); err != nil {
			t.Fatalf("写日志 %s 失败: %v", name, err)
		}
		if err := os.Chtimes(p, old, old); err != nil {
			t.Fatalf("设置日志时间失败: %v", err)
		}
	}

	if err := cleanTempFiles(fc); err != nil {
		t.Fatalf("cleanTempFiles: %v", err)
	}

	// 过期常驻日志（含轮转代）应被回收
	for _, name := range staleLogs {
		if _, err := os.Stat(filepath.Join(update, name)); err == nil {
			t.Errorf("过期常驻日志未清理: %s", name)
		}
	}
}
