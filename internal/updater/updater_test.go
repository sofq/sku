package updater_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"

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
		"aws-dynamodb", "aws-cloudfront",
		"azure-vm", "azure-sql",
	} {
		if _, ok := updater.DefaultSources[shard]; !ok {
			t.Errorf("DefaultSources missing shard %q", shard)
		}
	}
}
