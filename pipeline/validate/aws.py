"""AWS Pricing API revalidator.

Uses ``boto3.client('pricing', region_name='us-east-1')`` (the Pricing
endpoint is us-east-1-only) to re-fetch the upstream price for each sample.
OIDC credentials are injected by the GitHub Actions workflow via
``aws-actions/configure-aws-credentials``; locally, the default credential
chain is used.
"""

from __future__ import annotations

import json
import logging
from dataclasses import dataclass
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from mypy_boto3_pricing import PricingClient

from validate.sampler import Sample

logger = logging.getLogger(__name__)

_DRIFT_THRESHOLD = 0.01  # 1%


@dataclass
class DriftRecord:
    """A single price-drift observation."""

    sku_id: str
    catalog_amount: float
    upstream_amount: float
    delta_pct: float
    source: str = "aws"


def _service_code(resource_name: str, sku_id: str) -> str:
    """Derive the AWS Pricing ServiceCode from the resource name."""
    # sku_id typically encodes the service prefix: "aws-ec2/...", "aws-rds/..."
    prefix = sku_id.split("/")[0].lower()
    if "rds" in prefix or "db." in resource_name.lower():
        return "AmazonRDS"
    if "s3" in prefix or "s3" in resource_name.lower():
        return "AmazonS3"
    if "lambda" in prefix:
        return "AWSLambda"
    if "ebs" in prefix:
        return "AmazonEC2"  # EBS is under EC2 pricing
    if "dynamodb" in prefix:
        return "AmazonDynamoDB"
    if "cloudfront" in prefix:
        return "AmazonCloudFront"
    if "eks" in prefix or resource_name.startswith("eks-"):
        return "AmazonEKS"
    if "elasticache" in prefix or "cache." in resource_name.lower():
        return "AmazonElastiCache"
    if "aurora" in prefix:
        return "AmazonRDS"
    # Default: EC2
    return "AmazonEC2"


def _eks_item_matches(sample: Sample, item: dict) -> bool:
    attrs = item.get("product", {}).get("attributes", {})
    op = attrs.get("operation", "")
    usage = attrs.get("usagetype", "")
    match sample.resource_name:
        case "eks-standard":
            return op == "CreateOperation" and "Outposts" not in usage
        case "eks-extended-support":
            return op == "ExtendedSupport"
        case "eks-fargate":
            if sample.dimension == "vcpu":
                return "Fargate-vCPU-Hours" in usage
            if sample.dimension == "memory":
                return "Fargate-GB-Hours" in usage and "Ephemeral" not in usage
    return False


def _extract_price(price_list_item: str, sample: Sample | None = None) -> float | None:
    """Extract the matching OnDemand price from a PriceList JSON string."""
    try:
        obj = json.loads(price_list_item)
        if sample is not None and sample.resource_name.startswith("eks-"):
            if not _eks_item_matches(sample, obj):
                return None
        terms = obj.get("terms", {})
        on_demand = terms.get("OnDemand", {})
        for term in on_demand.values():
            for pd in term.get("priceDimensions", {}).values():
                per_unit = pd.get("pricePerUnit", {})
                usd = per_unit.get("USD", "0")
                val = float(usd)
                if val > 0:
                    return val
    except (json.JSONDecodeError, ValueError, AttributeError, TypeError):
        pass
    return None


def _filters_for_sample(s: Sample, service_code: str) -> tuple[list[dict[str, str]], int]:
    if service_code == "AmazonEKS":
        return ([{"Type": "TERM_MATCH", "Field": "regionCode", "Value": s.region}], 100)
    return (
        [
            {"Type": "TERM_MATCH", "Field": "instanceType", "Value": s.resource_name},
            {"Type": "TERM_MATCH", "Field": "regionCode", "Value": s.region},
        ],
        1,
    )


def revalidate(
    samples: list[Sample],
    *,
    client: PricingClient | None = None,
) -> tuple[list[DriftRecord], list[str]]:
    """Per-sample SigV4 ``pricing:GetProducts`` call via boto3 client.

    Returns
    -------
    tuple[list[DriftRecord], list[str]]
        ``(drift_records, missing_upstream_sku_ids)``.
        Samples where the SKU is missing upstream are logged but *not* treated
        as drift — that is a shard-freshness issue, not a mispricing.
    """
    import boto3

    if client is None:
        client = boto3.client("pricing", region_name="us-east-1")

    drift: list[DriftRecord] = []
    missing: list[str] = []

    for s in samples:
        service_code = _service_code(s.resource_name, s.sku_id)
        filters, max_results = _filters_for_sample(s, service_code)

        # EKS uses a regionCode-only filter and can exceed MaxResults=100
        # across the page (standard / extended-support / Outposts / Fargate
        # vCPU / GB / ephemeral storage). Paginate via NextToken so the
        # target SKU is never silently skipped past the first page.
        upstream: float | None = None
        next_token: str | None = None
        page_failed = False
        while True:
            kwargs: dict = {
                "ServiceCode": service_code,
                "Filters": filters,
                "FormatVersion": "aws_v1",
                "MaxResults": max_results,
            }
            if next_token:
                kwargs["NextToken"] = next_token
            try:
                resp = client.get_products(**kwargs)
            except Exception:
                logger.exception("AWS Pricing API call failed for %s", s.sku_id)
                page_failed = True
                break

            for item in resp.get("PriceList", []):
                upstream = _extract_price(item, s)
                if upstream is not None:
                    break
            if upstream is not None:
                break

            next_token = resp.get("NextToken")
            if not next_token or service_code != "AmazonEKS":
                # Only EKS may need pagination; other services use a
                # tight (instanceType, regionCode) filter that fits in
                # one page.
                break

        if page_failed or upstream is None or upstream == 0:
            logger.debug("Could not parse upstream price for %s", s.sku_id)
            missing.append(s.sku_id)
            continue

        delta_pct = abs(s.price_amount - upstream) / upstream * 100
        if delta_pct >= _DRIFT_THRESHOLD * 100:
            drift.append(
                DriftRecord(
                    sku_id=s.sku_id,
                    catalog_amount=s.price_amount,
                    upstream_amount=upstream,
                    delta_pct=delta_pct,
                )
            )

    return drift, missing
