"""Contiguity invariant checker for tiered price rows."""
from __future__ import annotations
from normalize.tier_tokens import parse_count_tier, parse_bytes_tier


def assert_tiers_contiguous(
    rows: list[dict],
    kind: str,
    parser_kind: str,  # "count" or "bytes"
) -> None:
    """Assert that tier rows for each (kind, dimension) group are contiguous."""
    parse = parse_count_tier if parser_kind == "count" else parse_bytes_tier

    # Group by (kind, dimension)
    groups: dict[tuple[str, str], list[dict]] = {}
    for row in rows:
        if row.get("kind") != kind:
            continue
        for price in row.get("prices", []):
            key = (row["kind"], price["dimension"])
            groups.setdefault(key, []).append(price)

    for (k, dim), prices in groups.items():
        # Sort by tier lower-bound
        try:
            sorted_prices = sorted(prices, key=lambda p: parse(p["tier"]))
        except ValueError as e:
            raise AssertionError(f"({k}, {dim}): invalid tier token: {e}")

        # Check contiguity
        for i, p in enumerate(sorted_prices):
            tier_upper = p.get("tier_upper", "")
            if i < len(sorted_prices) - 1:
                next_tier = sorted_prices[i + 1]["tier"]
                assert tier_upper == next_tier, (
                    f"({k}, {dim}): tier[{i}].tier_upper={tier_upper!r} "
                    f"!= tier[{i+1}].tier={next_tier!r} (not contiguous)"
                )
                assert tier_upper != "", (
                    f"({k}, {dim}): non-last entry has empty tier_upper"
                )
            else:
                assert tier_upper == "", (
                    f"({k}, {dim}): last entry tier_upper should be '' (unbounded), got {tier_upper!r}"
                )
