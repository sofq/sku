package sku

import "github.com/spf13/cobra"

// newGCPCmd returns the `sku gcp ...` parent. m3b.3 shipped gce + cloud-sql;
// m3b.4 adds gcs, run, and functions (on-demand only).
func newGCPCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "gcp",
		Short: "Google Cloud pricing subcommands",
	}
	c.AddCommand(newGCPGCECmd())
	c.AddCommand(newGCPCloudSQLCmd())
	c.AddCommand(newGCPGCSCmd())
	c.AddCommand(newGCPRunCmd())
	c.AddCommand(newGCPFunctionsCmd())
	c.AddCommand(newGCPSpannerCmd())
	c.AddCommand(newGCPMemorystoreCmd())
	return c
}
