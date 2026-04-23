"""Tests for the streaming-strip + per-region fetch helpers in aws_common."""

from __future__ import annotations

import json
from pathlib import Path

import pytest
import requests_mock as req_mock

from ingest.aws_common import (
    _strip_ec2_offer,
    aws_regions_from_yaml,
    fetch_offer_regions_stripped,
    shared_offer_basename,
)


def _write(path: Path, obj: dict) -> None:
    path.write_text(json.dumps(obj))


def test_strip_keeps_only_whitelisted_fields(tmp_path: Path) -> None:
    """Strip drops Reserved terms, offerTermCode, description, non-EC2 attrs."""
    raw = {
        "formatVersion": "v1.0",
        "offerCode": "AmazonEC2",
        "publicationDate": "2026-04-18T00:00:00Z",
        "version": "20260418000000",
        "products": {
            "sku-compute": {
                "productFamily": "Compute Instance",
                "sku": "sku-compute",
                "attributes": {
                    "instanceType": "m5.large",
                    "regionCode": "us-east-1",
                    "operatingSystem": "Linux",
                    "tenancy": "Shared",
                    "preInstalledSw": "NA",
                    "capacitystatus": "Used",
                    "vcpu": "2",
                    "memory": "8 GiB",
                    "physicalProcessor": "Intel",
                    "networkPerformance": "10 Gig",
                    "servicecode": "AmazonEC2",  # dropped
                    "location": "US East",  # dropped
                },
            },
            "sku-storage": {
                "productFamily": "Storage",
                "attributes": {
                    "volumeApiName": "gp3",
                    "regionCode": "us-east-1",
                    "servicecode": "AmazonEC2",  # dropped
                },
            },
            "sku-ignored": {
                "productFamily": "Data Transfer",  # whole product dropped
                "attributes": {"regionCode": "us-east-1"},
            },
        },
        "terms": {
            "OnDemand": {
                "sku-compute": {
                    "sku-compute.TERM": {
                        "offerTermCode": "JRTC",  # dropped
                        "effectiveDate": "2020-01-01",  # dropped
                        "termAttributes": {},  # dropped
                        "priceDimensions": {
                            "sku-compute.TERM.RATE": {
                                "rateCode": "abc",  # dropped
                                "description": "$0.096 per hour",  # dropped
                                "beginRange": "0",  # dropped
                                "endRange": "Inf",  # dropped
                                "unit": "Hrs",
                                "pricePerUnit": {"USD": "0.0960000000"},
                            }
                        },
                    }
                },
                "sku-ignored": {  # referenced but product dropped
                    "sku-ignored.TERM": {
                        "priceDimensions": {"x.y.z": {"unit": "GB", "pricePerUnit": {"USD": "1"}}}
                    }
                },
            },
            "Reserved": {  # entire Reserved block dropped
                "sku-compute": {"a.b": {"priceDimensions": {}}}
            },
        },
    }
    raw_path = tmp_path / "raw.json"
    out_path = tmp_path / "stripped.json"
    _write(raw_path, raw)

    _strip_ec2_offer(raw_path, out_path)
    stripped = json.loads(out_path.read_text())

    # Products: kept two, dropped one.
    assert set(stripped["products"]) == {"sku-compute", "sku-storage"}
    compute_attrs = stripped["products"]["sku-compute"]["attributes"]
    assert "servicecode" not in compute_attrs
    assert "location" not in compute_attrs
    assert compute_attrs["instanceType"] == "m5.large"
    assert compute_attrs["vcpu"] == "2"
    assert stripped["products"]["sku-storage"]["attributes"]["volumeApiName"] == "gp3"

    # Terms: Reserved gone; OnDemand for dropped products pruned.
    assert set(stripped["terms"]) == {"OnDemand"}
    assert set(stripped["terms"]["OnDemand"]) == {"sku-compute"}
    term = next(iter(stripped["terms"]["OnDemand"]["sku-compute"].values()))
    assert set(term) == {"priceDimensions"}  # offerTermCode, termAttributes gone
    pd = next(iter(term["priceDimensions"].values()))
    assert set(pd) == {"unit", "pricePerUnit"}
    assert pd["unit"] == "Hrs"
    assert pd["pricePerUnit"]["USD"] == "0.0960000000"


def test_strip_reduces_file_size(tmp_path: Path) -> None:
    """Sanity check: stripping the existing fixture produces a smaller file.

    The fixture is already hand-trimmed (no Reserved, short descriptions), so
    this mostly catches regressions where we accidentally stop trimming. Real
    per-region AWS offers see ~95%+ reduction.
    """
    fixture = Path(__file__).resolve().parent.parent / "testdata" / "aws_ec2" / "offer.json"
    out_path = tmp_path / "stripped.json"
    _strip_ec2_offer(fixture, out_path)
    assert out_path.stat().st_size < fixture.stat().st_size


def test_aws_regions_from_yaml_matches_regions_yaml() -> None:
    """Hardcoded regions list matches current regions.yaml coverage."""
    assert aws_regions_from_yaml() == [
        "ap-northeast-1",
        "ap-northeast-2",
        "ap-south-1",
        "ap-southeast-1",
        "ap-southeast-2",
        "ca-central-1",
        "eu-central-1",
        "eu-north-1",
        "eu-west-1",
        "eu-west-2",
        "eu-west-3",
        "sa-east-1",
        "us-east-1",
        "us-east-2",
        "us-west-1",
        "us-west-2",
    ]


def test_fetch_offer_regions_stripped(tmp_path: Path) -> None:
    """End-to-end: mock region_index + per-region fetches, verify strip produces output."""
    region_index_url = (
        "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/region_index.json"
    )
    region_a_url = (
        "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/20260418/us-east-1/index.json"
    )
    region_b_url = (
        "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/20260418/eu-west-1/index.json"
    )
    region_index = {
        "regions": {
            "us-east-1": {
                "regionCode": "us-east-1",
                "currentVersionUrl": "/offers/v1.0/aws/AmazonEC2/20260418/us-east-1/index.json",
            },
            "eu-west-1": {
                "regionCode": "eu-west-1",
                "currentVersionUrl": "/offers/v1.0/aws/AmazonEC2/20260418/eu-west-1/index.json",
            },
        }
    }
    tiny_offer = {
        "products": {
            "sku-x": {
                "productFamily": "Compute Instance",
                "attributes": {"instanceType": "m5.large", "regionCode": "us-east-1"},
            }
        },
        "terms": {
            "OnDemand": {
                "sku-x": {
                    "sku-x.T": {
                        "priceDimensions": {
                            "sku-x.T.R": {"unit": "Hrs", "pricePerUnit": {"USD": "0.1"}}
                        }
                    }
                }
            }
        },
    }
    body_index = json.dumps(region_index).encode()
    body_offer = json.dumps(tiny_offer).encode()

    idx_hdr = {"Content-Length": str(len(body_index))}
    offer_hdr = {"Content-Length": str(len(body_offer))}
    with req_mock.Mocker() as m:
        m.get(region_index_url, content=body_index, headers=idx_hdr)
        m.get(region_a_url, content=body_offer, headers=offer_hdr)
        m.get(region_b_url, content=body_offer, headers=offer_hdr)
        out_dir = tmp_path / "stripped"
        paths = fetch_offer_regions_stripped(
            "aws_ec2", out_dir, regions=["us-east-1", "eu-west-1", "ap-south-99"]
        )

    # Only two regions existed upstream; the missing one is silently skipped.
    assert sorted(p.name for p in paths) == [
        "aws_ec2-eu-west-1.json",
        "aws_ec2-us-east-1.json",
    ]
    # Raw files removed; stripped files remain.
    assert not any(out_dir.glob("*.raw.json"))
    # Each stripped file is valid JSON with our trimmed shape.
    for p in paths:
        doc = json.loads(p.read_text())
        assert list(doc["products"]) == ["sku-x"]
        assert list(doc["terms"]["OnDemand"]) == ["sku-x"]


def test_fetch_unsupported_shard_raises(tmp_path: Path) -> None:
    with pytest.raises(ValueError, match="per-region offer not supported"):
        fetch_offer_regions_stripped("aws_s3", tmp_path, regions=["us-east-1"])


def test_aws_ebs_shares_aws_ec2_stripped_offer() -> None:
    """aws_ebs and aws_ec2 both consume AmazonEC2 — the stripped on-disk
    file uses one shared basename so a single fetch serves both ingesters.
    """
    assert shared_offer_basename("aws_ec2") == "aws_ec2"
    assert shared_offer_basename("aws_ebs") == "aws_ec2"


def test_fetch_reuses_existing_stripped_file(tmp_path: Path) -> None:
    """If aws_ec2 already produced aws_ec2-<region>.json, a subsequent
    aws_ebs fetch skips re-downloading and re-stripping that region.
    """
    region_index_url = (
        "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/current/region_index.json"
    )
    region_a_url = (
        "https://pricing.us-east-1.amazonaws.com/offers/v1.0/aws/AmazonEC2/20260418/us-east-1/index.json"
    )
    region_index = {
        "regions": {
            "us-east-1": {
                "regionCode": "us-east-1",
                "currentVersionUrl": "/offers/v1.0/aws/AmazonEC2/20260418/us-east-1/index.json",
            },
        }
    }
    body_index = json.dumps(region_index).encode()
    idx_hdr = {"Content-Length": str(len(body_index))}

    out_dir = tmp_path / "stripped"
    out_dir.mkdir()
    # Pretend aws_ec2 already produced a stripped offer for us-east-1.
    preexisting = out_dir / "aws_ec2-us-east-1.json"
    sentinel_body = {"products": {}, "terms": {"OnDemand": {}}, "sentinel": "preserved"}
    preexisting.write_text(json.dumps(sentinel_body))

    # Only the region_index.json endpoint should be hit; the per-region URL
    # must NOT be fetched because the stripped file already exists.
    with req_mock.Mocker() as m:
        m.get(region_index_url, content=body_index, headers=idx_hdr)
        # Register a failing matcher for the per-region URL so any reach
        # for it surfaces as a test failure.
        m.get(region_a_url, status_code=500)

        paths = fetch_offer_regions_stripped(
            "aws_ebs", out_dir, regions=["us-east-1"]
        )

    assert paths == [preexisting]
    assert json.loads(preexisting.read_text())["sentinel"] == "preserved"
