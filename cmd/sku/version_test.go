package sku

import (
	"bytes"
	"encoding/json"
	"strings"
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

func TestVersionCmd_YAMLOutput(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--yaml", "version"})
	require.NoError(t, cmd.Execute())

	s := out.String()
	require.Contains(t, s, "version:")
	require.Contains(t, s, "go_version:")
	// YAML output must not be JSON.
	require.False(t, strings.HasPrefix(strings.TrimSpace(s), "{"),
		"yaml output should not start with {, got %q", s)
}
