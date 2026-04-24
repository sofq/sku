"""Generator output must match the committed files byte-for-byte."""
from __future__ import annotations

from pathlib import Path

from tools.gen_python import render_budgets, render_shards_gen


def test_render_budgets_matches_committed() -> None:
    repo = Path(__file__).resolve().parents[1]
    rendered = render_budgets(shards_dir=repo / "shards")
    committed = (repo / "package" / "budgets.py").read_text()
    assert rendered == committed, (
        "generated budgets.py diverges from committed file; "
        "run `make generate-python` and commit the result"
    )


def test_render_shards_gen_matches_committed() -> None:
    repo = Path(__file__).resolve().parents[1]
    rendered = render_shards_gen(shards_dir=repo / "shards")
    committed = (repo / "discover" / "_shards_gen.py").read_text()
    assert rendered == committed
