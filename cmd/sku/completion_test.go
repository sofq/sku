package sku

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompletion_Bash_ContainsBashCompletionSentinel(t *testing.T) {
	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetArgs([]string{"completion", "bash"})
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "bash completion")
}
