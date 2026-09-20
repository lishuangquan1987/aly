package config

import "testing"

// TestShouldSkipFolder 验证忽略文件夹（ignore_folders）/不复制文件夹（un_copy_folders）的匹配逻辑。
// ShouldSkipFolder 同时被 apply_update 的"不复制文件夹"复制控制使用。
func TestShouldSkipFolder(t *testing.T) {
	tests := []struct {
		name       string
		relPath    string
		ignoreList []string
		want       bool
	}{
		{"顶层精确匹配", "node_modules", []string{"node_modules"}, true},
		{"嵌套精确匹配（带相对路径）", "src/dist", []string{"src/dist"}, true},
		{"glob 通配嵌套", "src/node_modules", []string{"**/node_modules"}, true},
		{"前缀相同但非完整路径不匹配", "src/node_modules", []string{"node_modules"}, false},
		{"未在忽略列表", "assets", []string{"node_modules", "dist"}, false},
		{"大小写不敏感（EqualFold）", "Node_Modules", []string{"node_modules"}, true},
		{"空列表不忽略任何文件夹", "node_modules", nil, false},
	}
	for _, tt := range tests {
		got := ShouldSkipFolder(tt.relPath, tt.ignoreList)
		if got != tt.want {
			t.Errorf("ShouldSkipFolder(%q, %v) = %v, want %v",
				tt.relPath, tt.ignoreList, got, tt.want)
		}
	}
}

// TestShouldSkipFile 验证忽略文件（ignore_files）/不复制文件（un_copy_files）的匹配逻辑。
// ShouldSkipFile 同时被 apply_update 的"不复制文件"复制控制使用。
func TestShouldSkipFile(t *testing.T) {
	tests := []struct {
		name       string
		relPath    string
		ignoreList []string
		want       bool
	}{
		{"按文件名精确匹配", "app.log", []string{"app.log"}, true},
		{"按通配符匹配文件名", "logs/app.log", []string{"*.log"}, true},
		{"按相对路径精确匹配", "config/settings.json", []string{"config/settings.json"}, true},
		{"按相对路径通配", "config/settings.json", []string{"config/*.json"}, true},
		{"未在忽略列表", "config/settings.json", []string{"*.log"}, false},
		{"文件名与相对路径均不匹配", "a/b/x.txt", []string{"*.log", "y.txt"}, false},
		{"空列表不忽略任何文件", "app.log", nil, false},
	}
	for _, tt := range tests {
		got := ShouldSkipFile(tt.relPath, tt.ignoreList)
		if got != tt.want {
			t.Errorf("ShouldSkipFile(%q, %v) = %v, want %v",
				tt.relPath, tt.ignoreList, got, tt.want)
		}
	}
}
