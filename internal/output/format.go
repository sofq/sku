package output

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Encode serializes doc to the requested format.
//
//   - "json" / "": compact by default, indented when pretty. Always terminated
//     with a single newline.
//   - "yaml": yaml.v3 marshal; pretty is a no-op (yaml is always formatted).
//     yaml.v3 already emits a trailing newline so we do not add another.
//   - "toml": go-toml/v2. TOML cannot represent a top-level array, so any
//     slice-typed doc is wrapped as {"rows": doc} before marshaling.
//
// Unknown formats return an error.
func Encode(doc any, format string, pretty bool) ([]byte, error) {
	switch format {
	case "", "json":
		var (
			b   []byte
			err error
		)
		if pretty {
			b, err = json.MarshalIndent(doc, "", "  ")
		} else {
			b, err = json.Marshal(doc)
		}
		if err != nil {
			return nil, err
		}
		return append(b, '\n'), nil
	case "yaml":
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		if err := enc.Encode(doc); err != nil {
			_ = enc.Close()
			return nil, err
		}
		if err := enc.Close(); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	case "toml":
		// TOML has no syntax for a top-level array — wrap slices under "rows".
		wrapped := wrapForTOML(doc)
		b, err := toml.Marshal(wrapped)
		if err != nil {
			return nil, err
		}
		return b, nil
	default:
		return nil, fmt.Errorf("output: unknown format %q", format)
	}
}

// wrapForTOML wraps any slice-typed doc under {"rows": doc} so toml.Marshal
// can serialize it. Maps pass through unchanged.
func wrapForTOML(doc any) any {
	switch doc.(type) {
	case []any, []map[string]any:
		return map[string]any{"rows": doc}
	}
	return doc
}
