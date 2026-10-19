// +build windows

package bridge

import "testing"

func TestParseLineCheckUpdateForced(t *testing.T) {
	line := `{"isSuccess":true,"errorMsg":"","data":{"has_update":true,"need_download_update":true,"current_version":"1.0.0","new_version":"1.0.1","force_update":true}}`
	got, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != KindResult || !got.Success {
		t.Fatalf("want result success, got %+v", got)
	}
	if got.Check == nil {
		t.Fatalf("Check should not be nil, got %+v", got)
	}
	if !got.Check.HasUpdate || !got.Check.NeedDownloadUpdate {
		t.Fatalf("bad flags: %+v", got.Check)
	}
	if got.Check.ForceUpdate == nil || !*got.Check.ForceUpdate {
		t.Fatalf("force_update should be true")
	}
	if got.Check.NewVersion != "1.0.1" || got.Check.CurrentVersion != "1.0.0" {
		t.Fatalf("bad versions: %+v", got.Check)
	}
}

func TestParseLineForceUpdateAbsent(t *testing.T) {
	// downloaded/applying 续跑分支不返回 force_update —— 必须解析成 nil，而不是 false
	line := `{"isSuccess":true,"data":{"has_update":true,"need_download_update":false,"current_version":"1.0.0","new_version":"1.0.1"}}`
	got, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Check == nil {
		t.Fatalf("Check should not be nil")
	}
	if got.Check.ForceUpdate != nil {
		t.Fatalf("force_update absent must be nil, got %v", *got.Check.ForceUpdate)
	}
}

func TestParseLineCheckUpdateNoUpdateEmptyCurrentVersion(t *testing.T) {
	// 关键回归：has_update=false 且 current_version 为空时，
	// 不能因为字段全为零值就误判成 check_self_update。
	line := `{"isSuccess":true,"data":{"has_update":false,"need_download_update":false,"current_version":""}}`
	got, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != KindResult {
		t.Fatalf("want result, got %v", got.Kind)
	}
	if got.Check == nil {
		t.Fatalf("must parse as CheckUpdateData, got Self=%+v", got.Self)
	}
	if got.Self != nil {
		t.Fatal("must NOT parse as CheckSelfUpdateData")
	}
	if got.Check.HasUpdate {
		t.Fatalf("has_update should be false")
	}
}

func TestParseLineCheckSelfUpdate(t *testing.T) {
	got, err := ParseLine(`{"isSuccess":true,"data":{"need_update":true}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != KindResult {
		t.Fatalf("want result, got %v", got.Kind)
	}
	if got.Self == nil || !got.Self.NeedUpdate {
		t.Fatalf("want self need_update=true, got %+v", got.Self)
	}
	if got.Check != nil {
		t.Fatal("must NOT parse as CheckUpdateData")
	}
}

func TestParseLineProgress(t *testing.T) {
	line := `{"isSuccess":true,"data":{"index":2,"total":5,"file":"a.dll","status":"DONE","file_size":1024}}`
	got, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != KindProgress {
		t.Fatalf("want progress, got %v", got.Kind)
	}
	if got.Progress == nil || got.Progress.Index != 2 || got.Progress.Total != 5 || got.Progress.Status != "DONE" {
		t.Fatalf("bad progress: %+v", got.Progress)
	}
}

func TestParseLineProgressDone(t *testing.T) {
	got, err := ParseLine(`{"isSuccess":true,"data":null}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != KindDone {
		t.Fatalf("want done, got %v", got.Kind)
	}
}

func TestParseLineProgressFail(t *testing.T) {
	line := `{"isSuccess":false,"errorMsg":"a.dll: checksum mismatch","data":{"index":2,"total":5,"file":"a.dll","status":"FAIL","file_size":1024,"error":"checksum mismatch"}}`
	got, err := ParseLine(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != KindFail || got.Success {
		t.Fatalf("want fail, got %+v", got)
	}
	if got.ErrMsg != "a.dll: checksum mismatch" {
		t.Fatalf("bad errMsg %q", got.ErrMsg)
	}
	if got.Progress == nil || got.Progress.Error != "checksum mismatch" {
		t.Fatalf("progress error field should be parsed, got %+v", got.Progress)
	}
}

func TestParseLineNonJSONIsSkipped(t *testing.T) {
	// usage 文本 / panic 输出：不能报错，也不能当成结果
	for _, line := range []string{
		"Usage of apply_update:",
		"  -close-timeout int",
		"panic: runtime error: index out of range",
		"",
		"   ",
		"{\"broken\":",
	} {
		got, err := ParseLine(line)
		if err != nil {
			t.Fatalf("line %q should not error, got %v", line, err)
		}
		if got.Kind != KindIgnore {
			t.Fatalf("line %q should be ignored, got %v", line, got.Kind)
		}
	}
}

func TestParseLineErrorResult(t *testing.T) {
	got, err := ParseLine(`{"isSuccess":false,"errorMsg":"no server url configured","data":null}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Kind != KindResult || got.Success {
		t.Fatalf("want failed result, got %+v", got)
	}
	if got.ErrMsg != "no server url configured" {
		t.Fatalf("bad errMsg %q", got.ErrMsg)
	}
}

func TestParseLineRollbackList(t *testing.T) {
	got, err := ParseLine(`{"isSuccess":true,"data":{"current_version":"1.0.1","versions":["1.0.0","0.9.0"]}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Versions == nil {
		t.Fatalf("want RollbackListData, got %+v", got)
	}
	if got.Versions.CurrentVersion != "1.0.1" || len(got.Versions.Versions) != 2 {
		t.Fatalf("bad list: %+v", got.Versions)
	}
}

func TestParseLineRollbackResult(t *testing.T) {
	got, err := ParseLine(`{"isSuccess":true,"data":{"version":"1.0.0"}}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Rollback == nil || got.Rollback.Version != "1.0.0" {
		t.Fatalf("want RollbackData, got %+v", got)
	}
	if got.Versions != nil {
		t.Fatal("must NOT parse as RollbackListData")
	}
}
