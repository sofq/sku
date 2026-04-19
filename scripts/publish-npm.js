#!/usr/bin/env node
// Usage: node scripts/publish-npm.js <artifacts-dir> <version>
// Unpacks each goreleaser archive into a throwaway dir, assembles a
// @sofq/sku-<os>-<arch> package, and `npm publish`es it.
"use strict";
const fs = require("fs");
const path = require("path");
const { execFileSync } = require("child_process");

const [artifactsDir, version] = process.argv.slice(2);
if (!artifactsDir || !version) {
  console.error("usage: publish-npm.js <artifacts-dir> <version>");
  process.exit(2);
}

const matrix = [
  { gOs: "linux",   gArch: "amd64", nOs: "linux",  nArch: "x64",   ext: "tar.gz", bin: "sku" },
  { gOs: "linux",   gArch: "arm64", nOs: "linux",  nArch: "arm64", ext: "tar.gz", bin: "sku" },
  { gOs: "darwin",  gArch: "amd64", nOs: "darwin", nArch: "x64",   ext: "tar.gz", bin: "sku" },
  { gOs: "darwin",  gArch: "arm64", nOs: "darwin", nArch: "arm64", ext: "tar.gz", bin: "sku" },
  { gOs: "windows", gArch: "amd64", nOs: "win32",  nArch: "x64",   ext: "zip",    bin: "sku.exe" },
  { gOs: "windows", gArch: "arm64", nOs: "win32",  nArch: "arm64", ext: "zip",    bin: "sku.exe" },
];

for (const m of matrix) {
  const archive = path.join(artifactsDir, `sku_${version}_${m.gOs}_${m.gArch}.${m.ext}`);
  if (!fs.existsSync(archive)) {
    console.error(`missing archive: ${archive}`);
    process.exit(1);
  }
  const stage = fs.mkdtempSync(`/tmp/sku-npm-${m.gOs}-${m.gArch}-`);
  const binDir = path.join(stage, "bin");
  fs.mkdirSync(binDir, { recursive: true });
  if (m.ext === "tar.gz") {
    execFileSync("tar", ["-xzf", archive, "-C", stage, m.bin], { stdio: "inherit" });
  } else {
    execFileSync("unzip", ["-j", archive, m.bin, "-d", stage], { stdio: "inherit" });
  }
  fs.renameSync(path.join(stage, m.bin), path.join(binDir, m.bin));

  const pkg = {
    name: `@sofq/sku-${m.nOs}-${m.nArch}`,
    version,
    description: `Prebuilt sku binary for ${m.nOs}/${m.nArch}.`,
    homepage: "https://github.com/sofq/sku",
    license: "Apache-2.0",
    os: [m.nOs],
    cpu: [m.nArch],
    files: ["bin/"],
  };
  fs.writeFileSync(path.join(stage, "package.json"), JSON.stringify(pkg, null, 2));
  console.log(`publishing ${pkg.name}@${version}`);
  execFileSync("npm", ["publish", "--access", "public"], { cwd: stage, stdio: "inherit" });
}
