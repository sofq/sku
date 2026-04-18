package output

import (
	"encoding/json"
	"io"
	"time"
)

// Log emits one JSON object per call to w with a ts/event header.
// Intended for --verbose stderr tracing; errors marshaling are swallowed
// because verbose output is best-effort.
func Log(w io.Writer, event string, fields map[string]any) {
	if fields == nil {
		fields = map[string]any{}
	}
	fields["ts"] = time.Now().UTC().Format(time.RFC3339Nano)
	fields["event"] = event
	_ = json.NewEncoder(w).Encode(fields)
}
