package output

import (
	"io"
	"os"

	"golang.org/x/term"
)

// ShouldColorize reports whether human-facing output emitted to w may include
// ANSI color. It returns false when the caller set --no-color (consumed into
// Options.NoColor upstream from NO_COLOR / SKU_NO_COLOR), when w is not an
// *os.File, or when the underlying file descriptor is not a TTY. Data-format
// output paths never call this — agents always receive uncolored bytes.
func ShouldColorize(w io.Writer, opts Options) bool {
	if opts.NoColor {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	// File descriptors from os.Stdout/os.Stderr are small positive ints;
	// the uintptr→int narrowing is safe in every platform sku supports.
	return term.IsTerminal(int(f.Fd())) //nolint:gosec // G115: fd fits in int
}
