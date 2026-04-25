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


def test_azure_aks_fetches_and_passes_aci_prices(tmp_path: Path) -> None:
    repo = Path(__file__).resolve().parents[2]
    fake_bin = tmp_path / "bin"
    fake_bin.mkdir()
    python_log = tmp_path / "python.log"

    _write_executable(
        fake_bin / "python",
        """#!/usr/bin/env bash
set -euo pipefail
printf 'ARGS:%s\\n' "$*" >> "$PYTHON_CALL_LOG"
if [ "${1:-}" = "-m" ]; then
  out=""
  while [ "$#" -gt 0 ]; do
    if [ "$1" = "--out" ]; then
      out="$2"
      break
    fi
    shift
  done
  if [ -n "$out" ]; then
    mkdir -p "$(dirname "$out")"
    touch "$out"
  fi
  exit 0
fi
script="$(cat)"
printf 'SCRIPT:%s\\n' "$script" >> "$PYTHON_CALL_LOG"
exit 0
""",
    )

    env = os.environ.copy()
    env.update(
        {
            "PATH": f"{fake_bin}:{env['PATH']}",
            "SHARD": "azure_aks",
            "CATALOG_VERSION": "2026.04.25",
            "PYTHON_CALL_LOG": str(python_log),
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
    log = python_log.read_text()
    assert "Container Instances" in log
    assert "ARGS:-m ingest.azure_aks" in log
    assert "--aci-prices dist/pipeline/raw/azure_aks-aci-prices.json" in log
