package main

import (
	"os"
	"testing"
)

// withArgs 临时替换 os.Args（宿主 PID 解析直接读 os.Args）。
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
		{"absent", []string{"aly-client-ui", "-mode", "auto"}, 0},
		{"double-dash", []string{"aly-client-ui", "--host-pid", "4242", "-mode", "auto"}, 4242},
		{"single-dash", []string{"aly-client-ui", "-host-pid", "4242"}, 4242},
		{"last wins", []string{"aly-client-ui", "--host-pid", "111", "--host-pid", "4242"}, 4242},
		{"invalid falls back to zero", []string{"aly-client-ui", "--host-pid", "abc"}, 0},
		{"zero is unknown", []string{"aly-client-ui", "--host-pid", "0"}, 0},
		{"missing value", []string{"aly-client-ui", "--host-pid"}, 0},
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
