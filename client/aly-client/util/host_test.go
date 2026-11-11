package util

import (
	"testing"
)

// hostCase 是宿主跟随退出决策表的一行。
type hostCase struct {
	isApplyCommand bool
	applying       bool
	want            bool
}

// TestShouldExitOnHostGone 决策表：只有「宿主已退出 且 不在 apply 例外窗口」才退。
// 两个例外（命令级 apply_update、状态级 applying）取或，任一成立都不退。
func TestShouldExitOnHostGone(t *testing.T) {
	cases := []hostCase{
		{isApplyCommand: false, applying: false, want: true},
		{isApplyCommand: false, applying: true, want: false},
		{isApplyCommand: true, applying: false, want: false},
		{isApplyCommand: true, applying: true, want: false},
	}
	for _, c := range cases {
		got := ShouldExitOnHostGone(c.isApplyCommand, c.applying)
		if got != c.want {
			t.Errorf("isApplyCommand=%v applying=%v: got %v, want %v",
				c.isApplyCommand, c.applying, got, c.want)
		}
	}
}

// TestHostProcessIDNoPanic 宿主 PID 查询必须可调用、不 panic。
// 拿不到时返回 0 是合法结果（调用方据此不做跟随退出）。
func TestHostProcessIDNoPanic(t *testing.T) {
	_ = HostProcessID()
}
