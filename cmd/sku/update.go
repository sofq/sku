package sku

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	"github.com/sofq/sku/internal/config"
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

type shardStatus struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	AgeDays   int    `json:"age_days,omitempty"`
	Path      string `json:"path,omitempty"`
}

type statusResult struct {
	Shards []shardStatus `json:"shards"`
}

func runUpdateStatus(cmd *cobra.Command, s config.Settings) error {
	names := shardNames()
	statuses := make([]shardStatus, 0, len(names))
	for _, name := range names {
		path := catalog.ShardPath(name)
		if _, err := os.Stat(path); err != nil {
			statuses = append(statuses, shardStatus{Name: name, Installed: false})
			continue
		}
		cat, openErr := catalog.Open(path)
		if openErr != nil {
			statuses = append(statuses, shardStatus{Name: name, Installed: true, Path: path})
			continue
		}
		statuses = append(statuses, shardStatus{
			Name:      name,
			Installed: true,
			Version:   cat.CatalogVersion(),
			AgeDays:   cat.Age(time.Now().UTC()),
			Path:      path,
		})
		_ = cat.Close()
	}

	result := statusResult{Shards: statuses}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}

	var out []byte
	if s.Pretty {
		var buf bytes.Buffer
		if indentErr := json.Indent(&buf, raw, "", "  "); indentErr == nil {
			out = buf.Bytes()
		} else {
			out = raw
		}
	} else {
		out = raw
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n", out)
	return err
}

func runSingleShardUpdate(cmd *cobra.Command, shard, channelFlag string, s config.Settings) error {
	if _, ok := resolveUpdateBaseURL(shard); !ok {
		err := skuerrors.Validation(
			"unsupported_shard", "shard", shard,
			"supported shards: "+strings.Join(shardNames(), ", "),
		)
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}

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
}

func newUpdateCmd() *cobra.Command {
	var (
		channelFlag string
		statusFlag  bool
	)

	cmd := &cobra.Command{
		Use:   "update [<shard>...]",
		Short: "Download and install pricing shards (openrouter | aws-* | azure-* | gcp-*)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s := globalSettings(cmd)

			if statusFlag {
				return runUpdateStatus(cmd, s)
			}

			var shards []string
			if len(args) > 0 {
				shards = args
			} else {
				installed, err := catalog.InstalledShards()
				if err != nil {
					e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
					skuerrors.Write(cmd.ErrOrStderr(), e)
					return e
				}
				shards = installed
			}

			if len(shards) == 0 {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "no shards installed; run `sku update <shard>` to install one")
				return nil
			}

			var firstErr error
			for _, shard := range shards {
				if err := runSingleShardUpdate(cmd, shard, channelFlag, s); err != nil && firstErr == nil {
					firstErr = err
				}
			}
			return firstErr
		},
	}

	cmd.Flags().StringVar(&channelFlag, "channel", "", `update channel: "stable" (default, always full baseline) or "daily" (delta chain, falls back to baseline)`)
	cmd.Flags().BoolVar(&statusFlag, "status", false, "Print per-shard freshness JSON; no fetch.")
	return cmd
}
