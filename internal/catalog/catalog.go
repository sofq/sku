// Package catalog provides a read-only view over a sku SQLite shard. Spec §5.
package catalog

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Minimum/maximum shard schema_version this binary understands. Widening the
// range is a minor binary release; narrowing is a major.
const (
	minSchemaVersion = 1
	maxSchemaVersion = 1
)

// Catalog wraps an opened shard. Safe for concurrent use by multiple goroutines
// (the underlying *sql.DB is; SQLite WAL mode permits concurrent readers).
type Catalog struct {
	db             *sql.DB
	schemaVersion  string
	catalogVersion string
	currency       string
	generatedAt    string
	shardPath      string
}

// Open opens the shard at path in WAL mode and verifies its schema_version.
func Open(path string) (*Catalog, error) {
	// modernc.org/sqlite DSN accepts pragmas via URI query params.
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("catalog: open %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("catalog: ping %s: %w", path, err)
	}

	cat := &Catalog{db: db, shardPath: path}
	if err := cat.loadMetadata(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return cat, nil
}

func (c *Catalog) loadMetadata() error {
	rows, err := c.db.Query("SELECT key, value FROM metadata")
	if err != nil {
		return fmt.Errorf("catalog: read metadata: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		switch k {
		case "schema_version":
			c.schemaVersion = v
		case "catalog_version":
			c.catalogVersion = v
		case "currency":
			c.currency = v
		case "generated_at":
			c.generatedAt = v
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if c.schemaVersion == "" {
		return fmt.Errorf("catalog: shard %s has no schema_version metadata", c.shardPath)
	}
	// Range check.
	sv := 0
	_, _ = fmt.Sscanf(c.schemaVersion, "%d", &sv)
	if sv < minSchemaVersion || sv > maxSchemaVersion {
		return fmt.Errorf("catalog: shard schema_version=%s outside supported [%d,%d]",
			c.schemaVersion, minSchemaVersion, maxSchemaVersion)
	}
	return nil
}

// Close releases the underlying SQLite handle.
func (c *Catalog) Close() error { return c.db.Close() }

// SchemaVersion returns the shard's declared schema_version string.
func (c *Catalog) SchemaVersion() string { return c.schemaVersion }

// CatalogVersion returns the CalVer release string from metadata.
func (c *Catalog) CatalogVersion() string { return c.catalogVersion }

// Currency returns the shard's invariant currency.
func (c *Catalog) Currency() string { return c.currency }

// BuildFromSQL creates a fresh SQLite file at path, executes the provided SQL
// (schema + seed), and closes the handle. Used only by tests.
func BuildFromSQL(path string, ddl string) error {
	_ = os.Remove(path)
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(ddl); err != nil {
		return err
	}
	return nil
}
