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

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
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
	zstData, _ := buildTestZst(t)

	const wrongHex = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/openrouter.db.zst":
			_, _ = w.Write(zstData)
		case "/openrouter.db.zst.sha256":
			// wrong digest
			_, _ = w.Write([]byte(wrongHex + "  openrouter.db.zst\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dataDir := t.TempDir()
	t.Setenv("SKU_DATA_DIR", dataDir)
	t.Setenv("SKU_UPDATE_BASE_URL", srv.URL)

	_, stderr, err := runUpdate(t, "openrouter")
	require.Error(t, err)

	// The error envelope should carry code "conflict" (exit 6).
	require.Contains(t, stderr, `"conflict"`)
}

// TestUpdate_AllShards runs update against each shard the CLI knows about,
// served from a single httptest that answers every shard's URLs.
func TestUpdate_AllShards(t *testing.T) {
	zstData, hexSum := buildTestZst(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ".db.zst.sha256"):
			_, _ = w.Write([]byte(hexSum + "  " + filepath.Base(r.URL.Path) + "\n")) //nolint:gosec // G705: test server, content is controlled
		case strings.HasSuffix(r.URL.Path, ".db.zst"):
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(zstData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	for _, shard := range []string{"openrouter", "aws-ec2", "aws-rds", "aws-s3", "aws-lambda", "aws-ebs"} {
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

	_, stderr, err := runUpdate(t, "aws-dynamodb")
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
