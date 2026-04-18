package sku

import (
	"os"

	skuerrors "github.com/sofq/sku/internal/errors"
)

// Execute runs the root Cobra tree and returns the process exit code, writing
// any error to stderr as the spec §4 JSON envelope. Returning int (rather than
// calling os.Exit internally) lets Execute be covered by unit tests and keeps
// the exit-code taxonomy in one place — the skuerrors package.
//
// newRootCmd intentionally stays unexported: future milestones (M2 batch
// registry) populate the command registry from init() side-effects on leaves
// registered by NewCommand-style constructors, not by walking the Cobra tree
// externally. Keeping newRootCmd private prevents callers from reaching into
// Cobra internals and accidentally depending on traversal order.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		return skuerrors.Write(os.Stderr, err)
	}
	return 0
}
