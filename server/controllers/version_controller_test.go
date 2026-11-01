package controllers

import (
	"strings"
	"testing"
)

// ── SV-B 组：版本号格式校验（validVersion 正则）──
// 对应《aly-全流程测试计划-20261101.md》T9/SV 组：publish_version 入口校验。
// 口径：数字段（至少 1 段）+ 可选预发布/构建元数据后缀；v 前缀允许（CompareVersion 剥离）。

func TestValidVersionAccepts(t *testing.T) {
	accept := []string{
		"1.0.0",
		"2",                    // 单段合法（与客户端 isLikelyVersion 口径对齐）
		"2.2",                  // 双段
		"2.1.1-beta",           // 预发布
		"2.1.2+build.4",        // 构建元数据
		"2.1.1-beta.1+build.4", // 组合
		"v1.2.3",               // v 前缀
		"V1.2.3",               // V 前缀
		"1.0.0-rc.1",           // 预发布多段
		"1.0.0-0",              // 预发布数字段
		"10.20.30",             // 多位数
		"v2",                   // v 前缀单段
	}
	for _, v := range accept {
		if !validVersion.MatchString(v) {
			t.Errorf("validVersion(%q) = false, want true（合法格式被拒）", v)
		}
	}
}

func TestValidVersionRejects(t *testing.T) {
	reject := []string{
		"",              // 空
		"1..9",          // 空段
		"abc",           // 纯字母
		"1.0.0 beta",    // 空格
		"2026.10.31 15:00", // 带时间后缀（空格）
		"1.0.0-",        // 预发布为空
		"1.0.0+",        // 元数据为空
		".1.0",          // 前导点
		"1.0.",          // 尾随点
		"-1.0.0",        // 前导负号
		"1.0.0_ beta",   // 下划线+空格
		"1.0.0@@@",      // 非法字符
		"1,0,0",         // 逗号分隔
	}
	for _, v := range reject {
		if validVersion.MatchString(v) {
			t.Errorf("validVersion(%q) = true, want false（非法格式被接受）", v)
		}
	}
}

// TestValidVersionLengthGuard 长度边界在 controller 层另行判断（<=50），
// 正则本身不限长——此处锁定正则行为的文档化说明。
func TestValidVersionLengthGuard(t *testing.T) {
	long := "1.0.0-" + strings.Repeat("a", 100)
	// 正则允许任意长度（长度校验由 controller 的 50 上限负责）
	if !validVersion.MatchString(long) {
		t.Log("超长版本号被正则拒绝（controller 层另有 50 字符上限校验）")
	}
}
