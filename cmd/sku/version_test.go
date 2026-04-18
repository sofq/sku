package sku

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionCmd_EmitsValidJSON(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version"})

	require.NoError(t, cmd.Execute())

	var payload map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload), "stdout must be valid JSON, got %q", out.String())

	for _, key := range []string{"version", "commit", "date", "go_version", "os", "arch"} {
		require.Contains(t, payload, key, "missing key %q", key)
	}
}

func TestVersionCmd_CompactByDefault(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})
	require.NoError(t, cmd.Execute())

	require.NotContains(t, out.String(), "  ", "default output must be compact")
	require.Equal(t, byte('\n'), out.Bytes()[out.Len()-1])
}
