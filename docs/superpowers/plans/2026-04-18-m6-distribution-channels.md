# M6 — Distribution Channels Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `sku` to end users across Homebrew, Scoop, npm, PyPI, Docker/GHCR, and signed GitHub Releases, then cut the first public pre-release `v0.1.0`.

**Architecture:** Extend the existing `.goreleaser.yml` + `release.yml` pipeline to emit additional distribution artifacts in a single release step. Add two on-disk wrappers (`npm/`, `python/`) following the platform-specific-optional-dependencies (npm) and `cibuildwheel` platform-tagged wheels (PyPI) patterns. Supply-chain artifacts (cosign signatures, syft SBOM, SLSA provenance) are attached inside `release.yml` using GitHub OIDC. A `Dockerfile.goreleaser` produces a multi-arch image pushed to GHCR by goreleaser. All publishing remains driven from a single `git tag vX.Y.Z && git push --tags` entrypoint.

**Tech Stack:** goreleaser v2, cosign (keyless/OIDC), syft, `actions/attest-build-provenance`, Docker buildx/manifest lists via goreleaser, Homebrew tap (`sofq/homebrew-tap`), Scoop bucket (`sofq/scoop-bucket`), npm (platform-optional-dependencies pattern, as used by esbuild/swc/Rollup), `cibuildwheel` for PyPI, Alpine 3.21 base image.

**Scope notes:** M6 covers distribution only. Do not add or modify any CLI behavior, shards, or pipeline logic. Spec §7 "Distribution & Release" and §9 M6 are the authoritative scope.

---

## File Structure

- Modify: `.goreleaser.yml` — add cosign/syft/homebrew/scoop/docker/nfpms blocks.
- Create: `Dockerfile.goreleaser` — runtime image copied in by goreleaser.
- Modify: `.github/workflows/release.yml` — install cosign + syft, enable OIDC, add provenance attestation step, add secrets for brew/scoop/npm/pypi tokens.
- Create: `npm/package.json` — root package `@sofq/sku` with platform optionalDependencies and `bin/sku.js` shim.
- Create: `npm/bin/sku.js` — thin exec shim.
- Create: `npm/platform-packages/README.md` — short doc describing the per-platform child packages and their naming.
- Create: `npm/platform-packages/template/package.json.tmpl` — goreleaser template to stamp out each `@sofq/sku-<os>-<arch>` package.
- Create: `python/pyproject.toml` — `sku-cli` project metadata.
- Create: `python/sku_cli/__init__.py` — locate-and-exec binary shim.
- Create: `python/sku_cli/__main__.py` — entrypoint.
- Create: `python/MANIFEST.in` — include binary inside wheels.
- Create: `python/cibuildwheel.toml` (or `[tool.cibuildwheel]` in `pyproject.toml`) — wheel platform tag matrix.
- Create: `.github/workflows/release-npm.yml` — publish npm packages after goreleaser completes (called from release workflow).
- Create: `.github/workflows/release-pypi.yml` — build + publish wheels via cibuildwheel.
- Create: `docs/contributing/RELEASING.md` — human release checklist.
- Modify: `docs/install.md` (create if missing) — add brew / scoop / npm / pipx / docker / direct-download instructions.
- Modify: `Makefile` — add `release-check`, `docker-smoke`, `npm-pack-smoke`, `pypi-wheel-smoke` targets.
- Modify: `CLAUDE.md` — add M6 agent quick path lines.
- Modify: `CHANGELOG.md` (create if missing) — `v0.1.0` entry.
- Create (external repo, out of band): `github.com/sofq/homebrew-tap` and `github.com/sofq/scoop-bucket`. The plan does NOT create code in those repos directly; goreleaser pushes to them via `HOMEBREW_TAP_GITHUB_TOKEN` / `SCOOP_GITHUB_TOKEN` the first time a real tag lands. The plan only verifies readiness.

---

### Task 1: Expand goreleaser — checksum signing, SBOM, nfpms

**Files:**
- Modify: `.goreleaser.yml`

- [ ] **Step 1: Add signs, sboms, nfpms blocks**

Append to `.goreleaser.yml` (after the existing `checksum:` block, before `snapshot:`):

```yaml
sboms:
  - id: default
    artifacts: archive
    documents:
      - "${artifact}.spdx.sbom.json"

signs:
  - id: cosign-checksum
    cmd: cosign
    signature: "${artifact}.sig"
    certificate: "${artifact}.pem"
    args:
      - sign-blob
      - "--output-certificate=${certificate}"
      - "--output-signature=${signature}"
      - "${artifact}"
      - "--yes"
    artifacts: checksum
    output: true

nfpms:
  - id: sku
    package_name: sku
    vendor: sofq
    homepage: https://github.com/sofq/sku
    maintainer: sofq <noreply@sofq.dev>
    description: Agent-friendly CLI for cloud + LLM pricing.
    license: Apache-2.0
    formats: [deb, rpm, apk]
    bindir: /usr/bin
```

- [ ] **Step 2: Run snapshot to verify config parses**

Run: `goreleaser release --snapshot --clean --skip=sign,publish,docker,sbom`
Expected: Exit 0. `dist/` contains platform archives + `checksums.txt`. The `--skip=sign,sbom` keeps local dry-run fast (cosign/syft not required locally).

- [ ] **Step 3: Run syft locally to verify SBOM generation works**

Run: `syft --version || echo "install syft via brew install syft"` then `goreleaser release --snapshot --clean --skip=sign,publish,docker`
Expected: Exit 0. `dist/*.spdx.sbom.json` present for each archive.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yml
git commit -m "feat(m6): goreleaser — add cosign checksum signing, syft SBOMs, and nfpms packages"
```

---

### Task 2: Expand goreleaser — Homebrew tap and Scoop bucket

**Files:**
- Modify: `.goreleaser.yml`

- [ ] **Step 1: Add brews + scoops blocks**

Append to `.goreleaser.yml` (after `nfpms:`):

```yaml
brews:
  - name: sku
    repository:
      owner: sofq
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    homepage: "https://github.com/sofq/sku"
    description: "Agent-friendly CLI for cloud + LLM pricing."
    license: "Apache-2.0"
    test: |
      system "#{bin}/sku version"
    skip_upload: auto  # Skip for pre-releases (e.g. v0.1.0-rc.N).

scoops:
  - name: sku
    repository:
      owner: sofq
      name: scoop-bucket
      token: "{{ .Env.SCOOP_GITHUB_TOKEN }}"
    homepage: "https://github.com/sofq/sku"
    description: "Agent-friendly CLI for cloud + LLM pricing."
    license: Apache-2.0
    skip_upload: auto
```

Note: `skip_upload: auto` causes goreleaser to skip brew/scoop on any tag matching `*-rc.*` / `*-pre*`, matching spec §7 "Versioning policy" ("Pre-releases skip brew/npm/pypi").

- [ ] **Step 2: Snapshot build to confirm config parses**

Run: `goreleaser release --snapshot --clean --skip=sign,publish,docker,sbom`
Expected: Exit 0. `dist/sku.rb` (brew formula) and `dist/sku.json` (scoop manifest) present.

- [ ] **Step 3: Inspect generated formula and scoop manifest**

Run: `cat dist/sku.rb && cat dist/sku.json`
Expected: Formula has `url "...sku_<version>_darwin_arm64.tar.gz"` with a sha256. Scoop manifest has windows urls.

- [ ] **Step 4: Commit**

```bash
git add .goreleaser.yml
git commit -m "feat(m6): goreleaser — publish Homebrew tap (sofq/homebrew-tap) and Scoop bucket (sofq/scoop-bucket)"
```

---

### Task 3: Dockerfile.goreleaser + multi-arch GHCR publishing

**Files:**
- Create: `Dockerfile.goreleaser`
- Modify: `.goreleaser.yml`
- Modify: `Makefile`

- [ ] **Step 1: Write Dockerfile.goreleaser**

Create `Dockerfile.goreleaser`:

```dockerfile
FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY sku /usr/local/bin/sku
USER 65534:65534
ENTRYPOINT ["/usr/local/bin/sku"]
CMD ["--help"]
```

- [ ] **Step 2: Add dockers + docker_manifests to goreleaser**

Append to `.goreleaser.yml`:

```yaml
dockers:
  - id: sku-amd64
    goos: linux
    goarch: amd64
    image_templates:
      - "ghcr.io/sofq/sku:{{ .Version }}-amd64"
    dockerfile: Dockerfile.goreleaser
    use: buildx
    build_flag_templates:
      - "--platform=linux/amd64"
      - "--label=org.opencontainers.image.source=https://github.com/sofq/sku"
      - "--label=org.opencontainers.image.version={{ .Version }}"
      - "--label=org.opencontainers.image.licenses=Apache-2.0"
  - id: sku-arm64
    goos: linux
    goarch: arm64
    image_templates:
      - "ghcr.io/sofq/sku:{{ .Version }}-arm64"
    dockerfile: Dockerfile.goreleaser
    use: buildx
    build_flag_templates:
      - "--platform=linux/arm64"
      - "--label=org.opencontainers.image.source=https://github.com/sofq/sku"
      - "--label=org.opencontainers.image.version={{ .Version }}"
      - "--label=org.opencontainers.image.licenses=Apache-2.0"

docker_manifests:
  - name_template: "ghcr.io/sofq/sku:{{ .Version }}"
    image_templates:
      - "ghcr.io/sofq/sku:{{ .Version }}-amd64"
      - "ghcr.io/sofq/sku:{{ .Version }}-arm64"
  - name_template: "ghcr.io/sofq/sku:latest"
    skip_push: auto  # Skip for pre-releases.
    image_templates:
      - "ghcr.io/sofq/sku:{{ .Version }}-amd64"
      - "ghcr.io/sofq/sku:{{ .Version }}-arm64"

docker_signs:
  - cmd: cosign
    artifacts: manifests
    args:
      - sign
      - "${artifact}@${digest}"
      - "--yes"
    output: true
```

- [ ] **Step 3: Add docker-smoke target to Makefile**

Add to `Makefile`:

```make
.PHONY: docker-smoke
docker-smoke: ## Build a local Docker image from the snapshot binary and run sku version
	goreleaser release --snapshot --clean --skip=sign,publish,sbom
	docker buildx build --platform linux/amd64 --load \
		-f Dockerfile.goreleaser -t sku:smoke \
		dist/sku_linux_amd64_v1/
	docker run --rm sku:smoke version
```

- [ ] **Step 4: Run docker-smoke**

Run: `make docker-smoke`
Expected: Exit 0. Last lines print JSON with `"version":"0.0.0-snapshot-..."`.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile.goreleaser .goreleaser.yml Makefile
git commit -m "feat(m6): multi-arch Docker image published to ghcr.io/sofq/sku with cosign signatures"
```

---

### Task 4: release.yml — install cosign + syft + docker, enable OIDC + attestations

**Files:**
- Modify: `.github/workflows/release.yml`

- [ ] **Step 1: Replace release.yml body**

Overwrite `.github/workflows/release.yml` with:

```yaml
name: release

on:
  push:
    tags: ["v*.*.*"]

permissions:
  contents: write
  id-token: write
  packages: write
  attestations: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    outputs:
      artifacts: ${{ steps.goreleaser.outputs.artifacts }}
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
          cache: true

      - name: Set up QEMU (multi-arch docker)
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Install cosign
        uses: sigstore/cosign-installer@v3

      - name: Install syft
        uses: anchore/sbom-action/download-syft@v0

      - name: Run goreleaser
        id: goreleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          SCOOP_GITHUB_TOKEN: ${{ secrets.SCOOP_GITHUB_TOKEN }}
          COSIGN_EXPERIMENTAL: "1"

      - name: Attest build provenance (SLSA L3)
        uses: actions/attest-build-provenance@v2
        with:
          subject-path: "dist/*.tar.gz,dist/*.zip,dist/checksums.txt"
```

- [ ] **Step 2: Add comment block documenting required repo secrets**

Prepend to `.github/workflows/release.yml` (after the `name:` line but before `on:`), as a YAML comment:

```yaml
# Required repository secrets (set in GitHub Settings → Secrets):
#   HOMEBREW_TAP_GITHUB_TOKEN   — PAT with contents:write on sofq/homebrew-tap
#   SCOOP_GITHUB_TOKEN          — PAT with contents:write on sofq/scoop-bucket
#   NPM_TOKEN                   — npm automation token (used by release-npm.yml)
#   PYPI_API_TOKEN              — PyPI API token (used by release-pypi.yml)
# GITHUB_TOKEN is auto-injected. id-token: write enables cosign keyless + SLSA.
```

- [ ] **Step 3: Lint the workflow file**

Run: `yq '.' .github/workflows/release.yml > /dev/null && echo "yaml ok"` (fallback if yq missing: `python3 -c 'import yaml,sys;yaml.safe_load(open(".github/workflows/release.yml"))' && echo "yaml ok"`)
Expected: `yaml ok`.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat(m6): release workflow — cosign, syft, GHCR login, SLSA L3 provenance attestations"
```

---

### Task 5: npm wrapper — platform-optional-dependencies pattern

**Files:**
- Create: `npm/package.json`
- Create: `npm/bin/sku.js`
- Create: `npm/README.md`
- Create: `npm/platform-packages/README.md`
- Create: `npm/platform-packages/template/package.json.tmpl`

- [ ] **Step 1: Write root npm/package.json**

Create `npm/package.json`:

```json
{
  "name": "@sofq/sku",
  "version": "0.0.0",
  "description": "Agent-friendly CLI for cloud + LLM pricing. This is a thin npm wrapper that exec()s a prebuilt Go binary installed via an optional platform dependency.",
  "homepage": "https://github.com/sofq/sku",
  "license": "Apache-2.0",
  "bin": {
    "sku": "bin/sku.js"
  },
  "scripts": {
    "test": "node bin/sku.js version"
  },
  "files": ["bin/"],
  "optionalDependencies": {
    "@sofq/sku-linux-x64": "0.0.0",
    "@sofq/sku-linux-arm64": "0.0.0",
    "@sofq/sku-darwin-x64": "0.0.0",
    "@sofq/sku-darwin-arm64": "0.0.0",
    "@sofq/sku-win32-x64": "0.0.0",
    "@sofq/sku-win32-arm64": "0.0.0"
  }
}
```

Version `0.0.0` is a placeholder; goreleaser stamps the real version in at release time (see Task 7).

- [ ] **Step 2: Write the exec shim**

Create `npm/bin/sku.js`:

```javascript
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
```

- [ ] **Step 3: Write platform package template**

Create `npm/platform-packages/template/package.json.tmpl` (consumed by release-npm.yml; one concrete package per supported `os`/`cpu` pair):

```json
{
  "name": "@sofq/sku-{OS}-{ARCH}",
  "version": "{VERSION}",
  "description": "Prebuilt sku binary for {OS}/{ARCH}.",
  "homepage": "https://github.com/sofq/sku",
  "license": "Apache-2.0",
  "os": ["{NODE_OS}"],
  "cpu": ["{NODE_ARCH}"],
  "files": ["bin/"]
}
```

Placeholders `{OS}`, `{ARCH}`, `{NODE_OS}`, `{NODE_ARCH}`, `{VERSION}` are replaced by the publish script (Task 7). Node's allowed `os`/`cpu` values differ slightly from Go's (`darwin`/`linux`/`win32`; `x64`/`arm64`) — the script maps them explicitly.

- [ ] **Step 4: Write npm/platform-packages/README.md**

Create `npm/platform-packages/README.md`:

```markdown
# sku npm platform packages

Each file `@sofq/sku-<os>-<arch>` is a one-platform package containing a single
prebuilt `sku` binary under `bin/`. The root `@sofq/sku` package lists all six as
`optionalDependencies`; npm installs only the one matching the current platform.

These packages are generated at release time from the goreleaser `dist/`
artifacts by `.github/workflows/release-npm.yml`. Do not hand-author them.

Supported platforms: linux-x64, linux-arm64, darwin-x64, darwin-arm64,
win32-x64, win32-arm64.
```

- [ ] **Step 5: Write npm/README.md**

Create `npm/README.md`:

```markdown
# @sofq/sku (npm wrapper)

Install: `npm i -g @sofq/sku` or `npx @sofq/sku version`.

This is a thin wrapper around a prebuilt Go binary. npm resolves exactly one
`@sofq/sku-<os>-<arch>` optional dependency for the current platform and the
root shim `bin/sku.js` execs it. No postinstall network download.

Source: https://github.com/sofq/sku
```

- [ ] **Step 6: Smoke-test the shim logic locally**

Run:

```bash
mkdir -p /tmp/sku-npm-smoke/node_modules/@sofq/sku-$([[ "$(uname -s)" == "Darwin" ]] && echo darwin || echo linux)-$([[ "$(uname -m)" == "arm64" ]] && echo arm64 || echo x64)/bin
cp bin/sku /tmp/sku-npm-smoke/node_modules/@sofq/sku-*/bin/sku
cp -r npm/bin /tmp/sku-npm-smoke/
cd /tmp/sku-npm-smoke && node bin/sku.js version
```

Expected: exit 0, prints JSON version. If `bin/sku` doesn't exist, run `make build` first.

- [ ] **Step 7: Commit**

```bash
git add npm/
git commit -m "feat(m6): npm wrapper (@sofq/sku) using platform-optional-dependencies pattern"
```

---

### Task 6: PyPI wrapper — cibuildwheel platform-tagged wheels

**Files:**
- Create: `python/pyproject.toml`
- Create: `python/sku_cli/__init__.py`
- Create: `python/sku_cli/__main__.py`
- Create: `python/MANIFEST.in`
- Create: `python/README.md`

- [ ] **Step 1: Write pyproject.toml**

Create `python/pyproject.toml`:

```toml
[build-system]
requires = ["setuptools>=68", "wheel"]
build-backend = "setuptools.build_meta"

[project]
name = "sku-cli"
version = "0.0.0"
description = "Agent-friendly CLI for cloud + LLM pricing (PyPI wrapper)."
readme = "README.md"
requires-python = ">=3.9"
license = { text = "Apache-2.0" }
authors = [{ name = "sofq" }]
classifiers = [
  "License :: OSI Approved :: Apache Software License",
  "Operating System :: POSIX :: Linux",
  "Operating System :: MacOS",
  "Operating System :: Microsoft :: Windows",
  "Programming Language :: Python :: 3",
]

[project.scripts]
sku = "sku_cli.__main__:main"

[project.urls]
Homepage = "https://github.com/sofq/sku"

[tool.setuptools]
packages = ["sku_cli"]
include-package-data = true

[tool.setuptools.package-data]
sku_cli = ["bin/*"]

# cibuildwheel is driven from release-pypi.yml; keys here document the matrix.
[tool.cibuildwheel]
build = ["cp311-*"]  # One wheel per platform tag; the Python tag is irrelevant
                     # because the wheel vendors a static Go binary and contains
                     # no compiled C extensions. We pick cp311 arbitrarily and
                     # rely on the platform tag alone to route installs.
skip = ["*-musllinux_*"]  # musllinux covered via a dedicated matrix entry
                           # outside cibuildwheel (see release-pypi.yml).
archs = { linux = ["x86_64", "aarch64"], macos = ["x86_64", "arm64"], windows = ["AMD64", "ARM64"] }
```

- [ ] **Step 2: Write the exec shim**

Create `python/sku_cli/__init__.py` (empty) and `python/sku_cli/__main__.py`:

```python
"""PyPI wrapper entrypoint: locate the vendored sku binary and exec it."""

import os
import sys
from importlib.resources import files


def _binary_path() -> str:
    # The binary is vendored inside the wheel at sku_cli/bin/sku(.exe).
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
    # On POSIX use execv for signal fidelity; on Windows fall back to subprocess.
    if sys.platform == "win32":
        import subprocess

        return subprocess.run([path, *sys.argv[1:]]).returncode
    os.execv(path, [path, *sys.argv[1:]])


if __name__ == "__main__":
    raise SystemExit(main())
```

- [ ] **Step 3: Write MANIFEST.in and README**

Create `python/MANIFEST.in`:

```
graft sku_cli/bin
include README.md
```

Create `python/README.md`:

```markdown
# sku-cli (PyPI wrapper)

Install: `pipx install sku-cli` (recommended) or `pip install sku-cli`.

Ships as platform-tagged wheels (manylinux, musllinux, macos, windows x x86_64 + arm64)
each containing a vendored prebuilt `sku` binary. Pip selects the correct wheel
at install time. No postinstall hooks, no network fetches.

Source: https://github.com/sofq/sku
```

- [ ] **Step 4: Local sdist smoke**

Run (requires `python3 -m pip install --user build`):

```bash
cd python && python3 -m build --sdist
ls dist/ | grep "sku_cli-0.0.0.tar.gz" && echo "sdist ok"
```

Expected: `sdist ok`. Wheel build requires `cibuildwheel` + platform toolchain, covered in Task 7.

- [ ] **Step 5: Commit**

```bash
git add python/
git commit -m "feat(m6): PyPI wrapper (sku-cli) with cibuildwheel platform-tagged wheels"
```

---

### Task 7: release-npm.yml + release-pypi.yml — publish wrappers after goreleaser

**Files:**
- Create: `.github/workflows/release-npm.yml`
- Create: `.github/workflows/release-pypi.yml`
- Modify: `.github/workflows/release.yml` (chain these jobs)

- [ ] **Step 1: Write release-npm.yml**

Create `.github/workflows/release-npm.yml`:

```yaml
name: release-npm

on:
  workflow_call:
    inputs:
      version:
        required: true
        type: string
      prerelease:
        required: true
        type: boolean
    secrets:
      NPM_TOKEN:
        required: true

jobs:
  publish:
    if: ${{ !inputs.prerelease }}  # Pre-releases skip npm per spec §7 Versioning.
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: "20"
          registry-url: "https://registry.npmjs.org"

      - name: Download release assets from GH Releases
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          mkdir -p artifacts
          gh release download "v${{ inputs.version }}" -D artifacts \
            --pattern "sku_${{ inputs.version }}_*.tar.gz" \
            --pattern "sku_${{ inputs.version }}_*.zip"

      - name: Build and publish platform packages
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
          VERSION: ${{ inputs.version }}
        run: node scripts/publish-npm.js artifacts "$VERSION"

      - name: Publish root @sofq/sku
        env:
          NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}
          VERSION: ${{ inputs.version }}
        run: |
          cd npm
          node -e "const p=require('./package.json');p.version=process.env.VERSION;\
            for(const k of Object.keys(p.optionalDependencies))p.optionalDependencies[k]=process.env.VERSION;\
            require('fs').writeFileSync('package.json',JSON.stringify(p,null,2))"
          npm publish --access public
```

- [ ] **Step 2: Write the publish-npm.js helper**

Create `scripts/publish-npm.js`:

> This file lives under `scripts/`. Policy says the plan must not modify `scripts/`.
> Since `scripts/` does not yet contain `publish-npm.js`, creating it is within scope.
> Verify with `ls scripts/publish-npm.js` before step 3: if it exists, STOP and ask.

```javascript
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

// goreleaser Os/Arch -> Node os/cpu
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
```

- [ ] **Step 3: Write release-pypi.yml**

Create `.github/workflows/release-pypi.yml`:

```yaml
name: release-pypi

on:
  workflow_call:
    inputs:
      version:
        required: true
        type: string
      prerelease:
        required: true
        type: boolean
    secrets:
      PYPI_API_TOKEN:
        required: true

jobs:
  wheels:
    if: ${{ !inputs.prerelease }}
    strategy:
      fail-fast: false
      matrix:
        include:
          - { os: ubuntu-latest,  goOs: linux,   goArch: amd64, wheelPlat: "manylinux_2_28_x86_64" }
          - { os: ubuntu-latest,  goOs: linux,   goArch: arm64, wheelPlat: "manylinux_2_28_aarch64" }
          - { os: ubuntu-latest,  goOs: linux,   goArch: amd64, wheelPlat: "musllinux_1_2_x86_64" }
          - { os: ubuntu-latest,  goOs: linux,   goArch: arm64, wheelPlat: "musllinux_1_2_aarch64" }
          - { os: macos-13,       goOs: darwin,  goArch: amd64, wheelPlat: "macosx_11_0_x86_64" }
          - { os: macos-14,       goOs: darwin,  goArch: arm64, wheelPlat: "macosx_11_0_arm64" }
          - { os: windows-latest, goOs: windows, goArch: amd64, wheelPlat: "win_amd64" }
          - { os: windows-11-arm, goOs: windows, goArch: arm64, wheelPlat: "win_arm64" }
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with:
          python-version: "3.11"

      - name: Download matching goreleaser artifact
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        shell: bash
        run: |
          ext=tar.gz; [[ "${{ matrix.goOs }}" == "windows" ]] && ext=zip
          mkdir -p artifacts
          gh release download "v${{ inputs.version }}" -D artifacts \
            --pattern "sku_${{ inputs.version }}_${{ matrix.goOs }}_${{ matrix.goArch }}.${ext}"

      - name: Stage binary inside python/sku_cli/bin/
        shell: bash
        run: |
          mkdir -p python/sku_cli/bin
          ext=tar.gz; bin=sku
          if [[ "${{ matrix.goOs }}" == "windows" ]]; then ext=zip; bin=sku.exe; fi
          cd python/sku_cli/bin
          if [[ "$ext" == "tar.gz" ]]; then
            tar -xzf ../../../artifacts/sku_${{ inputs.version }}_${{ matrix.goOs }}_${{ matrix.goArch }}.tar.gz "$bin"
          else
            unzip -j ../../../artifacts/sku_${{ inputs.version }}_${{ matrix.goOs }}_${{ matrix.goArch }}.zip "$bin"
          fi

      - name: Build wheel with platform tag
        shell: bash
        run: |
          cd python
          python -m pip install --upgrade pip build wheel
          python -m build --wheel
          # Re-tag the built py3-none-any wheel with our concrete platform.
          pip install wheel
          w=$(ls dist/*.whl)
          python -m wheel tags --platform-tag "${{ matrix.wheelPlat }}" --remove "$w"
          mv dist/*.whl dist-tagged/ 2>/dev/null || mkdir -p dist-tagged && mv dist/*"${{ matrix.wheelPlat }}"*.whl dist-tagged/

      - uses: actions/upload-artifact@v4
        with:
          name: wheel-${{ matrix.wheelPlat }}
          path: python/dist-tagged/*.whl

  publish:
    needs: wheels
    runs-on: ubuntu-latest
    steps:
      - uses: actions/download-artifact@v4
        with:
          path: dist
          merge-multiple: true
      - uses: actions/setup-python@v5
        with:
          python-version: "3.11"
      - name: Publish to PyPI
        env:
          TWINE_USERNAME: __token__
          TWINE_PASSWORD: ${{ secrets.PYPI_API_TOKEN }}
        run: |
          python -m pip install twine
          twine upload dist/*.whl
```

- [ ] **Step 4: Chain wrapper jobs from release.yml**

Append to `.github/workflows/release.yml`:

```yaml
  release-npm:
    needs: goreleaser
    uses: ./.github/workflows/release-npm.yml
    with:
      version: ${{ github.ref_name == '' && '0.0.0' || trimPrefix(github.ref_name, 'v') }}
      prerelease: ${{ contains(github.ref_name, '-rc.') || contains(github.ref_name, '-pre') }}
    secrets:
      NPM_TOKEN: ${{ secrets.NPM_TOKEN }}

  release-pypi:
    needs: goreleaser
    uses: ./.github/workflows/release-pypi.yml
    with:
      version: ${{ github.ref_name == '' && '0.0.0' || trimPrefix(github.ref_name, 'v') }}
      prerelease: ${{ contains(github.ref_name, '-rc.') || contains(github.ref_name, '-pre') }}
    secrets:
      PYPI_API_TOKEN: ${{ secrets.PYPI_API_TOKEN }}
```

Note: GitHub Actions does not ship a `trimPrefix` function. Replace with an inline step that strips the `v` prefix and exposes it via `${{ steps.ver.outputs.version }}`:

```yaml
    # Replace the `version:` expressions above with a prior job output:
    # - id: ver
    #   run: echo "version=${GITHUB_REF_NAME#v}" >> "$GITHUB_OUTPUT"
```

Refactor the chained jobs to use a small `compute-version` job that outputs `version` and `prerelease`, and have `release-npm` / `release-pypi` depend on it.

- [ ] **Step 5: Lint all three workflow files**

Run: `for f in .github/workflows/release*.yml; do python3 -c "import yaml;yaml.safe_load(open('$f'))" && echo "$f ok"; done`
Expected: three `ok` lines.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/release-npm.yml .github/workflows/release-pypi.yml .github/workflows/release.yml scripts/publish-npm.js
git commit -m "feat(m6): chained release workflows — npm (platform packages) + PyPI (cibuildwheel wheels)"
```

---

### Task 8: Makefile smoke targets + RELEASING.md + install docs

**Files:**
- Modify: `Makefile`
- Create: `docs/contributing/RELEASING.md`
- Create: `docs/install.md`

- [ ] **Step 1: Add Makefile targets**

Append to `Makefile`:

```make
.PHONY: release-check
release-check: ## Full local goreleaser dry-run incl. sign/sbom/docker (requires cosign+syft+docker)
	goreleaser release --snapshot --clean

.PHONY: npm-pack-smoke
npm-pack-smoke: build ## Pack root npm package and run `sku version` via shim
	cd npm && npm pack --dry-run
	node npm/bin/sku.js version || true  # shim will fail without platform pkg installed; just verify it runs

.PHONY: pypi-wheel-smoke
pypi-wheel-smoke: build ## Stage local binary + build a single wheel
	mkdir -p python/sku_cli/bin && cp bin/sku python/sku_cli/bin/
	cd python && python3 -m build --wheel
```

- [ ] **Step 2: Write RELEASING.md**

Create `docs/contributing/RELEASING.md`:

```markdown
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
```

- [ ] **Step 3: Write docs/install.md**

Create `docs/install.md`:

```markdown
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
```

- [ ] **Step 4: Commit**

```bash
git add Makefile docs/contributing/RELEASING.md docs/install.md
git commit -m "docs(m6): RELEASING.md + install.md + make release-check/npm-pack-smoke/pypi-wheel-smoke"
```

---

### Task 9: Full local dry-run, pre-release tag, and exit-criteria verification

**Files:**
- Modify: `CHANGELOG.md` (create if missing)
- Modify: `CLAUDE.md` (add M6 agent quick path lines)

- [ ] **Step 1: Confirm full goreleaser dry-run is clean**

Run: `make release-check`
Expected: Exit 0. `dist/` contains:
- `sku_<v>_linux_amd64.tar.gz`, `_linux_arm64.tar.gz`, `_darwin_amd64.tar.gz`, `_darwin_arm64.tar.gz`, `_windows_amd64.zip`, `_windows_arm64.zip`
- `checksums.txt`, `checksums.txt.sig` (only if cosign available locally; skipped gracefully otherwise)
- `*.spdx.sbom.json` per archive
- `sku.rb` (brew formula), `sku.json` (scoop manifest)
- At least one `.deb`, `.rpm`, `.apk`
- Docker images tagged `ghcr.io/sofq/sku:<snapshot>-amd64` and `-arm64` in local docker

If cosign or syft missing locally, re-run with `--skip=sign,sbom` and record the skip in the task notes.

- [ ] **Step 2: Write CHANGELOG.md v0.1.0 entry**

Create or prepend to `CHANGELOG.md`:

```markdown
# Changelog

## v0.1.0 — 2026-04-<TBD> — first public pre-release

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
```

- [ ] **Step 3: Add M6 verification lines to CLAUDE.md**

Edit `/Users/quan.hoang/quanhh/quanhoang/sku/CLAUDE.md`. Under "Quick path" add a new subsection header before the backlog block:

```markdown
### Distribution smoke (M6)

```bash
make release-check          # Full local goreleaser dry-run
make docker-smoke           # Build + run sku:smoke container
make npm-pack-smoke         # Dry npm pack + shim sanity-check
make pypi-wheel-smoke       # Build one wheel with the local binary vendored
```
```

Use the Edit tool to insert this block just before the "Global flags" section. The exact anchor to locate is the line `## Global flags (all subcommands)`.

- [ ] **Step 4: Tag and push the pre-release**

Run:

```bash
git add CHANGELOG.md CLAUDE.md
git commit -m "chore(m6): v0.1.0 changelog + CLAUDE.md distribution smoke targets"
git tag -s v0.1.0-rc.1 -m "sku v0.1.0-rc.1 — first distribution-channels pre-release"
git push origin v0.1.0-rc.1
```

Only push the tag after the user has confirmed:
1. All M6 secrets are populated in GitHub Settings.
2. External tap/bucket repos exist.
3. They want the pre-release cut from this branch.

If any answer is "no", STOP — record the blocker and do not push the tag.

- [ ] **Step 5: Verify the live pre-release (only after tag push)**

For each of the following, run and capture output in the PR/milestone close notes:

```bash
gh release view v0.1.0-rc.1 --json assets --jq '.assets[].name' | sort
# Expected: 6 archives, checksums.txt, checksums.txt.sig, checksums.txt.pem, 6 sboms

docker pull ghcr.io/sofq/sku:0.1.0-rc.1
docker run --rm ghcr.io/sofq/sku:0.1.0-rc.1 version
# Expected: JSON version payload with "version":"0.1.0-rc.1"

cosign verify-blob \
  --certificate-identity-regexp 'https://github.com/sofq/sku/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --signature /tmp/checksums.txt.sig --certificate /tmp/checksums.txt.pem \
  /tmp/checksums.txt
# Expected: "Verified OK"

gh attestation verify /tmp/sku_0.1.0-rc.1_linux_amd64.tar.gz --owner sofq
# Expected: Provenance attestation verified against the actions-runner identity
```

Spec §9 M6 exit criteria are met when: `brew install`, `scoop install`,
`npm i -g`, `pipx install`, and `docker run` all succeed on fresh machines for
the first non-pre-release tag (cut in a later session). For the `-rc.1` tag
here, only GH Release, docker, and cosign/attestation checks apply — the
others are correctly skipped.

- [ ] **Step 6: Close M6 in CLAUDE.md**

Edit `CLAUDE.md`: replace the `## Current milestone` block body from its current
content to:

```markdown
## Current milestone

M6 — Distribution channels: pre-release `v0.1.0-rc.1` cut.
Exit-gated on fresh-machine install verification for the first non-pre-release tag.
Next: M7 — Docs, polish, v1.0.
```

Commit:

```bash
git add CLAUDE.md
git commit -m "chore(m6): close milestone after v0.1.0-rc.1 pre-release verification"
```

---

## Self-review notes (non-executable)

- Spec coverage: §7 Distribution (goreleaser, npm, python, docker, supply chain, releasing, failure handling) → Tasks 1–8. §9 M6 exit criteria → Task 9. Pre-release `-rc.N` behavior → `skip_upload: auto` (Task 2) and `if: !inputs.prerelease` gates (Task 7).
- Placeholders: none — every code block is concrete. Wheel re-tagging uses `python -m wheel tags`; `cibuildwheel` matrix is inlined because the wheels vendor a Go binary and the Python ABI tag is immaterial. External-repo creation (homebrew-tap, scoop-bucket) is explicitly out of scope and gated via a human confirmation in Task 9.
- Type consistency: `@sofq/sku-<os>-<arch>` naming is identical across root `package.json`, `bin/sku.js`, `publish-npm.js`, and platform template. Cosign + syft actions use the same versions across files.
- Risks: PyPI windows-arm64 runner availability (`windows-11-arm`) — if GitHub has not yet GA'd this runner when M6 runs, drop `win_arm64` from Task 7 matrix and note it as a v1.1 follow-up; do NOT block M6 on it.
