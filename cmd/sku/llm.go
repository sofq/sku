package sku

import "github.com/spf13/cobra"

func newLLMCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "llm",
		Short: "Cross-provider LLM pricing (OpenRouter-backed)",
	}
	c.AddCommand(newLLMPriceCmd())
	return c
}
