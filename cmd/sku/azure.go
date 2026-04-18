package sku

import "github.com/spf13/cobra"

// newAzureCmd returns the `sku azure ...` parent. m3b.1 adds vm and sql;
// m3b.2 will append blob, functions, disks.
func newAzureCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "azure",
		Short: "Azure pricing subcommands",
	}
	c.AddCommand(newAzureVMCmd())
	c.AddCommand(newAzureSQLCmd())
	return c
}
