"""Closed tier-token vocabularies for M-δ tiered pricing.

Two unit-typed vocabularies, never mixed. The token alone is ambiguous
(does "100" mean 100 requests or 100 GB?), so callers pick the parser
based on the dimension being walked.
"""
from __future__ import annotations

TIER_TOKENS_COUNT: frozenset[str] = frozenset({
    # Base tokens present since Phase 0
    "0", "25", "100",
    # M (millions)
    "1M", "13M", "100M", "250M", "300M", "333M", "667M", "2500M",
    # B (billions)
    "1B", "5B", "10B", "19B", "20B", "100B", "200B",
    # K (thousands)
    "10K",
})

TIER_TOKENS_BYTES: frozenset[str] = frozenset({
    "0", "100GB", "500GB", "1TB", "10TB", "40TB", "50TB", "100TB", "150TB", "500TB", "1PB", "5PB",
})


def parse_count_tier(token: str) -> float:
    """Return the lower-bound count for a count-domain tier token.

    Raises ValueError for tokens not in TIER_TOKENS_COUNT.
    """
    if token not in TIER_TOKENS_COUNT:
        raise ValueError(f"token {token!r} not in TIER_TOKENS_COUNT")
    if token == "0":
        return 0.0
    token_upper = token.upper()
    if token_upper.endswith("B"):
        return float(token_upper[:-1]) * 1e9
    if token_upper.endswith("M"):
        return float(token_upper[:-1]) * 1e6
    if token_upper.endswith("K"):
        return float(token_upper[:-1]) * 1e3
    return float(token)


def parse_bytes_tier(token: str) -> float:
    """Return the lower-bound bytes for a bytes-domain tier token.

    Raises ValueError for tokens not in TIER_TOKENS_BYTES.
    """
    if token not in TIER_TOKENS_BYTES:
        raise ValueError(f"token {token!r} not in TIER_TOKENS_BYTES")
    if token == "0":
        return 0.0
    token_upper = token.upper()
    if token_upper.endswith("PB"):
        return float(token_upper[:-2]) * 1e15
    if token_upper.endswith("TB"):
        return float(token_upper[:-2]) * 1e12
    if token_upper.endswith("GB"):
        return float(token_upper[:-2]) * 1e9
    return float(token)
