package sku

import (
	"fmt"
	"os"
)

// Execute is the public entrypoint called from the root main.go. It runs the
// Cobra tree and maps any error to a non-zero exit code. The full exit-code
// taxonomy (§4 of the design spec) is wired in M2; in M0 we only use 0 and 1.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}
