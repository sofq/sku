"""Shared types for the catalog profiler."""

from __future__ import annotations

from dataclasses import dataclass, field


@dataclass(frozen=True)
class CoverageRow:
    bucket_key: str
    bucket_label: str
    sku_count: int
    attribute_keys: tuple[str, ...] = field(default_factory=tuple)
    sample_sku_ids: tuple[str, ...] = field(default_factory=tuple)
    covered_by_shard: str | None = None
    coverage_ratio: float = 0.0

    def __post_init__(self) -> None:
        # Coerce list inputs to tuple so callers can pass either type.
        object.__setattr__(self, "attribute_keys", tuple(self.attribute_keys))
        object.__setattr__(self, "sample_sku_ids", tuple(self.sample_sku_ids))
