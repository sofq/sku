# M0 — Foundations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold the `sku` repo so that `make build && ./bin/sku version` emits valid JSON, CI is green across the 5-platform matrix, and `goreleaser release --snapshot --clean` produces signed archives — the spine every subsequent milestone plugs into.

**Architecture:** Single Go module `github.com/sofq/sku` targeting Go 1.25. Cobra-based CLI with a thin `cmd/sku` tree calling into `internal/*` (empty in M0 beyond `internal/version`). Pure-Go dependencies only (`CGO_ENABLED=0`) so goreleaser cross-compiles to 6 targets without Docker tricks. CI matrix: Go 1.25 + 1.26 × (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64).

**Tech Stack:** Go 1.25, `github.com/spf13/cobra`, `github.com/stretchr/testify`, goreleaser, golangci-lint, GitHub Actions, cosign (keyless OIDC), syft (SBOM).

**Spec ref:** `docs/superpowers/specs/2026-04-18-sku-design.md` §9 (M0), §8 (repo layout), §7 (CI/CD).

---

## File Structure

| Path | Responsibility |
|---|---|
| `go.mod`, `go.sum` | Module definition, `go 1.25` directive |
| `main.go` | Root shim that calls `cmd/sku.Execute()` so `go install github.com/sofq/sku@latest` works |
| `cmd/sku/root.go` | Cobra root command with global flags stubbed (no-op in M0) |
| `cmd/sku/version.go` | `sku version` subcommand emitting JSON |
| `cmd/sku/execute.go` | Exported `Execute()` called from root `main.go` |
| `internal/version/version.go` | Single source of truth for version string, build metadata |
| `internal/version/version_test.go` | Unit tests for the JSON rendering |
| `Makefile` | `build`, `test`, `lint`, `clean`, `generate`, `release-dry` |
| `.golangci.yml` | Linter config (enabled: govet, errcheck, staticcheck, gofmt, goimports, gosec, revive) |
| `.goreleaser.yml` | 6-target cross-compile, tar.gz/zip archives, checksums, cosign, syft |
| `.github/workflows/ci.yml` | PR/push: lint + test + build smoke, 5-platform × 2-Go-minor matrix |
| `.github/workflows/release.yml` | On tag `v*.*.*`: `goreleaser release` |
| `.gitignore` | `/bin/`, `/dist/`, Go coverage, `.DS_Store`, editor droppings |
| `LICENSE` | Apache-2.0 text |
| `NOTICE` | Apache-2.0 NOTICE stub |
| `README.md` | Short placeholder — name, status badge, install stub, "see docs/" |
| `CLAUDE.md` | Agent guide: dev commands, repo layout pointers, pattern cheatsheet |
| `docs/ops/m0-name-availability.md` | Record of name-availability check (PyPI, npm, brew, scoop) |

---

## Task 0: Binary-name availability check (spec §9 M0 exit gate)

**Why first:** §9 declares M0 cannot close until `sku` is claimable on PyPI, npm, Homebrew tap namespace, and Scoop bucket namespace. Renaming after shards publish would invalidate manifest URLs. Do this before any other file writes so the name is locked.

**Files:**
- Create: `docs/ops/m0-name-availability.md`

- [ ] **Step 1: Check PyPI**

Run: `curl -sS -o /dev/null -w "%{http_code}\n" https://pypi.org/pypi/sku/json`
Expected: `404` (available) or `200` (taken — fall back to `sku-cli` as already named in §7).

- [ ] **Step 2: Check npm**

Run: `curl -sS -o /dev/null -w "%{http_code}\n" https://registry.npmjs.org/@sofq/sku`
Expected: `404`. Also verify the bare `sku` package is unneeded (§7 publishes `@sofq/sku`).

- [ ] **Step 3: Check Homebrew tap namespace**

Run: `curl -sS -o /dev/null -w "%{http_code}\n" https://github.com/sofq/homebrew-tap`
Expected: `404` (org repo not yet created — reserve it) or `200` (owned).

- [ ] **Step 4: Check Scoop bucket namespace**

Run: `curl -sS -o /dev/null -w "%{http_code}\n" https://github.com/sofq/scoop-bucket`
Expected: `404` or `200` (owned by `sofq`).

- [ ] **Step 5: Record results**

Write `docs/ops/m0-name-availability.md`:

```markdown
# M0 Name-Availability Check

Ran: YYYY-MM-DD

| Registry | URL | HTTP | Status |
|---|---|---|---|
| PyPI `sku-cli` | https://pypi.org/pypi/sku-cli/json | 404 | available |
| npm `@sofq/sku` | https://registry.npmjs.org/@sofq/sku | 404 | available |
| Homebrew tap `sofq/homebrew-tap` | ... | 404 | reserve next |
| Scoop bucket `sofq/scoop-bucket` | ... | 404 | reserve next |

Decision: proceed with binary name `sku` and module path `github.com/sofq/sku`.
```

If any registry conflicts, STOP and escalate — a rename propagates through every downstream artifact URL.

- [ ] **Step 6: Commit**

```bash
git add docs/ops/m0-name-availability.md
git commit -m "ops: record M0 binary-name availability check"
```

---

## Task 1: Root files (LICENSE, NOTICE, README, .gitignore)

**Files:**
- Create: `LICENSE`, `NOTICE`, `README.md`, `.gitignore`

- [ ] **Step 1: Write LICENSE**

Fetch the canonical Apache-2.0 text:

```bash
curl -sSL https://www.apache.org/licenses/LICENSE-2.0.txt -o LICENSE
```

Verify: `head -1 LICENSE` → `Apache License`.

- [ ] **Step 2: Write NOTICE**

```
sku
Copyright 2026 Quan Hoang

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
```

- [ ] **Step 3: Write README.md** (placeholder; full README arrives in M7)

```markdown
# sku

Agent-friendly cloud & LLM pricing CLI.

**Status:** pre-alpha (M0). Not yet installable.

See [`docs/superpowers/specs/2026-04-18-sku-design.md`](docs/superpowers/specs/2026-04-18-sku-design.md) for the design.

## License

Apache-2.0 — see [LICENSE](LICENSE).
```

- [ ] **Step 4: Write .gitignore**

```
# Binaries
/bin/
/dist/

# Go
*.test
*.out
coverage.*
/vendor/

# Editor / OS
.DS_Store
.idea/
.vscode/
*.swp
```

- [ ] **Step 5: Commit**

```bash
git add LICENSE NOTICE README.md .gitignore
git commit -m "chore: add LICENSE, NOTICE, README placeholder, .gitignore"
```

---

## Task 2: Initialize Go module

**Files:**
- Create: `go.mod`

- [ ] **Step 1: Initialize module**

Run: `go mod init github.com/sofq/sku`

Expected output: `go: creating new go.mod: module github.com/sofq/sku`.

- [ ] **Step 2: Pin Go directive to 1.25**

Edit `go.mod` so the second line reads `go 1.25` (the exact floor the spec §7 commits to). If `go mod init` emitted `1.26`, downgrade; if `1.24`, bump.

- [ ] **Step 3: Verify `go version` satisfies the directive**

Run: `go version`
Expected: `go version go1.25.x ...` or `go1.26.x`. If older, install via `gvm`/`brew` before proceeding.

- [ ] **Step 4: Add cobra + testify**

Run:
```bash
go get github.com/spf13/cobra@latest
go get github.com/stretchr/testify@latest
```

Verify `go.sum` is created and `go.mod` has both modules in the `require` block.

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum
git commit -m "build: initialize Go module with cobra + testify"
```

---

## Task 3: `internal/version` package (TDD)

Single source of truth for version metadata. `sku version` marshals this struct.

**Files:**
- Create: `internal/version/version.go`
- Test: `internal/version/version_test.go`

- [ ] **Step 1: Write the failing test**

`internal/version/version_test.go`:

```go
package version

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfo_DefaultValuesAreDev(t *testing.T) {
	info := Get()
	require.Equal(t, "dev", info.Version)
	require.Equal(t, "unknown", info.Commit)
	require.Equal(t, "unknown", info.Date)
}

func TestInfo_JSONShape(t *testing.T) {
	info := Info{Version: "1.2.3", Commit: "abc123", Date: "2026-04-18T00:00:00Z", GoVersion: "go1.25.0", Os: "linux", Arch: "amd64"}
	raw, err := json.Marshal(info)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "1.2.3", got["version"])
	require.Equal(t, "abc123", got["commit"])
	require.Equal(t, "2026-04-18T00:00:00Z", got["date"])
	require.Equal(t, "go1.25.0", got["go_version"])
	require.Equal(t, "linux", got["os"])
	require.Equal(t, "amd64", got["arch"])
}

func TestInfo_OverrideViaLdflags(t *testing.T) {
	// Simulate goreleaser -ldflags="-X ...version=1.0.0"
	oldV, oldC, oldD := version, commit, date
	t.Cleanup(func() { version, commit, date = oldV, oldC, oldD })
	version, commit, date = "1.0.0", "deadbeef", "2026-04-18T12:00:00Z"

	info := Get()
	require.Equal(t, "1.0.0", info.Version)
	require.Equal(t, "deadbeef", info.Commit)
	require.Equal(t, "2026-04-18T12:00:00Z", info.Date)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/version/...`
Expected: FAIL — `package github.com/sofq/sku/internal/version is not in std`.

- [ ] **Step 3: Write the implementation**

`internal/version/version.go`:

```go
// Package version exposes build-time metadata injected by goreleaser via -ldflags.
package version

import "runtime"

// These are overwritten at build time with -ldflags "-X github.com/sofq/sku/internal/version.version=..."
// Kept as package-level vars (not consts) so ldflags can patch them and tests can override.
var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

// Info is the JSON payload emitted by `sku version`.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	GoVersion string `json:"go_version"`
	Os        string `json:"os"`
	Arch      string `json:"arch"`
}

// Get returns the current build's Info, including runtime-derived fields.
func Get() Info {
	return Info{
		Version:   version,
		Commit:    commit,
		Date:      date,
		GoVersion: runtime.Version(),
		Os:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/version/... -v`
Expected: all three tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/version/
git commit -m "feat(version): add internal/version with ldflags-injectable build metadata"
```

---

## Task 4: Cobra root + `Execute()` entrypoint

**Files:**
- Create: `cmd/sku/root.go`, `cmd/sku/execute.go`

- [ ] **Step 1: Write `cmd/sku/root.go`**

```go
// Package sku wires the Cobra command tree for the sku CLI.
package sku

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sku",
		Short:         "Agent-friendly cloud & LLM pricing CLI",
		Long:          "sku is an agent-friendly CLI for querying cloud and LLM pricing across AWS, Azure, Google Cloud, and OpenRouter.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd())
	return root
}
```

- [ ] **Step 2: Write `cmd/sku/execute.go`**

```go
package sku

import (
	"fmt"
	"os"
)

// Execute is the public entrypoint called from the root main.go. It runs the
// Cobra tree and maps any error to a non-zero exit code. The full exit-code
// taxonomy (§4 of the design spec) is wired in M2; in M0 we only use 0 and 1.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build to verify compilation** (version command added next task)

Run: `go build ./cmd/sku/`
Expected: error — `undefined: newVersionCmd`. That's fine; Task 5 defines it.

- [ ] **Step 4: Do not commit yet** — commit lands at end of Task 5 so HEAD always builds.

---

## Task 5: `sku version` subcommand (TDD)

**Files:**
- Create: `cmd/sku/version.go`
- Test: `cmd/sku/version_test.go`

- [ ] **Step 1: Write the failing test**

`cmd/sku/version_test.go`:

```go
package sku

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionCmd_EmitsValidJSON(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"version"})

	require.NoError(t, cmd.Execute())

	var payload map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &payload), "stdout must be valid JSON, got %q", out.String())

	for _, key := range []string{"version", "commit", "date", "go_version", "os", "arch"} {
		require.Contains(t, payload, key, "missing key %q", key)
	}
}

func TestVersionCmd_CompactByDefault(t *testing.T) {
	var out bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"version"})
	require.NoError(t, cmd.Execute())

	// One line of JSON + trailing newline — no indentation.
	require.NotContains(t, out.String(), "  ", "default output must be compact")
	require.Equal(t, byte('\n'), out.Bytes()[out.Len()-1])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/sku/...`
Expected: FAIL — `undefined: newVersionCmd`.

- [ ] **Step 3: Write the implementation**

`cmd/sku/version.go`:

```go
package sku

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(version.Get())
		},
	}
}
```

(`json.Encoder.Encode` writes compact JSON followed by `\n`, matching the second test.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./cmd/sku/... -v`
Expected: both tests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/sku/
git commit -m "feat(cmd): add root Cobra command + JSON-emitting version subcommand"
```

---

## Task 6: Root `main.go` shim

**Files:**
- Create: `main.go`

- [ ] **Step 1: Write the shim**

```go
// Binary sku is the agent-friendly cloud & LLM pricing CLI.
//
// Keeping main.go at the module root makes `go install github.com/sofq/sku@latest`
// work without a `/cmd/sku` path suffix. All logic lives in cmd/sku.
package main

import "github.com/sofq/sku/cmd/sku"

func main() {
	sku.Execute()
}
```

- [ ] **Step 2: Smoke-test end-to-end**

Run:
```bash
go build -o bin/sku .
./bin/sku version | python3 -m json.tool
```

Expected: indented JSON with `version`, `commit`, `date`, `go_version`, `os`, `arch` keys and no error.

- [ ] **Step 3: Confirm `go install` shape**

Run: `go install .`
Expected: binary lands at `$(go env GOBIN || echo $(go env GOPATH)/bin)/sku` with no error.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: add root main.go shim so \`go install github.com/sofq/sku@latest\` works"
```

---

## Task 7: Makefile

**Files:**
- Create: `Makefile`

- [ ] **Step 1: Write the Makefile**

```make
SHELL := bash
.DEFAULT_GOAL := help

GO            ?= go
BIN_DIR       := bin
BINARY        := $(BIN_DIR)/sku
PKG           := ./...
GO_LDFLAGS    := -s -w \
                 -X github.com/sofq/sku/internal/version.version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev) \
                 -X github.com/sofq/sku/internal/version.commit=$(shell git rev-parse HEAD 2>/dev/null || echo unknown) \
                 -X github.com/sofq/sku/internal/version.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_.-]+:.*?## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Compile ./bin/sku
	@mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -ldflags "$(GO_LDFLAGS)" -o $(BINARY) .

.PHONY: test
test: ## Run unit + integration tests
	$(GO) test -race -count=1 $(PKG)

.PHONY: lint
lint: ## Run golangci-lint
	golangci-lint run

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) dist

.PHONY: generate
generate: ## Run go generate across the module (placeholder; used from M4)
	$(GO) generate $(PKG)

.PHONY: release-dry
release-dry: ## Snapshot build via goreleaser; no publish
	goreleaser release --snapshot --clean
```

- [ ] **Step 2: Smoke-test each target**

Run:
```bash
make help
make clean build
./bin/sku version
make test
```

Expected: `make build` produces `bin/sku`; `./bin/sku version | jq .version` prints a git-describe string (e.g. `f4b1dac-dirty` if there are uncommitted changes in other files).

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "build: add Makefile with build, test, lint, clean, generate, release-dry"
```

---

## Task 8: golangci-lint config

**Files:**
- Create: `.golangci.yml`

- [ ] **Step 1: Write the config**

```yaml
version: "2"

run:
  timeout: 5m

linters:
  default: none
  enable:
    - errcheck
    - gosec
    - govet
    - ineffassign
    - misspell
    - revive
    - staticcheck
    - unused

formatters:
  enable:
    - gofmt
    - goimports

issues:
  max-issues-per-linter: 0
  max-same-issues: 0
```

- [ ] **Step 2: Install golangci-lint locally** (skip if already installed)

Run: `golangci-lint version`
If absent: `brew install golangci-lint` (macOS) or the binary installer script from golangci-lint.run.

- [ ] **Step 3: Run the linter**

Run: `make lint`
Expected: `0 issues.` (or a clean exit). Fix anything flagged before committing.

- [ ] **Step 4: Commit**

```bash
git add .golangci.yml
git commit -m "build: add golangci-lint config (errcheck, gosec, staticcheck, revive, ...)"
```

---

## Task 9: goreleaser config + dry-run

**Files:**
- Create: `.goreleaser.yml`

- [ ] **Step 1: Write the config**

```yaml
version: 2

project_name: sku

before:
  hooks:
    - go mod tidy

builds:
  - id: sku
    main: .
    binary: sku
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w
      - -X github.com/sofq/sku/internal/version.version={{.Version}}
      - -X github.com/sofq/sku/internal/version.commit={{.Commit}}
      - -X github.com/sofq/sku/internal/version.date={{.Date}}
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]

archives:
  - id: default
    formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        formats: [zip]
    files:
      - LICENSE
      - NOTICE
      - README.md

checksum:
  name_template: "checksums.txt"

snapshot:
  version_template: "{{ incpatch .Version }}-snapshot"

changelog:
  disable: true   # enabled in M7 alongside CHANGELOG.md workflow

release:
  draft: true     # flipped to false in M6 when distribution channels go live
```

(cosign + syft + Homebrew tap + Scoop + npm + PyPI + GHCR stanzas land in M6 alongside the distribution channels — §9 M6. Keeping the M0 config minimal so the dry-run is green without signing secrets.)

- [ ] **Step 2: Install goreleaser locally** (skip if already installed)

Run: `goreleaser --version`
If absent: `brew install goreleaser`.

- [ ] **Step 3: Dry-run**

Run: `make release-dry`
Expected: `dist/` populated with 6 archives (`sku_*_linux_amd64.tar.gz`, `sku_*_linux_arm64.tar.gz`, `sku_*_darwin_amd64.tar.gz`, `sku_*_darwin_arm64.tar.gz`, `sku_*_windows_amd64.zip`, `sku_*_windows_arm64.zip`) + `checksums.txt`.

- [ ] **Step 4: Spot-check a cross-compiled binary**

Run: `tar -tzf dist/sku_*_linux_arm64.tar.gz`
Expected: `sku`, `LICENSE`, `NOTICE`, `README.md` entries.

- [ ] **Step 5: Commit**

```bash
git add .goreleaser.yml
git commit -m "build: add minimal goreleaser config (6 targets, no signing yet)"
```

---

## Task 10: CI workflow (5-platform × 2-Go-minor matrix)

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow**

```yaml
name: ci

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
          cache: true
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  test:
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        go: ["1.25", "1.26"]
        include:
          - os: ubuntu-24.04-arm
            go: "1.25"
          - os: ubuntu-24.04-arm
            go: "1.26"
          - os: macos-14           # darwin/arm64 (Apple Silicon runner)
            go: "1.25"
          - os: macos-14
            go: "1.26"
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go }}
          cache: true
      - run: go build ./...
      - run: go test -race -count=1 ./...

  release-dry:
    runs-on: ubuntu-latest
    needs: [lint, test]
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
          cache: true
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --snapshot --clean --skip=sign,publish
```

Matrix note (§7): covers the 5 supported targets — linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 — across the two most recent stable Go minors (1.25, 1.26).

- [ ] **Step 2: Validate YAML locally**

Run: `python3 -c "import yaml, sys; yaml.safe_load(open('.github/workflows/ci.yml'))" && echo OK`
Expected: `OK`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: add PR/push workflow (lint, 5-platform × 2-Go-minor test, release dry-run)"
```

- [ ] **Step 4: Push branch and verify CI green before proceeding**

Run: `git push -u origin <branch>` (or open a PR).
Expected: all matrix cells green. Fix any platform-specific breakage before Task 11.

---

## Task 11: Release workflow template

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Write the workflow**

```yaml
name: release

on:
  push:
    tags: ["v*.*.*"]

permissions:
  contents: write    # create GitHub releases
  id-token: write    # cosign keyless OIDC (wired up in M6)
  packages: write    # GHCR (wired up in M6)

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: "1.25"
          cache: true
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

(Cosign, syft, Homebrew, Scoop, npm, PyPI, GHCR secrets + stanzas are introduced in M6. M0 only proves the workflow shape + `goreleaser release --snapshot --clean` works on a real tag.)

- [ ] **Step 2: Validate YAML**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))" && echo OK`
Expected: `OK`.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add release workflow template (tag-triggered goreleaser)"
```

---

## Task 12: CLAUDE.md agent guide

**Files:**
- Create: `CLAUDE.md`

- [ ] **Step 1: Write the guide**

```markdown
# CLAUDE.md

Agent quick-start for the `sku` repo.

## What this is

`sku` is an agent-friendly CLI for querying cloud + LLM pricing. Offline-only client, daily data pipeline, pure-Go binary. See `docs/superpowers/specs/2026-04-18-sku-design.md` for the full design.

## Dev commands

| Task | Command |
|---|---|
| Build binary | `make build` (output: `bin/sku`) |
| Run tests | `make test` |
| Lint | `make lint` |
| Release dry-run | `make release-dry` |
| Regenerate code/docs | `make generate` (no-op until M4) |

## Repo map

- `cmd/sku/` — Cobra command tree (thin; no business logic)
- `internal/` — all logic lives here; packages are added per milestone
- `internal/version/` — single source of truth for build metadata
- `pipeline/` — CI-only data pipeline (arrives in M1+)
- `docs/superpowers/specs/` — design spec (rev 4 dated 2026-04-18)
- `docs/superpowers/plans/` — per-milestone implementation plans
- `.github/workflows/` — `ci.yml` (PR/push), `release.yml` (tag), and data workflows from M3a

## Patterns

- **Pure Go, no CGO.** Every dependency must cross-compile without Docker tricks.
- **`cmd/` stays thin.** Flag parsing + calls into `internal/`. No business logic.
- **TDD.** Write failing test, implement, commit.
- **Exit codes are contract** (spec §4). Not wired up in M0; full taxonomy arrives in M2.

## Current milestone

M0 — foundations. See `docs/superpowers/plans/2026-04-18-m0-foundations.md`.
```

- [ ] **Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add CLAUDE.md agent quick-start"
```

---

## Task 13: M0 exit verification

No code changes — a gate before declaring M0 done.

- [ ] **Step 1: Confirm CI green** on the latest push/PR across the 5-platform × 2-Go-minor matrix + release-dry job.

- [ ] **Step 2: Confirm `sku version` output locally**

Run:
```bash
make clean build
./bin/sku version
```
Expected: one-line JSON with all six keys (`version`, `commit`, `date`, `go_version`, `os`, `arch`).

- [ ] **Step 3: Confirm name-availability record is checked in**

Run: `ls docs/ops/m0-name-availability.md`
Expected: file present with the registry table filled in.

- [ ] **Step 4: Confirm goreleaser snapshot produces 6 archives**

Run: `make release-dry && ls dist/*.tar.gz dist/*.zip | wc -l`
Expected: `6`.

- [ ] **Step 5: Tag M0 completion** (optional)

```bash
git tag -s m0-done -m "M0 foundations complete: scaffold + CI green + goreleaser dry-run"
```

---

## Self-review notes

- **Spec coverage:** the 10 TODOs in §9 ("First week (M0) concrete TODOs") each map to a task (1→Task 2, 2→Task 1+2, 3→Task 7, 4→Task 8, 5→Task 9, 6→Task 4+5, 7→Task 6, 8→Task 10, 9→Task 11, 10→Task 12). §9 M0 exit-criteria name-availability gate is Task 0. §7 CI matrix shape matches Task 10.
- **Deferred to later milestones (explicit in task bodies):** cosign keyless signing, SLSA provenance, syft SBOM, Homebrew tap, Scoop bucket, npm/PyPI/Docker publish, `sku configure`, `sku update`, exit-code taxonomy beyond 0/1, `make generate` codegen. Each is cross-referenced to the milestone that introduces it.
- **Type consistency:** `Info` struct fields (`Version`, `Commit`, `Date`, `GoVersion`, `Os`, `Arch`) match JSON tags (`version`, `commit`, `date`, `go_version`, `os`, `arch`) consistently across Tasks 3 + 5 tests + Task 7 ldflags.
- **Dependencies not yet pulled:** `modernc.org/sqlite`, `itchyny/gojq`, `klauspost/compress/zstd`, `yaml.v3`, `x/term` — these arrive in M1 with the first shard reader, not M0.
