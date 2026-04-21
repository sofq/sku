package updater_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/sofq/sku/internal/sqliteutil"
	"github.com/sofq/sku/internal/updater"
)

// buildChainDB creates a SQLite fixture with a metadata table and a rows table
// pre-populated with n rows. Returns the path to the .db file.
func buildChainDB(t *testing.T, dir, version string, n int) string {
	t.Helper()
	path := filepath.Join(dir, "shard.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS metadata (
			catalog_version TEXT NOT NULL,
			generated_at    TEXT NOT NULL
		);
		INSERT INTO metadata VALUES (?, ?);
		CREATE TABLE IF NOT EXISTS rows (id INTEGER PRIMARY KEY, val TEXT);
	`, version, "2026-04-18T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		_, err = db.ExecContext(ctx, "INSERT INTO rows (val) VALUES (?)", fmt.Sprintf("row-%d", i))
		if err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// gzipSQL compresses sql text and returns the bytes.
func gzipSQL(t *testing.T, sql string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write([]byte(sql)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func sha256HexBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// buildDeltaResponse returns a fakeRT body map entry for one delta.
func buildDeltaResponse(t *testing.T, sqlBody string) (body []byte, sha string) {
	t.Helper()
	body = gzipSQL(t, sqlBody)
	sha = sha256HexBytes(body)
	return body, sha
}

func countRows(t *testing.T, dbPath string) int {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM rows").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func getVersion(t *testing.T, dbPath string) string {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var v string
	if err := db.QueryRowContext(context.Background(), "SELECT catalog_version FROM metadata LIMIT 1").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestApplier_HappyPath1Delta(t *testing.T) {
	dir := t.TempDir()
	dbPath := buildChainDB(t, dir, "2026.04.18", 10)

	deltaSQL := "INSERT INTO rows (val) VALUES ('d1'); INSERT INTO rows (val) VALUES ('d2');"
	body, sha := buildDeltaResponse(t, deltaSQL)

	delta := updater.Delta{
		From:   "2026.04.18",
		To:     "2026.04.19",
		URL:    "https://delta.example.com/d1.sql.gz",
		SHA256: sha,
	}

	rt := &fakeRT{body: map[string][]byte{delta.URL: body}}
	applier := &updater.Applier{
		HTTPClient: &http.Client{Transport: rt},
		DBPath:     dbPath,
		MaxChain:   20,
	}

	err := applier.Apply(context.Background(), "2026.04.18", "2026.04.19", []updater.Delta{delta})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if n := countRows(t, dbPath); n != 12 {
		t.Errorf("row count: got %d, want 12", n)
	}
	if v := getVersion(t, dbPath); v != "2026.04.19" {
		t.Errorf("catalog_version: got %q, want 2026.04.19", v)
	}
}

func TestApplier_HappyPath3Deltas(t *testing.T) {
	dir := t.TempDir()
	dbPath := buildChainDB(t, dir, "2026.04.18", 10)

	type deltaSpec struct {
		from, to string
		rowsSQL  string
	}
	specs := []deltaSpec{
		{"2026.04.18", "2026.04.19", "INSERT INTO rows (val) VALUES ('a');"},
		{"2026.04.19", "2026.04.20", "INSERT INTO rows (val) VALUES ('b');"},
		{"2026.04.20", "2026.04.21", "INSERT INTO rows (val) VALUES ('c');"},
	}

	bodyMap := map[string][]byte{}
	var chain []updater.Delta
	for i, s := range specs {
		body, sha := buildDeltaResponse(t, s.rowsSQL)
		url := fmt.Sprintf("https://delta.example.com/d%d.sql.gz", i)
		bodyMap[url] = body
		chain = append(chain, updater.Delta{From: s.from, To: s.to, URL: url, SHA256: sha})
	}

	rt := &fakeRT{body: bodyMap}
	applier := &updater.Applier{
		HTTPClient: &http.Client{Transport: rt},
		DBPath:     dbPath,
		MaxChain:   20,
	}

	err := applier.Apply(context.Background(), "2026.04.18", "2026.04.21", chain)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n := countRows(t, dbPath); n != 13 {
		t.Errorf("row count: got %d, want 13", n)
	}
	if v := getVersion(t, dbPath); v != "2026.04.21" {
		t.Errorf("catalog_version: got %q, want 2026.04.21", v)
	}
}

func TestApplier_ChainTooLong(t *testing.T) {
	dir := t.TempDir()
	dbPath := buildChainDB(t, dir, "2026.04.18", 5)

	var chain []updater.Delta
	for i := 0; i < 3; i++ {
		chain = append(chain, updater.Delta{
			From: fmt.Sprintf("2026.04.%02d", 18+i),
			To:   fmt.Sprintf("2026.04.%02d", 19+i),
			URL:  fmt.Sprintf("https://delta.example.com/d%d.sql.gz", i),
		})
	}

	applier := &updater.Applier{
		HTTPClient: http.DefaultClient,
		DBPath:     dbPath,
		MaxChain:   2,
	}
	err := applier.Apply(context.Background(), "2026.04.18", "2026.04.21", chain)
	if !errors.Is(err, updater.ErrChainTooLong) {
		t.Fatalf("want ErrChainTooLong, got %v", err)
	}
}

func TestApplier_FromMismatch(t *testing.T) {
	dir := t.TempDir()
	dbPath := buildChainDB(t, dir, "2026.04.18", 5)

	chain := []updater.Delta{{
		From: "2026.04.17", // does not match from="2026.04.18"
		To:   "2026.04.18",
		URL:  "https://delta.example.com/d.sql.gz",
	}}

	applier := &updater.Applier{
		HTTPClient: http.DefaultClient,
		DBPath:     dbPath,
		MaxChain:   20,
	}
	err := applier.Apply(context.Background(), "2026.04.18", "2026.04.18", chain)
	if !errors.Is(err, updater.ErrChainStartsElsewhere) {
		t.Fatalf("want ErrChainStartsElsewhere, got %v", err)
	}
}

func TestApplier_SHAMismatchMidChain_Rollback(t *testing.T) {
	dir := t.TempDir()
	dbPath := buildChainDB(t, dir, "2026.04.18", 10)

	body1, sha1 := buildDeltaResponse(t, "INSERT INTO rows (val) VALUES ('d1');")
	body2, _ := buildDeltaResponse(t, "INSERT INTO rows (val) VALUES ('d2');")
	// corrupt body2's SHA so second delta fails
	badSHA2 := strings.Repeat("0", 64)

	chain := []updater.Delta{
		{From: "2026.04.18", To: "2026.04.19", URL: "https://delta.example.com/d1.sql.gz", SHA256: sha1},
		{From: "2026.04.19", To: "2026.04.20", URL: "https://delta.example.com/d2.sql.gz", SHA256: badSHA2},
	}
	rt := &fakeRT{body: map[string][]byte{
		chain[0].URL: body1,
		chain[1].URL: body2,
	}}

	applier := &updater.Applier{
		HTTPClient: &http.Client{Transport: rt},
		DBPath:     dbPath,
		MaxChain:   20,
	}
	err := applier.Apply(context.Background(), "2026.04.18", "2026.04.20", chain)
	if err == nil {
		t.Fatal("want SHA mismatch error, got nil")
	}
	// Row count must be unchanged — rollback worked
	if n := countRows(t, dbPath); n != 10 {
		t.Errorf("row count after rollback: got %d, want 10 (rollback failed)", n)
	}
	if v := getVersion(t, dbPath); v != "2026.04.18" {
		t.Errorf("version after rollback: got %q, want 2026.04.18", v)
	}
}

func TestApplier_NetworkFailureMidChain_Rollback(t *testing.T) {
	dir := t.TempDir()
	dbPath := buildChainDB(t, dir, "2026.04.18", 10)

	body1, sha1 := buildDeltaResponse(t, "INSERT INTO rows (val) VALUES ('d1');")

	chain := []updater.Delta{
		{From: "2026.04.18", To: "2026.04.19", URL: "https://delta.example.com/d1.sql.gz", SHA256: sha1},
		{From: "2026.04.19", To: "2026.04.20", URL: "https://delta.example.com/d2.sql.gz", SHA256: strings.Repeat("0", 64)},
	}
	// Only d1 is in the map; d2 returns 404
	rt := &fakeRT{body: map[string][]byte{chain[0].URL: body1}}

	applier := &updater.Applier{
		HTTPClient: &http.Client{Transport: rt},
		DBPath:     dbPath,
		MaxChain:   20,
	}
	err := applier.Apply(context.Background(), "2026.04.18", "2026.04.20", chain)
	if err == nil {
		t.Fatal("want error for missing second delta")
	}
	if n := countRows(t, dbPath); n != 10 {
		t.Errorf("row count after network failure: got %d, want 10 (rollback failed)", n)
	}
	if v := getVersion(t, dbPath); v != "2026.04.18" {
		t.Errorf("version after rollback: got %q, want 2026.04.18", v)
	}
}

func TestApplier_ConcurrentCallGetsLockConflict(t *testing.T) {
	dir := t.TempDir()
	dbPath := buildChainDB(t, dir, "2026.04.18", 5)

	// Acquire the lock externally to simulate a concurrent Apply.
	unlock, err := sqliteutil.Flock(dbPath)
	if err != nil {
		t.Fatalf("pre-lock: %v", err)
	}
	defer unlock() //nolint:errcheck

	applier := &updater.Applier{
		HTTPClient: http.DefaultClient,
		DBPath:     dbPath,
		MaxChain:   20,
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var applyErr error
	go func() {
		defer wg.Done()
		applyErr = applier.Apply(context.Background(), "2026.04.18", "2026.04.19", []updater.Delta{
			{From: "2026.04.18", To: "2026.04.19", URL: "https://delta.example.com/d.sql.gz"},
		})
	}()
	wg.Wait()

	if !errors.Is(applyErr, updater.ErrLocked) {
		t.Fatalf("want ErrLocked on second concurrent call, got %v", applyErr)
	}
}

func TestApplier_OnProgressCallback(t *testing.T) {
	dir := t.TempDir()
	dbPath := buildChainDB(t, dir, "2026.04.18", 5)

	body, sha := buildDeltaResponse(t, "INSERT INTO rows (val) VALUES ('p1');")
	delta := updater.Delta{From: "2026.04.18", To: "2026.04.19", URL: "https://delta.example.com/d.sql.gz", SHA256: sha}

	rt := &fakeRT{body: map[string][]byte{delta.URL: body}}

	var events []string
	applier := &updater.Applier{
		HTTPClient: &http.Client{Transport: rt},
		DBPath:     dbPath,
		MaxChain:   20,
		OnProgress: func(event string, d updater.Delta) {
			events = append(events, event+":"+d.From+"->"+d.To)
		},
	}

	if err := applier.Apply(context.Background(), "2026.04.18", "2026.04.19", []updater.Delta{delta}); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if len(events) != 1 || events[0] != "delta-applied:2026.04.18->2026.04.19" {
		t.Errorf("progress events: %v", events)
	}
}
