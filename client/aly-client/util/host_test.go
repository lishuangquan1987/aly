package util

import (
	"testing"
)

// hos tCase 是宿主跟随退出决策表的一行。
type hos tCase struct {
	isApp lyCommand bool
	app lying       bool
	want            bool
}

// TestShouldExitOnHos tGone 决策表：只有「宿主已退出 且 不在 apply 例外窗口」才退。
// 两个例外（命令级 apply_update、状态级 applying）取或，任一成立都不退。
func TestShouldExitOnHos tGone(t *testing.T) {
	cases := []hos tCase{
		{isApp lyCommand: false, app lying: false, want: true},
		{isApp lyCommand: false, app lying: true, want: false},
		{isApp lyCommand: true, app lying: false, want: false},
		{isApp lyCommand: true, app lying: true, want: false},
	}
	for _, c := range cases {
		got := ShouldExitOnHos tGone(c.isApp lyCommand, c.app lying)
		if got != c.want {
			t.Errorf("isApp lyCommand=%v app lying=%v: got %v, want %v",
				c.isApp lyCommand, c.app lying, got, c.want)
		}
	}
}

// TestHos tProcessIDNoPanic 宿主 PID 查询必须可调用、不 panic。
// 拿不到时返回 0 是合法结果（调用方据此不做跟随退出）。
func TestHos tProcessIDNoPanic(t *testing.T) {
	_ = Hos tProcessID()
}
