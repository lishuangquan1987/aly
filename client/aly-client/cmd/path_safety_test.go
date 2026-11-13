package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aly/client/aly-client/config"
)

// TestSafeRelPath 表驱动覆盖 M12 路径白名单的全部边界：
// 非空 / 无 NUL / 非绝对 / 非盘符 / 非 UNC / 每段非空且非 ./..。
func TestSafeRelPath(t *testing.T) {
	cases := []struct {
		name string
		rel  string
		want bool
	}{
		{"空串", "", false},
		{"单点", ".", false},
		{"双点", "..", false},
		{"父目录穿越", "../x", false},
		{"中段穿越", "a/../../b", false},
		{"盘符 C", `C:\x`, false},
		{"盘符小写", `c:/x`, false},
		{"UNC", `\\srv\x`, false},
		{"双斜杠 UNC", `//srv/x`, false},
		{"前导斜杠绝对路径", "/x", false},
		{"空段", "a//b", false},
		{"末段为空", "a/b/", false},
		{"含 NUL", "a\x00b", false},
		{"单段点混合", "a/./b", false},
		{"合法单段", "app.exe", true},
		{"合法子目录", "bin/app.exe", true},
		{"合法 Windows 风格", `bin\app.exe`, true},
		{"合法带点文件名", "a.b/c.d", true},
		{"合法版本脚本", "scripts/post_2026.1.sh", true},
	}
	for _, c := range cases {
		if got := safeRelPath(c.rel); got != c.want {
			t.Errorf("safeRelPath(%q) = %v, want %v（%s）", c.rel, got, c.want, c.name)
		}
	}
}

// TestStatMissingOrErr 覆盖 M9 的三分类：
// 存在 / 确定不存在 / 其它 stat 错误（用非法字符路径模拟"状态不可读"）。
func TestStatMissingOrErr(t *testing.T) {
	root, err := ioutil.TempDir("", "stat-kind")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(root)

	// 存在
	missing, statErr := statMissingOrErr(root)
	if missing || statErr != nil {
		t.Errorf("存在的目录应为 (false, nil)，实际 (%v, %v)", missing, statErr)
	}
	// 确定不存在
	missing, statErr = statMissingOrErr(filepath.Join(root, "nope"))
	if !missing || statErr != nil {
		t.Errorf("不存在的路径应为 (true, nil)，实际 (%v, %v)", missing, statErr)
	}
	// 其它错误：Windows 下非法字符路径 stat 返回 ERROR_INVALID_NAME（非 IsNotExist）。
	// 该用例在 Unix 上无非法字符，跳过（Unix 侧无法用同样方式稳定注入）。
	if os.PathSeparator == '\\' {
		bad := filepath.Join(root, "Bad<>Name")
		missing, statErr = statMissingOrErr(bad)
		if missing || statErr == nil {
			t.Errorf("不可读路径应为 (false, err)，实际 (%v, %v)", missing, statErr)
		}
	}
}

// TestValidatePendingDir 覆盖 F3/A1 提取后的边界：A1（Version 空）、F3（缺失/空/残缺）、
// 正常（文件数达标）、老数据（DownloadedFileCount=0 的弱校验）。
func TestValidatePendingDir(t *testing.T) {
	cases := []struct {
		name       string
		versionJS  string
		files      []string // 版本目录内文件（不创建表示目录缺失）
		createDir  bool
		wantDir    bool
		wantIncomp bool
		wantErr    bool
	}{
		{
			name:       "A1 Version 为空",
			versionJS:  `{"version_previous":"1.0","version":"","version_status":"downloaded"}`,
			wantIncomp: true,
		},
		{
			name:       "F3 目录缺失",
			versionJS:  `{"version_previous":"1.0","version":"2.0","version_status":"downloaded"}`,
			wantIncomp: true,
			wantDir:    true,
		},
		{
			name:       "F3 目录存在但为空",
			versionJS:  `{"version_previous":"1.0","version":"2.0","version_status":"downloaded"}`,
			createDir:  true,
			wantIncomp: true,
			wantDir:    true,
		},
		{
			name:       "F3 文件数少于记录",
			versionJS:  `{"version_previous":"1.0","version":"2.0","version_status":"downloaded","downloaded_file_count":3}`,
			createDir:  true,
			files:      []string{"only.txt"},
			wantIncomp: true,
			wantDir:    true,
		},
		{
			name:       "正常文件数达标",
			versionJS:  `{"version_previous":"1.0","version":"2.0","version_status":"downloaded","downloaded_file_count":2}`,
			createDir:  true,
			files:      []string{"a.txt", "b.txt"},
			wantIncomp: false,
			wantDir:    true,
		},
		{
			name:       "老数据弱校验（记录 0）",
			versionJS:  `{"version_previous":"1.0","version":"2.0","version_status":"downloaded"}`,
			createDir:  true,
			files:      []string{"a.txt"},
			wantIncomp: false,
			wantDir:    true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pkg := setupRollbackLayout(t, c.versionJS)
			if c.createDir {
				dir := filepath.Join(pkg, "ApplicationFolder_2.0")
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("创建版本目录失败: %v", err)
				}
				for _, f := range c.files {
					if err := ioutil.WriteFile(filepath.Join(dir, f), []byte("x"), 0644); err != nil {
						t.Fatalf("写文件 %s 失败: %v", f, err)
					}
				}
			}
			fc := loadTestFullConfig(t, pkg)
			vi, err := config.ReadVersion()
			if err != nil {
				t.Fatalf("ReadVersion 失败: %v", err)
			}
			dir, incomplete, verr := validatePendingDir(fc, vi)
			if (verr != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr = %v", verr, c.wantErr)
			}
			if incomplete != c.wantIncomp {
				t.Errorf("incomplete = %v, want %v（dir=%q）", incomplete, c.wantIncomp, dir)
			}
			if c.wantDir != (dir != "") {
				t.Errorf("dir = %q, wantDir(非空) = %v", dir, c.wantDir)
			}
		})
	}
}

// loadTestFullConfig 构造与命令入口一致的 FullConfig（不重新读 client.json，便于单元测试）。
func loadTestFullConfig(t *testing.T, pkg string) *FullConfig {
	t.Helper()
	return &FullConfig{
		ExeCfg:     &config.Config{MainExeRelativePath: "../ApplicationFolder/app.exe"},
		MainFolder: filepath.Join(pkg, "ApplicationFolder"),
	}
}

// TestRunPostApplyScriptRejectsUnsafePath 验证 M12 对 after_apply_update_script 的接入：
// 非法相对路径（../ 逃逸）不执行、不写 marker，并在 update.log 留痕。
func TestRunPostApplyScriptRejectsUnsafePath(t *testing.T) {
	appDir, err := ioutil.TempDir("", "post-script-unsafe")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(appDir)
	config.SetExeDir(appDir)
	defer config.SetExeDir("")

	// 逃逸脚本放在主目录之外
	escapeDir, err := ioutil.TempDir("", "post-script-escape")
	if err != nil {
		t.Fatalf("创建逃逸目录失败: %v", err)
	}
	defer os.RemoveAll(escapeDir)
	escapeScript := writeScriptForPlatform(t, escapeDir, "evil",
		"@echo off\r\necho pwn > pwned.txt\r\n",
		"echo pwn > pwned.txt\n")
	rel := filepath.Join("..", filepath.Base(escapeDir), filepath.Base(escapeScript))

	fc := &FullConfig{MainFolder: appDir}
	runPostApplyScript(fc, rel, "2.0.0")

	if _, err := os.Stat(filepath.Join(escapeDir, "pwned.txt")); err == nil {
		t.Errorf("非法相对路径的脚本被执行了（M12 白名单失效）")
	}
	if _, err := os.Stat(filepath.Join(appDir, ".updator", "after_apply_2.0.0.done")); err == nil {
		t.Errorf("非法路径不应写 marker")
	}
	data, err := ioutil.ReadFile(filepath.Join(appDir, "update.log"))
	if err != nil {
		t.Fatalf("读 update.log 失败: %v", err)
	}
	if !strings.Contains(string(data), "M12 白名单拒绝") {
		t.Errorf("应留痕拒绝原因，实际日志: %s", data)
	}
}
