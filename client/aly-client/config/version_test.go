package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

// setupVersionDir 构造临时 UpdateFolder 并设置 ExeDir 覆盖（测试专用）。
func setupVersionDir(t *testing.T) string {
	t.Helper()
	dir, err := ioutil.TempDir("", "version-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	SetExeDir(dir)
	t.Cleanup(func() { SetExeDir(""); os.RemoveAll(dir) })
	return dir
}

func writeVersionJSON(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := ioutil.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("写 %s 失败: %v", name, err)
	}
}

// TestReadVersionCorruptRestoresFromBak 验证 Bug#4 自愈①：version.json 损坏但 .bak 有效时，
// 从 .bak 恢复并重写 version.json，且 .bak 不被损坏内容覆盖。
func TestReadVersionCorruptRestoresFromBak(t *testing.T) {
	dir := setupVersionDir(t)
	// 半截 JSON（模拟断电写坏）
	writeVersionJSON(t, dir, "version.json", `{"version_previous":"1.0.0","version":"2.0.0","version_status":"downloaded"`)
	writeVersionJSON(t, dir, "version.json.bak", `{
  "version_previous": "1.0.0",
  "version": "2.0.0",
  "version_status": "applied"
}`)

	vi, err := ReadVersion()
	if err != nil {
		t.Fatalf("损坏且 .bak 有效时应从 .bak 恢复而非报错，实际: %v", err)
	}
	if vi.Version != "2.0.0" || vi.VersionStatus != VersionStatusApplied {
		t.Errorf("应从 .bak 恢复 {2.0.0, applied}，实际 %+v", vi)
	}

	// 恢复后 version.json 已重写为有效内容，可正常二次读取
	vi2, err := ReadVersion()
	if err != nil {
		t.Fatalf("恢复后二次读取失败: %v", err)
	}
	if vi2.VersionStatus != VersionStatusApplied {
		t.Errorf("恢复后 version.json 应为 applied，实际 %q", vi2.VersionStatus)
	}

	// .bak 仍是有效旧内容（WriteVersion 不得把损坏内容写进 last-known-good）
	bak, err := ioutil.ReadFile(filepath.Join(dir, "version.json.bak"))
	if err != nil {
		t.Fatalf("读 .bak 失败: %v", err)
	}
	var bakInfo VersionInfo
	if err := json.Unmarshal(bak, &bakInfo); err != nil {
		t.Fatalf(".bak 被损坏内容覆盖: %v", err)
	}
	if bakInfo.VersionStatus != VersionStatusApplied {
		t.Errorf(".bak 应保持有效内容（applied），实际 %q", bakInfo.VersionStatus)
	}
}

// TestReadVersionCorruptNoBakResets 验证 Bug#4 自愈②：损坏且无可用 .bak 时，
// 隔离为 .corrupt 并重置为首次部署语义（status="" → checkUpdateApplied → 重下自愈）。
func TestReadVersionCorruptNoBakResets(t *testing.T) {
	dir := setupVersionDir(t)
	writeVersionJSON(t, dir, "version.json", `{not-valid-json`)

	vi, err := ReadVersion()
	if err != nil {
		t.Fatalf("损坏且无备份时应重置而非报错，实际: %v", err)
	}
	if vi.Version != "" || vi.VersionStatus != "" {
		t.Errorf("应重置为空 VersionInfo（首次部署语义），实际 %+v", vi)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "version.json.corrupt")); statErr != nil {
		t.Errorf("损坏文件应被隔离为 .corrupt: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "version.json")); !os.IsNotExist(statErr) {
		t.Errorf("version.json 应已被移走，实际存在: %v", statErr)
	}
}

// TestReadVersionValidNoSideEffects 验证合法文件正常解析、无副作用。
func TestReadVersionValidNoSideEffects(t *testing.T) {
	dir := setupVersionDir(t)
	writeVersionJSON(t, dir, "version.json", `{"version_previous":"1.0.0","version":"1.0.0","version_status":"applied"}`)

	vi, err := ReadVersion()
	if err != nil {
		t.Fatalf("合法文件读取失败: %v", err)
	}
	if vi.Version != "1.0.0" || vi.VersionStatus != VersionStatusApplied {
		t.Errorf("解析结果错误: %+v", vi)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "version.json.corrupt")); statErr == nil {
		t.Error("合法读取不应产生 .corrupt")
	}
}

// TestWriteVersionBacksUpPrevious 验证 WriteVersion 把上一版内容保留到 .bak（供自愈回退）。
func TestWriteVersionBacksUpPrevious(t *testing.T) {
	dir := setupVersionDir(t)
	writeVersionJSON(t, dir, "version.json", `{"version_previous":"1.0.0","version":"1.0.0","version_status":"applied"}`)

	if err := WriteVersion(&VersionInfo{VersionPrevious: "1.0.0", Version: "2.0.0", VersionStatus: VersionStatusDownloaded}); err != nil {
		t.Fatalf("WriteVersion 失败: %v", err)
	}

	bak, err := ioutil.ReadFile(filepath.Join(dir, "version.json.bak"))
	if err != nil {
		t.Fatalf("读 .bak 失败: %v", err)
	}
	var bakInfo VersionInfo
	if err := json.Unmarshal(bak, &bakInfo); err != nil {
		t.Fatalf(".bak 内容损坏: %v", err)
	}
	if bakInfo.Version != "1.0.0" {
		t.Errorf(".bak 应保存上一版 {1.0.0}，实际 %+v", bakInfo)
	}

	vi, err := ReadVersion()
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if vi.Version != "2.0.0" {
		t.Errorf("version.json 应为 2.0.0，实际 %+v", vi)
	}
}

// TestWriteVersionDoesNotClobberBakWithCorrupt 验证：当前文件损坏时 WriteVersion 不覆盖有效 .bak，
// 保证自愈链在"恢复过程中再次写入"时不被破坏。
func TestWriteVersionDoesNotClobberBakWithCorrupt(t *testing.T) {
	dir := setupVersionDir(t)
	writeVersionJSON(t, dir, "version.json", `{broken`)
	writeVersionJSON(t, dir, "version.json.bak", `{"version":"1.0.0","version_status":"applied"}`)

	if err := WriteVersion(&VersionInfo{Version: "2.0.0", VersionStatus: VersionStatusDownloaded}); err != nil {
		t.Fatalf("WriteVersion 失败: %v", err)
	}

	bak, err := ioutil.ReadFile(filepath.Join(dir, "version.json.bak"))
	if err != nil {
		t.Fatalf("读 .bak 失败: %v", err)
	}
	var bakInfo VersionInfo
	if err := json.Unmarshal(bak, &bakInfo); err != nil {
		t.Fatalf(".bak 被损坏内容覆盖: %v", err)
	}
	if bakInfo.Version != "1.0.0" {
		t.Errorf(".bak 应保持有效旧内容 {1.0.0}，实际 %+v", bakInfo)
	}
}
