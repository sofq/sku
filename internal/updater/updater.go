// Package updater downloads and installs pricing shards.
//
// M3a.3 scope: extract the one-shot baseline download flow from
// cmd/sku/update.go so it can be unit-tested behind an http.RoundTripper
// fake. Delta-chain + manifest walking + ETag arrive in m3a.4.
package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// ErrSHAMismatch is returned when the downloaded .zst's sha256 does not
// match the published .sha256 file. Callers (the Cobra wrapper) translate
// this to skuerrors.CodeConflict so the exit code stays 6 per spec §4.
var ErrSHAMismatch = errors.New("updater: sha256 mismatch")

// DefaultSources maps shard name → release-asset base URL. Grown in
// lockstep with the spec §3 inventory.
var DefaultSources = map[string]string{
	"openrouter":     "https://github.com/sofq/sku/releases/download/data-bootstrap-openrouter",
	"aws-ec2":        "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-ec2",
	"aws-rds":        "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-rds",
	"aws-s3":         "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-s3",
	"aws-lambda":     "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-lambda",
	"aws-ebs":        "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-ebs",
	"aws-dynamodb":   "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-dynamodb",
	"aws-cloudfront": "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-cloudfront",
	"azure-vm":        "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-vm",
	"azure-sql":       "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-sql",
	"azure-blob":      "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-blob",
	"azure-functions": "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-functions",
	"azure-disks":     "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-disks",
}

// ShardNames returns the keys of DefaultSources. Used by the Cobra
// `update` short-help to list supported shards.
func ShardNames() []string {
	out := make([]string, 0, len(DefaultSources))
	for k := range DefaultSources {
		out = append(out, k)
	}
	return out
}

// Options controls a single Install call.
type Options struct {
	BaseURL    string
	HTTPClient *http.Client
	DestDir    string
}

// Install downloads <BaseURL>/<shard>.db.zst and its .sha256 sibling,
// verifies the hash, decompresses to DestDir/<shard>.db via a .part file,
// and atomically renames on success. On any error the .part file is
// cleaned up and DestDir/<shard>.db is left untouched.
func Install(ctx context.Context, shard string, opts Options) error {
	if opts.BaseURL == "" {
		return errors.New("updater.Install: empty BaseURL")
	}
	if opts.DestDir == "" {
		return errors.New("updater.Install: empty DestDir")
	}
	if opts.HTTPClient == nil {
		opts.HTTPClient = http.DefaultClient
	}
	if err := os.MkdirAll(opts.DestDir, 0o750); err != nil {
		return fmt.Errorf("updater: mkdir %s: %w", opts.DestDir, err)
	}

	base := strings.TrimRight(opts.BaseURL, "/")
	zstURL := base + "/" + shard + ".db.zst"
	shaURL := base + "/" + shard + ".db.zst.sha256"

	zstData, err := httpGet(ctx, opts.HTTPClient, zstURL)
	if err != nil {
		return err
	}
	shaData, err := httpGet(ctx, opts.HTTPClient, shaURL)
	if err != nil {
		return err
	}
	fields := strings.Fields(string(shaData))
	if len(fields) == 0 {
		return fmt.Errorf("updater: sha256 file at %s is empty", shaURL)
	}
	expected := fields[0]
	h := sha256.Sum256(zstData)
	got := hex.EncodeToString(h[:])
	if got != expected {
		return fmt.Errorf("%w for %s: got %s want %s", ErrSHAMismatch, shard, got, expected)
	}

	dbPath := filepath.Join(opts.DestDir, shard+".db")
	dbPart := dbPath + ".part"
	_ = os.Remove(dbPart)
	defer func() { _ = os.Remove(dbPart) }()

	if err := decompressZstd(zstData, dbPart); err != nil {
		return fmt.Errorf("updater: decompress %s: %w", shard, err)
	}
	if err := os.Rename(dbPart, dbPath); err != nil {
		return fmt.Errorf("updater: rename %s -> %s: %w", dbPart, dbPath, err)
	}
	return nil
}

func httpGet(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("updater: GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("updater: GET %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func decompressZstd(zstData []byte, destPath string) error {
	r, err := zstd.NewReader(bytes.NewReader(zstData))
	if err != nil {
		return err
	}
	defer r.Close()
	out, err := os.Create(destPath) //nolint:gosec // destPath derived from caller-provided DestDir
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, r); err != nil {
		_ = os.Remove(destPath)
		return err
	}
	return nil
}
