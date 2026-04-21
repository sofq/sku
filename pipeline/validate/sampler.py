"""Stratified sampler for shard validation.

Spec §6 stratified sampling — partitions a budget across:
  (a) top 3 regions by row count: 70% of budget
  (b) one SKU from the long-tail region set: 10%
  (c) top-N resource families by row count: 15%
  (d) remainder filled from any row not yet selected: 5%

Returns a deterministic list of Sample records given the same seed.
"""

from __future__ import annotations

import math
import random
import sqlite3
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Sample:
    sku_id: str
    region: str
    resource_name: str
    price_amount: float
    price_currency: str
    dimension: str


def sample(
    shard_db: Path,
    *,
    budget: int = 20,
    seed: int | None = None,
) -> list[Sample]:
    """Return a stratified sample from a shard SQLite database.

    Parameters
    ----------
    shard_db:
        Path to the SQLite shard file.
    budget:
        Maximum number of samples to return.
    seed:
        Random seed for deterministic results. ``None`` for random.

    Returns
    -------
    list[Sample]
        At most ``budget`` samples, de-duplicated by (sku_id, dimension).
    """
    rng = random.Random(seed)

    con = sqlite3.connect(f"file:{shard_db}?mode=ro", uri=True)
    try:
        # Read the global currency once.
        row = con.execute(
            "SELECT value FROM metadata WHERE key = 'currency'"
        ).fetchone()
        currency = row[0] if row else "USD"

        # Pull every (sku_id, region, resource_name, dimension, amount) row.
        rows = con.execute(
            """
            SELECT s.sku_id, s.region, s.resource_name,
                   p.dimension, p.amount
            FROM skus s
            JOIN prices p USING (sku_id)
            """
        ).fetchall()
    finally:
        con.close()

    if not rows:
        return []

    # Build lookup structures.
    all_items: list[tuple[str, str, str, str, float]] = rows  # typed alias

    # Region row counts.
    region_counts: dict[str, int] = {}
    for _, region, _, _, _ in all_items:
        region_counts[region] = region_counts.get(region, 0) + 1

    sorted_regions = sorted(region_counts, key=lambda r: -region_counts[r])
    top_regions = set(sorted_regions[:3])
    long_tail_regions = set(sorted_regions[3:])

    # Family extraction (first segment of resource_name before '.').
    def _family(resource_name: str) -> str:
        return resource_name.split(".")[0] if "." in resource_name else resource_name

    family_counts: dict[str, int] = {}
    for _, _, resource_name, _, _ in all_items:
        fam = _family(resource_name)
        family_counts[fam] = family_counts.get(fam, 0) + 1

    sorted_families = sorted(family_counts, key=lambda f: -family_counts[f])
    # Top families cover 15% of budget (at least 1).
    top_family_n = max(1, math.ceil(budget * 0.15 / max(1, len(sorted_families))))
    top_families = set(sorted_families[:top_family_n])

    # Partition items by stratum.
    top_region_items: list[tuple] = []
    long_tail_items: list[tuple] = []
    top_family_items: list[tuple] = []

    for item in all_items:
        _, region, resource_name, _, _ = item
        fam = _family(resource_name)
        if region in top_regions:
            top_region_items.append(item)
        elif region in long_tail_regions:
            long_tail_items.append(item)
        if fam in top_families:
            top_family_items.append(item)

    # Allocate budget slots.
    top_region_budget = max(1, int(budget * 0.70))
    long_tail_budget = max(1, int(budget * 0.10))
    top_family_budget = max(1, int(budget * 0.15))
    remainder_budget = budget - top_region_budget - long_tail_budget - top_family_budget
    remainder_budget = max(0, remainder_budget)

    selected: list[tuple] = []
    seen: set[tuple[str, str]] = set()

    def _pick(pool: list[tuple], n: int) -> None:
        shuffled = list(pool)
        rng.shuffle(shuffled)
        for item in shuffled:
            if len(selected) >= budget:
                break
            key = (item[0], item[3])  # (sku_id, dimension)
            if key not in seen and n > 0:
                selected.append(item)
                seen.add(key)
                n -= 1

    _pick(top_region_items, top_region_budget)
    if long_tail_items:
        _pick(long_tail_items, long_tail_budget)
    _pick(top_family_items, top_family_budget)
    # Fill remainder from all items not yet chosen.
    _pick(all_items, remainder_budget + budget - len(selected))

    return [
        Sample(
            sku_id=sku_id,
            region=region,
            resource_name=resource_name,
            price_amount=amount,
            price_currency=currency,
            dimension=dimension,
        )
        for sku_id, region, resource_name, dimension, amount in selected
    ]
