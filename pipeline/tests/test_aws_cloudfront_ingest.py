"""Golden-row test: fixture CloudFront offer JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

import pytest

from ingest.aws_cloudfront import ingest
from normalize.cdn_locations import LOCATION_MAP
from ._tier_contiguity import assert_tiers_contiguous

_DATA = Path(__file__).resolve().parent.parent / "testdata"
FIXTURE = _DATA / "aws_cloudfront" / "offer.json"
GOLDEN = _DATA / "golden" / "aws_cloudfront_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(offer_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_network_cdn_kind():
    rows = list(ingest(offer_path=FIXTURE))
    assert rows
    assert {r["kind"] for r in rows} == {"network.cdn"}


def test_every_row_has_single_egress_dimension():
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        assert [p["dimension"] for p in r["prices"]] == ["egress"]
        assert r["prices"][0]["unit"] == "gb"
        assert r["prices"][0]["tier"] == "0"
        assert r["prices"][0]["tier_upper"] == ""


def test_extra_fields():
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        extra = r["resource_attrs"]["extra"]
        assert extra["mode"] == "edge-egress"
        assert extra["sku"] == "cloudfront-global"
        assert extra["tier"] == "PriceClass_All"


def test_tiers_contiguous():
    rows = list(ingest(offer_path=FIXTURE))
    # Check contiguity per row: each row is an independent single-tier group
    for row in rows:
        assert_tiers_contiguous([row], "network.cdn", "bytes")


def test_resource_name_is_standard():
    rows = list(ingest(offer_path=FIXTURE))
    assert {r["resource_name"] for r in rows} == {"standard"}


def test_regions_cover_all_edge_locations_in_fixture():
    rows = list(ingest(offer_path=FIXTURE))
    assert {r["region"] for r in rows} == {"us-east-1", "eu-west-1", "ap-northeast-1"}


def test_unknown_location_rejected(tmp_path):
    bad = json.loads(FIXTURE.read_text())
    first_sku = next(iter(bad["products"]))
    bad["products"][first_sku]["attributes"]["fromLocation"] = "Antarctica"
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    with pytest.raises(KeyError, match="Antarctica"):
        list(ingest(offer_path=p))


def test_any_location_skipped_does_not_overwrite_us(tmp_path):
    # Upstream emits a fromLocation="Any" SKU carrying the CloudFront free
    # tier (1 TB/mo @ $0). Earlier the ingestor mapped "Any"→us-east-1 and
    # the dict-grouping let the $0 SKU overwrite the real US data-transfer
    # SKU, so `aws cloudfront price --region us-east-1` returned $0.0.
    bad = json.loads(FIXTURE.read_text())
    bad["products"]["SKU-CF-ANY-DTO"] = {
        "sku": "SKU-CF-ANY-DTO",
        "productFamily": "Data Transfer",
        "attributes": {
            "servicecode": "AmazonCloudFront",
            "location": "Any",
            "locationType": "Edge Location",
            "fromLocation": "Any",
            "transferType": "CloudFront Outbound",
            "usagetype": "Global-DataTransfer-Out-Bytes",
        },
    }
    bad["terms"]["OnDemand"]["SKU-CF-ANY-DTO"] = {
        "SKU-CF-ANY-DTO.A1": {
            "offerTermCode": "A1",
            "sku": "SKU-CF-ANY-DTO",
            "priceDimensions": {
                "SKU-CF-ANY-DTO.A1.F1": {
                    "unit": "GB",
                    "pricePerUnit": {"USD": "0.0000000000"},
                    "description": "First 1 TB free under CloudFront free tier",
                    "beginRange": "0",
                    "endRange": "1024",
                },
            },
        },
    }
    p = tmp_path / "with_any.json"
    p.write_text(json.dumps(bad))
    rows = list(ingest(offer_path=p))
    us = [r for r in rows if r["region"] == "us-east-1"]
    assert len(us) == 1, "should have exactly one us-east-1 row"
    assert us[0]["prices"][0]["amount"] == 0.085, "us-east-1 must keep real $0.085 pricing, not be overwritten by Any/free-tier $0.0"

