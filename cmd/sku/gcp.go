package sku

import "github.com/spf13/cobra"

// newGCPCmd returns the `sku gcp ...` parent. m3b.3 ships gce + cloud-sql;
// gcs, run, and functions arrive in m3b.4.
func newGCPCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "gcp",
		Short: "Google Cloud pricing subcommands",
	}
	c.AddCommand(newGCPGCECmd())
	c.AddCommand(newGCPCloudSQLCmd())
	return c
}
