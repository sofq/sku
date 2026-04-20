# Installing sku

`sku` is a single static Go binary with no runtime dependencies.

## Recommended

| Platform | Command |
|---|---|
| macOS / Linux (Homebrew) | `brew install sofq/tap/sku` |
| Windows (Scoop) | `scoop bucket add sofq https://github.com/sofq/scoop-bucket && scoop install sku` |
| Node/JS ecosystems | `npm i -g @sofq/sku` or `npx @sofq/sku version` |
| Python ecosystems | `pipx install sku-cli` (or `pip install sku-cli`) |
| Docker / CI | `docker run --rm ghcr.io/sofq/sku:latest version` |
| Direct download | see [GitHub Releases](https://github.com/sofq/sku/releases) |

All channels ship the identical binary built by goreleaser from the tagged
commit. Checksums are signed with cosign (keyless, GitHub OIDC) and attested
with SLSA L3 provenance via `actions/attest-build-provenance`.

## Verifying a release

Every release ships a cosign-signed `checksums.txt` plus SLSA L3 build-provenance attestations (spec §7 *Supply chain*).

```bash
# 1. Download the binary + signatures
V=0.1.0
gh release download "v$V" --pattern 'sku_*_linux_amd64.tar.gz' \
                          --pattern 'checksums.txt*'

# 2. Verify cosign signature on checksums
cosign verify-blob \
  --certificate-identity-regexp 'https://github.com/sofq/sku/.*' \
  --certificate-oidc-issuer     https://token.actions.githubusercontent.com \
  --signature   checksums.txt.sig \
  --certificate checksums.txt.pem \
  checksums.txt

# 3. Verify the archive checksum matches the signed file
sha256sum -c checksums.txt --ignore-missing

# 4. (optional) verify the SLSA provenance attestation
gh attestation verify "sku_${V}_linux_amd64.tar.gz" --owner sofq
```

Expected: each command exits 0; step 2 prints `Verified OK`; step 4 prints a verified-attestations summary.

## Uninstall

| Channel | Command |
|---|---|
| Homebrew | `brew uninstall sku && brew untap sofq/tap` |
| Scoop | `scoop uninstall sku` |
| npm | `npm rm -g @sofq/sku` |
| pipx | `pipx uninstall sku-cli` |
| Docker | `docker rmi ghcr.io/sofq/sku` |
| Direct | `rm $(which sku)` |

Data directory (safe to delete):

```bash
rm -rf "${SKU_DATA_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/sku}"
```

## Troubleshooting

- **`sku: command not found`** — ensure the install location is on `PATH`. Homebrew puts it in `$(brew --prefix)/bin`; npm global in `$(npm bin -g)`; pipx in `~/.local/bin`.
- **`missing shard: openrouter`** — run `sku update openrouter`, or pass `--auto-fetch` to fetch on demand.
- **`Verified OK` but archive sha mismatch** — the signed `checksums.txt` is the authority; re-download the archive.
