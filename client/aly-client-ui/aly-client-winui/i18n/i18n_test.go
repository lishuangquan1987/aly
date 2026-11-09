// +build windows

package i18n

import (
	"strings"
	"testing"
)

func TestLangDefaultIsChinese(t *testing.T) {
	if got := ParseLang(""); got != ZH {
		t.Fatalf("empty lang should default to ZH, got %v", got)
	}
	if got := ParseLang("bogus"); got != ZH {
		t.Fatalf("unknown lang should default to ZH, got %v", got)
	}
	if got := ParseLang("en-US"); got != EN {
		t.Fatalf("en-US should map to EN, got %v", got)
	}
	if got := ParseLang("EN_us"); got != EN {
		t.Fatalf("EN_us should map to EN (case/underscore tolerant), got %v", got)
	}
}

func TestKeySetsMatch(t *testing.T) {
	if len(zhCN) == 0 || len(enUS) == 0 {
		t.Fatal("text tables must not be empty")
	}
	for k := range zhCN {
		if _, ok := enUS[k]; !ok {
			t.Errorf("key %q missing in en-US", k)
		}
	}
	for k := range enUS {
		if _, ok := zhCN[k]; !ok {
			t.Errorf("key %q missing in zh-CN", k)
		}
	}
}

func TestNoEmptyText(t *testing.T) {
	for k, v := range zhCN {
		if v == "" {
			t.Errorf("zh-CN key %q is empty", k)
		}
	}
	for k, v := range enUS {
		if v == "" {
			t.Errorf("en-US key %q is empty", k)
		}
	}
}

func TestTAndFormat(t *testing.T) {
	SetLang(ZH)
	zhNoUpdate := T(KeyNoUpdate)
	if zhNoUpdate == "" {
		t.Fatal("KeyNoUpdate should not be empty in ZH")
	}

	SetLang(EN)
	enNoUpdate := T(KeyNoUpdate)
	if enNoUpdate == "" {
		t.Fatal("KeyNoUpdate should not be empty in EN")
	}
	if enNoUpdate == zhNoUpdate {
		t.Fatal("EN and ZH text should differ")
	}

	// 带占位符的键必须能被 Fmt 正确填充
	filled := Fmt(KeyFoundVersion, "1.2.3")
	if filled == T(KeyFoundVersion) {
		t.Fatalf("Fmt should substitute the placeholder, got %q", filled)
	}
	if !strings.Contains(filled, "1.2.3") {
		t.Fatalf("Fmt result should contain the version, got %q", filled)
	}
}

func TestTUnknownKeyReturnsKey(t *testing.T) {
	SetLang(ZH)
	if got := T("__no_such_key__"); got != "__no_such_key__" {
		t.Fatalf("unknown key should return itself, got %q", got)
	}
}

func TestSetLangAffectsT(t *testing.T) {
	SetLang(ZH)
	zh := T(KeyApplying)
	SetLang(EN)
	en := T(KeyApplying)
	if zh == en {
		t.Fatal("SetLang should change the text returned by T")
	}
	SetLang(ZH) // 复位，避免影响其它用例
}
