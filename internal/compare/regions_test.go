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

func TestExpand_r1GroupsExpand(t *testing.T) {
	lits, groups, err := Expand([]string{"africa", "middle-east"})
	require.NoError(t, err)
	require.Contains(t, lits, "af-south-1")
	require.Contains(t, lits, "southafricanorth")
	require.Contains(t, lits, "africa-south1")
	require.Contains(t, lits, "me-central-1")
	require.Contains(t, lits, "qatarcentral")
	require.Contains(t, lits, "me-central1")
	require.Equal(t, []string{"africa", "middle-east"}, groups)
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

func TestExpand_globalLiteralAccepted(t *testing.T) {
	// Globally-priced services (DNS, GCP Pub/Sub) emit region="global".
	// `--regions global` must be accepted so users can compare them.
	lits, groups, err := Expand([]string{"global"})
	require.NoError(t, err)
	require.Equal(t, []string{"global"}, lits)
	require.Empty(t, groups)
}

func TestExpand_groupIncludesGroupKeyAndGlobal(t *testing.T) {
	// Some shards pin `region` to the normalized group key (e.g. Azure
	// Front Door bills per multi-region zone and stores `region="us-east"`).
	// And globally-priced services emit region="global". Expanding a group
	// must include both so a regional compare query catches them.
	lits, _, err := Expand([]string{"us-east"})
	require.NoError(t, err)
	require.Contains(t, lits, "us-east", "group key itself must appear in literals")
	require.Contains(t, lits, "global", "global must appear so DNS/Pub/Sub rows match")
	require.Contains(t, lits, "us-east-1", "expanded provider regions must still appear")
}
