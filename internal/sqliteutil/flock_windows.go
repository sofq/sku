//go:build windows

// Package sqliteutil provides small helpers for SQLite database files.
package sqliteutil

import "errors"

// ErrLocked is returned when an advisory flock cannot be acquired.
var ErrLocked = errors.New("sqliteutil: shard is locked by another process")

// Flock is a no-op stub on Windows. Advisory file locking via syscall.Flock
// is not available on this platform. Callers should treat the returned unlock
// func as valid — it does nothing.
//
// TODO: implement using LockFileEx on Windows if concurrent-update protection
// is needed on that platform.
func Flock(dbPath string) (unlock func() error, err error) {
	return func() error { return nil }, nil
}
