package catalog

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// InstalledShards lists the shard basenames (without ".db") found in the
// current SKU data directory. Files that do not end in ".db" (including WAL
// sidecars like "aws-ec2.db-wal") are ignored. The returned slice is
// lexicographically sorted.
func InstalledShards() ([]string, error) {
	dir := DataDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".db") {
			continue
		}
		out = append(out, strings.TrimSuffix(filepath.Base(name), ".db"))
	}
	sort.Strings(out)
	return out, nil
}
