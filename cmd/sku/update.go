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

// resolveManifestPrimaryURL returns the manifest.json URL based on
// SKU_UPDATE_BASE_URL or the default GitHub releases data branch.
func resolveManifestPrimaryURL() string {
	if v := os.Getenv("SKU_UPDATE_BASE_URL"); v != "" {
		return strings.TrimRight(v, "/") + "/manifest.json"
	}
	return "https://github.com/sofq/sku/releases/download/data-latest/manifest.json"
}

func shardNames() []string { return updater.ShardNames() }

// shouldUseManifestUpdate always returns true: all installs and updates go
// through the manifest. The data-bootstrap-* release tags are no longer used.
func shouldUseManifestUpdate(string, updater.Channel, bool) bool { return true }

func newUpdateCmd() *cobra.Command {
	var channelFlag string

	cmd := &cobra.Command{
		Use:   "update <shard>",
		Short: "Download and install a pricing shard (openrouter | aws-* | azure-* | gcp-*)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s := globalSettings(cmd)
			shard := args[0]

			// Validate shard name.
			_, ok := resolveUpdateBaseURL(shard)
			if !ok {
				err := skuerrors.Validation(
					"unsupported_shard", "shard", shard,
					"supported shards: "+strings.Join(shardNames(), ", "),
				)
				skuerrors.Write(cmd.ErrOrStderr(), err)
				return err
			}

			// Resolve channel: flag > SKU_UPDATE_CHANNEL env > profile > stable.
			channel, err := updater.ResolveChannel(
				channelFlag,
				os.Getenv("SKU_UPDATE_CHANNEL"),
				s.Channel,
			)
			if err != nil {
				skuerrors.Write(cmd.ErrOrStderr(), err)
				return err
			}

			dataDir := catalog.DataDir()

			if s.Verbose {
				output.Log(cmd.ErrOrStderr(), "update.fetch", map[string]any{
					"shard": shard, "channel": string(channel),
				})
			}

			primaryManifestURL := resolveManifestPrimaryURL()
			fallbackManifestURL := "https://cdn.jsdelivr.net/gh/sofq/sku@data/manifest.json"

			manifestSrc := updater.NewHTTPSource(primaryManifestURL, fallbackManifestURL, nil)

			opts := updater.UpdateOptions{
				Options: updater.Options{
					DestDir: dataDir,
				},
				Channel:  channel,
				Manifest: manifestSrc,
				MaxChain: 20,
			}

			result, err := updater.Update(cmd.Context(), shard, opts)
			if err != nil {
				code := skuerrors.CodeServer
				var ve *skuerrors.E
				if errors.As(err, &ve) {
					skuerrors.Write(cmd.ErrOrStderr(), ve)
					return ve
				}
				if errors.Is(err, updater.ErrSHAMismatch) {
					code = skuerrors.CodeConflict
				} else if errors.Is(err, updater.ErrLocked) {
					code = skuerrors.CodeConflict
				}
				e := &skuerrors.E{Code: code, Message: err.Error()}
				skuerrors.Write(cmd.ErrOrStderr(), e)
				return e
			}

			if s.Verbose {
				if result.FellBackToBaseline {
					output.Log(cmd.ErrOrStderr(), "update.fallback-to-baseline", map[string]any{
						"shard": shard, "from": result.From,
					})
				}
				if result.Baseline {
					output.Log(cmd.ErrOrStderr(), "update.baseline-installed", map[string]any{
						"shard": shard, "version": result.To,
					})
				} else if len(result.Applied) > 0 {
					for _, d := range result.Applied {
						output.Log(cmd.ErrOrStderr(), "update.delta-applied", map[string]any{
							"shard": shard, "from": d.From, "to": d.To,
						})
					}
				} else {
					output.Log(cmd.ErrOrStderr(), "update.304", map[string]any{
						"shard": shard, "version": result.From,
					})
				}
			}

			_, _ = cmd.ErrOrStderr().Write([]byte("installed " + shard + " -> " + catalog.ShardPath(shard) + "\n"))
			return nil
		},
	}

	cmd.Flags().StringVar(&channelFlag, "channel", "", `update channel: "stable" (default, always full baseline) or "daily" (delta chain, falls back to baseline)`)
	return cmd
}
