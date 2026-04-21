//go:build unix

package sqliteutil_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sofq/sku/internal/sqliteutil"
)

func TestFlock_AcquireRelease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	// Create the file so flock has something to open.
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	unlock, err := sqliteutil.Flock(path)
	if err != nil {
		t.Fatalf("first Flock: %v", err)
	}
	// Release.
	if err := unlock(); err != nil {
		t.Fatalf("unlock: %v", err)
	}

	// Can acquire again after release.
	unlock2, err := sqliteutil.Flock(path)
	if err != nil {
		t.Fatalf("second Flock after release: %v", err)
	}
	_ = unlock2()
}

func TestFlock_Conflict(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	unlock, err := sqliteutil.Flock(path)
	if err != nil {
		t.Fatalf("first Flock: %v", err)
	}
	defer unlock() //nolint:errcheck

	// Second attempt on same sidecar file → conflict.
	_, err2 := sqliteutil.Flock(path)
	if err2 == nil {
		t.Fatal("want conflict error, got nil")
	}
	if !errors.Is(err2, sqliteutil.ErrLocked) {
		t.Fatalf("want ErrLocked, got %v", err2)
	}
}

func TestFlock_CreatesLockSidecar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	unlock, err := sqliteutil.Flock(path)
	if err != nil {
		t.Fatalf("Flock: %v", err)
	}
	defer unlock() //nolint:errcheck

	// Sidecar lock file should exist.
	lockPath := path + ".lock"
	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Fatalf("expected sidecar lock file at %s: %v", lockPath, statErr)
	}
}
