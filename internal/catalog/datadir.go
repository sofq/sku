package catalog

import (
	"os"
	"path/filepath"
	"runtime"
)

// DataDir resolves the platform-default shard storage root, honoring
// $SKU_DATA_DIR when set. Spec §4 Environment variables.
func DataDir() string {
	if v := os.Getenv("SKU_DATA_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Caches", "sku")
	case "windows":
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, "sku")
		}
		return filepath.Join(home, "AppData", "Local", "sku")
	default:
		if v := os.Getenv("XDG_CACHE_HOME"); v != "" {
			return filepath.Join(v, "sku")
		}
		return filepath.Join(home, ".cache", "sku")
	}
}

// ShardPath returns the canonical on-disk path for a shard under DataDir().
func ShardPath(shard string) string {
	return filepath.Join(DataDir(), shard+".db")
}
