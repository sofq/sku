"""Shared AWS ingest helpers: region normalization, SKU ID passthrough."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Mapping

import yaml

_REGIONS_YAML = Path(__file__).resolve().parent.parent / "normalize" / "regions.yaml"


@dataclass(frozen=True)
class RegionNormalizer:
    """Maps (provider, provider-region) -> canonical group."""

    table: Mapping[tuple[str, str], str]

    def normalize(self, provider: str, region: str) -> str:
        key = (provider, region)
        try:
            return self.table[key]
        except KeyError as exc:
            raise KeyError(f"{provider}/{region}") from exc


def load_region_normalizer() -> RegionNormalizer:
    """Load the repo's regions.yaml and build a (provider, region) -> group map."""
    with _REGIONS_YAML.open() as fh:
        doc = yaml.safe_load(fh)
    table: dict[tuple[str, str], str] = {}
    for group, entries in (doc.get("groups") or {}).items():
        for entry in entries:
            key = (entry["provider"], entry["region"])
            table[key] = group
    return RegionNormalizer(table)
