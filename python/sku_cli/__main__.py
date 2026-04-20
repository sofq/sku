"""PyPI wrapper entrypoint: locate the vendored sku binary and exec it."""

import os
import sys
from importlib.resources import files


def _binary_path() -> str:
    name = "sku.exe" if sys.platform == "win32" else "sku"
    return str(files("sku_cli").joinpath("bin").joinpath(name))


def main() -> int:
    path = _binary_path()
    if not os.path.isfile(path):
        print(
            f"sku-cli: prebuilt binary missing at {path}. "
            "Reinstall via `pipx install sku-cli` or `pip install --force-reinstall sku-cli`.",
            file=sys.stderr,
        )
        return 1
    try:
        os.chmod(path, 0o755)
    except OSError:
        pass
    if sys.platform == "win32":
        import subprocess

        return subprocess.run([path, *sys.argv[1:]]).returncode
    os.execv(path, [path, *sys.argv[1:]])


if __name__ == "__main__":
    raise SystemExit(main())
