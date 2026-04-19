package batch

import (
	"errors"
	"strings"
	"testing"
)

func TestParse_array(t *testing.T) {
	in := `[
	  {"command":"llm price","args":{"model":"anthropic/claude-opus-4.6"}},
	  {"command":"compare","args":{"kind":"compute.vm","vcpu":4}}
	]`
	ops, f, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f != FormatArray || len(ops) != 2 || ops[0].Command != "llm price" {
		t.Fatalf("unexpected result: %v %v", f, ops)
	}
}

func TestParse_ndjson_withCommentsAndBlanks(t *testing.T) {
	in := "# header\n\n" +
		`{"command":"llm price","args":{"model":"m"}}` + "\n" +
		"   # indented comment\n" +
		`{"command":"estimate","args":{"items":["aws/ec2:m5.large:region=us-east-1:count=1:hours=1"]}}` + "\n"
	ops, f, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if f != FormatNDJSON || len(ops) != 2 {
		t.Fatalf("format=%v ops=%v", f, ops)
	}
}

func TestParse_badFirstByte(t *testing.T) {
	_, _, err := Parse(strings.NewReader("not-json"))
	if !errors.Is(err, ErrBadFormat) {
		t.Fatalf("want ErrBadFormat, got %v", err)
	}
}

func TestParse_unknownFieldRejected(t *testing.T) {
	_, _, err := Parse(strings.NewReader(`[{"commnad":"llm price"}]`))
	if err == nil {
		t.Fatal("expected decode error on typo'd key")
	}
}

func TestParse_empty(t *testing.T) {
	ops, f, err := Parse(strings.NewReader(""))
	if err != nil {
		t.Fatalf("empty input must not error: %v", err)
	}
	if len(ops) != 0 || f != FormatNDJSON {
		t.Fatalf("empty default: ops=%v f=%v", ops, f)
	}
}
