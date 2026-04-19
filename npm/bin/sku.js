#!/usr/bin/env node
// Thin shim: resolve the platform-specific @sofq/sku-<os>-<arch> package
// installed by npm via optionalDependencies, then exec its bundled binary.
// No postinstall network fetch — works under pnpm, yarn PnP, npm --ignore-scripts.
"use strict";

const { spawnSync } = require("child_process");
const path = require("path");
const os = require("os");

const platformMap = {
  "linux-x64": "@sofq/sku-linux-x64",
  "linux-arm64": "@sofq/sku-linux-arm64",
  "darwin-x64": "@sofq/sku-darwin-x64",
  "darwin-arm64": "@sofq/sku-darwin-arm64",
  "win32-x64": "@sofq/sku-win32-x64",
  "win32-arm64": "@sofq/sku-win32-arm64",
};

const key = `${process.platform}-${process.arch}`;
const pkg = platformMap[key];
if (!pkg) {
  console.error(`sku: no prebuilt binary for ${key} (${os.platform()}/${os.arch()})`);
  process.exit(1);
}

let binPath;
try {
  const pkgJson = require.resolve(`${pkg}/package.json`);
  const binName = process.platform === "win32" ? "sku.exe" : "sku";
  binPath = path.join(path.dirname(pkgJson), "bin", binName);
} catch (err) {
  console.error(
    `sku: platform package ${pkg} not installed. ` +
      `Reinstall @sofq/sku without --no-optional / --omit=optional.`,
  );
  process.exit(1);
}

const res = spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" });
if (res.error) {
  console.error(`sku: failed to exec ${binPath}: ${res.error.message}`);
  process.exit(1);
}
process.exit(res.status === null ? 1 : res.status);
