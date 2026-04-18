import json
from pathlib import Path

import pytest

from normalize.terms import canonicalize_terms, terms_hash

REPO_ROOT = Path(__file__).resolve().parents[2]
GOLDEN = REPO_ROOT / "internal" / "schema" / "testdata" / "terms_golden.jsonl"


def load_golden():
    rows = []
    with GOLDEN.open() as fh:
        for line in fh:
            line = line.strip()
            if line:
                rows.append(json.loads(line))
    return rows


def test_canonical_and_hash_match_golden():
    for row in load_golden():
        got_canonical = canonicalize_terms(row["input"])
        assert got_canonical == row["canonical"], row["name"]
        assert terms_hash(row["input"]) == row["terms_hash"], row["name"]


def test_hash_is_32_char_lowercase_hex():
    h = terms_hash({
        "commitment": "on_demand",
        "tenancy": "",
        "os": "",
        "support_tier": "",
        "upfront": "",
        "payment_option": "",
    })
    assert len(h) == 32
    assert h == h.lower()
    assert all(c in "0123456789abcdef" for c in h)


def test_missing_key_raises():
    with pytest.raises(KeyError):
        terms_hash({"commitment": "on_demand"})  # incomplete
