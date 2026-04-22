package sku

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/batch"
	skuerrors "github.com/sofq/sku/internal/errors"
)

func newBatchCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "batch",
		Short: "Run multiple sku ops from stdin (NDJSON or JSON array)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			g := globalSettings(cmd)

			if g.Format != "json" {
				e := skuerrors.Validation("flag_invalid", "--"+g.Format, g.Format,
					"batch output is always JSON; --yaml and --toml are not supported")
				skuerrors.Write(cmd.ErrOrStderr(), e)
				return e
			}

			settings := ToBatchSettings(g)

			ops, format, err := batch.Parse(cmd.InOrStdin())
			if err != nil {
				e := skuerrors.Validation("flag_invalid", "stdin", "", err.Error())
				skuerrors.Write(cmd.ErrOrStderr(), e)
				return e
			}

			env := batch.Env{Settings: &settings, Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr()}
			records := batch.Dispatch(context.Background(), ops, env)

			if err := emitBatch(cmd, format, records, g.Pretty); err != nil {
				return fmt.Errorf("batch: emit: %w", err)
			}

			code := batch.AggregateExit(records)
			if code == 0 {
				return nil
			}
			// Wrap with ErrAggregate so Execute() suppresses the stderr envelope:
			// per-op errors already live inside the stdout records.
			return fmt.Errorf("%w: %w", batch.ErrAggregate, &skuerrors.E{Code: exitCodeToErrCode(code), Message: "batch completed with per-op errors"})
		},
	}
	return c
}

func emitBatch(cmd *cobra.Command, f batch.Format, recs []batch.Record, pretty bool) error {
	w := cmd.OutOrStdout()
	switch f {
	case batch.FormatArray:
		enc := json.NewEncoder(w)
		if pretty {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(recs)
	case batch.FormatNDJSON:
		for _, r := range recs {
			enc := json.NewEncoder(w)
			if pretty {
				enc.SetIndent("", "  ")
			}
			if err := enc.Encode(r); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown format %v", f)
	}
}

// exitCodeToErrCode maps an exit code int back to the canonical Code so the
// batch wrapper can return one structured error that carries the aggregated
// severity to Execute().
func exitCodeToErrCode(exit int) skuerrors.Code {
	switch exit {
	case 2:
		return skuerrors.CodeAuth
	case 3:
		return skuerrors.CodeNotFound
	case 4:
		return skuerrors.CodeValidation
	case 5:
		return skuerrors.CodeRateLimited
	case 6:
		return skuerrors.CodeConflict
	case 7:
		return skuerrors.CodeServer
	case 8:
		return skuerrors.CodeStaleData
	}
	return skuerrors.CodeGeneric
}
