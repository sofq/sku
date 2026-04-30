package sku

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/sofq/sku/internal/batch"
	"github.com/sofq/sku/internal/catalog"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/schema"
)

// newSchemaCmd builds the `sku schema` subcommand tree. The command is a
// discovery surface for agents: it enumerates providers, services, global
// flags, error codes, and serving-provider metadata. Default output is JSON
// (agents default — spec §4 §8); `--format text` is accepted as a placeholder
// and currently falls through to the same JSON renderer until a tabwriter
// view lands in a later task.
func newSchemaCmd() *cobra.Command {
	var (
		list              bool
		errs              bool
		listServing       bool
		listCommands      bool
		kindTermOverrides bool
		format            string
	)
	c := &cobra.Command{
		Use:   "schema [provider [service [verb]]]",
		Short: "Discover providers, services, flags, and error codes",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := cmd.OutOrStdout()
			enc := json.NewEncoder(w)

			switch {
			case listCommands:
				return enc.Encode(map[string]any{
					"commands": batch.RegisteredNames(),
				})

			case kindTermOverrides:
				return enc.Encode(schema.KindTermOverridesCatalog())

			case errs:
				return enc.Encode(skuerrors.ErrorCatalog())

			case list:
				return enc.Encode(map[string]any{
					"shards": []string{"openrouter"},
				})

			case listServing:
				shardPath := catalog.ShardPath("openrouter")
				if _, statErr := os.Stat(shardPath); statErr != nil {
					e := &skuerrors.E{
						Code:       skuerrors.CodeNotFound,
						Message:    "openrouter shard not installed",
						Suggestion: "Run: sku update openrouter",
						Details:    map[string]any{"shard": "openrouter"},
					}
					skuerrors.Write(cmd.ErrOrStderr(), e)
					return e
				}
				cat, err := catalog.Open(shardPath)
				if err != nil {
					e := &skuerrors.E{
						Code:       skuerrors.CodeServer,
						Message:    err.Error(),
						Suggestion: "Check that the shard file is readable and not truncated",
					}
					skuerrors.Write(cmd.ErrOrStderr(), e)
					return e
				}
				defer func() { _ = cat.Close() }()

				providers, err := cat.ServingProviders()
				if err != nil {
					e := &skuerrors.E{
						Code:    skuerrors.CodeServer,
						Message: err.Error(),
					}
					skuerrors.Write(cmd.ErrOrStderr(), e)
					return e
				}
				return enc.Encode(map[string]any{
					"serving_providers": providers,
				})

			case len(args) == 0:
				return enc.Encode(map[string]any{
					"providers": []string{"openrouter"},
					"kinds":     []string{"llm.text", "llm.multimodal"},
					"globals":   listGlobalFlags(cmd.Root()),
				})

			default:
				// M2: leaf discovery (`sku schema openrouter [llm [price]]`)
				// returns the raw arg echo. The richer schema drill-down
				// (flags + allowed values per leaf) lands in a later task.
				return enc.Encode(map[string]any{
					"provider": args[0],
					"args":     args,
				})
			}
		},
	}
	c.Flags().BoolVar(&list, "list", false, "flat list of shard names")
	c.Flags().BoolVar(&errs, "errors", false, "emit error-code catalog")
	c.Flags().BoolVar(&listServing, "list-serving-providers", false,
		"list serving providers in the openrouter shard")
	c.Flags().BoolVar(&listCommands, "list-commands", false,
		"list batch-registered command names")
	c.Flags().BoolVar(&kindTermOverrides, "kind-term-overrides", false,
		"emit which `terms` slots are repurposed for which kinds (e.g. tenancy=engine for db.relational)")
	c.Flags().StringVar(&format, "format", "json", "json | text")
	return c
}

// listGlobalFlags walks the root persistent flag set and returns a
// JSON-friendly description of each flag. Hidden flags are skipped.
func listGlobalFlags(root *cobra.Command) []map[string]any {
	var out []map[string]any
	root.PersistentFlags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		out = append(out, map[string]any{
			"name":    f.Name,
			"usage":   f.Usage,
			"default": f.DefValue,
		})
	})
	return out
}
