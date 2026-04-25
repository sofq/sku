package updater_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"

	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/updater"
)

// fakeRT routes GETs to a test-supplied map.
type fakeRT struct {
	body   map[string][]byte
	status map[string]int
}

func (f *fakeRT) RoundTrip(req *http.Request) (*http.Response, error) {
	b, ok := f.body[req.URL.String()]
	if !ok {
		return &http.Response{
			StatusCode: 404,
			Body:       io.NopCloser(bytes.NewReader(nil)),
			Header:     make(http.Header),
		}, nil
	}
	st := 200
	if s := f.status[req.URL.String()]; s != 0 {
		st = s
	}
	return &http.Response{
		StatusCode: st,
		Body:       io.NopCloser(bytes.NewReader(b)),
		Header:     make(http.Header),
	}, nil
}

func zstdCompress(t *testing.T, in []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(in); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256Hex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestInstall_HappyPath(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("hello sqlite")
	zstBytes := zstdCompress(t, payload)

	rt := &fakeRT{body: map[string][]byte{
		"https://example/aws-dynamodb.db.zst":        zstBytes,
		"https://example/aws-dynamodb.db.zst.sha256": []byte(sha256Hex(zstBytes) + "  aws-dynamodb.db.zst\n"),
	}}
	err := updater.Install(context.Background(), "aws-dynamodb", updater.Options{
		BaseURL:    "https://example",
		HTTPClient: &http.Client{Transport: rt},
		DestDir:    dir,
	})
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "aws-dynamodb.db")) //nolint:gosec // G304: test helper reads from t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %q want %q", got, payload)
	}
}

func TestInstall_SHAMismatchAborts(t *testing.T) {
	dir := t.TempDir()
	zstBytes := zstdCompress(t, []byte("payload"))

	rt := &fakeRT{body: map[string][]byte{
		"https://example/aws-cloudfront.db.zst":        zstBytes,
		"https://example/aws-cloudfront.db.zst.sha256": []byte(strings.Repeat("0", 64) + "  bad\n"),
	}}
	err := updater.Install(context.Background(), "aws-cloudfront", updater.Options{
		BaseURL:    "https://example",
		HTTPClient: &http.Client{Transport: rt},
		DestDir:    dir,
	})
	if err == nil {
		t.Fatal("want sha mismatch error, got nil")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "aws-cloudfront.db")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("dest file must not exist after sha mismatch")
	}
}

func TestInstall_HTTP5xx(t *testing.T) {
	dir := t.TempDir()
	rt := &fakeRT{
		body:   map[string][]byte{"https://example/aws-dynamodb.db.zst": {}},
		status: map[string]int{"https://example/aws-dynamodb.db.zst": 503},
	}
	err := updater.Install(context.Background(), "aws-dynamodb", updater.Options{
		BaseURL:    "https://example",
		HTTPClient: &http.Client{Transport: rt},
		DestDir:    dir,
	})
	if err == nil {
		t.Fatal("want HTTP 503 error")
	}
}

func TestInstall_DecompressError(t *testing.T) {
	dir := t.TempDir()
	garbage := []byte("not zstd")
	rt := &fakeRT{body: map[string][]byte{
		"https://example/aws-dynamodb.db.zst":        garbage,
		"https://example/aws-dynamodb.db.zst.sha256": []byte(sha256Hex(garbage) + "  x\n"),
	}}
	err := updater.Install(context.Background(), "aws-dynamodb", updater.Options{
		BaseURL:    "https://example",
		HTTPClient: &http.Client{Transport: rt},
		DestDir:    dir,
	})
	if err == nil {
		t.Fatal("want decompress error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "aws-dynamodb.db")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("dest file must not exist after decompress error")
	}
}

func TestDefaultSources_Contains(t *testing.T) {
	for _, shard := range []string{
		"openrouter", "aws-ec2", "aws-rds", "aws-s3", "aws-lambda", "aws-ebs",
		"aws-dynamodb", "aws-cloudfront", "aws-aurora", "aws-elasticache",
		"azure-vm", "azure-sql",
		"azure-blob", "azure-functions", "azure-disks",
		"azure-cosmosdb", "azure-redis",
		"gcp-gce", "gcp-cloud-sql",
		"gcp-gcs", "gcp-run", "gcp-functions", "gcp-spanner", "gcp-memorystore",
	} {
		if _, ok := updater.DefaultSources[shard]; !ok {
			t.Errorf("DefaultSources missing shard %q", shard)
		}
	}
}

// ---- Update() tests ----

// fakeManifestSource is a ManifestSource backed by a static manifest.
type fakeManifestSource struct {
	m           *updater.Manifest
	returnETag  string
	notModified bool
	err         error
}

func (f *fakeManifestSource) Fetch(_ context.Context, _ string) (*updater.Manifest, string, bool, error) {
	return f.m, f.returnETag, f.notModified, f.err
}

// seedShardDB creates a minimal SQLite shard with the given catalog_version.
func seedShardDB(t *testing.T, path, version string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	_, err = db.ExecContext(context.Background(), `
		CREATE TABLE metadata (key TEXT PRIMARY KEY, value TEXT);
		INSERT INTO metadata(key, value) VALUES ('catalog_version', ?);
		INSERT INTO metadata(key, value) VALUES ('generated_at',    ?);
		CREATE TABLE rows (id INTEGER PRIMARY KEY, val TEXT);
	`, version, "2026-04-18T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
}

// gzipBytes compresses b with gzip.
func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(b); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// buildFakeManifest returns a Manifest with the given shard entry.
func buildFakeManifest(shard string, entry updater.ShardEntry) *updater.Manifest {
	return &updater.Manifest{
		SchemaVersion:  1,
		GeneratedAt:    time.Now(),
		CatalogVersion: entry.HeadVersion,
		Shards:         map[string]updater.ShardEntry{shard: entry},
	}
}

// makeZstdShard builds a zstd-compressed minimal SQLite database with
// catalog_version set to version for use as a fake baseline download.
func makeZstdShard(t *testing.T, version string) []byte {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "shard.db")
	seedShardDB(t, dbPath, version)
	raw, err := os.ReadFile(dbPath) //nolint:gosec // G304: test helper reads from t.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	// Compress with zstd using klauspost/compress.
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUpdate_StableChannelAlwaysBaseline(t *testing.T) {
	dir := t.TempDir()
	const shard = "aws-ec2"

	// Shard exists locally at an older version.
	dbPath := filepath.Join(dir, shard+".db")
	seedShardDB(t, dbPath, "2026.04.18")

	zstBytes := makeZstdShard(t, "2026.04.20")
	shaHex := sha256Hex(zstBytes)
	shaFile := shaHex + "  " + shard + ".db.zst\n"

	baselineURL := "https://baseline.example.com/data-2026.04.20/" + shard + ".db.zst"
	shaURL := "https://baseline.example.com/data-2026.04.20/" + shard + ".db.zst.sha256"

	rt := &fakeRT{body: map[string][]byte{
		baselineURL: zstBytes,
		shaURL:      []byte(shaFile),
	}}

	entry := updater.ShardEntry{
		BaselineVersion:    "2026.04.18",
		BaselineURL:        "https://baseline.example.com/data-2026.04.20/" + shard + ".db.zst",
		BaselineSHA256:     shaHex,
		HeadVersion:        "2026.04.20",
		ShardSchemaVersion: 1,
	}
	m := buildFakeManifest(shard, entry)

	opts := updater.UpdateOptions{
		Options:  updater.Options{DestDir: dir, HTTPClient: &http.Client{Transport: rt}},
		Channel:  updater.ChannelStable,
		Manifest: &fakeManifestSource{m: m, returnETag: `"new-etag"`},
	}

	result, err := updater.Update(context.Background(), shard, opts)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.Baseline {
		t.Error("want Baseline=true for stable channel")
	}
	if result.To != "2026.04.20" {
		t.Errorf("To: got %q, want 2026.04.20", result.To)
	}
}

func TestUpdate_NotModified304_Noop(t *testing.T) {
	dir := t.TempDir()
	const shard = "aws-ec2"

	dbPath := filepath.Join(dir, shard+".db")
	seedShardDB(t, dbPath, "2026.04.20")

	opts := updater.UpdateOptions{
		Options:  updater.Options{DestDir: dir},
		Channel:  updater.ChannelDaily,
		Manifest: &fakeManifestSource{notModified: true},
	}

	result, err := updater.Update(context.Background(), shard, opts)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if result.From != "2026.04.20" {
		t.Errorf("From: got %q, want 2026.04.20", result.From)
	}
	if result.To != "2026.04.20" {
		t.Errorf("To: got %q, want 2026.04.20", result.To)
	}
	if result.Baseline {
		t.Error("want Baseline=false on noop")
	}
}

func TestUpdate_DailyChannelAppliesDeltas(t *testing.T) {
	dir := t.TempDir()
	const shard = "aws-ec2"

	dbPath := filepath.Join(dir, shard+".db")
	seedShardDB(t, dbPath, "2026.04.18")

	// Build a delta that adds 2 rows.
	deltaSQL := "INSERT INTO rows (val) VALUES ('upd1'); INSERT INTO rows (val) VALUES ('upd2');"
	deltaBody := gzipBytes(t, []byte(deltaSQL))
	deltaSHA := sha256Hex(deltaBody)
	deltaURL := "https://delta.example.com/aws-ec2-d1.sql.gz"

	entry := updater.ShardEntry{
		BaselineVersion:    "2026.04.18",
		HeadVersion:        "2026.04.19",
		ShardSchemaVersion: 1,
		Deltas: []updater.Delta{
			{From: "2026.04.18", To: "2026.04.19", URL: deltaURL, SHA256: deltaSHA},
		},
	}
	m := buildFakeManifest(shard, entry)

	rt := &fakeRT{body: map[string][]byte{deltaURL: deltaBody}}
	opts := updater.UpdateOptions{
		Options:  updater.Options{DestDir: dir, HTTPClient: &http.Client{Transport: rt}},
		Channel:  updater.ChannelDaily,
		MaxChain: 20,
		Manifest: &fakeManifestSource{m: m, returnETag: `"etag2"`},
	}

	result, err := updater.Update(context.Background(), shard, opts)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if result.Baseline {
		t.Error("want Baseline=false on delta apply")
	}
	if len(result.Applied) != 1 {
		t.Errorf("Applied count: got %d, want 1", len(result.Applied))
	}
	if result.To != "2026.04.19" {
		t.Errorf("To: got %q, want 2026.04.19", result.To)
	}
}

func TestUpdate_DailyChainTooLong_FallsBackToBaseline(t *testing.T) {
	dir := t.TempDir()
	const shard = "aws-ec2"

	dbPath := filepath.Join(dir, shard+".db")
	seedShardDB(t, dbPath, "2026.04.18")

	// 3 deltas but MaxChain=2 → fallback to baseline.
	entry := updater.ShardEntry{
		BaselineVersion:    "2026.04.18",
		BaselineURL:        "https://baseline.example.com/data/" + shard + ".db.zst",
		HeadVersion:        "2026.04.21",
		ShardSchemaVersion: 1,
		Deltas: []updater.Delta{
			{From: "2026.04.18", To: "2026.04.19"},
			{From: "2026.04.19", To: "2026.04.20"},
			{From: "2026.04.20", To: "2026.04.21"},
		},
	}

	zstBytes := makeZstdShard(t, "2026.04.21")
	shaHex := sha256Hex(zstBytes)
	entry.BaselineSHA256 = shaHex
	shaURL := entry.BaselineURL + ".sha256"
	rt := &fakeRT{body: map[string][]byte{
		entry.BaselineURL: zstBytes,
		shaURL:            []byte(shaHex + "  x\n"),
	}}

	m := buildFakeManifest(shard, entry)
	opts := updater.UpdateOptions{
		Options:  updater.Options{DestDir: dir, HTTPClient: &http.Client{Transport: rt}},
		Channel:  updater.ChannelDaily,
		MaxChain: 2,
		Manifest: &fakeManifestSource{m: m},
	}

	result, err := updater.Update(context.Background(), shard, opts)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.Baseline {
		t.Error("want Baseline=true after chain-too-long fallback")
	}
	if !result.FellBackToBaseline {
		t.Error("want FellBackToBaseline=true after chain-too-long fallback")
	}
}

func TestUpdate_ShardSchemaVersionTooNew(t *testing.T) {
	dir := t.TempDir()
	const shard = "aws-ec2"

	dbPath := filepath.Join(dir, shard+".db")
	seedShardDB(t, dbPath, "2026.04.18")

	entry := updater.ShardEntry{
		HeadVersion:        "2026.04.20",
		ShardSchemaVersion: 99, // way above MaxSupportedShardSchema=1
	}
	m := buildFakeManifest(shard, entry)
	opts := updater.UpdateOptions{
		Options:  updater.Options{DestDir: dir},
		Channel:  updater.ChannelDaily,
		Manifest: &fakeManifestSource{m: m},
	}

	_, err := updater.Update(context.Background(), shard, opts)
	if err == nil {
		t.Fatal("want error for shard_too_new, got nil")
	}
	var e *skuerrors.E
	if !errors.As(err, &e) {
		t.Fatalf("want *skuerrors.E, got %T: %v", err, err)
	}
	if e.Code != skuerrors.CodeValidation {
		t.Errorf("want CodeValidation, got %v", e.Code)
	}
	if reason, _ := e.Details["reason"].(string); reason != "shard_too_new" {
		t.Errorf("want reason=shard_too_new, got %q", reason)
	}
}

func TestUpdate_NoLocalShard_InstallsBaseline(t *testing.T) {
	dir := t.TempDir()
	const shard = "openrouter"
	// No .db file exists — should trigger a baseline install.

	zstBytes := makeZstdShard(t, "2026.04.20")
	shaHex := sha256Hex(zstBytes)
	baselineURL := "https://baseline.example.com/data/" + shard + ".db.zst"
	shaURL := baselineURL + ".sha256"

	entry := updater.ShardEntry{
		BaselineVersion:    "2026.04.20",
		BaselineURL:        baselineURL,
		BaselineSHA256:     shaHex,
		HeadVersion:        "2026.04.20",
		ShardSchemaVersion: 1,
	}
	m := buildFakeManifest(shard, entry)

	rt := &fakeRT{body: map[string][]byte{
		baselineURL: zstBytes,
		shaURL:      []byte(shaHex + "  x\n"),
	}}

	opts := updater.UpdateOptions{
		Options:  updater.Options{DestDir: dir, HTTPClient: &http.Client{Transport: rt}},
		Channel:  updater.ChannelDaily,
		Manifest: &fakeManifestSource{m: m},
	}

	result, err := updater.Update(context.Background(), shard, opts)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !result.Baseline {
		t.Error("want Baseline=true for fresh install")
	}

	// File should exist now.
	if _, statErr := os.Stat(filepath.Join(dir, shard+".db")); statErr != nil {
		t.Errorf("expected shard .db to exist after install: %v", statErr)
	}
}

// Ensure json/time imports are used (via buildFakeManifest + time.Now).
var _ = json.Marshal
var _ = time.Now
