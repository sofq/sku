package sku

import (
	"github.com/spf13/cobra"

	skuerrors "github.com/sofq/sku/internal/errors"
)

type searchFlags struct {
	provider     string
	service      string
	kind         string
	resourceName string
	region       string
	minVCPU      int64
	minMemoryGB  float64
	maxPrice     float64
	sort         string
	limit        int
}

func (f *searchFlags) bind(c *cobra.Command) {
	c.Flags().StringVar(&f.provider, "provider", "", "cloud provider (aws | azure | gcp)")
	c.Flags().StringVar(&f.service, "service", "", "service within provider (ec2 | rds | ...)")
	c.Flags().StringVar(&f.kind, "kind", "", "resource kind (compute.vm | db.relational | ...)")
	c.Flags().StringVar(&f.resourceName, "resource-name", "", "exact resource name (e.g. m5.large)")
	c.Flags().StringVar(&f.region, "region", "", "provider region (e.g. us-east-1)")
	c.Flags().Int64Var(&f.minVCPU, "min-vcpu", 0, "minimum vCPU count")
	c.Flags().Float64Var(&f.minMemoryGB, "min-memory", 0, "minimum memory in GB")
	c.Flags().Float64Var(&f.maxPrice, "max-price", 0, "maximum unit price across any dimension")
	c.Flags().StringVar(&f.sort, "sort", "", "sort column: resource_name | price | vcpu | memory")
	c.Flags().IntVar(&f.limit, "limit", 50, "maximum rows to return (0 = unlimited)")
}

func newSearchCmd() *cobra.Command {
	var f searchFlags
	c := &cobra.Command{
		Use:   "search",
		Short: "List SKUs matching filters within a single shard",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSearch(cmd, &f)
		},
	}
	f.bind(c)
	return c
}

func runSearch(cmd *cobra.Command, f *searchFlags) error {
	if f.provider == "" {
		e := skuerrors.Validation("flag_invalid", "provider", "",
			"pass --provider <aws|azure|gcp>; multi-provider search arrives in M4.2")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.service == "" {
		e := skuerrors.Validation("flag_invalid", "service", "",
			"pass --service <service>, e.g. --service ec2")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.maxPrice < 0 {
		e := skuerrors.Validation("flag_invalid", "max-price", "",
			"--max-price must be non-negative")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.minVCPU < 0 {
		e := skuerrors.Validation("flag_invalid", "min-vcpu", "",
			"--min-vcpu must be non-negative")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.minMemoryGB < 0 {
		e := skuerrors.Validation("flag_invalid", "min-memory", "",
			"--min-memory must be non-negative")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	return nil // body lands in Task 9
}
