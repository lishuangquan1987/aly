package api

// publish_version 响应兼容性测试（审查 P3-a）。
//
// 背景：服务端为诊断"DB 已是新版本但发布清单没写成"（客户端 download_update
// 依清单删文件 → 清单陈旧会漏删）把 publish_version 的 data 从
//
//	{"id":..,"version":"1.2.0",...}                      ← 老结构
//
// 改成了
//
//	{"changelog":{...},"manifestWritten":true}            ← 新结构
//
// CLI 旧实现把整个 data 直接反序列化成 ProjectChangeLog：新服务端下
// 会拿到一个全零 changelog（version 为空），且 manifestWritten 被彻底丢弃，
// 发布端永远看不到清单写失败。这里钉死三种形态的解析口径：
//  1. 新结构：changelog 取 data.changelog，manifestWritten 如实透出；
//  2. 老结构：data 本身就是 changelog，manifestWritten 视为 true（老服务端
//     同样会写清单，只是没回传结果，不能对老服务端误报警告）；
//  3. data 缺失/为 null：不报错，manifestWritten 视为 true。

import (
	"encoding/json"
	"testing"
)

func TestParsePublishVersionData(t *testing.T) {
	cases := []struct {
		name            string
		raw             string
		wantVersion     string
		wantManifest    bool
		wantNilChangelo bool
		wantErr         bool
	}{
		{
			name:         "新结构-清单写成功",
			raw:          `{"changelog":{"id":7,"version":"1.2.0","logs":["a"],"time":"2026-11-23 10:00:00"},"manifestWritten":true}`,
			wantVersion:  "1.2.0",
			wantManifest: true,
		},
		{
			name:         "新结构-清单写失败",
			raw:          `{"changelog":{"id":8,"version":"1.2.1"},"manifestWritten":false}`,
			wantVersion:  "1.2.1",
			wantManifest: false,
		},
		{
			name:            "新结构-changelog缺失但不该panic",
			raw:             `{"changelog":null,"manifestWritten":true}`,
			wantManifest:    true,
			wantNilChangelo: true,
		},
		{
			// 老服务端：data 直接是 changelog，没有任何包装字段。
			name:         "老结构-裸changelog",
			raw:          `{"id":9,"project_id":1,"version":"1.0.9","logs":["x"],"after_apply_update_script":"echo hi"}`,
			wantVersion:  "1.0.9",
			wantManifest: true,
		},
		{
			name:            "data为null",
			raw:             `null`,
			wantManifest:    true,
			wantNilChangelo: true,
		},
		{
			name:            "data为空",
			raw:             `   `,
			wantManifest:    true,
			wantNilChangelo: true,
		},
		{
			name:    "非法JSON必须报错而不是静默全零",
			raw:     `{"changelog":`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := parsePublishVersionData(json.RawMessage(tc.raw))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("期望报错，实际返回 %+v", res)
				}
				return
			}
			if err != nil {
				t.Fatalf("不应报错: %v", err)
			}
			if res == nil {
				t.Fatal("结果不应为 nil")
			}
			if res.ManifestWritten != tc.wantManifest {
				t.Errorf("manifestWritten = %v, 期望 %v", res.ManifestWritten, tc.wantManifest)
			}
			if tc.wantNilChangelo {
				if res.Changelog != nil {
					t.Errorf("changelog 期望为 nil, 实际 %+v", res.Changelog)
				}
				return
			}
			if res.Changelog == nil {
				t.Fatal("changelog 不应为 nil")
			}
			if res.Changelog.Version != tc.wantVersion {
				t.Errorf("version = %q, 期望 %q", res.Changelog.Version, tc.wantVersion)
			}
		})
	}
}

// TestParsePublishVersionDataLegacyScript 老结构下 after_apply_update_script 必须保留。
// 该字段发布后由客户端执行，丢字段等于静默跳过发布后脚本，属高危回归。
func TestParsePublishVersionDataLegacyScript(t *testing.T) {
	res, err := parsePublishVersionData(json.RawMessage(
		`{"id":3,"version":"2.0.0","after_apply_update_script":"start app.exe"}`))
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if got := res.Changelog.AfterApplyUpdateScript; got != "start app.exe" {
		t.Errorf("after_apply_update_script = %q, 期望 %q", got, "start app.exe")
	}
}
