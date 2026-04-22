package updater

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"

	_ "modernc.org/sqlite" // register "sqlite" driver

	"github.com/sofq/sku/internal/sqliteutil"
)

// ErrChainStartsElsewhere is returned when the first delta in the chain has a
// From version that does not match the provided from argument. The caller
// should fall back to a full baseline download.
var ErrChainStartsElsewhere = errors.New("updater: delta chain starts at a different version than the local shard")

// ErrChainTooLong is returned when len(chain) > Applier.MaxChain. The caller
// should fall back to a full baseline download.
var ErrChainTooLong = errors.New("updater: delta chain exceeds MaxChain limit")

// ErrLocked is re-exported from sqliteutil so callers only need to import
// this package. It signals that another process holds the shard advisory lock.
// Callers map this to skuerrors.CodeConflict (exit 6) per spec §4.
var ErrLocked = sqliteutil.ErrLocked

// Applier applies a delta chain to an existing SQLite shard file in a single
// all-or-nothing transaction. It takes an advisory flock on the shard before
// opening SQLite to prevent concurrent modification.
type Applier struct {
	// HTTPClient is the client used to fetch delta files. If nil, http.DefaultClient is used.
	HTTPClient *http.Client
	// DBPath is the absolute path to the shard .db file.
	DBPath string
	// MaxChain is the maximum number of deltas allowed in one Apply call.
	// Defaults to 20 if zero.
	MaxChain int
	// OnProgress is called after each delta is successfully applied.
	// It may be nil.
	OnProgress func(event string, delta Delta)
}

func (a *Applier) maxChain() int {
	if a.MaxChain > 0 {
		return a.MaxChain
	}
	return 20
}

func (a *Applier) client() *http.Client {
	if a.HTTPClient != nil {
		return a.HTTPClient
	}
	return http.DefaultClient
}

// Apply applies chain to the shard at a.DBPath. All deltas are applied in a
// single BEGIN IMMEDIATE transaction; any failure causes a ROLLBACK so the
// database is left unchanged.
//
// Pre-conditions checked before opening the database:
//   - len(chain) == 0 || from != chain[0].From → ErrChainStartsElsewhere
//   - len(chain) > MaxChain → ErrChainTooLong
func (a *Applier) Apply(ctx context.Context, from, to string, chain []Delta) error {
	if len(chain) == 0 || from != chain[0].From {
		return ErrChainStartsElsewhere
	}
	if len(chain) > a.maxChain() {
		return ErrChainTooLong
	}

	// Take advisory flock before opening SQLite.
	unlock, err := sqliteutil.Flock(a.DBPath)
	if err != nil {
		return err // ErrLocked or I/O error
	}
	defer func() { _ = unlock() }()

	db, err := sql.Open("sqlite", a.DBPath)
	if err != nil {
		return fmt.Errorf("updater: open shard %s: %w", a.DBPath, err)
	}
	defer func() { _ = db.Close() }()

	// Use a raw connection so we can issue BEGIN IMMEDIATE (not available via
	// db.BeginTx) on the same connection object. SQLite requires IMMEDIATE on
	// the connection that will do writes to avoid "database is locked" during
	// COMMIT on a busy WAL file.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("updater: acquire conn: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		return fmt.Errorf("updater: BEGIN IMMEDIATE: %w", err)
	}

	rollback := func() {
		_, _ = conn.ExecContext(context.Background(), "ROLLBACK")
	}

	for _, d := range chain {
		sqlBody, err := a.fetchDelta(ctx, d)
		if err != nil {
			rollback()
			return fmt.Errorf("updater: fetch delta %s->%s: %w", d.From, d.To, err)
		}
		if _, err := conn.ExecContext(ctx, sqlBody); err != nil {
			rollback()
			return fmt.Errorf("updater: exec delta %s->%s: %w", d.From, d.To, err)
		}
		if a.OnProgress != nil {
			a.OnProgress("delta-applied", d)
		}
	}

	// Reassert metadata head version. Delta bodies already replay the
	// metadata table in full (pipeline build_delta.py), but we still
	// pin catalog_version/generated_at here so the applied state is
	// correct even if a delta body is ever produced without a metadata
	// replay. Schema is key/value, matching pipeline/package/schema.sql.
	_, err = conn.ExecContext(ctx,
		"INSERT OR REPLACE INTO metadata(key, value) VALUES ('catalog_version', ?), ('generated_at', datetime('now'))",
		to,
	)
	if err != nil {
		rollback()
		return fmt.Errorf("updater: update metadata: %w", err)
	}

	if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
		rollback()
		return fmt.Errorf("updater: COMMIT: %w", err)
	}
	return nil
}

// fetchDelta downloads a delta file, verifies its SHA256, gunzips it, and
// returns the SQL body as a string.
func (a *Applier) fetchDelta(ctx context.Context, d Delta) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.URL, http.NoBody)
	if err != nil {
		return "", err
	}
	resp, err := a.client().Do(req)
	if err != nil {
		return "", fmt.Errorf("GET %s: %w", d.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GET %s: HTTP %d", d.URL, resp.StatusCode)
	}

	gzBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read body %s: %w", d.URL, err)
	}

	// Verify SHA256 over the compressed bytes (per spec instructions).
	if d.SHA256 != "" {
		h := sha256.Sum256(gzBytes)
		got := hex.EncodeToString(h[:])
		if got != d.SHA256 {
			return "", fmt.Errorf("%w for delta %s->%s: got %s want %s",
				ErrSHAMismatch, d.From, d.To, got, d.SHA256)
		}
	}

	// Gunzip the SQL body.
	gr, err := gzip.NewReader(io.LimitReader(bytesReader(gzBytes), 64<<20)) // 64 MiB limit
	if err != nil {
		return "", fmt.Errorf("gzip open %s: %w", d.URL, err)
	}
	defer func() { _ = gr.Close() }()

	sqlBytes, err := io.ReadAll(gr)
	if err != nil {
		return "", fmt.Errorf("gzip read %s: %w", d.URL, err)
	}
	return string(sqlBytes), nil
}

// bytesReader wraps a byte slice as an io.Reader without importing "bytes".
// (We'd normally just use bytes.NewReader, which is fine — using it directly.)
func bytesReader(b []byte) io.Reader {
	return &bytesReaderImpl{b: b}
}

type bytesReaderImpl struct {
	b   []byte
	off int
}

func (r *bytesReaderImpl) Read(p []byte) (n int, err error) {
	if r.off >= len(r.b) {
		return 0, io.EOF
	}
	n = copy(p, r.b[r.off:])
	r.off += n
	return n, nil
}
