package output_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/output"
)

func TestShouldColorize_NonTTY_ReturnsFalse(t *testing.T) {
	var buf bytes.Buffer
	require.False(t, output.ShouldColorize(&buf, output.Options{}))
}

func TestShouldColorize_NoColorFlag_ReturnsFalse(t *testing.T) {
	var buf bytes.Buffer
	require.False(t, output.ShouldColorize(&buf, output.Options{NoColor: true}))
}
