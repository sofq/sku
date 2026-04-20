package sku

import (
	"errors"
	"os"

	"github.com/sofq/sku/internal/batch"
	skuerrors "github.com/sofq/sku/internal/errors"
)

// Execute runs the root Cobra tree and returns the process exit code, writing
// any error to stderr as the spec §4 JSON envelope. Returning int (rather than
// calling os.Exit internally) lets Execute be covered by unit tests and keeps
// the exit-code taxonomy in one place — the skuerrors package.
func Execute() int {
	err := newRootCmd().Execute()
	if err == nil {
		return 0
	}
	// Batch aggregate: per-op errors already live inside the stdout records,
	// so the batch command returns an ErrAggregate-wrapped *skuerrors.E that
	// carries only the aggregated exit code — no stderr envelope.
	if errors.Is(err, batch.ErrAggregate) {
		var e *skuerrors.E
		if errors.As(err, &e) {
			return e.Code.ExitCode()
		}
		return 1
	}
	return skuerrors.Write(os.Stderr, err)
}
