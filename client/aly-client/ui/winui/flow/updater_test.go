// +build windows

package flow

import "testing"

func boolPtr(b bool) *bool { return &b }

// 强制更新：必须自动下载，不询问。
func TestDecideForceUpdateTrue(t *testing.T) {
	got := Decide(CheckView{
		HasUpdate: true, NeedDownload: true,
		ForceUpdate: boolPtr(true), NewVersion: "1.0.1",
	})
	if got.Action != ActionDownload || got.Ask {
		t.Fatalf("forced update must auto-download without asking, got %+v", got)
	}
}

// 非强制：必须询问。
func TestDecideForceUpdateFalseAsks(t *testing.T) {
	got := Decide(CheckView{
		HasUpdate: true, NeedDownload: true,
		ForceUpdate: boolPtr(false), NewVersion: "1.0.1",
	})
	if got.Action != ActionDownload || !got.Ask {
		t.Fatalf("non-forced must ask before download, got %+v", got)
	}
}

// 字段缺失（nil）：按询问处理，**绝不能**当成强制。
func TestDecideForceUpdateAbsentAsks(t *testing.T) {
	got := Decide(CheckView{
		HasUpdate: true, NeedDownload: true,
		ForceUpdate: nil, NewVersion: "1.0.1",
	})
	if got.Action != ActionDownload || !got.Ask {
		t.Fatalf("absent force_update must ask, got %+v", got)
	}
}

// applying（崩溃恢复）：主目录可能已丢失，必须无条件自动续跑，不能询问。
func TestDecideApplyingAlwaysAuto(t *testing.T) {
	for _, fu := range []*bool{nil, boolPtr(false), boolPtr(true)} {
		got := Decide(CheckView{HasUpdate: true, NeedDownload: false, Applying: true, ForceUpdate: fu})
		if got.Action != ActionApply || got.Ask {
			t.Fatalf("applying must auto-apply without asking (force=%v), got %+v", fu, got)
		}
	}
}

// applying 优先级高于 force 与 needDownload。
func TestDecideApplyingOverridesNeedDownload(t *testing.T) {
	got := Decide(CheckView{HasUpdate: true, NeedDownload: true, Applying: true})
	if got.Action != ActionApply || got.Ask {
		t.Fatalf("applying must win over NeedDownload, got %+v", got)
	}
}

// 无更新：什么都不做。
func TestDecideNoUpdate(t *testing.T) {
	for _, fu := range []*bool{nil, boolPtr(false), boolPtr(true)} {
		got := Decide(CheckView{HasUpdate: false, ForceUpdate: fu})
		if got.Action != ActionNone {
			t.Fatalf("no update must be ActionNone, got %+v", got)
		}
		if got.Ask {
			t.Fatalf("no update must not ask, got %+v", got)
		}
	}
}

// downloaded 续跑 apply：非强制/缺失都要询问。
func TestDecidePendingApplyAsksWhenNotForced(t *testing.T) {
	for _, fu := range []*bool{nil, boolPtr(false)} {
		got := Decide(CheckView{HasUpdate: true, NeedDownload: false, ForceUpdate: fu, NewVersion: "1.0.1"})
		if got.Action != ActionApply || !got.Ask {
			t.Fatalf("pending apply without force must ask (force=%v), got %+v", fu, got)
		}
	}
}

// downloaded 续跑 apply：强制则自动应用。
func TestDecidePendingApplyAutoWhenForced(t *testing.T) {
	got := Decide(CheckView{HasUpdate: true, NeedDownload: false, ForceUpdate: boolPtr(true)})
	if got.Action != ActionApply || got.Ask {
		t.Fatalf("forced pending apply must be automatic, got %+v", got)
	}
}

// 决策表完整性：4 种 (Applying, HasUpdate, NeedDownload) 组合都有确定行为。
func TestDecideTableIsTotal(t *testing.T) {
	for _, applying := range []bool{false, true} {
		for _, has := range []bool{false, true} {
			for _, need := range []bool{false, true} {
				got := Decide(CheckView{Applying: applying, HasUpdate: has, NeedDownload: need})
				switch {
				case applying:
					if got.Action != ActionApply {
						t.Errorf("applying=%v has=%v need=%v -> %v, want ActionApply", applying, has, need, got.Action)
					}
				case !has:
					if got.Action != ActionNone {
						t.Errorf("applying=%v has=%v need=%v -> %v, want ActionNone", applying, has, need, got.Action)
					}
				case need:
					if got.Action != ActionDownload {
						t.Errorf("applying=%v has=%v need=%v -> %v, want ActionDownload", applying, has, need, got.Action)
					}
				default:
					if got.Action != ActionApply {
						t.Errorf("applying=%v has=%v need=%v -> %v, want ActionApply", applying, has, need, got.Action)
					}
				}
			}
		}
	}
}
