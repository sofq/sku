package updater_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sofq/sku/internal/updater"
)

// roundTripperFunc wraps a function as an http.RoundTripper.
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func loadManifestFixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "manifest.v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestHTTPSource_200WithETag(t *testing.T) {
	fixture := loadManifestFixture(t)
	const serverETag = `"abc123"`

	rt := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		h := make(http.Header)
		h.Set("ETag", serverETag)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(string(fixture))),
			Header:     h,
		}, nil
	})

	src := updater.NewHTTPSource(
		"https://primary.example.com/manifest.json",
		"https://fallback.example.com/manifest.json",
		rt,
	)

	m, newETag, notModified, err := src.Fetch(context.Background(), "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if notModified {
		t.Fatal("want notModified=false on 200")
	}
	if newETag != serverETag {
		t.Errorf("ETag: got %q, want %q", newETag, serverETag)
	}
	if m == nil {
		t.Fatal("want non-nil Manifest")
	}
	if m.SchemaVersion != 1 {
		t.Errorf("SchemaVersion: got %d, want 1", m.SchemaVersion)
	}
	if len(m.Shards) != 3 {
		t.Errorf("Shards count: got %d, want 3", len(m.Shards))
	}
	ec2 := m.Shards["aws-ec2"]
	if len(ec2.Deltas) != 2 {
		t.Errorf("aws-ec2 delta count: got %d, want 2", len(ec2.Deltas))
	}
	openrouter := m.Shards["openrouter"]
	if len(openrouter.Deltas) != 0 {
		t.Errorf("openrouter delta count: got %d, want 0", len(openrouter.Deltas))
	}
	azureVM := m.Shards["azure-vm"]
	if len(azureVM.Deltas) != 1 {
		t.Errorf("azure-vm delta count: got %d, want 1", len(azureVM.Deltas))
	}
}

func TestHTTPSource_304NotModified(t *testing.T) {
	const cachedETag = `"stale-etag"`
	var capturedIfNoneMatch string

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		capturedIfNoneMatch = req.Header.Get("If-None-Match")
		return &http.Response{
			StatusCode: 304,
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}, nil
	})

	src := updater.NewHTTPSource(
		"https://primary.example.com/manifest.json",
		"https://fallback.example.com/manifest.json",
		rt,
	)

	m, newETag, notModified, err := src.Fetch(context.Background(), cachedETag)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !notModified {
		t.Fatal("want notModified=true on 304")
	}
	if m != nil {
		t.Fatal("want nil Manifest on 304")
	}
	if newETag != "" {
		t.Errorf("want empty newETag on 304, got %q", newETag)
	}
	if capturedIfNoneMatch != cachedETag {
		t.Errorf("If-None-Match: got %q, want %q", capturedIfNoneMatch, cachedETag)
	}
}

func TestHTTPSource_500PrimaryFallsBackToSecondary(t *testing.T) {
	fixture := loadManifestFixture(t)
	var callCount int

	rt := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		callCount++
		if strings.Contains(req.URL.Host, "primary") {
			return &http.Response{
				StatusCode: 500,
				Body:       io.NopCloser(strings.NewReader("internal server error")),
				Header:     make(http.Header),
			}, nil
		}
		// fallback
		h := make(http.Header)
		h.Set("ETag", `"fallback-etag"`)
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(string(fixture))),
			Header:     h,
		}, nil
	})

	src := updater.NewHTTPSource(
		"https://primary.example.com/manifest.json",
		"https://fallback.example.com/manifest.json",
		rt,
	)

	m, _, notModified, err := src.Fetch(context.Background(), "")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if notModified {
		t.Fatal("want notModified=false")
	}
	if m == nil {
		t.Fatal("want non-nil Manifest from fallback")
	}
	if callCount != 2 {
		t.Errorf("expected 2 HTTP calls (primary + fallback), got %d", callCount)
	}
}

func TestHTTPSource_BothFail_ErrorContainsBothURLs(t *testing.T) {
	primaryURL := "https://primary.example.com/manifest.json"
	fallbackURL := "https://fallback.example.com/manifest.json"

	rt := roundTripperFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 503,
			Body:       io.NopCloser(strings.NewReader("unavailable")),
			Header:     make(http.Header),
		}, nil
	})

	src := updater.NewHTTPSource(primaryURL, fallbackURL, rt)
	_, _, _, err := src.Fetch(context.Background(), "")
	if err == nil {
		t.Fatal("want error when both URLs fail")
	}
	msg := err.Error()
	if !strings.Contains(msg, "primary.example.com") {
		t.Errorf("error missing primary URL; got: %s", msg)
	}
	if !strings.Contains(msg, "fallback.example.com") {
		t.Errorf("error missing fallback URL; got: %s", msg)
	}
}

func TestManifestStructure(t *testing.T) {
	fixture := loadManifestFixture(t)
	var m updater.Manifest
	if err := json.Unmarshal(fixture, &m); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	wantGenerated := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC)
	if !m.GeneratedAt.Equal(wantGenerated) {
		t.Errorf("GeneratedAt: got %v, want %v", m.GeneratedAt, wantGenerated)
	}
	if m.CatalogVersion != "2026.04.20" {
		t.Errorf("CatalogVersion: got %q", m.CatalogVersion)
	}
}
