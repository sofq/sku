"""Loader + validator for the enums.yaml / terms_defaults.yaml files."""

from __future__ import annotations

from functools import lru_cache
from pathlib import Path
from typing import Mapping

import yaml

_HERE = Path(__file__).resolve().parent


@lru_cache(maxsize=1)
def load_enums() -> dict[str, list[str]]:
    with (_HERE / "enums.yaml").open() as fh:
        doc = yaml.safe_load(fh)
    out = {k: v for k, v in doc.items() if k != "version"}
    return out


@lru_cache(maxsize=1)
def load_terms_defaults() -> dict[str, dict[str, str]]:
    with (_HERE / "terms_defaults.yaml").open() as fh:
        doc = yaml.safe_load(fh)
    return doc["defaults"]


def validate_enum(field: str, value: str) -> None:
    enums = load_enums()
    if field not in enums:
        raise KeyError(f"unknown enum field: {field!r}")
    if value not in enums[field]:
        allowed = ", ".join(repr(v) for v in enums[field])
        raise ValueError(f"{field}={value!r} not in allowed: {allowed}")


def apply_kind_defaults(kind: str, terms: Mapping[str, str]) -> dict[str, str]:
    """Return a new dict where missing keys are filled from the kind's defaults."""
    defaults = load_terms_defaults().get(kind)
    if defaults is None:
        raise KeyError(f"no terms defaults for kind={kind!r}")
    out = dict(defaults)
    out.update({k: v for k, v in terms.items() if v is not None})
    return out
