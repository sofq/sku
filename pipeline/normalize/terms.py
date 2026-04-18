"""Canonical terms encoding + terms_hash. Must match internal/schema/terms.go byte-for-byte."""

from __future__ import annotations

import hashlib
import json
from typing import Mapping

# Fixed field order — MUST NOT CHANGE without a schema_version bump.
_FIELD_ORDER: tuple[str, ...] = (
    "commitment",
    "tenancy",
    "os",
    "support_tier",
    "upfront",
    "payment_option",
)


def canonicalize_terms(terms: Mapping[str, str]) -> str:
    """Return the canonical JSON encoding of a terms mapping.

    Missing keys raise KeyError — callers are expected to fill defaults
    (via terms_defaults.yaml) before hashing.
    """
    values = [terms[k] for k in _FIELD_ORDER]
    # separators=(",", ":") -> no whitespace; ensure_ascii=False -> UTF-8 passthrough.
    return json.dumps(values, separators=(",", ":"), ensure_ascii=False)


def terms_hash(terms: Mapping[str, str]) -> str:
    """128-bit hex digest of the canonical encoding (first 32 hex chars of sha256)."""
    canonical = canonicalize_terms(terms)
    return hashlib.sha256(canonical.encode("utf-8")).hexdigest()[:32]
