# Releasing sku

Every release is driven by a signed, annotated tag pushed to `main`.

## Prerequisites (one-time)

- Repo secrets set: `HOMEBREW_TAP_GITHUB_TOKEN`, `SCOOP_GITHUB_TOKEN`,
  `NPM_TOKEN`, `PYPI_API_TOKEN` (see `.github/workflows/release.yml` header).
- External repos exist and are empty: `sofq/homebrew-tap`, `sofq/scoop-bucket`.
- GHCR package `ghcr.io/sofq/sku` visibility = public.
- npm scope `@sofq` exists; publisher has automation-token access.
- PyPI project `sku-cli` reserved under the releasing account.
- `sku` project name is confirmed available on all four registries
  (M0 exit gate; re-verify before the first tag).

## Cutting a release

1. Merge intended PRs to main; wait for CI green.
2. Update `CHANGELOG.md` with a human summary of user-visible changes.
3. `git tag -s vX.Y.Z -m "sku vX.Y.Z"` (pre-releases: `-rc.N` suffix).
4. `git push origin vX.Y.Z`.
5. Watch `release` workflow: goreleaser must complete before `release-npm`
   and `release-pypi` fire.
6. Verify the release:
   - GitHub Release page shows archives + `checksums.txt` + `.sig` + `.pem` + SBOMs.
   - `brew install sofq/tap/sku` succeeds on macOS (skip for pre-releases).
   - `scoop install sku` succeeds on Windows (skip for pre-releases).
   - `npm i -g @sofq/sku && sku version` (skip for pre-releases).
   - `pipx install sku-cli && sku version` (skip for pre-releases).
   - `docker run --rm ghcr.io/sofq/sku:X.Y.Z version` succeeds.
   - `cosign verify-blob --certificate-identity-regexp '.*' \
      --certificate-oidc-issuer https://token.actions.githubusercontent.com \
      --signature checksums.txt.sig --certificate checksums.txt.pem checksums.txt`
      succeeds.
7. Update `docs/install.md` if any channel changed.
8. Announce in `docs/news/` and README.

## Failure handling

- Build fails mid-matrix: delete the draft GH release, fix, retag.
- One distribution channel fails: rerun that job from the workflow UI.
- Critical regression: cut a patch X.Y.Z+1 immediately; brew/scoop/npm/pypi
  pick it up automatically.
