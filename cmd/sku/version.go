package sku

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			enc := json.NewEncoder(cmd.OutOrStdout())
			return enc.Encode(version.Get())
		},
	}
}
