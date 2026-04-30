"""Verify that internal/estimate/tiertokens.go matches the generated output."""
import sys
from pathlib import Path
import pytest

REPO_ROOT = Path(__file__).parent.parent.parent
GENERATED_PATH = REPO_ROOT / "internal" / "estimate" / "tiertokens.go"
GEN_SCRIPT = REPO_ROOT / "tools" / "gen_go_tier_tokens.py"
VENV_PYTHON = REPO_ROOT / "pipeline" / ".venv" / "bin" / "python"


def test_tiertokens_go_is_up_to_date():
    """Check that tiertokens.go matches what the generator would produce.

    Imports render_go_code() directly to avoid subprocess + backup/restore risk.
    """
    if not VENV_PYTHON.exists():
        pytest.skip("pipeline venv not set up")

    assert GENERATED_PATH.exists(), "tiertokens.go does not exist; run `make generate`"

    # Import the generator's render function directly
    sys.path.insert(0, str(REPO_ROOT / "tools"))
    sys.path.insert(0, str(REPO_ROOT / "pipeline"))
    import importlib.util
    spec = importlib.util.spec_from_file_location("gen_go_tier_tokens", GEN_SCRIPT)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)

    expected = mod.render_go_code()
    committed = GENERATED_PATH.read_text()

    assert committed == expected, (
        "tiertokens.go is out of sync with tier_tokens.py; run `make generate`"
    )
