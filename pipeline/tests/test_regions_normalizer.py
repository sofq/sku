"""R1 commercial-region coverage tests for pipeline/normalize/regions.yaml."""

from __future__ import annotations

from pathlib import Path

import yaml

from ingest.aws_common import load_region_normalizer

_REGIONS_YAML = Path(__file__).resolve().parent.parent / "normalize" / "regions.yaml"


def test_r1_aws_region_coverage():
    """R1 AWS commercial regions from the standard AWS account region list."""
    norm = load_region_normalizer()
    expected_aws = {
        "us-east-1", "us-east-2", "us-west-1", "us-west-2",
        "ca-central-1", "ca-west-1",
        "eu-west-1", "eu-west-2", "eu-west-3",
        "eu-central-1", "eu-central-2", "eu-north-1", "eu-south-1", "eu-south-2",
        "ap-east-1", "ap-east-2",
        "ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
        "ap-south-1", "ap-south-2",
        "ap-southeast-1", "ap-southeast-2", "ap-southeast-3",
        "ap-southeast-4", "ap-southeast-5", "ap-southeast-6", "ap-southeast-7",
        "af-south-1",
        "il-central-1", "me-south-1", "me-central-1",
        "mx-central-1",
        "sa-east-1",
    }
    actual = {key[1] for key in norm.table if key[0] == "aws"}
    missing = expected_aws - actual
    assert not missing, f"missing AWS regions: {missing}"
    extra = actual - expected_aws
    assert not extra, f"unexpected AWS regions: {extra}"


def test_r1_azure_region_coverage():
    norm = load_region_normalizer()
    expected_azure = {
        "australiacentral", "australiacentral2", "australiaeast", "australiasoutheast",
        "austriaeast", "belgiumcentral",
        "brazilsouth", "brazilsoutheast",
        "canadacentral", "canadaeast",
        "centralindia", "centralus", "chilecentral",
        "denmarkeast",
        "eastasia", "eastus", "eastus2",
        "francecentral", "francesouth",
        "germanynorth", "germanywestcentral",
        "indonesiacentral", "israelcentral", "italynorth",
        "japaneast", "japanwest", "koreacentral", "koreasouth",
        "malaysiawest", "mexicocentral",
        "newzealandnorth", "northcentralus", "northeurope",
        "norwayeast", "norwaywest",
        "polandcentral", "qatarcentral",
        "southafricanorth", "southafricawest",
        "southcentralus", "southindia", "southeastasia",
        "spaincentral", "swedencentral",
        "switzerlandnorth", "switzerlandwest",
        "uaecentral", "uaenorth",
        "uksouth", "ukwest",
        "westcentralus", "westeurope", "westindia", "westus", "westus2", "westus3",
    }
    actual = {key[1] for key in norm.table if key[0] == "azure"}
    missing = expected_azure - actual
    assert not missing, f"missing Azure regions: {missing}"
    extra = actual - expected_azure
    assert not extra, f"unexpected Azure regions: {extra}"


def test_r1_gcp_region_coverage():
    norm = load_region_normalizer()
    expected_gcp = {
        "africa-south1",
        "asia-east1", "asia-east2",
        "asia-northeast1", "asia-northeast2", "asia-northeast3",
        "asia-south1", "asia-south2",
        "asia-southeast1", "asia-southeast2", "asia-southeast3",
        "australia-southeast1", "australia-southeast2",
        "europe-central2",
        "europe-north1", "europe-north2",
        "europe-southwest1",
        "europe-west1", "europe-west2", "europe-west3", "europe-west4",
        "europe-west6", "europe-west8", "europe-west9", "europe-west10", "europe-west12",
        "me-central1", "me-central2", "me-west1",
        "northamerica-northeast1", "northamerica-northeast2", "northamerica-south1",
        "southamerica-east1", "southamerica-west1",
        "us-central1", "us-east1", "us-east4", "us-east5",
        "us-south1",
        "us-west1", "us-west2", "us-west3", "us-west4",
    }
    # Exclude BigQuery multi-region pseudo-regions from the strict geographic check.
    actual = {key[1] for key in norm.table if key[0] == "gcp" and not key[1].startswith("bq-")}
    missing = expected_gcp - actual
    assert not missing, f"missing GCP regions: {missing}"
    extra = actual - expected_gcp
    assert not extra, f"unexpected GCP regions: {extra}"


def test_bq_pseudo_regions_normalize():
    """bq-us / bq-eu are self-referential: normalize(gcp, bq-us) == bq-us."""
    norm = load_region_normalizer()
    assert norm.normalize("gcp", "bq-us") == "bq-us"
    assert norm.normalize("gcp", "bq-eu") == "bq-eu"


def test_bq_bare_region_names_raise():
    """Raw BigQuery multi-region names US / EU must not map — ingest converts them first."""
    norm = load_region_normalizer()
    try:
        norm.normalize("gcp", "US")
        raise AssertionError("Expected KeyError for 'US'")
    except KeyError:
        pass
    try:
        norm.normalize("gcp", "EU")
        raise AssertionError("Expected KeyError for 'EU'")
    except KeyError:
        pass


def test_r1_adds_africa_and_middle_east_groups():
    with _REGIONS_YAML.open() as fh:
        groups = yaml.safe_load(fh)["groups"]
    actual_groups = set(groups)
    assert "africa" in actual_groups
    assert "middle-east" in actual_groups

    for provider in {"aws", "azure", "gcp"}:
        assert any(provider == entry["provider"] for entry in groups["africa"])
        assert any(provider == entry["provider"] for entry in groups["middle-east"])
