//go:build !windows

package tc

// 非 windows 平台：aly-client-ui.exe 本身就无法运行，这里给 no-op 实现
// 让测试模块在各平台都能编译通过（用例内部会因 exe 不可执行而 SKIP）。

import "time"

func dismissMessageBox(title string, wait time.Duration) bool { return false }

func messageBoxExists(title string) bool { return false }