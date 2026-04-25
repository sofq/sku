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
// Expanded to R1 commercial-region set on 2026-04-24.
var groupMap = map[string][]string{
	"us-east":     {"us-east-1", "us-east-2", "ca-central-1", "canadaeast", "eastus", "eastus2", "canadacentral", "us-east1", "us-east4", "us-east5", "northamerica-northeast1", "northamerica-northeast2"},
	"us-central":  {"mx-central-1", "centralus", "northcentralus", "southcentralus", "mexicocentral", "us-central1", "us-south1", "northamerica-south1"},
	"us-west":     {"us-west-1", "us-west-2", "ca-west-1", "westus", "westcentralus", "westus2", "westus3", "us-west1", "us-west2", "us-west3", "us-west4"},
	"eu-west":     {"eu-west-1", "eu-west-2", "eu-west-3", "eu-south-1", "eu-south-2", "belgiumcentral", "westeurope", "uksouth", "ukwest", "francecentral", "francesouth", "italynorth", "spaincentral", "europe-west1", "europe-west2", "europe-west4", "europe-west8", "europe-west9", "europe-west12", "europe-southwest1"},
	"eu-central":  {"eu-central-1", "eu-central-2", "austriaeast", "germanywestcentral", "germanynorth", "polandcentral", "switzerlandnorth", "switzerlandwest", "europe-west3", "europe-west6", "europe-west10", "europe-central2"},
	"eu-north":    {"eu-north-1", "denmarkeast", "northeurope", "norwayeast", "norwaywest", "swedencentral", "europe-north1", "europe-north2"},
	"asia-ne":     {"ap-east-1", "ap-east-2", "ap-northeast-1", "ap-northeast-2", "ap-northeast-3", "eastasia", "japaneast", "japanwest", "koreacentral", "koreasouth", "asia-northeast1", "asia-northeast2", "asia-northeast3", "asia-east1", "asia-east2"},
	"asia-se":     {"ap-southeast-1", "ap-southeast-3", "ap-southeast-5", "ap-southeast-7", "indonesiacentral", "malaysiawest", "southeastasia", "asia-southeast1", "asia-southeast2", "asia-southeast3"},
	"asia-south":  {"ap-south-1", "ap-south-2", "centralindia", "southindia", "westindia", "asia-south1", "asia-south2"},
	"oceania":     {"ap-southeast-2", "ap-southeast-4", "ap-southeast-6", "australiacentral", "australiacentral2", "australiaeast", "australiasoutheast", "newzealandnorth", "australia-southeast1", "australia-southeast2"},
	"sa":          {"sa-east-1", "brazilsouth", "brazilsoutheast", "chilecentral", "southamerica-east1", "southamerica-west1"},
	"africa":      {"af-south-1", "southafricanorth", "southafricawest", "africa-south1"},
	"middle-east": {"il-central-1", "me-south-1", "me-central-1", "israelcentral", "qatarcentral", "uaecentral", "uaenorth", "me-west1", "me-central1", "me-central2"},
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
		return nil, nil, fmt.Errorf("compare: unknown region or group %q; pass a known group (us-east, us-central, us-west, eu-west, eu-central, eu-north, asia-ne, asia-se, asia-south, oceania, sa, africa, middle-east) or a provider region literal", in)
	}
	lits := make([]string, 0, len(litSet))
	for r := range litSet {
		lits = append(lits, r)
	}
	sort.Strings(lits)
	return lits, groups, nil
}
