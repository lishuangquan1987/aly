package client

import "testing"

func TestParseCheckUpdate(t *testing.T) {
	got, err := ParseLine(`{"isSuccess":true,"data":{"has_update":true,"need_download_update":true,"current_version":"1.0.0","new_version":"1.0.1","force_update":true}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != KindResult || got.Check == nil || !got.Check.HasUpdate ||
		!got.Check.NeedDownloadUpdate || got.Check.ForceUpdate == nil || !*got.Check.ForceUpdate {
		t.Fatalf("bad parse: %+v", got)
	}
}

func TestParseCheckForceAbsentIsNil(t *testing.T) {
	got, err := ParseLine(`{"isSuccess":true,"data":{"has_update":true,"need_download_update":false,"current_version":"1.0.0","new_version":"1.0.1"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Check == nil || got.Check.ForceUpdate != nil {
		t.Fatalf("force_update absent must be nil: %+v", got.Check)
	}
}

func TestParseNoUpdateEmptyCurrentVersion(t *testing.T) {
	got, err := ParseLine(`{"isSuccess":true,"data":{"has_update":false,"need_download_update":false,"current_version":""}}`)
	if err != nil {
		t.Fatal(err)
	}
	if got.Check == nil || got.Self != nil {
		t.Fatalf("must be CheckUpdate, got %+v", got)
	}
}

func TestParseProgressDoneFail(t *testing.T) {
	p, _ := ParseLine(`{"isSuccess":true,"data":{"index":2,"total":5,"file":"a.dll","status":"DONE","file_size":1024,"downloaded_bytes":512,"total_bytes":2048,"speed_bps":100}}`)
	if p.Kind != KindProgress || p.Progress == nil || p.Progress.DownloadedBytes != 512 {
		t.Fatalf("bad progress: %+v", p)
	}
	d, _ := ParseLine(`{"isSuccess":true,"data":null}`)
	if d.Kind != KindDone {
		t.Fatalf("want done, got %v", d.Kind)
	}
	f, _ := ParseLine(`{"isSuccess":false,"errorMsg":"checksum","data":{"index":2,"total":5,"file":"a.dll","status":"FAIL","error":"bad"}}`)
	if f.Kind != KindFail || f.Progress == nil || f.ErrMsg != "checksum" {
		t.Fatalf("bad fail: %+v", f)
	}
}

func TestParseSelfUpdateAndVersions(t *testing.T) {
	s, _ := ParseLine(`{"isSuccess":true,"data":{"updated":false,"reason":"already_up_to_date"}}`)
	if s.SelfUpdate == nil || s.SelfUpdate.Updated || s.SelfUpdate.Reason != "already_up_to_date" {
		t.Fatalf("bad self update: %+v", s)
	}
	v, _ := ParseLine(`{"isSuccess":true,"data":{"current_version":"1.0.0","versions":["0.9.0","0.8.0"]}}`)
	if v.Versions == nil || len(v.Versions.Versions) != 2 {
		t.Fatalf("bad versions: %+v", v)
	}
	r, _ := ParseLine(`{"isSuccess":true,"data":{"version":"0.9.0"}}`)
	if r.Rollback == nil || r.Rollback.Version != "0.9.0" {
		t.Fatalf("bad rollback: %+v", r)
	}
}

func TestParseIgnoreNonJSON(t *testing.T) {
	for _, line := range []string{"", "Usage:", "panic: x", `{"broken":`} {
		got, err := ParseLine(line)
		if err != nil || got.Kind != KindIgnore {
			t.Fatalf("%q: got %+v err=%v", line, got, err)
		}
	}
}
