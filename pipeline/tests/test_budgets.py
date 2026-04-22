from __future__ import annotations

import pytest

from package.budgets import BudgetExceeded, check_shard_size, SHARD_BUDGETS


def test_check_shard_size_passes_under_budget():
    # aws-ec2 today is ~2.2 MB; budget is ~20 MB — nowhere near the limit.
    check_shard_size(shard="aws_ec2", compressed_bytes=2_000_000)


def test_check_shard_size_fails_over_budget():
    with pytest.raises(BudgetExceeded) as exc:
        check_shard_size(shard="aws_ec2", compressed_bytes=100_000_000)
    # Error message must name the shard and both numbers for a reviewer.
    msg = str(exc.value)
    assert "aws_ec2" in msg
    assert "100,000,000" in msg or "100000000" in msg
    assert str(SHARD_BUDGETS["aws_ec2"]) in msg or "20,000,000" in msg


def test_unknown_shard_raises():
    # Defensive: if a new shard appears without a budget entry, fail loudly
    # rather than silently accept any size.
    with pytest.raises(KeyError, match="no budget configured for shard"):
        check_shard_size(shard="bogus_shard", compressed_bytes=1)


def test_every_registered_shard_has_a_budget():
    # Cross-reference against ALL_SHARDS — catch drift when someone adds a
    # shard but forgets the budget.
    from discover.driver import ALL_SHARDS
    missing = [s for s in ALL_SHARDS if s not in SHARD_BUDGETS]
    assert not missing, f"shards without budget: {missing}"
