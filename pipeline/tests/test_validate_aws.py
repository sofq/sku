"""Tests for pipeline.validate.aws — AWS Pricing API revalidator."""

from __future__ import annotations

import json

import boto3
import pytest
from botocore.stub import Stubber

from validate.aws import DriftRecord, revalidate
from validate.sampler import Sample

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

_SAMPLE = Sample(
    sku_id="aws-ec2/m5.large/us-east-1",
    region="us-east-1",
    resource_name="m5.large",
    price_amount=0.096,
    price_currency="USD",
    dimension="on-demand",
)

_EC2_FILTERS = [
    {"Type": "TERM_MATCH", "Field": "instanceType", "Value": "m5.large"},
    {"Type": "TERM_MATCH", "Field": "regionCode", "Value": "us-east-1"},
]


def _price_list_item(amount: str) -> str:
    """Return a JSON-encoded PriceList item string as returned by Pricing API."""
    return json.dumps(
        {
            "product": {
                "attributes": {
                    "instanceType": "m5.large",
                    "location": "US East (N. Virginia)",
                    "operatingSystem": "Linux",
                    "tenancy": "Shared",
                }
            },
            "terms": {
                "OnDemand": {
                    "term1": {
                        "priceDimensions": {
                            "pd1": {
                                "pricePerUnit": {"USD": amount},
                                "unit": "Hrs",
                                "description": "Linux m5.large",
                            }
                        }
                    }
                }
            },
        }
    )


def _eks_price_list_item(*, sku_id: str, operation: str, usage_type: str, amount: str) -> str:
    """Return a minimal AmazonEKS PriceList item."""
    return json.dumps(
        {
            "product": {
                "sku": sku_id,
                "attributes": {
                    "operation": operation,
                    "regionCode": "us-east-1",
                    "usagetype": usage_type,
                },
            },
            "terms": {
                "OnDemand": {
                    "term1": {
                        "priceDimensions": {
                            "pd1": {
                                "pricePerUnit": {"USD": amount},
                                "unit": "Hrs",
                            }
                        }
                    }
                }
            },
        }
    )


def _make_stubbed_client(responses: list[dict]) -> boto3.client:
    """Return a pricing client with queued stub responses."""
    client = boto3.client("pricing", region_name="us-east-1")
    stubber = Stubber(client)
    for resp in responses:
        stubber.add_response(
            "get_products",
            resp,
            expected_params=resp.pop("_expected", None),
        )
    stubber.activate()
    return client, stubber


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------


def test_revalidate_no_drift_within_tolerance() -> None:
    """Catalog price within 1% of upstream -> no drift record."""
    client = boto3.client("pricing", region_name="us-east-1")
    stubber = Stubber(client)
    stubber.add_response(
        "get_products",
        {
            "PriceList": [_price_list_item("0.096")],
            "NextToken": "",
            "FormatVersion": "aws_v1",
        },
        expected_params={
            "ServiceCode": "AmazonEC2",
            "Filters": _EC2_FILTERS,
            "FormatVersion": "aws_v1",
            "MaxResults": 1,
        },
    )
    with stubber:
        drift, missing = revalidate([_SAMPLE], client=client)
    assert drift == []
    assert missing == []


def test_revalidate_drift_detected() -> None:
    """Catalog price differs by >1% from upstream -> drift record returned."""
    client = boto3.client("pricing", region_name="us-east-1")
    stubber = Stubber(client)
    # Upstream price is 5% higher.
    upstream_amount = 0.096 * 1.05
    stubber.add_response(
        "get_products",
        {
            "PriceList": [_price_list_item(f"{upstream_amount:.6f}")],
            "NextToken": "",
            "FormatVersion": "aws_v1",
        },
        expected_params={
            "ServiceCode": "AmazonEC2",
            "Filters": _EC2_FILTERS,
            "FormatVersion": "aws_v1",
            "MaxResults": 1,
        },
    )
    with stubber:
        drift, missing = revalidate(samples=[_SAMPLE], client=client)
    assert len(drift) == 1
    rec = drift[0]
    assert isinstance(rec, DriftRecord)
    assert rec.sku_id == _SAMPLE.sku_id
    assert rec.catalog_amount == pytest.approx(0.096)
    assert rec.upstream_amount == pytest.approx(upstream_amount)
    assert rec.delta_pct == pytest.approx(abs(upstream_amount - 0.096) / upstream_amount * 100)
    assert rec.source == "aws"


def test_revalidate_missing_upstream() -> None:
    """SKU not found upstream -> no drift record, but entry in missing list."""
    client = boto3.client("pricing", region_name="us-east-1")
    stubber = Stubber(client)
    stubber.add_response(
        "get_products",
        {
            "PriceList": [],
            "NextToken": "",
            "FormatVersion": "aws_v1",
        },
        expected_params={
            "ServiceCode": "AmazonEC2",
            "Filters": _EC2_FILTERS,
            "FormatVersion": "aws_v1",
            "MaxResults": 1,
        },
    )
    with stubber:
        drift, missing = revalidate(samples=[_SAMPLE], client=client)
    assert drift == []
    assert len(missing) == 1
    assert missing[0] == _SAMPLE.sku_id


def test_revalidate_multiple_samples() -> None:
    """Multiple samples processed: one match, one drift, one missing."""
    client = boto3.client("pricing", region_name="us-east-1")
    stubber = Stubber(client)

    s1 = Sample(
        sku_id="aws-ec2/m5.large/us-east-1",
        region="us-east-1",
        resource_name="m5.large",
        price_amount=0.096,
        price_currency="USD",
        dimension="on-demand",
    )
    s2 = Sample(
        sku_id="aws-ec2/c5.large/eu-west-1",
        region="eu-west-1",
        resource_name="c5.large",
        price_amount=0.085,
        price_currency="USD",
        dimension="on-demand",
    )
    s3 = Sample(
        sku_id="aws-ec2/r5.large/ap-southeast-1",
        region="ap-southeast-1",
        resource_name="r5.large",
        price_amount=0.126,
        price_currency="USD",
        dimension="on-demand",
    )

    # s1: exact match
    stubber.add_response(
        "get_products",
        {"PriceList": [_price_list_item("0.096")], "NextToken": "", "FormatVersion": "aws_v1"},
        expected_params={
            "ServiceCode": "AmazonEC2",
            "Filters": [
                {"Type": "TERM_MATCH", "Field": "instanceType", "Value": "m5.large"},
                {"Type": "TERM_MATCH", "Field": "regionCode", "Value": "us-east-1"},
            ],
            "FormatVersion": "aws_v1",
            "MaxResults": 1,
        },
    )
    # s2: ~10% drift
    stubber.add_response(
        "get_products",
        {
            "PriceList": [_price_list_item("0.0935")],
            "NextToken": "",
            "FormatVersion": "aws_v1",
        },
        expected_params={
            "ServiceCode": "AmazonEC2",
            "Filters": [
                {"Type": "TERM_MATCH", "Field": "instanceType", "Value": "c5.large"},
                {"Type": "TERM_MATCH", "Field": "regionCode", "Value": "eu-west-1"},
            ],
            "FormatVersion": "aws_v1",
            "MaxResults": 1,
        },
    )
    # s3: missing
    stubber.add_response(
        "get_products",
        {"PriceList": [], "NextToken": "", "FormatVersion": "aws_v1"},
        expected_params={
            "ServiceCode": "AmazonEC2",
            "Filters": [
                {"Type": "TERM_MATCH", "Field": "instanceType", "Value": "r5.large"},
                {"Type": "TERM_MATCH", "Field": "regionCode", "Value": "ap-southeast-1"},
            ],
            "FormatVersion": "aws_v1",
            "MaxResults": 1,
        },
    )

    with stubber:
        drift, missing = revalidate(samples=[s1, s2, s3], client=client)

    assert len(drift) == 1
    assert drift[0].sku_id == s2.sku_id
    assert len(missing) == 1
    assert missing[0] == s3.sku_id


def test_revalidate_eks_control_plane_uses_eks_filters_and_operation_match() -> None:
    """EKS control-plane validation matches AmazonEKS operation, not instanceType."""
    sample = Sample(
        sku_id="ZYWMR684YSMFHWEU",
        region="us-east-1",
        resource_name="eks-standard",
        price_amount=0.10,
        price_currency="USD",
        dimension="cluster",
    )
    client = boto3.client("pricing", region_name="us-east-1")
    stubber = Stubber(client)
    stubber.add_response(
        "get_products",
        {
            "PriceList": [
                _eks_price_list_item(
                    sku_id="ZYWMR684YSMFHWEU",
                    operation="CreateOperation",
                    usage_type="USE1-AmazonEKS-Hours:perCluster",
                    amount="0.1000000000",
                )
            ],
            "NextToken": "",
            "FormatVersion": "aws_v1",
        },
        expected_params={
            "ServiceCode": "AmazonEKS",
            "Filters": [{"Type": "TERM_MATCH", "Field": "regionCode", "Value": "us-east-1"}],
            "FormatVersion": "aws_v1",
            "MaxResults": 100,
        },
    )
    with stubber:
        drift, missing = revalidate([sample], client=client)
    assert drift == []
    assert missing == []


def test_revalidate_eks_paginates_when_match_on_second_page() -> None:
    """EKS uses NextToken pagination when the matching SKU is past page 1."""
    sample = Sample(
        sku_id="ZYWMR684YSMFHWEU",
        region="us-east-1",
        resource_name="eks-standard",
        price_amount=0.10,
        price_currency="USD",
        dimension="cluster",
    )
    client = boto3.client("pricing", region_name="us-east-1")
    stubber = Stubber(client)
    # Page 1: only Fargate / Outposts items, no CreateOperation -> no match.
    stubber.add_response(
        "get_products",
        {
            "PriceList": [
                _eks_price_list_item(
                    sku_id="OUTPOSTS",
                    operation="CreateOperation",
                    usage_type="USE1-AmazonEKS-Hours-Outposts:perCluster",
                    amount="0.1000000000",
                ),
            ],
            "NextToken": "page2-token",
            "FormatVersion": "aws_v1",
        },
        expected_params={
            "ServiceCode": "AmazonEKS",
            "Filters": [{"Type": "TERM_MATCH", "Field": "regionCode", "Value": "us-east-1"}],
            "FormatVersion": "aws_v1",
            "MaxResults": 100,
        },
    )
    # Page 2: contains the standard cluster SKU.
    stubber.add_response(
        "get_products",
        {
            "PriceList": [
                _eks_price_list_item(
                    sku_id="ZYWMR684YSMFHWEU",
                    operation="CreateOperation",
                    usage_type="USE1-AmazonEKS-Hours:perCluster",
                    amount="0.1000000000",
                ),
            ],
            "NextToken": "",
            "FormatVersion": "aws_v1",
        },
        expected_params={
            "ServiceCode": "AmazonEKS",
            "Filters": [{"Type": "TERM_MATCH", "Field": "regionCode", "Value": "us-east-1"}],
            "FormatVersion": "aws_v1",
            "MaxResults": 100,
            "NextToken": "page2-token",
        },
    )
    with stubber:
        drift, missing = revalidate([sample], client=client)
    assert drift == []
    assert missing == []


def test_revalidate_eks_fargate_selects_requested_dimension() -> None:
    """EKS Fargate validation compares the vCPU sample to the vCPU upstream SKU."""
    sample = Sample(
        sku_id="eks-fargate-us-east-1",
        region="us-east-1",
        resource_name="eks-fargate",
        price_amount=0.04048,
        price_currency="USD",
        dimension="vcpu",
    )
    client = boto3.client("pricing", region_name="us-east-1")
    stubber = Stubber(client)
    stubber.add_response(
        "get_products",
        {
            "PriceList": [
                _eks_price_list_item(
                    sku_id="PT22UKNZNU9D3XSN",
                    operation="",
                    usage_type="USE1-Fargate-GB-Hours",
                    amount="0.0044450000",
                ),
                _eks_price_list_item(
                    sku_id="3JPC5EER47BUUFC6",
                    operation="",
                    usage_type="USE1-Fargate-vCPU-Hours:perCPU",
                    amount="0.0404800000",
                ),
            ],
            "NextToken": "",
            "FormatVersion": "aws_v1",
        },
        expected_params={
            "ServiceCode": "AmazonEKS",
            "Filters": [{"Type": "TERM_MATCH", "Field": "regionCode", "Value": "us-east-1"}],
            "FormatVersion": "aws_v1",
            "MaxResults": 100,
        },
    )
    with stubber:
        drift, missing = revalidate([sample], client=client)
    assert drift == []
    assert missing == []
