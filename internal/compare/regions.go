// Package compare composes cross-provider equivalence queries over multiple
// catalog shards. Spec §2 boundary: commands call compare.Run with a
// per-kind Spec; compare fans out over installed shards, merges the rows,
// and returns them sorted.
package compare

import (
	"fmt"
	"sort"
)

// groupMap hardcodes the v1 subset of pipeline/normalize/regions.yaml. It
// covers every region_normalized value that today's aws-ec2 / azure-vm /
// gcp-gce shards emit. Keep this list in sync with the YAML until m5 wires
// codegen; drift is caught by the cross-shard compare tests.
// Expanded to P1 popular-path set on 2026-04-22.
var groupMap = map[string][]string{
	"us-east":    {"us-east-1", "us-east-2", "ca-central-1", "eastus", "eastus2", "canadacentral", "us-east1", "us-east4", "northamerica-northeast1"},
	"us-central": {"centralus", "southcentralus", "us-central1"},
	"us-west":    {"us-west-1", "us-west-2", "westus2", "westus3", "us-west1"},
	"eu-west":    {"eu-west-1", "eu-west-2", "eu-west-3", "westeurope", "uksouth", "francecentral", "europe-west1", "europe-west2", "europe-west4"},
	"eu-central": {"eu-central-1", "germanywestcentral", "europe-west3"},
	"eu-north":   {"eu-north-1", "northeurope"},
	"asia-ne":    {"ap-northeast-1", "ap-northeast-2", "japaneast", "koreacentral", "asia-northeast1"},
	"asia-se":    {"ap-southeast-1", "southeastasia", "asia-southeast1"},
	"asia-south": {"ap-south-1", "centralindia", "asia-south1"},
	"oceania":    {"ap-southeast-2", "australiaeast", "australia-southeast1"},
	"sa":         {"sa-east-1", "brazilsouth", "southamerica-east1"},
}

var literalSet = func() map[string]struct{} {
	m := map[string]struct{}{}
	for _, v := range groupMap {
		for _, r := range v {
			m[r] = struct{}{}
		}
	}
	return m
}()

// Expand translates --regions input into (literal region list, matched groups).
// Input entries may be either a group key (e.g. "us-east") or a literal
// provider region (e.g. "us-east-1"). Duplicates are removed; the literal
// slice is sorted for deterministic SQL binding.
func Expand(inputs []string) ([]string, []string, error) {
	litSet := map[string]struct{}{}
	var groups []string
	for _, in := range inputs {
		if in == "" {
			continue
		}
		if expanded, ok := groupMap[in]; ok {
			groups = append(groups, in)
			for _, r := range expanded {
				litSet[r] = struct{}{}
			}
			continue
		}
		if _, ok := literalSet[in]; ok {
			litSet[in] = struct{}{}
			continue
		}
		return nil, nil, fmt.Errorf("compare: unknown region or group %q; pass a known group (us-east, us-central, us-west, eu-west, eu-central, eu-north, asia-ne, asia-se, asia-south, oceania, sa) or a provider region literal", in)
	}
	lits := make([]string, 0, len(litSet))
	for r := range litSet {
		lits = append(lits, r)
	}
	sort.Strings(lits)
	return lits, groups, nil
}
