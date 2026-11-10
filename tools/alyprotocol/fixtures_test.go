package main

import (
	"strings"
	"testing"
)

const fixtureSchema = `{"$schema":"http://json-schema.org/draft-07/schema#","x-protocol-version":"1.0.0","$defs":{
"Output":{"type":"object","additionalProperties":false,"required":["isSuccess","errorMsg","data"],
 "properties":{"isSuccess":{"type":"boolean"},"errorMsg":{"type":"string"},"data":{}}},
"CheckUpdateData":{"type":"object","additionalProperties":false,
 "required":["has_update","need_download_update","current_version"],
 "properties":{"has_update":{"type":"boolean"},"need_download_update":{"type":"boolean"},
 "current_version":{"type":"string"},"new_version":{"type":"string"},
 "force_update":{"type":"boolean"},"release_notes":{"type":"array","items":{"type":"string"}}}},
"DownloadProgress":{"type":"object","additionalProperties":false,
 "required":["index","total","file","status","file_size"],
 "properties":{"index":{"type":"integer"},"total":{"type":"integer"},"file":{"type":"string"},
 "status":{"type":"string","enum":["BEGIN","START","PROGRESS","DONE","SKIP","FAIL"]},
 "file_size":{"type":"integer"},"error":{"type":"string"},
 "downloaded_bytes":{"type":"integer"},"total_bytes":{"type":"integer"},"speed_bps":{"type":"integer"}}},
"DownloadUpdateData":{"type":"object","additionalProperties":false,
 "required":["version"],"properties":{"version":{"type":"string"},
 "failed_files":{"type":"array","items":{"type":"string"}}}}
}}`

const validCheckScenario = `{
  "name": "check",
  "description": "check_update 正常返回",
  "commands": {
    "check_update": {
      "lines": [
        {"isSuccess": true, "errorMsg": "", "data": {"has_update": true, "need_download_update": true, "current_version": "1.0.0"}}
      ]
    }
  }
}`

func mustScenario(t *testing.T, raw string) *Scenario {
	t.Helper()
	sc, err := ParseScenario([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScenario: %v", err)
	}
	return sc
}

func TestValidateScenarioAcceptsValidCheck(t *testing.T) {
	s := mustSchema(t, fixtureSchema)
	errs := ValidateScenario(s, mustScenario(t, validCheckScenario))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateScenarioAcceptsDownloadStream(t *testing.T) {
	s := mustSchema(t, fixtureSchema)
	raw := `{
	  "name": "download",
	  "commands": {
	    "download_update": {
	      "lines": [
	        {"isSuccess": true, "errorMsg": "", "data": {"index": 0, "total": 1, "file": "", "status": "BEGIN", "file_size": 0}},
	        {"isSuccess": true, "errorMsg": "", "data": {"index": 1, "total": 1, "file": "a.dll", "status": "DONE", "file_size": 10}},
	        {"isSuccess": true, "errorMsg": "", "data": null}
	      ]
	    }
	  }
	}`
	errs := ValidateScenario(s, mustScenario(t, raw))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateScenarioAcceptsPartialFailure(t *testing.T) {
	s := mustSchema(t, fixtureSchema)
	raw := `{
	  "name": "partial",
	  "commands": {
	    "download_update": {
	      "lines": [
	        {"isSuccess": true, "errorMsg": "", "data": {"index": 1, "total": 1, "file": "a.dll", "status": "FAIL", "file_size": 10, "error": "checksum mismatch"}},
	        {"isSuccess": false, "errorMsg": "下载完成但有 1 个文件失败: a.dll", "data": {"version": "1.0.1", "failed_files": ["a.dll"]}}
	      ]
	    }
	  }
	}`
	errs := ValidateScenario(s, mustScenario(t, raw))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestValidateScenarioRejectsDrift(t *testing.T) {
	cases := []struct {
		name    string
		sc      string
		wantErr string
	}{
		{
			name:    "wrong type",
			sc:      strings.Replace(validCheckScenario, `"has_update": true`, `"has_update": "yes"`, 1),
			wantErr: "has_update",
		},
		{
			name:    "null isSuccess",
			sc:      strings.Replace(validCheckScenario, `"isSuccess": true`, `"isSuccess": null`, 1),
			wantErr: "isSuccess",
		},
		{
			name:    "null errorMsg",
			sc:      strings.Replace(validCheckScenario, `"errorMsg": ""`, `"errorMsg": null`, 1),
			wantErr: "errorMsg",
		},
		{
			name:    "null for typed field",
			sc:      strings.Replace(validCheckScenario, `"has_update": true`, `"has_update": null`, 1),
			wantErr: "null",
		},
		{
			name:    "extra property",
			sc:      strings.Replace(validCheckScenario, `"current_version": "1.0.0"`, `"current_version": "1.0.0", "bogus": 1`, 1),
			wantErr: "bogus",
		},
		{
			name:    "missing required",
			sc:      strings.Replace(validCheckScenario, `, "current_version": "1.0.0"`, ``, 1),
			wantErr: "current_version",
		},
		{
			name:    "unknown command",
			sc:      strings.Replace(validCheckScenario, `"check_update"`, `"nope"`, 1),
			wantErr: "nope",
		},
		{
			name:    "non zero exit",
			sc:      strings.Replace(validCheckScenario, `"lines"`, `"exit_code": 2, "lines"`, 1),
			wantErr: "exit_code",
		},
		{
			name: "null before last",
			sc: `{"name":"d","commands":{"download_update":{"lines":[
			  {"isSuccess":true,"errorMsg":"","data":null},
			  {"isSuccess":true,"errorMsg":"","data":null}]}}}`,
			wantErr: "最后一行",
		},
		{
			name: "bad status enum",
			sc: `{"name":"d","commands":{"download_update":{"lines":[
			  {"isSuccess":true,"errorMsg":"","data":{"index":1,"total":1,"file":"a","status":"BOGUS","file_size":1}},
			  {"isSuccess":true,"errorMsg":"","data":null}]}}}`,
			wantErr: "BOGUS",
		},
		{
			name: "apply with payload",
			sc: `{"name":"a","commands":{"apply_update":{"lines":[
			  {"isSuccess":true,"errorMsg":"","data":{"version":"1"}}]}}}`,
			wantErr: "apply_update",
		},
		{
			name: "final download object must be failure",
			sc: `{"name":"d","commands":{"download_update":{"lines":[
			  {"isSuccess":true,"errorMsg":"","data":{"index":1,"total":1,"file":"a","status":"DONE","file_size":1}},
			  {"isSuccess":true,"errorMsg":"","data":{"version":"1.0.1"}}]}}}`,
			wantErr: "isSuccess",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustSchema(t, fixtureSchema)
			errs := ValidateScenario(s, mustScenario(t, tc.sc))
			if len(errs) == 0 {
				t.Fatalf("expected an error containing %q", tc.wantErr)
			}
			joined := make([]string, len(errs))
			for i, e := range errs {
				joined[i] = e.Error()
			}
			all := strings.Join(joined, "\n")
			if !strings.Contains(all, tc.wantErr) {
				t.Fatalf("errors do not contain %q:\n%s", tc.wantErr, all)
			}
		})
	}
}

func TestParseScenarioRejectsUnknownField(t *testing.T) {
	if _, err := ParseScenario([]byte(`{"name":"x","commands":{},"extra":1}`)); err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestFixtureCoverageRequiresEveryCommand(t *testing.T) {
	errs := ValidateFixtureCoverage([]*Scenario{mustScenario(t, validCheckScenario)})
	if len(errs) == 0 {
		t.Fatal("expected missing-command errors")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), `"clean"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected clean to be reported missing: %v", errs)
	}
}
