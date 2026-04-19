package batch

import (
	"context"
	"errors"
	"io"
	"testing"

	skuerrors "github.com/sofq/sku/internal/errors"
)

func TestDispatch_happyPathAndUnknown(t *testing.T) {
	ResetForTest(t)
	Register("hi", func(ctx context.Context, args map[string]any, env Env) (any, error) {
		return map[string]any{"ok": true}, nil
	})
	Register("notfound", func(ctx context.Context, args map[string]any, env Env) (any, error) {
		return nil, skuerrors.NotFound("aws", "ec2", map[string]any{"instance_type": "x"}, "try another")
	})

	ops := []Op{
		{Command: "hi"},
		{Command: "notfound"},
		{Command: "missing"},
	}
	recs := Dispatch(context.Background(), ops, Env{Settings: &Settings{}, Stdout: io.Discard, Stderr: io.Discard})

	if len(recs) != 3 {
		t.Fatalf("want 3 records, got %d", len(recs))
	}
	if recs[0].ExitCode != 0 || recs[0].Error != nil {
		t.Fatalf("rec 0: %+v", recs[0])
	}
	if recs[1].ExitCode != 3 || recs[1].Error == nil || recs[1].Error.Code != skuerrors.CodeNotFound {
		t.Fatalf("rec 1: %+v", recs[1])
	}
	if recs[2].ExitCode != 4 || recs[2].Error == nil || recs[2].Error.Details["reason"] != "unknown_command" {
		t.Fatalf("rec 2: %+v", recs[2])
	}
}

func TestDispatch_genericErrorBoxed(t *testing.T) {
	ResetForTest(t)
	Register("boom", func(ctx context.Context, _ map[string]any, _ Env) (any, error) {
		return nil, errors.New("raw failure")
	})
	recs := Dispatch(context.Background(), []Op{{Command: "boom"}}, Env{Settings: &Settings{}})
	if recs[0].ExitCode != 1 || recs[0].Error.Code != skuerrors.CodeGeneric {
		t.Fatalf("want generic boxed error, got %+v", recs[0])
	}
}

func TestAggregateExit(t *testing.T) {
	recs := []Record{{ExitCode: 0}, {ExitCode: 3}, {ExitCode: 7}}
	if got := AggregateExit(recs); got != 7 {
		t.Fatalf("AggregateExit = %d, want 7", got)
	}
	if got := AggregateExit(nil); got != 0 {
		t.Fatalf("empty AggregateExit = %d, want 0", got)
	}
}
