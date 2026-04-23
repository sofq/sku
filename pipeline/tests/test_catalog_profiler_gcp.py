from __future__ import annotations

from pathlib import Path

from catalog_profiler.__main__ import main
from catalog_profiler.gcp import scan_catalog


def test_scan_gcp_gce_fixture():
    path = Path(__file__).resolve().parent.parent / "testdata" / "gcp_gce" / "skus.json"
    rows = scan_catalog(skus_path=path)
    assert any(r.bucket_label.startswith("Compute Engine") for r in rows)
    ce = next(r for r in rows if r.bucket_label.startswith("Compute Engine"))
    assert "description" in ce.attribute_keys
    assert "category.resourceGroup" in ce.attribute_keys


def test_gcp_command_rejects_empty_catalog_path_set(tmp_path: Path, capsys):
    out = tmp_path / "gcp.md"

    rc = main(["gcp", "--catalog-paths", str(tmp_path / "*.json"), "--out", str(out)])

    assert rc == 2
    assert not out.exists()
    assert "no GCP catalog JSON files found" in capsys.readouterr().err
