# sku npm platform packages

Each file `@sofq/sku-<os>-<arch>` is a one-platform package containing a single
prebuilt `sku` binary under `bin/`. The root `@sofq/sku` package lists all six as
`optionalDependencies`; npm installs only the one matching the current platform.

These packages are generated at release time from the goreleaser `dist/`
artifacts by `.github/workflows/release-npm.yml`. Do not hand-author them.

Supported platforms: linux-x64, linux-arm64, darwin-x64, darwin-arm64,
win32-x64, win32-arm64.
