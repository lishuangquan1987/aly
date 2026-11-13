package util

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAppendToLogRotatesWhenTooLarge 轻微项：常驻日志（update.log/download.log/check_diff.log）
// 无轮转且 clean 不清本身 → 无限增长。超过 5MB 时 AppendToLog 必须把当前日志改名为 .1，
// 新日志从零开始写。
func TestAppendToLogRotatesWhenTooLarge(t *testing.T) {
	dir, err := ioutil.TempDir("", "log-rotate")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)

	logPath := filepath.Join(dir, "update.log")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("创建日志失败: %v", err)
	}
	// 用 Truncate 造一个超过阈值的大文件（稀疏文件，快）
	if err := f.Truncate(maxLogSize + 1); err != nil {
		f.Close()
		t.Fatalf("截断日志失败: %v", err)
	}
	f.Close()
	// 上一代已存在：轮转应覆盖它
	oldRotated := logPath + ".1"
	if err := ioutil.WriteFile(oldRotated, []byte("previous-generation"), 0644); err != nil {
		t.Fatalf("写上一代日志失败: %v", err)
	}

	AppendToLog(dir, "update.log", "new line")

	info, err := os.Stat(oldRotated)
	if err != nil {
		t.Fatalf("轮转后的 .1 日志不存在: %v", err)
	}
	if info.Size() <= maxLogSize {
		t.Errorf(".1 应是大文件（> %d），实际 %d", maxLogSize, info.Size())
	}
	data, err := ioutil.ReadFile(logPath)
	if err != nil {
		t.Fatalf("读新日志失败: %v", err)
	}
	if string(data) != "new line\n" {
		t.Errorf("轮转后新日志应只包含本次行，实际 %q", data)
	}
}

// TestAppendToLogNoRotateWhenSmall 小日志按原逻辑追加，不产生 .1。
func TestAppendToLogNoRotateWhenSmall(t *testing.T) {
	dir, err := ioutil.TempDir("", "log-norotate")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(dir)

	AppendToLog(dir, "download.log", "line1")
	AppendToLog(dir, "download.log", "line2")

	if _, err := os.Stat(filepath.Join(dir, "download.log.1")); err == nil {
		t.Errorf("小日志不应轮转出 .1")
	}
	data, err := ioutil.ReadFile(filepath.Join(dir, "download.log"))
	if err != nil {
		t.Fatalf("读日志失败: %v", err)
	}
	if !strings.Contains(string(data), "line1") || !strings.Contains(string(data), "line2") {
		t.Errorf("两行都应保留，实际 %q", data)
	}
}
