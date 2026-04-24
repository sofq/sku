from __future__ import annotations

import os
import stat
import subprocess
from pathlib import Path


def _write_executable(path: Path, body: str) -> None:
    path.write_text(body)
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def test_aws_etag_304_reuses_previous_shard_without_set_e_abort(tmp_path: Path) -> None:
    repo = Path(__file__).resolve().parents[2]
    fake_bin = tmp_path / "bin"
    fake_bin.mkdir()

    _write_executable(
        fake_bin / "python",
        """#!/usr/bin/env bash
set -euo pipefail
script="$(cat)"
case "$script" in
  *"fetch_offer"*"NotModified"*) exit 7 ;;
  *) echo "unexpected python invocation" >&2; exit 99 ;;
esac
""",
    )
    _write_executable(
        fake_bin / "gh",
        """#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" > "$GH_CALL_LOG"
touch dist/pipeline/aws-s3.db.zst
exit 0
""",
    )
    _write_executable(
        fake_bin / "zstd",
        """#!/usr/bin/env bash
set -euo pipefail
for arg in "$@"; do
  case "$arg" in
    *.zst) touch "${arg%.zst}" ;;
  esac
done
exit 0
""",
    )

    env = os.environ.copy()
    env.update(
        {
            "PATH": f"{fake_bin}:{env['PATH']}",
            "SHARD": "aws_s3",
            "CATALOG_VERSION": "2026.04.24",
            "PREVIOUS_VERSION": "2026.04.23",
            "SKU_ETAG_MODE": "strict",
            "GH_CALL_LOG": str(tmp_path / "gh.log"),
        }
    )

    result = subprocess.run(
        ["bash", "scripts/ci/ingest_shard.sh"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )

    assert result.returncode == 0, result.stderr
    assert "ETag 304" in result.stdout
    assert "--pattern aws-s3.db.zst" in (tmp_path / "gh.log").read_text()
