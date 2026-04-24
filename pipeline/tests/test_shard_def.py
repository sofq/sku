# pipeline/tests/test_shard_def.py
from __future__ import annotations

import textwrap
from pathlib import Path

import pytest

from shards import ShardDef, load_all


def _write(tmp_path: Path, name: str, body: str) -> Path:
    p = tmp_path / f"{name}.yaml"
    p.write_text(textwrap.dedent(body))
    return p


def test_load_minimal_shard(tmp_path: Path) -> None:
    _write(tmp_path, "aws_ec2", """
        shard: aws_ec2
        provider: aws
        service: ec2
        kind: compute.vm
        cli:
          group: aws
          command: ec2
          resource_flag: instance-type
        ingest:
          module: aws_ec2
          discover: aws.publication_date
        budget_bytes: 20000000
    """)
    shards = load_all(tmp_path)
    assert set(shards) == {"aws_ec2"}
    s = shards["aws_ec2"]
    assert isinstance(s, ShardDef)
    assert s.provider == "aws"
    assert s.budget_bytes == 20_000_000
    assert s.cli.resource_flag == "instance-type"


def test_reject_filename_shard_mismatch(tmp_path: Path) -> None:
    _write(tmp_path, "aws_ec2", """
        shard: aws_rds
        provider: aws
        service: rds
        kind: db.relational
        cli: {group: aws, command: rds, resource_flag: instance-type}
        ingest: {module: aws_rds, discover: aws.publication_date}
        budget_bytes: 5000000
    """)
    with pytest.raises(ValueError, match="filename.*shard"):
        load_all(tmp_path)


def test_reject_unknown_provider(tmp_path: Path) -> None:
    _write(tmp_path, "weird_x", """
        shard: weird_x
        provider: mystery
        service: x
        kind: compute.vm
        cli: {group: weird, command: x, resource_flag: x}
        ingest: {module: weird_x, discover: weird.thing}
        budget_bytes: 1000
    """)
    with pytest.raises(ValueError, match="provider"):
        load_all(tmp_path)
