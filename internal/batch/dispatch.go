package batch

import (
	"context"
	"errors"

	skuerrors "github.com/sofq/sku/internal/errors"
)

// ErrAggregate tags the synthetic *skuerrors.E the batch Cobra wrapper returns
// when one or more ops produced errors. Execute() uses errors.Is to suppress
// the stderr envelope on aggregate exit.
var ErrAggregate = errors.New("batch aggregate")

// Dispatch runs ops sequentially against the registry. Errors become per-op
// Record.Error envelopes; the process stderr stays clean.
func Dispatch(ctx context.Context, ops []Op, env Env) []Record {
	out := make([]Record, 0, len(ops))
	for i, op := range ops {
		if err := ctx.Err(); err != nil {
			out = append(out, Record{
				Index: i, ExitCode: skuerrors.CodeServer.ExitCode(),
				Error: &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()},
			})
			continue
		}
		h, ok := Lookup(op.Command)
		if !ok {
			e := skuerrors.Validation("unknown_command", "command", op.Command,
				"Run `sku schema --list-commands` to see supported batch commands")
			out = append(out, Record{Index: i, ExitCode: e.Code.ExitCode(), Error: e})
			continue
		}
		base := Settings{}
		if env.Settings != nil {
			base = *env.Settings
		}
		perOp := ApplyOverrides(base, op)
		result, err := h(ctx, op.Args, Env{Settings: &perOp, Stdout: env.Stdout, Stderr: env.Stderr})
		if err != nil {
			e := boxErr(err)
			out = append(out, Record{Index: i, ExitCode: e.Code.ExitCode(), Error: e})
			continue
		}
		out = append(out, Record{Index: i, ExitCode: 0, Output: result})
	}
	return out
}

// AggregateExit returns max(exit_code) across records; 0 on empty.
func AggregateExit(recs []Record) int {
	worst := 0
	for _, r := range recs {
		if r.ExitCode > worst {
			worst = r.ExitCode
		}
	}
	return worst
}

func boxErr(err error) *skuerrors.E {
	var e *skuerrors.E
	if errors.As(err, &e) {
		return e
	}
	return &skuerrors.E{Code: skuerrors.CodeGeneric, Message: err.Error()}
}
