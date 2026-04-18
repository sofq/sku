// Package version exposes build-time metadata injected by goreleaser via -ldflags.
package version

import "runtime"

// These are overwritten at build time with -ldflags "-X github.com/sofq/sku/internal/version.version=..."
// Kept as package-level vars (not consts) so ldflags can patch them and tests can override.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Info is the JSON payload emitted by `sku version`.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Get returns the current build's Info, including runtime-derived fields.
func Get() Info {
	return Info{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}
