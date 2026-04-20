# sku-cli (PyPI wrapper)

Install: `pipx install sku-cli` (recommended) or `pip install sku-cli`.

Ships as platform-tagged wheels (manylinux, musllinux, macos, windows x x86_64 + arm64)
each containing a vendored prebuilt `sku` binary. Pip selects the correct wheel
at install time. No postinstall hooks, no network fetches.

Source: https://github.com/sofq/sku
