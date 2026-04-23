"""P1 region coverage tests for pipeline/normalize/regions.yaml."""

from __future__ import annotations

from ingest.aws_common import load_region_normalizer


def test_p1_aws_region_coverage():
    """P1 region set: ~16 AWS regions covering 95% of real usage."""
    norm = load_region_normalizer()
    expected_aws = {
        "us-east-1", "us-east-2", "us-west-1", "us-west-2",
        "ca-central-1",
        "eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1", "eu-north-1",
        "ap-northeast-1", "ap-northeast-2",
        "ap-southeast-1", "ap-southeast-2",
        "ap-south-1",
        "sa-east-1",
    }
    actual = {key[1] for key in norm.table if key[0] == "aws"}
    missing = expected_aws - actual
    assert not missing, f"missing AWS regions: {missing}"


def test_p1_azure_region_coverage():
    norm = load_region_normalizer()
    expected_azure = {
        "eastus", "eastus2", "westus2", "westus3", "centralus", "southcentralus",
        "canadacentral",
        "westeurope", "northeurope", "uksouth", "francecentral", "germanywestcentral",
        "japaneast", "koreacentral",
        "southeastasia", "centralindia",
        "australiaeast",
        "brazilsouth",
    }
    actual = {key[1] for key in norm.table if key[0] == "azure"}
    missing = expected_azure - actual
    assert not missing, f"missing Azure regions: {missing}"


def test_p1_gcp_region_coverage():
    norm = load_region_normalizer()
    expected_gcp = {
        "us-east1", "us-east4", "us-central1", "us-west1",
        "northamerica-northeast1",
        "europe-west1", "europe-west2", "europe-west3", "europe-west4",
        "asia-northeast1", "asia-southeast1", "asia-south1",
        "australia-southeast1",
        "southamerica-east1",
    }
    actual = {key[1] for key in norm.table if key[0] == "gcp"}
    missing = expected_gcp - actual
    assert not missing, f"missing GCP regions: {missing}"
