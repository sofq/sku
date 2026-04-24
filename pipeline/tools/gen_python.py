"""Python code generation from pipeline/shards/*.yaml.

Regenerates:
  - pipeline/package/budgets.py
  - pipeline/discover/_shards_gen.py

Determinism: shards are rendered in sorted(shard) order so output is
reproducible regardless of filesystem iteration order.
"""

from __future__ import annotations

import argparse
from pathlib import Path

from jinja2 import Environment, FileSystemLoader, StrictUndefined

from shards import ShardDef, load_all

_PIPELINE_ROOT = Path(__file__).resolve().parents[1]
_TEMPLATES = Path(__file__).resolve().parent / "templates"


def _env() -> Environment:
    return Environment(
        loader=FileSystemLoader(str(_TEMPLATES)),
        keep_trailing_newline=True,
        trim_blocks=True,
        lstrip_blocks=True,
        undefined=StrictUndefined,
        autoescape=False,
    )


def _sorted_shards(shards_dir: Path) -> list[ShardDef]:
    shards = load_all(shards_dir)
    return sorted(shards.values(), key=lambda s: s.shard)


def render_budgets(*, shards_dir: Path) -> str:
    tpl = _env().get_template("budgets.py.j2")
    return tpl.render(shards=_sorted_shards(shards_dir))


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--shards-dir", type=Path, default=_PIPELINE_ROOT / "shards")
    ap.add_argument("--out-budgets", type=Path, default=_PIPELINE_ROOT / "package" / "budgets.py")
    args = ap.parse_args(argv)
    args.out_budgets.write_text(render_budgets(shards_dir=args.shards_dir))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
