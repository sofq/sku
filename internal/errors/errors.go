// Package errors implements the spec §4 error envelope + exit-code taxonomy.
//
// Every non-zero exit path funnels through Write, which either unwraps an *E
// (a structured sku error) or boxes a plain Go error as a generic_error
// envelope. The exit code taxonomy is stable — agents depend on it per spec §4.
package errors

import (
	"encoding/json"
	"errors"
	"io"
)

// Code is the stable error-code enum emitted in the envelope's error.code and
// mapped to a process exit code via ExitCode.
type Code string

// Exit-code constants — one per spec §4 taxonomy entry.
const (
	CodeOK          Code = ""              // exit 0
	CodeGeneric     Code = "generic_error" // exit 1
	CodeAuth        Code = "auth"          // exit 2
	CodeNotFound    Code = "not_found"     // exit 3
	CodeValidation  Code = "validation"    // exit 4
	CodeRateLimited Code = "rate_limited"  // exit 5
	CodeConflict    Code = "conflict"      // exit 6
	CodeServer      Code = "server"        // exit 7
	CodeStaleData   Code = "stale_data"    // exit 8
)

// ExitCode returns the process exit code for this error code.
func (c Code) ExitCode() int {
	switch c {
	case CodeOK:
		return 0
	case CodeAuth:
		return 2
	case CodeNotFound:
		return 3
	case CodeValidation:
		return 4
	case CodeRateLimited:
		return 5
	case CodeConflict:
		return 6
	case CodeServer:
		return 7
	case CodeStaleData:
		return 8
	default:
		return 1
	}
}

// E is the canonical structured sku error. It implements the error interface
// and renders as the §4 JSON envelope when passed to Write.
type E struct {
	Code       Code           `json:"code"`
	Message    string         `json:"message"`
	Suggestion string         `json:"suggestion,omitempty"`
	Details    map[string]any `json:"details,omitempty"`
}

// Error implements error.
func (e *E) Error() string { return e.Message }

// envelope is the outer wrapper per §4.
type envelope struct {
	Error *E `json:"error"`
}

// Write marshals err to the §4 JSON envelope on w and returns the exit code
// the process should use. A nil err writes nothing and returns 0. Any non-*E
// error is boxed as CodeGeneric.
func Write(w io.Writer, err error) int {
	if err == nil {
		return 0
	}

	var e *E
	if !errors.As(err, &e) {
		e = &E{Code: CodeGeneric, Message: err.Error()}
	}
	enc := json.NewEncoder(w)
	// json.Encoder writes compact JSON + trailing \n, matching §4 stderr shape.
	if marshalErr := enc.Encode(envelope{Error: e}); marshalErr != nil {
		// Fallback: a minimal hand-rolled envelope. Should never trip in practice
		// because map[string]any with primitive values always marshals.
		_, _ = io.WriteString(w,
			`{"error":{"code":"generic_error","message":"error envelope marshal failed"}}`+"\n",
		)
	}
	return e.Code.ExitCode()
}

// NotFound builds an E with CodeNotFound and the common details shape (§4).
func NotFound(provider, service string, appliedFilters map[string]any, suggestion string) *E {
	return &E{
		Code:       CodeNotFound,
		Message:    "No SKU matches filters",
		Suggestion: suggestion,
		Details: map[string]any{
			"provider":        provider,
			"service":         service,
			"applied_filters": appliedFilters,
		},
	}
}

// Validation builds an E with CodeValidation and the common details shape (§4).
func Validation(reason, flag, value string, hint string) *E {
	d := map[string]any{"reason": reason}
	if flag != "" {
		d["flag"] = flag
	}
	if value != "" {
		d["value"] = value
	}
	if hint != "" {
		d["hint"] = hint
	}
	return &E{
		Code:       CodeValidation,
		Message:    "Invalid input",
		Suggestion: hint,
		Details:    d,
	}
}
