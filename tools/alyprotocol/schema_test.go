package main

import "testing"

func TestParseSchemaKeepsDefinitionAndPropertyOrder(t *testing.T) {
	raw := []byte(`{
	  "$schema": "http://json-schema.org/draft-07/schema#",
	  "x-protocol-version": "1.0.0",
	  "$defs": {
	    "Second": {"type": "object", "required": ["b"], "additionalProperties": false,
	      "properties": {"b": {"type": "string"}, "a": {"type": "string"}}},
	    "First": {"type": "object", "required": [], "additionalProperties": false,
	      "properties": {"x": {"type": "integer"}}}
	  }
	}`)
	s, err := ParseSchema(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.DefNames(); len(got) != 2 || got[0] != "Second" || got[1] != "First" {
		t.Fatalf("def order = %v", got)
	}
	if got := s.Def("Second").PropertyNames(); len(got) != 2 || got[0] != "b" || got[1] != "a" {
		t.Fatalf("property order = %v", got)
	}
	if s.ProtocolVersion != "1.0.0" {
		t.Fatalf("version = %q", s.ProtocolVersion)
	}
}

func TestValidateRejectsRequiredNotInProperties(t *testing.T) {
	raw := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","x-protocol-version":"1.0.0",
	  "$defs":{"X":{"type":"object","required":["nope"],"additionalProperties":false,
	  "properties":{"a":{"type":"string"}}}}}`)
	if _, err := ParseSchema(raw); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateRejectsOpenObject(t *testing.T) {
	raw := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","x-protocol-version":"1.0.0",
	  "$defs":{"X":{"type":"object","additionalProperties":true,
	  "properties":{"a":{"type":"string"}}}}}`)
	if _, err := ParseSchema(raw); err == nil {
		t.Fatal("expected error for additionalProperties=true")
	}
}

func TestValidateRejectsUnresolvedRef(t *testing.T) {
	raw := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","x-protocol-version":"1.0.0",
	  "$defs":{"X":{"type":"object","additionalProperties":false,
	  "properties":{"a":{"$ref":"#/$defs/Missing"}}}}}`)
	if _, err := ParseSchema(raw); err == nil {
		t.Fatal("expected error for unresolved $ref")
	}
}

func TestValidateRejectsDuplicateProperty(t *testing.T) {
	raw := []byte(`{"$schema":"http://json-schema.org/draft-07/schema#","x-protocol-version":"1.0.0",
	  "$defs":{"X":{"type":"object","additionalProperties":false,
	  "properties":{"a":{"type":"string"},"a":{"type":"integer"}}}}}`)
	if _, err := ParseSchema(raw); err == nil {
		t.Fatal("expected error for duplicate property")
	}
}
