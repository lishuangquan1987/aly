package cmd

import (
	"reflect"
	"testing"

	"aly/client/aly-client/config"
)

// TestPartitionHolders 验证持有者按白名单分区：白名单内 → killable，白名单外 → blocked（#13）。
func TestPartitionHolders(t *testing.T) {
	holders := []uint32{100, 200, 300, 400}
	wl := map[uint32]bool{200: true, 400: true}

	killable, blocked := partitionHolders(holders, wl)
	if !reflect.DeepEqual(killable, []uint32{200, 400}) {
		t.Errorf("killable 应为 [200 400]，实际 %v", killable)
	}
	if !reflect.DeepEqual(blocked, []uint32{100, 300}) {
		t.Errorf("blocked 应为 [100 300]，实际 %v", blocked)
	}
}

// TestBuildKillWhitelist 验证白名单组成：explorer + 主程序 exe 名（去扩展名）+ must_close_process_name。
func TestBuildKillWhitelist(t *testing.T) {
	fc := &FullConfig{
		ExeCfg: &config.Config{
			MainExeRelativePath:  "../ApplicationFolder/YOFC.OTDR3001.exe",
			MustCloseProcessName: []string{"helper.exe"},
		},
	}
	got := buildKillWhitelist(fc)
	want := []string{"explorer", "YOFC.OTDR3001", "helper.exe"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildKillWhitelist = %v, want %v", got, want)
	}

	// 未配置主程序路径时仅 explorer + must_close
	fc2 := &FullConfig{ExeCfg: &config.Config{}}
	if got2 := buildKillWhitelist(fc2); len(got2) != 1 || got2[0] != "explorer" {
		t.Errorf("未配置时白名单应仅含 explorer，实际 %v", got2)
	}
}
