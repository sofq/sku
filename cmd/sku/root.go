// Package sku wires the Cobra command tree for the sku CLI.
package sku

import (
	"context"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sku",
		Short:         "Agent-friendly cloud & LLM pricing CLI",
		Long:          "sku is an agent-friendly CLI for querying cloud and LLM pricing across AWS, Azure, Google Cloud, and OpenRouter.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveSettings(cmd)
			if err != nil {
				return err
			}
			ctx := context.WithValue(cmd.Context(), settingsKey, s)
			cmd.SetContext(ctx)
			return nil
		},
	}
	pf := root.PersistentFlags()
	pf.String("profile", "", "named config profile (default \"default\")")
	pf.String("preset", "", "agent | full | price | compare (default agent)")
	pf.String("jq", "", "jq filter on response")
	pf.String("fields", "", "comma-separated dot-path projection")
	pf.Bool("include-raw", false, "include raw passthrough object")
	pf.Bool("include-aggregated", false, "include OpenRouter's aggregated rows")
	pf.Bool("pretty", false, "pretty-print output")
	pf.Bool("stale-ok", false, "suppress stale-catalog warning")
	pf.Bool("auto-fetch", false, "download missing shards on demand")
	pf.Bool("dry-run", false, "show resolved query plan without executing")
	pf.Bool("verbose", false, "stderr JSON log")
	pf.Bool("no-color", false, "disable color")
	pf.Bool("json", false, "output format: JSON (default)")
	pf.Bool("yaml", false, "output format: YAML")
	pf.Bool("toml", false, "output format: TOML")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newLLMCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newSchemaCmd())
	root.AddCommand(newConfigureCmd())
	return root
}
