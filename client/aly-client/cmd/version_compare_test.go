package cmd

import "testing"

// TestCompareVersionSemver 语义化版本排序矩阵（精简计划条目 8a）。
// 每行断言 v1 与 v2 的比较结果：want>0 / ==0 / <0。
func TestCompareVersionSemver(t *testing.T) {
	cases := []struct {
		v1, v2 string
		want   int // 1 = v1>v2, 0 = 相等, -1 = v1<v2
	}{
		// 基础数值比较（旧格式行为保持）
		{"1.0.0", "1.0.0", 0},
		{"1.2.0", "1.10.0", -1}, // 数值比较，非字典序
		{"2.0.0", "1.99.99", 1},
		{"1.2", "1.2.0", 0},   // 段数不足按 0 补齐
		{"1.0", "0.9.9", 1},   //
		{"V1.0.0", "1.0.0", 0}, // v 前缀忽略
		{"v2.1", "2.1.0", 0},

		// 预发布语义
		{"1.0.0", "1.0.0-beta", 1},     // 正式 > 预发布
		{"1.0.0-beta", "1.0.0", -1},    //
		{"1.0.0-beta", "1.0.0-beta", 0}, //
		{"1.0.0-alpha", "1.0.0-beta", -1}, // 字典序
		{"1.0.0-beta", "1.0.0-beta.1", -1}, // 段数多者大
		{"1.0.0-beta.1", "1.0.0-beta.2", -1},
		{"1.0.0-beta.2", "1.0.0-beta.10", -1}, // 数值比较
		{"1.0.0-beta.10", "1.0.0-beta.9", 1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha", 1},
		{"1.0.0-alpha.1", "1.0.0-beta", -1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-rc.1", "1.0.0", -1},
		{"1.0.0-rc.1", "1.0.1", -1},

		// 数字标识符 < 字母标识符
		{"1.0.0-1", "1.0.0-alpha", -1},
		{"1.0.0-alpha", "1.0.0-1", 1},

		// 构建元数据忽略
		{"1.0.0+build5", "1.0.0", 0},
		{"1.0.0+build.1", "1.0.0+build.2", 0},
		{"1.0.0-beta+b1", "1.0.0-beta+b2", 0},

		// 混合：预发布 vs 更高主版本
		{"1.0.1-beta", "1.0.0", 1},
		{"0.9.9", "1.0.0-beta", -1},

		// 旧格式兼容（非数字段按 0）
		{"1.0.0", "1.0", 0},
		{"abc", "0.0.1", -1}, // 非数字核心段按 0 处理
	}

	for _, c := range cases {
		got := compareVersion(c.v1, c.v2)
		if (c.want > 0 && got <= 0) || (c.want < 0 && got >= 0) || (c.want == 0 && got != 0) {
			t.Errorf("compareVersion(%q, %q) = %d, want %d", c.v1, c.v2, got, c.want)
		}
		// 对称性检查（除相等外，交换参数结果应取反）
		if c.want != 0 {
			rev := compareVersion(c.v2, c.v1)
			if rev != -got {
				t.Errorf("compareVersion(%q, %q) = %d but reversed = %d, expect %d",
					c.v1, c.v2, got, rev, -got)
			}
		}
	}
}

// TestIsLikelyVersionSemver 校验目录名版本识别的放开。
func TestIsLikelyVersionSemver(t *testing.T) {
	valid := []string{"1.0.0", "1.0", "10.20.30", "1.0.0.1", "1.0.0-beta", "1.0.0-beta.1", "1.0.0+build", "1.0.0-beta.1+build.5"}
	for _, s := range valid {
		if !isLikelyVersion(s) {
			t.Errorf("isLikelyVersion(%q) = false, want true", s)
		}
	}
	invalid := []string{"", "abc", "beta", "a.b.c", "v", "1", ".", "..", "-1.0"}
	for _, s := range invalid {
		if isLikelyVersion(s) {
			t.Errorf("isLikelyVersion(%q) = true, want false", s)
		}
	}
}
