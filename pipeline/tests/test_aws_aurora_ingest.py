from pathlib import Path

from ingest.aws_aurora import ingest

FIXTURE = Path(__file__).resolve().parent.parent / "testdata" / "aws_aurora" / "offer.json"


def test_ingest_emits_provisioned_rows():
    rows = list(ingest(offer_path=FIXTURE))
    provisioned = [r for r in rows if r["resource_attrs"]["extra"].get("capacity_mode") == "provisioned"]
    assert len(provisioned) == 8
    sample = provisioned[0]
    assert sample["kind"] == "db.relational"
    assert sample["provider"] == "aws"
    assert sample["service"] == "aurora"
    assert sample["resource_attrs"]["extra"]["engine"] in {"aurora-mysql", "aurora-postgres"}


def test_ingest_emits_serverless_v2_rows():
    rows = list(ingest(offer_path=FIXTURE))
    sv2 = [r for r in rows if r["resource_attrs"]["extra"].get("capacity_mode") == "serverless-v2"]
    assert len(sv2) == 4
    sample = sv2[0]
    assert sample["resource_name"] == "aurora-serverless-v2"
    assert sample["resource_attrs"]["vcpu"] is None
    assert "acu_hour_usd" in sample["resource_attrs"]["extra"]
    assert sample["prices"][0]["unit"] == "acu-hr"


def test_ingest_carries_storage_in_extra():
    rows = list(ingest(offer_path=FIXTURE))
    east_rows = [r for r in rows if r["region"] == "us-east-1" and r["resource_attrs"]["extra"].get("capacity_mode") == "provisioned"]
    assert east_rows, "fixture should produce us-east-1 provisioned rows"
    for r in east_rows:
        assert "storage_gb_month_usd" in r["resource_attrs"]["extra"]


def test_ingest_skips_unknown_engines():
    rows = list(ingest(offer_path=FIXTURE))
    engines = {r["resource_attrs"]["extra"]["engine"] for r in rows}
    assert engines.issubset({"aurora-mysql", "aurora-postgres"})


def test_ingest_empty_offer_returns_no_rows(tmp_path):
    empty = tmp_path / "empty.json"
    empty.write_text('{"products":{}, "terms":{"OnDemand":{}}}')
    rows = list(ingest(offer_path=empty))
    assert rows == []
