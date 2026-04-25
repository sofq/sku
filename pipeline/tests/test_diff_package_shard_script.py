from __future__ import annotations

import os
import stat
import subprocess
from pathlib import Path


def _write_executable(path: Path, body: str) -> None:
    path.write_text(body)
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def test_baseline_rebuild_runs_sanity_without_previous_db(tmp_path: Path) -> None:
    repo = Path(__file__).resolve().parents[2]
    fake_bin = tmp_path / "bin"
    fake_bin.mkdir()

    _write_executable(
        fake_bin / "python",
        """#!/usr/bin/env bash
set -euo pipefail
printf '%s\\n' "$*" >> "$PYTHON_CALL_LOG"
if [ "${1:-}" = "-m" ] && [ "${2:-}" = "package.sanity_check" ]; then
  for arg in "$@"; do
    if [ "$arg" = "--previous-db" ]; then
      echo "forced baseline rebuild must not compare previous db" >&2
      exit 42
    fi
  done
  exit 0
fi
if [ "${1:-}" = "-c" ]; then
  echo 123
  exit 0
fi
cat >/dev/null
echo '{"shard": "gcp-gce", "row_count": 123, "has_baseline": true}'
""",
    )
    _write_executable(
        fake_bin / "zstd",
        """#!/usr/bin/env bash
set -euo pipefail
out=""
while [ "$#" -gt 0 ]; do
  if [ "$1" = "-o" ]; then
    shift
    out="$1"
  fi
  shift || true
done
test -n "$out"
touch "$out"
""",
    )
    _write_executable(
        fake_bin / "sha256sum",
        """#!/usr/bin/env bash
set -euo pipefail
echo "abc123  $1"
""",
    )

    today = repo / "dist" / "pipeline" / "today"
    prev = repo / "dist" / "pipeline" / "prev"
    today.mkdir(parents=True, exist_ok=True)
    prev.mkdir(parents=True, exist_ok=True)
    (today / "gcp-gce.db").write_text("today")
    (prev / "gcp-gce.db").write_text("prev")

    env = os.environ.copy()
    env.update(
        {
            "PATH": f"{fake_bin}:{env['PATH']}",
            "SHARD": "gcp_gce",
            "CATALOG_VERSION": "2026.04.25",
            "PREVIOUS_VERSION": "2026.04.24",
            "BASELINE_REBUILD": "true",
            "PYTHON_CALL_LOG": str(tmp_path / "python.log"),
        }
    )

    result = subprocess.run(
        ["bash", "scripts/ci/diff_package_shard.sh"],
        cwd=repo,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )

    assert result.returncode == 0, result.stderr
    assert "--previous-db" not in (tmp_path / "python.log").read_text()
