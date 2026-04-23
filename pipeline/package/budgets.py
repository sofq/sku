"""Per-shard compressed-byte budgets enforced by the manifest builder.

Values are bytes of the `.db.zst` shard file. Numbers were set at roughly
5× current (2026-04-22) size or 1 MB, whichever is larger, so the P1
regions expansion (~4× row scale) fits without a fresh budget decision.

**Overflow policy: hard fail.** When a shard exceeds its budget, the
manifest build raises BudgetExceeded and the daily workflow fails. The
forced human decision — raise the budget, split the shard, or narrow
the scope — is the entire point.
"""

from __future__ import annotations


SHARD_BUDGETS: dict[str, int] = {
    # Large shards (~MB-scale).
    "aws_ec2":          20_000_000,
    "aws_rds":           5_000_000,
    "azure_vm":         20_000_000,
    "azure_sql":         1_000_000,
    "azure_postgres":    1_000_000,
    "azure_mysql":       1_000_000,
    "azure_mariadb":       500_000,  # service is deprecated by Azure; lower ceiling
    "azure_disks":       1_000_000,
    "gcp_cloud_sql":     1_000_000,
    "openrouter":        1_000_000,
    # Medium shards.
    "aws_dynamodb":        500_000,
    "aws_s3":              500_000,
    "aws_lambda":          500_000,
    "aws_ebs":             500_000,
    "aws_cloudfront":      500_000,
    "azure_blob":          500_000,
    "azure_functions":     500_000,
    "gcp_gce":           2_000_000,   # grows most after machine-types fix
    "gcp_gcs":             500_000,
    "gcp_run":             500_000,
    "gcp_functions":       500_000,
}


class BudgetExceeded(Exception):
    """Raised by check_shard_size when compressed_bytes > budget."""


def check_shard_size(*, shard: str, compressed_bytes: int) -> None:
    if shard not in SHARD_BUDGETS:
        raise KeyError(f"no budget configured for shard {shard!r}")
    budget = SHARD_BUDGETS[shard]
    if compressed_bytes > budget:
        raise BudgetExceeded(
            f"shard {shard!r} exceeds budget: {compressed_bytes:,} bytes "
            f"(budget {budget:,}). Either raise the budget in "
            f"pipeline/package/budgets.py, split the shard, or narrow scope."
        )
