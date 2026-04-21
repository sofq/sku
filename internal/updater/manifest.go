package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Manifest is the top-level structure of the daily-published manifest.json.
// It describes all available shards, their current baseline, and any available
// delta files that allow incremental updates.
type Manifest struct {
	SchemaVersion  int                   `json:"schema_version"`
	GeneratedAt    time.Time             `json:"generated_at"`
	CatalogVersion string                `json:"catalog_version"`
	Shards         map[string]ShardEntry `json:"shards"`
}

// ShardEntry describes a single shard's current state in the manifest.
type ShardEntry struct {
	BaselineVersion    string    `json:"baseline_version"`
	BaselineURL        string    `json:"baseline_url"`
	BaselineSHA256     string    `json:"baseline_sha256"`
	BaselineSize       int64     `json:"baseline_size"`
	HeadVersion        string    `json:"head_version"`
	MinBinaryVersion   string    `json:"min_binary_version"`
	ShardSchemaVersion int       `json:"shard_schema_version"`
	Deltas             []Delta   `json:"deltas"`
	RowCount           int64     `json:"row_count"`
	LastUpdated        time.Time `json:"last_updated"`
}

// Delta describes one incremental SQL patch between two catalog versions.
type Delta struct {
	From   string `json:"from"`
	To     string `json:"to"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// ManifestSource is the interface for fetching the manifest. The ETag string
// is passed as If-None-Match so the source can return notModified=true when
// the manifest has not changed since the last fetch.
type ManifestSource interface {
	// Fetch retrieves the manifest. If the server returns 304, notModified is
	// true and m is nil. On success, newETag holds the server's ETag header
	// value (empty if the server did not send one).
	Fetch(ctx context.Context, etag string) (m *Manifest, newETag string, notModified bool, err error)
}

// httpSource implements ManifestSource over HTTP with a primary/fallback URL
// pair. A 5xx response on the primary causes a single retry on the fallback.
type httpSource struct {
	primary  string
	fallback string
	client   *http.Client
}

// NewHTTPSource returns a ManifestSource that GETs from primary, falling back
// to fallback on 5xx. rt may be nil (uses http.DefaultTransport).
func NewHTTPSource(primary, fallback string, rt http.RoundTripper) ManifestSource {
	c := &http.Client{}
	if rt != nil {
		c.Transport = rt
	}
	return &httpSource{
		primary:  strings.TrimRight(primary, "/"),
		fallback: strings.TrimRight(fallback, "/"),
		client:   c,
	}
}

// Fetch implements ManifestSource.
func (s *httpSource) Fetch(ctx context.Context, etag string) (*Manifest, string, bool, error) {
	m, newETag, notModified, primaryErr := s.fetchURL(ctx, s.primary, etag)
	if primaryErr == nil {
		return m, newETag, notModified, nil
	}

	// Fall back to secondary URL.
	m2, newETag2, notModified2, fallbackErr := s.fetchURL(ctx, s.fallback, etag)
	if fallbackErr == nil {
		return m2, newETag2, notModified2, nil
	}

	return nil, "", false, fmt.Errorf("updater: manifest fetch failed — primary (%s): %w; fallback (%s): %v",
		s.primary, primaryErr, s.fallback, fallbackErr)
}

func (s *httpSource) fetchURL(ctx context.Context, url, etag string) (*Manifest, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, "", false, fmt.Errorf("updater: build request for %s: %w", url, err)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", false, fmt.Errorf("updater: GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotModified {
		return nil, "", true, nil
	}

	if resp.StatusCode >= 500 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, "", false, fmt.Errorf("updater: GET %s: HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if resp.StatusCode >= 400 {
		return nil, "", false, fmt.Errorf("updater: GET %s: HTTP %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", false, fmt.Errorf("updater: read body from %s: %w", url, err)
	}

	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, "", false, fmt.Errorf("updater: parse manifest from %s: %w", url, err)
	}

	newETag := resp.Header.Get("ETag")
	return &m, newETag, false, nil
}
