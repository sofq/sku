from __future__ import annotations

import os
import subprocess
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
SCRIPT = REPO_ROOT / "scripts" / "ci" / "previous_release_version.sh"


def test_previous_release_version_ignores_data_latest(tmp_path: Path):
    fake_gh = tmp_path / "gh"
    fake_gh.write_text(
        "#!/usr/bin/env bash\n"
        "printf '%s\\n' "
        '\'[{"tagName":"data-latest"},'
        '{"tagName":"data-2026.04.23"},'
        '{"tagName":"v1.0.0"}]\'\n'
    )
    fake_gh.chmod(0o755)

    env = dict(os.environ)
    env["PATH"] = f"{tmp_path}:{env['PATH']}"

    result = subprocess.run(
        [str(SCRIPT)],
        cwd=REPO_ROOT,
        env=env,
        text=True,
        capture_output=True,
        check=True,
    )

    assert result.stdout.strip() == "2026.04.23"
