// +build windows

package app

import (
	"os"
	"testing"
)

// withArgs 临时替换 os.Args（宿主 PID 解析直接读 os.Args）。
// 返回恢复函数（Go 1.10 兼容：不用 t.Cleanup，调用方 defer）。
func withArgs(args []string) func() {
	orig := os.Args
	os.Args = append([]string(nil), args...)
	return func() { os.Args = orig }
}

// TestHostPidFromArgs 收尾重启链传下来的原始宿主 PID 解析：
// 取最后一个匹配；--host-pid 与 -host-pid 等价；缺失/非法/为 0 回落 0。
func TestHostPidFromArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want uint32
	}{
		{"absent", []string{"ui.exe", "-mode", "auto"}, 0},
		{"double-dash", []string{"ui.exe", "--host-pid", "4242", "-mode", "auto"}, 4242},
		{"single-dash", []string{"ui.exe", "-host-pid", "4242"}, 4242},
		{"last wins", []string{"ui.exe", "--host-pid", "111", "--host-pid", "4242"}, 4242},
		{"invalid falls back to zero", []string{"ui.exe", "--host-pid", "abc"}, 0},
		{"zero is unknown", []string{"ui.exe", "--host-pid", "0"}, 0},
		{"missing value", []string{"ui.exe", "--host-pid"}, 0},
		{"restart args position", []string{"ui.exe", "-mode", "auto", "--host-pid", "777", "-lang", "zh-CN"}, 777},
	}
	for _, c := range cases {
		restore := withArgs(c.args)
		got := hostPidFromArgs()
		restore()
		if got != c.want {
			t.Fatalf("%s: hostPidFromArgs() = %d; want %d", c.name, got, c.want)
		}
	}
}

// TestStripHostPIDArgs 收尾进程在追加规范 --host-pid 前必须剥掉 restartArgs
// 里残留的旧值（连续两次自更新时会出现两份），避免重复/过期 PID。
func TestStripHostPIDArgs(t *testing.T) {
	got := stripHostPIDArgs([]string{
		"-mode", "auto", "--host-pid", "111", "-lang", "zh-CN", "-host-pid", "222",
	})
	want := []string{"-mode", "auto", "-lang", "zh-CN"}
	if len(got) != len(want) {
		t.Fatalf("stripHostPIDArgs = %#v; want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("stripHostPIDArgs[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}
