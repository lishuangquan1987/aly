package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aly/client/aly-client/config"
)

// TestMergeMustCloseFlag 验证 #1 修复：--must-close-process-name（逗号分隔）
// 解析后合并进配置，且不会污染空 flag / 重复追加。
func TestMergeMustCloseFlag(t *testing.T) {
	fc := &FullConfig{ExeCfg: &config.Config{MustCloseProcessName: []string{"YourApp"}}}

	// 空 flag：不修改
	mergeMustCloseFlag(fc, "")
	if len(fc.ExeCfg.MustCloseProcessName) != 1 {
		t.Fatalf("空 flag 不应修改配置，实际 %v", fc.ExeCfg.MustCloseProcessName)
	}

	// 逗号分隔 + 空白：追加去空白
	mergeMustCloseFlag(fc, "cmd.exe, ,conhost")
	want := []string{"YourApp", "cmd.exe", "conhost"}
	if len(fc.ExeCfg.MustCloseProcessName) != len(want) {
		t.Fatalf("合并后长度应为 %d，实际 %d (%v)", len(want), len(fc.ExeCfg.MustCloseProcessName), fc.ExeCfg.MustCloseProcessName)
	}
	for i := range want {
		if fc.ExeCfg.MustCloseProcessName[i] != want[i] {
			t.Errorf("第 %d 项应为 %q，实际 %q", i, want[i], fc.ExeCfg.MustCloseProcessName[i])
		}
	}
}

// TestNextAsideNameStripsOldSuffix 验证 #18 修复：对已是 X.old 的路径，
// 旁移命名应剥掉 .old 后缀收敛到 X.old / X.old.1 家族，绝不生成 X.old.old。
func TestNextAsideNameStripsOldSuffix(t *testing.T) {
	root, err := ioutil.TempDir("", "aside-strip-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	// to = X.old 且 X.old 不存在：应返回 X.old（剥离后缀后 base=X）
	to := filepath.Join(root, "ApplicationFolder_1.0.old")
	if got := nextAsideName(to); got != filepath.Join(root, "ApplicationFolder_1.0.old") {
		t.Errorf("X.old 不存在时应返回自身（base=X 的 X.old），实际 %q", got)
	}

	// to = X.old 且 X.old 存在：应返回 X.old.1，而不是 X.old.old
	if err := os.MkdirAll(to, 0755); err != nil {
		t.Fatalf("创建 X.old 失败: %v", err)
	}
	got := nextAsideName(to)
	if got == filepath.Join(root, "ApplicationFolder_1.0.old.old") {
		t.Errorf("不应生成 X.old.old 链式名，实际 %q", got)
	}
	if got != filepath.Join(root, "ApplicationFolder_1.0.old.1") {
		t.Errorf("X.old 存在时旁移名应为 X.old.1，实际 %q", got)
	}
}

// TestRemoveAsideVariants 验证 #18 修复：清理 X.old / X.old.N 全部旁移变体，
// 且不误删 base 本身。
func TestRemoveAsideVariants(t *testing.T) {
	root, err := ioutil.TempDir("", "aside-remove-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	base := filepath.Join(root, "ApplicationFolder_1.0")
	if err := os.MkdirAll(base, 0755); err != nil {
		t.Fatalf("创建 base 失败: %v", err)
	}
	for _, p := range []string{base + ".old", base + ".old.1", base + ".old.2"} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatalf("创建旁移目录 %s 失败: %v", p, err)
		}
	}
	removeAsideVariants(base)

	for _, p := range []string{base + ".old", base + ".old.1", base + ".old.2"} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("旁移目录 %s 应被清理: %v", p, err)
		}
	}
	// base 本身不能被误删
	if _, err := os.Stat(base); err != nil {
		t.Errorf("base %s 不应被删除: %v", base, err)
	}
}

// mustMkdirFile 创建目录并在其中写入一个文件
func mustMkdirFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll(%s) 失败: %v", dir, err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%s) 失败: %v", filepath.Join(dir, name), err)
	}
}

// TestRenameDirWithKillOverExistingTarget 回归测试：
// 目标目录已存在且非空（上一次更新留下的旧版本备份）时，
// 直接 os.Rename 会报 Access denied（MoveFileEx 无法覆盖非空目录，与占用无关）。
// renameDirWithKill 应先把目标挪到 to+".old"，再把 from 重命名为 to。
func TestRenameDirWithKillOverExistingTarget(t *testing.T) {
	root, err := ioutil.TempDir("", "rename-kill-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "win-x64")
	to := filepath.Join(root, "win-x64_1.0")

	// 源目录（当前主程序目录）
	mustMkdirFile(t, from, "app.exe", "new app")
	// 目标目录已存在且非空（旧版本备份）
	mustMkdirFile(t, to, "old.exe", "old app")

	if err := renameDirWithKill(from, to, 5*time.Second); err != nil {
		t.Fatalf("目标存在时应自动挪开后成功，实际失败: %v", err)
	}

	// from 应已不存在，内容移动到 to
	if _, err := os.Stat(from); !os.IsNotExist(err) {
		t.Errorf("源目录 %s 应已不存在", from)
	}
	if _, err := os.Stat(filepath.Join(to, "app.exe")); err != nil {
		t.Errorf("目标 %s 应包含新内容 app.exe: %v", to, err)
	}
	// 旧备份应被挪到 to.old
	aside := to + ".old"
	if _, err := os.Stat(filepath.Join(aside, "old.exe")); err != nil {
		t.Errorf("旧备份应被挪到 %s (old.exe): %v", aside, err)
	}
}

// TestRenameDirWithKillNoTarget 验证目标不存在时正常重命名，不产生 .old 目录。
func TestRenameDirWithKillNoTarget(t *testing.T) {
	root, err := ioutil.TempDir("", "rename-kill-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "win-x64")
	to := filepath.Join(root, "win-x64_2.0")
	mustMkdirFile(t, from, "app.exe", "new app")

	if err := renameDirWithKill(from, to, 5*time.Second); err != nil {
		t.Fatalf("renameDirWithKill 失败: %v", err)
	}
	if _, err := os.Stat(filepath.Join(to, "app.exe")); err != nil {
		t.Errorf("目标 %s 应包含 app.exe: %v", to, err)
	}
	if _, err := os.Stat(to + ".old"); !os.IsNotExist(err) {
		t.Errorf("目标不存在时不应产生 %s.old", to)
	}
}

// TestRenameDirWithKillAsideCollision 验证目标已存在且 to.old 也被占用（上次残留）时，
// 会把目标挪到下一个不冲突的名称 to.old.1，而不是失败或覆盖。
func TestRenameDirWithKillAsideCollision(t *testing.T) {
	root, err := ioutil.TempDir("", "rename-kill-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "win-x64")
	to := filepath.Join(root, "win-x64_1.0")

	mustMkdirFile(t, from, "app.exe", "new app")
	mustMkdirFile(t, to, "old.exe", "old app")
	// 上次残留的 to.old 也存在
	mustMkdirFile(t, to+".old", "residue.exe", "residue")

	if err := renameDirWithKill(from, to, 5*time.Second); err != nil {
		t.Fatalf("目标与 .old 均存在时应挪到 .old.1 后成功，实际失败: %v", err)
	}

	// from → to
	if _, err := os.Stat(filepath.Join(to, "app.exe")); err != nil {
		t.Errorf("目标 %s 应包含 app.exe: %v", to, err)
	}
	// 原目标被挪到 to.old.1
	if _, err := os.Stat(filepath.Join(to+".old.1", "old.exe")); err != nil {
		t.Errorf("原目标应被挪到 %s (old.exe): %v", to+".old.1", err)
	}
	// 上次残留的 to.old 保持不变
	if _, err := os.Stat(filepath.Join(to+".old", "residue.exe")); err != nil {
		t.Errorf("残留的 %s 应保持不动: %v", to+".old", err)
	}
}

// TestRenameDirWithKillSourceMissing 验证源文件夹不存在时直接报错（规则 1）。
func TestRenameDirWithKillSourceMissing(t *testing.T) {
	root, err := ioutil.TempDir("", "rename-kill-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	from := filepath.Join(root, "not-exist")
	to := filepath.Join(root, "win-x64_1.0")

	if err := renameDirWithKill(from, to, 5*time.Second); err == nil {
		t.Fatal("源文件夹不存在时应返回错误")
	}
}
