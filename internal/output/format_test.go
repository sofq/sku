package output_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/output"
)

func TestEncode_JSONCompact(t *testing.T) {
	b, err := output.Encode(map[string]any{"a": 1}, "json", false)
	require.NoError(t, err)
	require.Equal(t, `{"a":1}`+"\n", string(b))
}

func TestEncode_JSONPretty(t *testing.T) {
	b, err := output.Encode(map[string]any{"a": 1}, "json", true)
	require.NoError(t, err)
	require.Contains(t, string(b), "\n  \"a\"")
}

func TestEncode_YAML(t *testing.T) {
	b, err := output.Encode(map[string]any{"a": 1}, "yaml", false)
	require.NoError(t, err)
	require.Contains(t, string(b), "a: 1")
}

func TestEncode_TOML(t *testing.T) {
	b, err := output.Encode(map[string]any{"a": 1}, "toml", false)
	require.NoError(t, err)
	require.Contains(t, string(b), "a = 1")
}

func TestEncode_UnknownFormat(t *testing.T) {
	_, err := output.Encode(map[string]any{"a": 1}, "xml", false)
	require.Error(t, err)
}
