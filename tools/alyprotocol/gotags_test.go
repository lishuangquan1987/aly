package main

import (
	"strings"
	"testing"
)

const driftGoSrc = `package model

type Output struct {
	IsSuccess bool        ` + "`json:\"isSuccess\"`" + `
	ErrMsg    string      ` + "`json:\"errorMsg\"`" + `
	Data      interface{} ` + "`json:\"data\"`" + `
}

type DownloadProgress struct {
	Index           int      ` + "`json:\"index\"`" + `
	FileSize        int64    ` + "`json:\"file_size\"`" + `
	Status          string   ` + "`json:\"status\"`" + `
	Error           string   ` + "`json:\"error,omitempty\"`" + `
	DownloadedBytes int64    ` + "`json:\"downloaded_bytes,omitempty\"`" + `
	Files           []string ` + "`json:\"files,omitempty\"`" + `
	Remote          *bool    ` + "`json:\"remote,omitempty\"`" + `
	Ignored         string   ` + "`json:\"-\"`" + `
}
`

const driftSchema = `{"$schema":"http://json-schema.org/draft-07/schema#","x-protocol-version":"1.0.0","$defs":{
"Output":{"type":"object","additionalProperties":false,"required":["isSuccess","errorMsg","data"],
 "properties":{"isSuccess":{"type":"boolean"},"errorMsg":{"type":"string"},"data":{}}},
"DownloadProgress":{"type":"object","additionalProperties":false,
 "required":["index","file_size","status"],
 "properties":{"index":{"type":"integer"},"file_size":{"type":"integer"},"status":{"type":"string"},
 "error":{"type":"string"},"downloaded_bytes":{"type":"integer"},
 "files":{"type":"array","items":{"type":"string"}},"remote":{"type":"boolean"}}}}}`

func driftConfig() *Config {
	return &Config{Go: GoSection{Structs: map[string]string{
		"Output":           "Output",
		"DownloadProgress": "DownloadProgress",
	}}}
}

func TestParseGoStructs(t *testing.T) {
	structs, err := ParseGoStructs([]byte(driftGoSrc))
	if err != nil {
		t.Fatal(err)
	}
	out := structs["Output"]
	if out == nil {
		t.Fatal("Output not parsed")
	}
	if f := out.FieldByTag("data"); f == nil || !f.Type.IsInterface {
		t.Fatalf("Output.Data should be interface{}, got %+v", f)
	}
	prog := structs["DownloadProgress"]
	if prog == nil {
		t.Fatal("DownloadProgress not parsed")
	}
	if f := prog.FieldByTag("error"); f == nil || !f.OmitEmpty || f.Type.IsPointer {
		t.Fatalf("error field = %+v", f)
	}
	if f := prog.FieldByTag("remote"); f == nil || !f.Type.IsPointer || !f.OmitEmpty || f.Type.Base != "bool" {
		t.Fatalf("remote field = %+v", f)
	}
	if f := prog.FieldByTag("files"); f == nil || !f.Type.IsArray || f.Type.Elem != "string" {
		t.Fatalf("files field = %+v", f)
	}
	if prog.FieldByTag("-") != nil || prog.FieldByName("Ignored") != nil {
		t.Fatal("json:\"-\" field must be skipped")
	}
}

func TestVerifyAcceptsMatchingSchemaAndGo(t *testing.T) {
	s := mustSchema(t, driftSchema)
	structs, err := ParseGoStructs([]byte(driftGoSrc))
	if err != nil {
		t.Fatal(err)
	}
	if errs := Verify(s, driftConfig(), structs); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}

func TestVerifyDetectsDrift(t *testing.T) {
	cases := []struct {
		name    string
		schema  string
		goSrc   string
		wantErr string
	}{
		{
			name:    "schema property not emitted",
			schema:  strings.Replace(driftSchema, `"remote":{"type":"boolean"}`, `"remote":{"type":"boolean"},"note":{"type":"string"}`, 1),
			goSrc:   driftGoSrc,
			wantErr: `属性 "DownloadProgress.note"`,
		},
		{
			name:    "go field not in schema",
			schema:  strings.Replace(driftSchema, `"files":{"type":"array","items":{"type":"string"}},`, "", 1),
			goSrc:   driftGoSrc,
			wantErr: `json:"files"`,
		},
		{
			name:    "required with omitempty",
			schema:  strings.Replace(driftSchema, `"required":["index","file_size","status"]`, `"required":["index","file_size","status","error"]`, 1),
			goSrc:   driftGoSrc,
			wantErr: "omitempty",
		},
		{
			name:    "optional without omitempty",
			schema:  driftSchema,
			goSrc:   strings.Replace(driftGoSrc, `json:"downloaded_bytes,omitempty"`, `json:"downloaded_bytes"`, 1),
			wantErr: "omitempty",
		},
		{
			name:    "type mismatch",
			schema:  strings.Replace(driftSchema, `"remote":{"type":"boolean"}`, `"remote":{"type":"string"}`, 1),
			goSrc:   driftGoSrc,
			wantErr: "不兼容",
		},
		{
			name:    "any must be interface",
			schema:  strings.Replace(driftSchema, `"data":{}`, `"data":{"type":"string"}`, 1),
			goSrc:   driftGoSrc,
			wantErr: "不兼容",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := mustSchema(t, tc.schema)
			structs, err := ParseGoStructs([]byte(tc.goSrc))
			if err != nil {
				t.Fatal(err)
			}
			errs := Verify(s, driftConfig(), structs)
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