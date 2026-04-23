from __future__ import annotations

from pathlib import Path

from catalog_profiler.gcp import scan_catalog


def test_scan_gcp_gce_fixture():
    path = Path(__file__).resolve().parent.parent / "testdata" / "gcp_gce" / "skus.json"
    rows = scan_catalog(skus_path=path)
    assert any(r.bucket_label.startswith("Compute Engine") for r in rows)
    ce = next(r for r in rows if r.bucket_label.startswith("Compute Engine"))
    assert "description" in ce.attribute_keys
    assert "category.resourceGroup" in ce.attribute_keys
