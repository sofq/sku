package batch

import (
	"context"
	"io"
	"testing"
)

func TestRegister_and_Lookup(t *testing.T) {
	ResetForTest(t)
	want := Handler(func(_ context.Context, _ map[string]any, _ Env) (any, error) {
		return "hi", nil
	})
	Register("demo cmd", want)
	got, ok := Lookup("demo cmd")
	if !ok {
		t.Fatal("expected handler to be registered")
	}
	res, err := got(context.Background(), nil, Env{Stdout: io.Discard, Stderr: io.Discard})
	if err != nil || res.(string) != "hi" {
		t.Fatalf("lookup returned wrong handler: %v %v", res, err)
	}
}

func TestRegister_duplicatePanics(t *testing.T) {
	ResetForTest(t)
	Register("dup", func(_ context.Context, _ map[string]any, _ Env) (any, error) { return nil, nil })
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate Register")
		}
	}()
	Register("dup", func(_ context.Context, _ map[string]any, _ Env) (any, error) { return nil, nil })
}

func TestRegisteredNames_sorted(t *testing.T) {
	ResetForTest(t)
	Register("b", func(_ context.Context, _ map[string]any, _ Env) (any, error) { return nil, nil })
	Register("a", func(_ context.Context, _ map[string]any, _ Env) (any, error) { return nil, nil })
	got := RegisteredNames()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("RegisteredNames = %v, want [a b]", got)
	}
}
