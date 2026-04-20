# @sofq/sku (npm wrapper)

Install: `npm i -g @sofq/sku` or `npx @sofq/sku version`.

This is a thin wrapper around a prebuilt Go binary. npm resolves exactly one
`@sofq/sku-<os>-<arch>` optional dependency for the current platform and the
root shim `bin/sku.js` execs it. No postinstall network download.

Source: https://github.com/sofq/sku
