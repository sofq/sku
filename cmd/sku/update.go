package sku

import (
	"errors"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
	"github.com/sofq/sku/internal/updater"
)

// shardSources kept as an in-package alias so the existing env-override
// tests keep reading this name. Points at the same map exported by
// internal/updater so there is one source of truth.
var shardSources = updater.DefaultSources

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

func shardNames() []string { return updater.ShardNames() }

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <shard>",
		Short: "Download and install a pricing shard (openrouter | aws-ec2 | aws-rds | aws-s3 | aws-lambda | aws-ebs | aws-dynamodb | aws-cloudfront | azure-vm | azure-sql)",
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
			if s.Verbose {
				output.Log(cmd.ErrOrStderr(), "update.fetch", map[string]any{
					"shard": shard, "base_url": baseURL,
				})
			}
			if err := updater.Install(cmd.Context(), shard, updater.Options{
				BaseURL: baseURL,
				DestDir: catalog.DataDir(),
			}); err != nil {
				code := skuerrors.CodeServer
				if errors.Is(err, updater.ErrSHAMismatch) {
					code = skuerrors.CodeConflict
				}
				e := &skuerrors.E{Code: code, Message: err.Error()}
				skuerrors.Write(cmd.ErrOrStderr(), e)
				return e
			}
			_, _ = cmd.ErrOrStderr().Write([]byte("installed " + shard + " -> " + catalog.ShardPath(shard) + "\n"))
			return nil
		},
	}
}
