package sku

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/output"
	"github.com/sofq/sku/internal/version"
)

// newVersionCmd emits build metadata via the global output renderer so
// agents can pipe it the same way as any other sku command: --yaml / --toml
// / --pretty / --jq / --fields all apply.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s := globalSettings(cmd)
			info := version.Get()

			// Round-trip via JSON so --fields / --jq can operate on a
			// map[string]any with snake_case keys (matching the
			// json tags on version.Info).
			raw, err := json.Marshal(info)
			if err != nil {
				return err
			}
			var doc map[string]any
			if err := json.Unmarshal(raw, &doc); err != nil {
				return err
			}

			var out any = doc
			if s.Fields != "" {
				out = output.ApplyFields(doc, s.Fields)
			}
			if s.JQ != "" {
				out, err = output.ApplyJQ(out, s.JQ)
				if err != nil {
					return err
				}
			}

			b, err := output.Encode(out, s.Format, s.Pretty)
			if err != nil {
				return err
			}
			_, err = cmd.OutOrStdout().Write(b)
			return err
		},
	}
}
