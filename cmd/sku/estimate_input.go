package sku

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sofq/sku/internal/estimate"
)

func readEstimateStdin(in io.Reader) ([]estimate.Item, error) {
	return estimate.DecodeWorkload(in, "json")
}

func readEstimateConfig(path string) ([]estimate.Item, error) {
	ext := strings.ToLower(filepath.Ext(path))
	var format string
	switch ext {
	case ".yaml", ".yml":
		format = "yaml"
	case ".json":
		format = "json"
	default:
		return nil, fmt.Errorf("estimate/config: unsupported extension %q (use .yaml, .yml, or .json)", ext)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("estimate/config: open %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return estimate.DecodeWorkload(f, format)
}
