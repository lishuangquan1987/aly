package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// commandUnderTest 9 个命令入口（A6 全覆盖）。
func commandUnderTest(name string) func() {
	switch name {
	case "check_update":
		return CheckUpdate
	case "check_diff":
		return CheckDiff
	case "download_update":
		return DownloadUpdate
	case "apply_update":
		return ApplyUpdate
	case "list_rollback_versions":
		return ListRollbackVersions
	case "rollback":
		return Rollback
	case "clean":
		return Clean
	case "check_self_update":
		return CheckSelfUpdate
	case "self_update":
		return SelfUpdate
	}
	return nil
}

var allCommandNames = []string{
	"check_update", "check_diff", "download_update", "apply_update",
	"list_rollback_versions", "rollback", "clean", "check_self_update", "self_update",
}

// testBinaryPath 在包初始化时记下测试二进制路径：多个命令级测试会改写 os.Args，
// 若在测试执行期读取 os.Args[0] 会拿到被改写后的 "aly-client" 而启动失败。
var testBinaryPath = os.Args[0]

// TestCommandFlagParseErrorExits2WithJSON A6：未知 flag 时 stdout 必须有 JSON 信封且退出码 2
// （历史 #1 事故：flag.ExitOnError 直接 os.Exit(2) 且不输出 JSON，宿主静默无效）。
// 通过测试二进制自拉起子进程验证 exit code（os.Exit 无法在测试进程内断言）。
func TestCommandFlagParseErrorExits2WithJSON(t *testing.T) {
	for _, name := range allCommandNames {
		name := name
		t.Run(name, func(t *testing.T) {
			out, code := runHelperProcess(t, name, "--definitely-unknown-flag-from-contract-test")
			if code != 2 {
				t.Fatalf("未知 flag 的退出码应为 2，实际 %d（stdout=%q）", code, out)
			}
			if !strings.Contains(out, `"isSuccess":false`) {
				t.Errorf("stdout 必须有 JSON 错误信封，实际: %q", out)
			}
			if !strings.Contains(out, "参数解析失败") {
				t.Errorf("errorMsg 应说明参数解析失败，实际: %q", out)
			}
		})
	}
}

// TestCommandHelpExits0 A6：-h 打印 usage 后 exit 0（不输出 JSON 失败信封）。
func TestCommandHelpExits0(t *testing.T) {
	for _, name := range allCommandNames {
		name := name
		t.Run(name, func(t *testing.T) {
			out, code := runHelperProcess(t, name, "-h")
			if code != 0 {
				t.Fatalf("-h 的退出码应为 0，实际 %d（stdout=%q）", code, out)
			}
			if !strings.Contains(out, "用法: aly-client "+name) {
				t.Errorf("-h 应打印命令 usage，实际: %q", out)
			}
			if strings.Contains(out, `"isSuccess":false`) {
				t.Errorf("-h 不应输出失败 JSON，实际: %q", out)
			}
		})
	}
}

// runHelperProcess 以子进程方式执行 TestCommandFlagParseHelper，返回 stdout 与退出码。
// 命令参数经环境变量传递：测试二进制自身的 flag 解析会先吃掉 -h / 未知 flag，
// 直接放在命令行上无法到达命令入口。
func runHelperProcess(t *testing.T, command string, arg string) (string, int) {
	t.Helper()
	cmd := exec.Command(testBinaryPath, "-test.run=TestCommandFlagParseHelper")
	cmd.Env = append(os.Environ(),
		"ALY_CMD_UNDER_TEST="+command,
		"ALY_CMD_ARG="+arg,
	)
	out, err := cmd.Output()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("启动子进程失败: %v", err)
		}
	}
	return string(out), code
}

// TestCommandFlagParseHelper 仅在子进程（带 ALY_CMD_UNDER_TEST）里真正执行命令入口；
// 父进程正常运行时直接跳过。命令入口在参数解析阶段就退出，不会触碰真实配置。
func TestCommandFlagParseHelper(t *testing.T) {
	name := os.Getenv("ALY_CMD_UNDER_TEST")
	if name == "" {
		t.Skip("仅作为子进程 helper 运行")
	}
	fn := commandUnderTest(name)
	if fn == nil {
		t.Fatalf("未知命令 %q", name)
	}
	os.Args = []string{"aly-client", name, os.Getenv("ALY_CMD_ARG")}
	fn()
}
