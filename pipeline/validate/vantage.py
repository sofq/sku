"""Offline EC2 cross-check against vantage-sh/ec2instances.info.

Downloads ``instances.json`` at workflow start (handled by the GitHub Actions
step) and joins with the EC2 shard on
``(instance_type, region, os='linux', tenancy='shared')``.
"""

from __future__ import annotations

import json
import logging
import sqlite3
from dataclasses import dataclass
from pathlib import Path

logger = logging.getLogger(__name__)

_VANTAGE_URL = (
    "https://raw.githubusercontent.com/vantage-sh/ec2instances.info"
    "/master/www/instances.json"
)
_DRIFT_THRESHOLD = 0.01  # 1%


@dataclass
class DriftRecord:
    """A single price-drift observation from the vantage cross-check."""

    sku_id: str
    catalog_amount: float
    upstream_amount: float
    delta_pct: float
    source: str = "vantage"


def cross_check(
    shard_db: Path,
    *,
    instances_json: Path,
) -> list[DriftRecord]:
    """Offline cross-check: load instances.json, join with EC2 shard.

    Joins on ``(instance_type, region, os='linux', tenancy='shared')``.
    Flags rows where the catalog price differs from vantage's on-demand
    linux price by more than 1%.

    Parameters
    ----------
    shard_db:
        Path to the EC2 SQLite shard.
    instances_json:
        Path to the vantage ``instances.json`` snapshot.

    Returns
    -------
    list[DriftRecord]
        One record per (instance_type, region) pair that exceeds the drift
        threshold.
    """
    with instances_json.open() as fh:
        instances = json.load(fh)

    # Build vantage lookup: (instance_type, region) -> price
    vantage_prices: dict[tuple[str, str], float] = {}
    for entry in instances:
        itype = entry.get("instance_type", "")
        pricing = entry.get("pricing", {})
        for region, os_map in pricing.items():
            linux_map = os_map.get("linux", {})
            raw = linux_map.get("ondemand")
            if raw is not None:
                try:
                    price = float(raw)
                    if price > 0:
                        vantage_prices[(itype, region)] = price
                except (ValueError, TypeError):
                    pass

    if not vantage_prices:
        logger.warning("No vantage prices loaded from %s", instances_json)
        return []

    # Query shard for linux/shared on-demand rows.
    con = sqlite3.connect(f"file:{shard_db}?mode=ro", uri=True)
    try:
        rows = con.execute(
            """
            SELECT s.sku_id, s.resource_name, s.region, p.amount
            FROM skus s
            JOIN prices p USING (sku_id)
            JOIN terms t USING (sku_id)
            WHERE p.dimension = 'on-demand'
              AND LOWER(t.os) IN ('linux', '')
              AND LOWER(t.tenancy) IN ('shared', '')
              AND t.commitment IN ('on_demand', '')
            """
        ).fetchall()
    finally:
        con.close()

    drift: list[DriftRecord] = []
    for sku_id, resource_name, region, catalog_price in rows:
        key = (resource_name, region)
        vantage_price = vantage_prices.get(key)
        if vantage_price is None:
            # Not found in vantage — treat as freshness issue, not drift.
            continue

        delta_pct = abs(catalog_price - vantage_price) / vantage_price * 100
        if delta_pct >= _DRIFT_THRESHOLD * 100:
            drift.append(
                DriftRecord(
                    sku_id=sku_id,
                    catalog_amount=catalog_price,
                    upstream_amount=vantage_price,
                    delta_pct=delta_pct,
                )
            )

    return drift
