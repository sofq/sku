// Package updater downloads and installs pricing shards.
//
// M3a.3 scope: extract the one-shot baseline download flow from
// cmd/sku/update.go so it can be unit-tested behind an http.RoundTripper
// fake. M3a.4.3 adds delta-chain + manifest walking + ETag support.
package updater

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"

	_ "modernc.org/sqlite" // register "sqlite" driver (idempotent; also in chain.go)

	skuerrors "github.com/sofq/sku/internal/errors"
)

// ErrSHAMismatch is returned when the downloaded .zst's sha256 does not
// match the published .sha256 file. Callers (the Cobra wrapper) translate
// this to skuerrors.CodeConflict so the exit code stays 6 per spec §4.
var ErrSHAMismatch = errors.New("updater: sha256 mismatch")

// DefaultSources maps shard name → release-asset base URL. Grown in
// lockstep with the spec §3 inventory.
var DefaultSources = map[string]string{
	"openrouter":      "https://github.com/sofq/sku/releases/download/data-bootstrap-openrouter",
	"aws-ec2":         "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-ec2",
	"aws-rds":         "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-rds",
	"aws-s3":          "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-s3",
	"aws-lambda":      "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-lambda",
	"aws-ebs":         "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-ebs",
	"aws-dynamodb":    "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-dynamodb",
	"aws-cloudfront":  "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-cloudfront",
	"aws-aurora":      "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-aurora",
	"aws-elasticache": "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-elasticache",
	"aws-eks":         "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-eks",
	"azure-vm":        "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-vm",
	"azure-sql":       "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-sql",
	"azure-blob":      "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-blob",
	"azure-functions": "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-functions",
	"azure-disks":     "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-disks",
	"azure-postgres":  "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-postgres",
	"azure-mysql":     "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-mysql",
	"azure-mariadb":   "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-mariadb",
	"azure-cosmosdb":  "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-cosmosdb",
	"azure-redis":     "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-redis",
	"azure-aks":       "https://github.com/sofq/sku/releases/download/data-bootstrap-azure-aks",
	"gcp-gce":         "https://github.com/sofq/sku/releases/download/data-bootstrap-gcp-gce",
	"gcp-cloud-sql":   "https://github.com/sofq/sku/releases/download/data-bootstrap-gcp-cloud-sql",
	"gcp-gcs":         "https://github.com/sofq/sku/releases/download/data-bootstrap-gcp-gcs",
	"gcp-run":         "https://github.com/sofq/sku/releases/download/data-bootstrap-gcp-run",
	"gcp-functions":   "https://github.com/sofq/sku/releases/download/data-bootstrap-gcp-functions",
	"gcp-spanner":     "https://github.com/sofq/sku/releases/download/data-bootstrap-gcp-spanner",
	"gcp-memorystore": "https://github.com/sofq/sku/releases/download/data-bootstrap-gcp-memorystore",
	"gcp-gke":         "https://github.com/sofq/sku/releases/download/data-bootstrap-gcp-gke",
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

// MinSupportedShardSchema is the minimum shard schema version this binary
// understands. Shards with a lower value are rejected as shard_too_old.
const MinSupportedShardSchema = 1

// MaxSupportedShardSchema is the maximum shard schema version this binary
// understands. Shards with a higher value are rejected as shard_too_new.
const MaxSupportedShardSchema = 1

// UpdateOptions controls a single Update call.
type UpdateOptions struct {
	// Options embeds the base Install options (DestDir, HTTPClient, BaseURL).
	Options
	// Channel controls whether to walk deltas or always use the baseline.
	Channel Channel
	// Manifest is the source for the manifest.json.
	Manifest ManifestSource
	// ETag is the cached ETag from the previous manifest fetch.
	// A 304 response causes Update to return a no-op Result.
	ETag string
	// MaxChain limits the number of deltas applied in one Update.
	// Zero uses the default (20).
	MaxChain int
}

// Result describes the outcome of a single Update call.
type Result struct {
	// From is the catalog_version the shard was at before Update ran.
	// Empty when the shard file did not exist before (fresh install).
	From string
	// To is the catalog_version the shard is at after Update ran.
	To string
	// Applied contains the deltas that were successfully applied.
	// Empty when the update used a baseline download or was a no-op.
	Applied []Delta
	// Baseline is true when the shard was installed from the full baseline.
	Baseline bool
	// FellBackToBaseline is true when the daily delta chain was attempted but
	// fell back to baseline due to ErrChainTooLong or ErrChainStartsElsewhere.
	FellBackToBaseline bool
	// NewETag is the ETag returned by the manifest server for future caching.
	NewETag string
}

// Update either applies a delta chain or re-downloads the full baseline for
// shard, depending on opts.Channel, the local shard state, and the manifest.
//
// Flow:
//  1. Read local catalog_version from <DestDir>/<shard>.db metadata table.
//     If the file does not exist, treat it as "need baseline".
//  2. Fetch the manifest. On 304, return a noop Result (nothing to do).
//  3. Validate shard schema version — reject with CodeValidation on mismatch.
//  4. If channel==Stable OR local is missing OR local < entry.BaselineVersion:
//     call Install with the baseline URL.
//  5. Else (Daily): filter deltas with d.From >= local; if empty → noop;
//     else call Applier.Apply. On ErrChainTooLong / ErrChainStartsElsewhere /
//     ErrLocked (actually just first two — locks propagate), fall back to Install.
//  6. Return Result.
func Update(ctx context.Context, shard string, opts UpdateOptions) (Result, error) {
	dbPath := filepath.Join(opts.DestDir, shard+".db")

	// Step 1: read local version.
	localVersion, err := readLocalVersion(dbPath)
	if err != nil {
		return Result{}, fmt.Errorf("updater: read local version: %w", err)
	}
	hasShard := localVersion != ""

	// Step 2: fetch manifest.
	m, newETag, notModified, err := opts.Manifest.Fetch(ctx, opts.ETag)
	if err != nil {
		return Result{}, fmt.Errorf("updater: fetch manifest: %w", err)
	}
	if notModified {
		return Result{From: localVersion, To: localVersion, NewETag: ""}, nil
	}

	// Step 3: validate shard entry.
	entry, ok := m.Shards[shard]
	if !ok {
		return Result{}, fmt.Errorf("updater: shard %q not found in manifest", shard)
	}
	if entry.ShardSchemaVersion > MaxSupportedShardSchema {
		return Result{}, skuerrors.Validation(
			"shard_too_new", "shard", shard,
			fmt.Sprintf("shard schema version %d exceeds max supported %d; upgrade sku binary",
				entry.ShardSchemaVersion, MaxSupportedShardSchema),
		)
	}
	if entry.ShardSchemaVersion < MinSupportedShardSchema {
		return Result{}, skuerrors.Validation(
			"shard_too_old", "shard", shard,
			fmt.Sprintf("shard schema version %d below min supported %d",
				entry.ShardSchemaVersion, MinSupportedShardSchema),
		)
	}

	// Step 4: decide whether to use a baseline or delta chain.
	useBaseline := opts.Channel == ChannelStable ||
		!hasShard ||
		localVersion < entry.BaselineVersion

	if useBaseline {
		return installBaseline(ctx, shard, localVersion, entry, opts, newETag)
	}

	// Step 5: daily channel — try delta chain.
	var chain []Delta
	for _, d := range entry.Deltas {
		if d.From >= localVersion {
			chain = append(chain, d)
		}
	}
	if len(chain) == 0 {
		// Already up to date.
		return Result{From: localVersion, To: localVersion, NewETag: newETag}, nil
	}

	maxChain := opts.MaxChain
	if maxChain == 0 {
		maxChain = 20
	}
	applier := &Applier{
		HTTPClient: opts.HTTPClient,
		DBPath:     dbPath,
		MaxChain:   maxChain,
	}

	var applied []Delta
	applier.OnProgress = func(_ string, d Delta) {
		applied = append(applied, d)
	}

	applyErr := applier.Apply(ctx, localVersion, entry.HeadVersion, chain)
	if applyErr == nil {
		return Result{
			From:    localVersion,
			To:      entry.HeadVersion,
			Applied: applied,
			NewETag: newETag,
		}, nil
	}

	// Fall back to baseline on recoverable chain errors.
	if errors.Is(applyErr, ErrChainTooLong) || errors.Is(applyErr, ErrChainStartsElsewhere) {
		r, err := installBaseline(ctx, shard, localVersion, entry, opts, newETag)
		r.FellBackToBaseline = true
		return r, err
	}

	return Result{}, applyErr
}

// installBaseline downloads the full baseline for shard and returns a Result.
func installBaseline(ctx context.Context, shard, localVersion string, entry ShardEntry, opts UpdateOptions, newETag string) (Result, error) {
	// Build an Install Options from the entry.
	baseURL := trimToDir(entry.BaselineURL)
	installOpts := Options{
		BaseURL:    baseURL,
		HTTPClient: opts.HTTPClient,
		DestDir:    opts.DestDir,
	}
	if err := Install(ctx, shard, installOpts); err != nil {
		return Result{}, err
	}
	return Result{
		From:     localVersion,
		To:       entry.HeadVersion,
		Baseline: true,
		NewETag:  newETag,
	}, nil
}

// trimToDir strips the filename component from a URL to produce a base URL
// suitable for Install (which appends /<shard>.db.zst).
// e.g. "https://example.com/data/aws-ec2.db.zst" → "https://example.com/data"
func trimToDir(url string) string {
	if i := strings.LastIndex(url, "/"); i > 0 {
		return url[:i]
	}
	return url
}

// readLocalVersion reads the catalog_version from the metadata table of a
// SQLite shard. Returns ("", nil) if the file does not exist.
func readLocalVersion(dbPath string) (string, error) {
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		return "", nil
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = db.Close() }()

	var v string
	err = db.QueryRowContext(context.Background(),
		"SELECT value FROM metadata WHERE key = 'catalog_version' LIMIT 1").Scan(&v)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return v, nil
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
