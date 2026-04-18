// Package sku wires the Cobra command tree for the sku CLI.
package sku

import "github.com/spf13/cobra"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sku",
		Short:         "Agent-friendly cloud & LLM pricing CLI",
		Long:          "sku is an agent-friendly CLI for querying cloud and LLM pricing across AWS, Azure, Google Cloud, and OpenRouter.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newVersionCmd())
	root.AddCommand(newLLMCmd())
	root.AddCommand(newUpdateCmd())
	return root
}
