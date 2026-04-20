package compare

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpand_literalPassthrough(t *testing.T) {
	lits, groups, err := Expand([]string{"us-east-1", "eastus", "us-east1"})
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"us-east-1", "eastus", "us-east1"}, lits)
	require.Empty(t, groups)
}

func TestExpand_knownGroupExpands(t *testing.T) {
	lits, groups, err := Expand([]string{"us-east"})
	require.NoError(t, err)
	require.Contains(t, lits, "us-east-1")
	require.Contains(t, lits, "eastus")
	require.Contains(t, lits, "us-east1")
	require.Equal(t, []string{"us-east"}, groups)
}

func TestExpand_dedupsMixedInput(t *testing.T) {
	lits, _, err := Expand([]string{"us-east", "us-east-1"})
	require.NoError(t, err)
	seen := map[string]int{}
	for _, l := range lits {
		seen[l]++
	}
	require.Equal(t, 1, seen["us-east-1"], "literal must not duplicate when both group and literal passed")
}

func TestExpand_unknownInput(t *testing.T) {
	_, _, err := Expand([]string{"mars-crater-1"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mars-crater-1")
}
