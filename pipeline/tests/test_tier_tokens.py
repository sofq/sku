"""Tests for pipeline/normalize/tier_tokens.py"""
import pytest
from normalize.tier_tokens import (
    TIER_TOKENS_COUNT, TIER_TOKENS_BYTES,
    parse_count_tier, parse_bytes_tier,
)


def test_all_count_tokens_parse():
    for t in TIER_TOKENS_COUNT:
        v = parse_count_tier(t)
        assert isinstance(v, float)
        assert v >= 0.0


def test_all_bytes_tokens_parse():
    for t in TIER_TOKENS_BYTES:
        v = parse_bytes_tier(t)
        assert isinstance(v, float)
        assert v >= 0.0


def test_cross_family_raises():
    with pytest.raises(ValueError):
        parse_count_tier("10TB")
    with pytest.raises(ValueError):
        parse_bytes_tier("1B")


def test_unknown_token_raises():
    with pytest.raises(ValueError):
        parse_count_tier("999")
    with pytest.raises(ValueError):
        parse_bytes_tier("999")


def test_zero_parses_in_both():
    assert parse_count_tier("0") == 0.0
    assert parse_bytes_tier("0") == 0.0


def test_count_values():
    assert parse_count_tier("300M") == 300e6
    assert parse_count_tier("1B") == 1e9
    assert parse_count_tier("10B") == 10e9
    assert parse_count_tier("19B") == 19e9


def test_bytes_values():
    assert parse_bytes_tier("1TB") == 1e12
    assert parse_bytes_tier("10TB") == 10e12
    assert parse_bytes_tier("1PB") == 1e15
    assert parse_bytes_tier("5PB") == 5e15
