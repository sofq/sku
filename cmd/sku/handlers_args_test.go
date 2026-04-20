package sku

import "testing"

func TestArgString(t *testing.T) {
	m := map[string]any{"model": "anthropic/claude-opus-4.6", "x": 1.0}
	if got := argString(m, "model"); got != "anthropic/claude-opus-4.6" {
		t.Fatalf("argString = %q", got)
	}
	if got := argString(m, "missing"); got != "" {
		t.Fatalf("missing should be empty, got %q", got)
	}
	if got := argString(m, "x"); got != "" {
		t.Fatalf("non-string arg must return empty, got %q", got)
	}
}

func TestArgFloat(t *testing.T) {
	m := map[string]any{"vcpu": 4.0, "mem": "8", "bad": "x"}
	if got, ok := argFloat(m, "vcpu"); !ok || got != 4 {
		t.Fatalf("vcpu: %v %v", got, ok)
	}
	if got, ok := argFloat(m, "mem"); !ok || got != 8 {
		t.Fatalf("mem string should parse: %v %v", got, ok)
	}
	if _, ok := argFloat(m, "bad"); ok {
		t.Fatal("non-numeric string must not parse")
	}
}

func TestArgStringSlice(t *testing.T) {
	m := map[string]any{"items": []any{"a", "b"}, "notSlice": "x"}
	got := argStringSlice(m, "items")
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("items = %v", got)
	}
	if argStringSlice(m, "notSlice") != nil {
		t.Fatal("non-slice must return nil")
	}
}
