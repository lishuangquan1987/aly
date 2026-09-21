package cmd

import "testing"

// TestNeedUpdate 验证统一版本判断口径（#6）：
// 仅当服务器版本严格高于本地版本时才需要更新，与 download_update 的
// compareVersion(newVersion, currentVersion) > 0 语义保持一致。
func TestNeedUpdate(t *testing.T) {
	tests := []struct {
		name    string
		server  string
		local   string
		want    bool
	}{
		{"服务器更高（补丁）", "2.0.1", "2.0.0", true},
		{"服务器更高（主版本）", "3.0.0", "2.9.9", true},
		{"相同版本", "2.0.0", "2.0.0", false},
		{"服务器更低（回滚发布）", "1.9.0", "2.0.0", false},
		{"服务器更低（主版本）", "1.0.0", "2.0.0", false},
		{"本地位数更多且更高", "2.0", "2.0.1", false},
	}
	for _, tt := range tests {
		got := needUpdate(tt.server, tt.local)
		if got != tt.want {
			t.Errorf("needUpdate(%q, %q) = %v, want %v", tt.server, tt.local, got, tt.want)
		}
	}
}
