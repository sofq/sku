# Changelog

## v0.1.0 — 2026-04-19 — first public pre-release

### Added

- Six-platform distribution: Homebrew tap, Scoop bucket, npm (`@sofq/sku`),
  PyPI (`sku-cli`), Docker (`ghcr.io/sofq/sku`), signed GH Releases.
- Cosign keyless checksum signatures + SBOMs (syft, SPDX).
- SLSA L3 build-provenance attestations on all release archives.
- `docs/install.md` and `docs/contributing/RELEASING.md`.

### Known limitations

- Pre-release: Homebrew / Scoop / npm / PyPI publish steps are intentionally
  skipped (`skip_upload: auto`). The tagged GH release + Docker image are
  the authoritative pre-release artifacts.
