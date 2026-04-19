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

## Verifying a download

```bash
cosign verify-blob \
  --certificate-identity-regexp 'https://github.com/sofq/sku/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --signature checksums.txt.sig \
  --certificate checksums.txt.pem \
  checksums.txt
sha256sum -c checksums.txt
```
