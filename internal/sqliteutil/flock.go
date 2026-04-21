//go:build unix

// Package sqliteutil provides small helpers for SQLite database files.
package sqliteutil

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// ErrLocked is returned when an advisory flock cannot be acquired because
// another process already holds it. Callers map this to
// skuerrors.CodeConflict (exit 6) per spec §4.
var ErrLocked = errors.New("sqliteutil: shard is locked by another process")

// Flock acquires an exclusive advisory lock on a sidecar file <dbPath>.lock.
// It does NOT lock the SQLite file directly to avoid interfering with SQLite's
// own locking protocol. The sidecar file is created if it does not exist.
//
// Returns an unlock function that releases the lock and closes the sidecar fd.
// If the lock is already held by another process, ErrLocked is returned.
func Flock(dbPath string) (unlock func() error, err error) {
	lockPath := dbPath + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // path derived from caller
	if err != nil {
		return nil, fmt.Errorf("sqliteutil: open lock file %s: %w", lockPath, err)
	}

	fd := f.Fd()
	if err := syscall.Flock(int(fd), syscall.LOCK_EX|syscall.LOCK_NB); err != nil { //nolint:gosec // G115: fd is a valid int-range file descriptor
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, fmt.Errorf("sqliteutil: flock %s: %w", lockPath, err)
	}

	unlock = func() error {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:gosec // G115: fd is a valid int-range file descriptor
		return f.Close()
	}
	return unlock, nil
}
