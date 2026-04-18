package sku

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

// shardSources maps a shard name to the release tag's asset base URL.
// Grow this map in lockstep with the spec §3 shard inventory.
var shardSources = map[string]string{
	"openrouter": "https://github.com/sofq/sku/releases/download/data-bootstrap-openrouter",
	"aws-ec2":    "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-ec2",
	"aws-rds":    "https://github.com/sofq/sku/releases/download/data-bootstrap-aws-rds",
}

// resolveUpdateBaseURL returns the asset base URL for shard. Precedence:
//  1. SKU_UPDATE_BASE_URL (applied to every shard — test hook)
//  2. SKU_UPDATE_BASE_URL_<SHARD> (per-shard override, hyphens -> underscores)
//  3. shardSources[shard] default
func resolveUpdateBaseURL(shard string) (string, bool) {
	if v := os.Getenv("SKU_UPDATE_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/"), true
	}
	upperShard := strings.ToUpper(strings.ReplaceAll(shard, "-", "_"))
	if v := os.Getenv("SKU_UPDATE_BASE_URL_" + upperShard); v != "" {
		return strings.TrimRight(v, "/"), true
	}
	base, ok := shardSources[shard]
	if !ok {
		return "", false
	}
	return strings.TrimRight(base, "/"), true
}

func shardNames() []string {
	out := make([]string, 0, len(shardSources))
	for name := range shardSources {
		out = append(out, name)
	}
	return out
}

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <shard>",
		Short: "Download and install a pricing shard (openrouter | aws-ec2 | aws-rds)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s := globalSettings(cmd)
			shard := args[0]
			baseURL, ok := resolveUpdateBaseURL(shard)
			if !ok {
				err := skuerrors.Validation(
					"unsupported_shard", "shard", shard,
					"supported shards: "+strings.Join(shardNames(), ", "),
				)
				skuerrors.Write(cmd.ErrOrStderr(), err)
				return err
			}

			dataDir := catalog.DataDir()
			if err := os.MkdirAll(dataDir, 0o750); err != nil { //nolint:gosec // standard cache dir
				e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
				skuerrors.Write(cmd.ErrOrStderr(), e)
				return e
			}

			zstURL := baseURL + "/" + shard + ".db.zst"
			shaURL := baseURL + "/" + shard + ".db.zst.sha256"

			// Download .zst to a .part file.
			zstPartPath := catalog.ShardPath(shard) + ".zst.part"
			defer func() { _ = os.Remove(zstPartPath) }()

			if s.Verbose {
				output.Log(cmd.ErrOrStderr(), "update.fetch", map[string]any{
					"shard": shard,
					"url":   zstURL,
				})
			}
			zstData, err := httpGet(zstURL)
			if err != nil {
				skuerrors.Write(cmd.ErrOrStderr(), err)
				return err
			}
			if s.Verbose {
				output.Log(cmd.ErrOrStderr(), "update.fetched", map[string]any{
					"shard": shard,
					"bytes": len(zstData),
				})
			}
			if writeErr := os.WriteFile(zstPartPath, zstData, 0o600); writeErr != nil { //nolint:gosec // zstPartPath is derived from catalog.ShardPath
				e := &skuerrors.E{Code: skuerrors.CodeServer, Message: writeErr.Error()}
				skuerrors.Write(cmd.ErrOrStderr(), e)
				return e
			}

			// Fetch sha256 and verify.
			if s.Verbose {
				output.Log(cmd.ErrOrStderr(), "update.sha_fetch", map[string]any{
					"url": shaURL,
				})
			}
			shaData, err := httpGet(shaURL)
			if err != nil {
				skuerrors.Write(cmd.ErrOrStderr(), err)
				return err
			}
			expectedHex := strings.Fields(string(shaData))[0]

			h := sha256.Sum256(zstData)
			gotHex := hex.EncodeToString(h[:])
			if gotHex != expectedHex {
				e := &skuerrors.E{
					Code:    skuerrors.CodeConflict,
					Message: fmt.Sprintf("sha256 mismatch: got %s, want %s", gotHex, expectedHex),
				}
				skuerrors.Write(cmd.ErrOrStderr(), e)
				return e
			}

			// Decompress to a .part file then atomically rename.
			dbPartPath := catalog.ShardPath(shard) + ".part"
			if s.Verbose {
				output.Log(cmd.ErrOrStderr(), "update.decompress", map[string]any{
					"shard": shard,
					"dest":  dbPartPath,
				})
			}
			if err := decompressZstd(zstData, dbPartPath); err != nil {
				skuerrors.Write(cmd.ErrOrStderr(), err)
				return err
			}

			dbPath := catalog.ShardPath(shard)
			if renameErr := os.Rename(dbPartPath, dbPath); renameErr != nil {
				_ = os.Remove(dbPartPath)
				e := &skuerrors.E{Code: skuerrors.CodeServer, Message: renameErr.Error()}
				skuerrors.Write(cmd.ErrOrStderr(), e)
				return e
			}

			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "installed %s -> %s\n", shard, dbPath)
			return nil
		},
	}
}

// httpGet performs a GET request and returns the body bytes.
// Returns a *skuerrors.E with CodeServer if the status >= 400.
func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url) //nolint:gosec // URL is caller-controlled (env override + hardcoded default)
	if err != nil {
		return nil, &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, &skuerrors.E{
			Code:    skuerrors.CodeServer,
			Message: fmt.Sprintf("HTTP %d from %s", resp.StatusCode, url),
		}
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
	}
	return data, nil
}

// decompressZstd decompresses zstData and writes the result to destPath.
func decompressZstd(zstData []byte, destPath string) error { //nolint:gosec // destPath is derived from catalog.ShardPath
	r, err := zstd.NewReader(bytes.NewReader(zstData))
	if err != nil {
		return &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
	}
	defer r.Close()

	out, err := os.Create(destPath) //nolint:gosec // destPath is derived from catalog.ShardPath
	if err != nil {
		return &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, r); err != nil {
		return &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
	}
	return nil
}
