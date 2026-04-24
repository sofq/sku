package sku

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"

	"github.com/sofq/sku/internal/catalog"
	"github.com/sofq/sku/internal/updater"
)

// buildTestZst returns a minimal valid .zst file wrapping a copy of an
// existing SQLite DB (or any bytes), plus its sha256 hex string.
func buildTestZst(t *testing.T) (zstData []byte, hexSum string) {
	t.Helper()

	// Build a tiny DB from seed SQL so the decompressed file is a valid SQLite.
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "openrouter.db")
	ddl, err := os.ReadFile(filepath.Join("..", "..", "internal", "catalog", "testdata", "seed.sql"))
	require.NoError(t, err)
	require.NoError(t, catalog.BuildFromSQL(dbPath, string(ddl)))

	raw, err := os.ReadFile(dbPath) //nolint:gosec // test helper reads a known path
	require.NoError(t, err)

	var buf bytes.Buffer
	enc, err := zstd.NewWriter(&buf)
	require.NoError(t, err)
	_, err = enc.Write(raw)
	require.NoError(t, err)
	require.NoError(t, enc.Close())

	zstData = buf.Bytes()
	h := sha256.Sum256(zstData)
	hexSum = hex.EncodeToString(h[:])
	return
}

// singleShardManifest builds a minimal manifest JSON for one shard whose
// baseline lives at srvURL/<shard>.db.zst. hexSum is used for baseline_sha256
// (informational only — the actual SHA check happens via the .sha256 file).
func singleShardManifest(shard, srvURL, hexSum string) string {
	return `{"schema_version":1,"generated_at":"2026-04-22T04:00:00Z","catalog_version":"2026.04.22","shards":{"` +
		shard + `":{"baseline_version":"2026.04.22","baseline_url":"` + srvURL + `/` + shard + `.db.zst",` +
		`"baseline_sha256":"` + hexSum + `","baseline_size":1,"head_version":"2026.04.22",` +
		`"min_binary_version":"0.0.1","shard_schema_version":1,"deltas":[],"row_count":1,"last_updated":"2026.04.22"}}}`
}

// runUpdate invokes `sku update <args...>` against a captured root command.
func runUpdate(t *testing.T, args ...string) (stdout, stderr string, exitErr error) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := newRootCmd()
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(append([]string{"update"}, args...))
	exitErr = cmd.Execute()
	return out.String(), errb.String(), exitErr
}

// TestUpdate_HappyPath downloads a zst from a test server, verifies sha256,
// decompresses, and installs as openrouter.db.
func TestUpdate_HappyPath(t *testing.T) {
	zstData, hexSum := buildTestZst(t)

	var manifest string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(manifest))
		case "/openrouter.db.zst":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(zstData)
		case "/openrouter.db.zst.sha256":
			_, _ = w.Write([]byte(hexSum + "  openrouter.db.zst\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	manifest = singleShardManifest("openrouter", srv.URL, hexSum)

	dataDir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dataDir)
	t.Setenv("SKU_UPDATE_BASE_URL", srv.URL)

	_, stderr, err := runUpdate(t, "openrouter")
	require.NoError(t, err)
	require.Contains(t, stderr, "installed openrouter")

	dbPath := filepath.Join(dataDir, "openrouter.db")
	fi, statErr := os.Stat(dbPath)
	require.NoError(t, statErr, "openrouter.db should exist after update")
	require.Greater(t, fi.Size(), int64(0))
}

// TestUpdate_SHA256Mismatch returns CodeConflict (exit 6) when the digest does
// not match.
func TestUpdate_SHA256Mismatch(t *testing.T) {
	zstData, hexSum := buildTestZst(t)

	const wrongHex = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	var manifest string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(manifest))
		case "/openrouter.db.zst":
			_, _ = w.Write(zstData)
		case "/openrouter.db.zst.sha256":
			_, _ = w.Write([]byte(wrongHex + "  openrouter.db.zst\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	manifest = singleShardManifest("openrouter", srv.URL, hexSum)

	dataDir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dataDir)
	t.Setenv("SKU_UPDATE_BASE_URL", srv.URL)

	_, stderr, err := runUpdate(t, "openrouter")
	require.Error(t, err)

	// The error envelope should carry code "conflict" (exit 6).
	require.Contains(t, stderr, `"conflict"`)
}

// TestUpdate_AllShards runs update against each shard the CLI knows about,
// served from a single httptest that answers manifest + shard URLs.
func TestUpdate_AllShards(t *testing.T) {
	zstData, hexSum := buildTestZst(t)

	shards := []string{"openrouter", "aws-ec2", "aws-rds", "aws-s3", "aws-lambda", "aws-ebs", "aws-dynamodb", "aws-cloudfront"}

	// Build a manifest with an entry for every shard under test.
	shardEntries := make([]string, 0, len(shards))
	for _, s := range shards {
		shardEntries = append(shardEntries, `"`+s+`": {
			"baseline_version": "2026.04.22",
			"baseline_url": "SRVURL/`+s+`.db.zst",
			"baseline_sha256": "HEXSUM",
			"baseline_size": 1,
			"head_version": "2026.04.22",
			"min_binary_version": "0.0.1",
			"shard_schema_version": 1,
			"deltas": [],
			"row_count": 1,
			"last_updated": "2026.04.22"
		}`)
	}
	manifest := `{"schema_version":1,"generated_at":"2026-04-22T04:00:00Z","catalog_version":"2026.04.22","shards":{` +
		strings.Join(shardEntries, ",") + `}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/manifest.json":
			body := strings.ReplaceAll(manifest, "HEXSUM", hexSum)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		case strings.HasSuffix(r.URL.Path, ".db.zst.sha256"):
			_, _ = w.Write([]byte(hexSum + "  " + filepath.Base(r.URL.Path) + "\n"))
		case strings.HasSuffix(r.URL.Path, ".db.zst"):
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(zstData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	manifest = strings.ReplaceAll(manifest, "SRVURL", srv.URL)

	for _, shard := range shards {
		t.Run(shard, func(t *testing.T) {
			dataDir := t.TempDir()
			t.Setenv("SKU_DATA_DIR", dataDir)
			t.Setenv("SKU_UPDATE_BASE_URL", srv.URL)

			_, stderr, err := runUpdate(t, shard)
			require.NoError(t, err)
			require.Contains(t, stderr, "installed "+shard)

			fi, statErr := os.Stat(filepath.Join(dataDir, shard+".db"))
			require.NoError(t, statErr)
			require.Greater(t, fi.Size(), int64(0))
		})
	}
}

func TestUpdate_UnsupportedShard(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dataDir)

	_, stderr, err := runUpdate(t, "aws-glacier")
	require.Error(t, err)
	require.Contains(t, stderr, "unsupported_shard")
}

// TestUpdate_HTTPError returns CodeServer when the server replies 502.
func TestUpdate_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dataDir)
	t.Setenv("SKU_UPDATE_BASE_URL", srv.URL)

	_, stderr, err := runUpdate(t, "openrouter")
	require.Error(t, err)
	require.Contains(t, stderr, "502")
}

// TestResolveManifestPrimaryURL verifies that the hardcoded primary manifest
// URL contains "data-latest" and is an HTTPS GitHub releases URL.
// Catches regressions where the URL is accidentally changed to something
// that doesn't follow the data-latest floating tag convention.
func TestResolveManifestPrimaryURL(t *testing.T) {
	// No env override — must return the hardcoded default.
	t.Setenv("SKU_UPDATE_BASE_URL", "")
	url := resolveManifestPrimaryURL()
	if !strings.Contains(url, "data-latest") {
		t.Errorf("primary manifest URL should reference data-latest tag, got: %q", url)
	}
	if !strings.HasPrefix(url, "https://github.com/") {
		t.Errorf("primary manifest URL should be a GitHub HTTPS URL, got: %q", url)
	}
	if !strings.HasSuffix(url, "/manifest.json") {
		t.Errorf("primary manifest URL should end with /manifest.json, got: %q", url)
	}
}

func TestShouldUseManifestUpdateAlwaysTrue(t *testing.T) {
	for _, shard := range []string{"openrouter", "aws-ec2", "azure-postgres", "gcp-gce"} {
		for _, exists := range []bool{true, false} {
			if !shouldUseManifestUpdate(shard, updater.ChannelStable, exists) {
				t.Fatalf("shouldUseManifestUpdate(%s, stable, %v) = false, want true", shard, exists)
			}
		}
	}
}

// TestUpdate_DailyChannel_ManifestParsed verifies that --channel daily fetches
// and parses a real manifest JSON (including last_updated as a catalog-version
// string like "2026.04.22", not RFC3339) and completes the install.
func TestUpdate_DailyChannel_ManifestParsed(t *testing.T) {
	zstData, hexSum := buildTestZst(t)

	const shard = "openrouter"
	baselineURL := "/" + shard + ".db.zst"
	shaURL := "/" + shard + ".db.zst.sha256"

	// Build a manifest that uses the real pipeline's catalog-version string
	// format for last_updated. If the Go struct uses time.Time, this will
	// fail to parse and the test will catch it before any fix is needed.
	manifest := `{
		"schema_version": 1,
		"generated_at": "2026-04-22T04:22:27Z",
		"catalog_version": "2026.04.22",
		"shards": {
			"openrouter": {
				"baseline_version": "2026.04.22",
				"baseline_url": "SRVURL/openrouter.db.zst",
				"baseline_sha256": "HEXSUM",
				"baseline_size": 1,
				"head_version": "2026.04.22",
				"min_binary_version": "1.0.0",
				"shard_schema_version": 1,
				"deltas": [],
				"row_count": 1116,
				"last_updated": "2026.04.22"
			}
		}
	}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest.json":
			body := strings.ReplaceAll(manifest, "HEXSUM", hexSum)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		case baselineURL:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(zstData)
		case shaURL:
			_, _ = w.Write([]byte(hexSum + "  " + shard + ".db.zst\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Replace SRVURL placeholder with the actual test server URL.
	manifest = strings.ReplaceAll(manifest, "SRVURL", srv.URL)

	dataDir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dataDir)
	// SKU_UPDATE_BASE_URL overrides both the shard base URL and the manifest URL.
	t.Setenv("SKU_UPDATE_BASE_URL", srv.URL)

	_, stderr, err := runUpdate(t, shard, "--channel", "daily")
	require.NoError(t, err, "stderr: %s", stderr)
	require.Contains(t, stderr, "installed "+shard)

	fi, statErr := os.Stat(filepath.Join(dataDir, shard+".db"))
	require.NoError(t, statErr)
	require.Greater(t, fi.Size(), int64(0))
}
