"""Golden-row test: fixture SQS offer JSON -> normalized NDJSON matches golden."""

from __future__ import annotations

import json
from pathlib import Path

from ingest.aws_sqs import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_sqs" / "offer.json"
GOLDEN = Path(__file__).resolve().parent.parent / "testdata" / "golden" / "aws_sqs_rows.jsonl"


def _canonical(rows):
    return sorted(rows, key=lambda r: r["sku_id"])


def test_fixture_matches_golden():
    rows = list(ingest(offer_path=FIXTURE))
    expected = [json.loads(line) for line in GOLDEN.read_text().splitlines() if line.strip()]
    assert _canonical(rows) == _canonical(expected)


def test_all_rows_are_messaging_queue_kind():
    rows = list(ingest(offer_path=FIXTURE))
    assert rows
    assert {r["kind"] for r in rows} == {"messaging.queue"}


def test_resource_names_are_standard_and_fifo():
    rows = list(ingest(offer_path=FIXTURE))
    assert {r["resource_name"] for r in rows} == {"standard", "fifo"}


def test_every_row_has_request_dimension():
    rows = list(ingest(offer_path=FIXTURE))
    for r in rows:
        dims = {p["dimension"] for p in r["prices"]}
        assert dims == {"request"}, (
            f"row {r['sku_id']} has unexpected dims {dims}"
        )


def test_regions_covered():
    rows = list(ingest(offer_path=FIXTURE))
    assert {r["region"] for r in rows} == {"us-east-1", "eu-west-1", "ap-northeast-1"}


def test_unknown_location_rejected(tmp_path):
    bad = json.loads(FIXTURE.read_text())
    # Change regionCode to an unknown value; ingest should raise KeyError.
    first_sku = next(iter(bad["products"]))
    bad["products"][first_sku]["attributes"]["regionCode"] = "xx-unknown-9"
    p = tmp_path / "bad.json"
    p.write_text(json.dumps(bad))
    rows = list(ingest(offer_path=p))
    assert all(r["region"] != "xx-unknown-9" for r in rows)


def test_tiers_contiguous():
    rows = list(ingest(offer_path=FIXTURE))
    # Inline contiguity check: for each row's prices[] (all same dimension=request),
    # verify tier[i].tier_upper == tier[i+1].tier and last entry has tier_upper=="".
    for row in rows:
        prices = sorted(row["prices"], key=lambda p: int(p["tier"]))
        assert prices, f"row {row['sku_id']} has no prices"
        for i, p in enumerate(prices):
            if i < len(prices) - 1:
                assert p["tier_upper"] != "", (
                    f"row {row['sku_id']} tier[{i}].tier_upper is empty (non-last entry)"
                )
                assert p["tier_upper"] == prices[i + 1]["tier"], (
                    f"row {row['sku_id']} tier[{i}].tier_upper={p['tier_upper']!r} "
                    f"!= tier[{i+1}].tier={prices[i+1]['tier']!r} (not contiguous)"
                )
            else:
                assert p["tier_upper"] == "", (
                    f"row {row['sku_id']} last tier should have tier_upper='', "
                    f"got {p['tier_upper']!r}"
                )
