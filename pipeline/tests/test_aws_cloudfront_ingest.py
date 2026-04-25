"""Golden-row test: fixture CloudFront offer JSON -> normalized NDJSON matches golden."""

import json
from pathlib import Path

import pytest

from ingest.aws_cloudfront import LOCATION_MAP, ingest

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


def test_every_row_has_single_dto_dimension():
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        assert [p["dimension"] for p in r["prices"]] == ["data_transfer_out"]
        assert r["prices"][0]["unit"] == "gb"


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


def test_location_map_fans_out_to_p1_regions():
    values = set(LOCATION_MAP.values())
    assert "sa-east-1" in values, "South America must fan out to sa-east-1"
    assert "ap-southeast-2" in values, "Australia must fan out to ap-southeast-2"
    assert "ap-south-1" in values, "India must fan out to ap-south-1"
    assert "ca-central-1" in values, "Canada must fan out to ca-central-1"
    assert "me-central-1" in values, "Middle East must fan out to me-central-1"
    assert "af-south-1" in values, "South Africa must fan out to af-south-1"


def test_location_map_exhaustiveness():
    # Any edge-location string we know about must map to a region that
    # regions.yaml would accept. Guards against silent drift if someone
    # adds to LOCATION_MAP without extending regions.yaml.
    from ingest.aws_common import load_region_normalizer
    norm = load_region_normalizer()
    for region in LOCATION_MAP.values():
        assert norm.normalize("aws", region), f"{region} not in regions.yaml"
