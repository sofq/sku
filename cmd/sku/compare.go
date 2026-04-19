package sku

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/catalog"
	"github.com/sofq/sku/internal/compare"
	skuerrors "github.com/sofq/sku/internal/errors"
	"github.com/sofq/sku/internal/output"
)

type compareFlags struct {
	kind     string
	vcpu     int64
	memoryGB float64
	gpuCount int64
	maxPrice float64
	regions  string
	sort     string
	limit    int
}

// compareVMShards is the static allow list used in m4.2. Other kinds will add
// their own mappings; the static list keeps shell completions stable.
var compareVMShards = []string{"aws-ec2", "azure-vm", "gcp-gce"}

func newCompareCmd() *cobra.Command {
	var f compareFlags
	c := &cobra.Command{
		Use:   "compare",
		Short: "Cross-provider equivalence compare (compute.vm in m4.2)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runCompare(cmd, &f) },
	}
	c.Flags().StringVar(&f.kind, "kind", "", "equivalence kind (compute.vm only in m4.2)")
	c.Flags().Int64Var(&f.vcpu, "vcpu", 0, "minimum vCPU count")
	c.Flags().Float64Var(&f.memoryGB, "memory", 0, "minimum memory in GB")
	c.Flags().Int64Var(&f.gpuCount, "gpu-count", 0, "minimum GPU count (0 excludes GPU SKUs)")
	c.Flags().Float64Var(&f.maxPrice, "max-price", 0, "maximum per-dimension price")
	c.Flags().StringVar(&f.regions, "regions", "", "comma-separated region literals or groups (us-east, us-west, ...)")
	c.Flags().StringVar(&f.sort, "sort", "price", "sort column: price | vcpu | memory")
	c.Flags().IntVar(&f.limit, "limit", 20, "maximum rows across all providers")
	return c
}

func runCompare(cmd *cobra.Command, f *compareFlags) error {
	s := globalSettings(cmd)

	if f.kind == "" {
		e := skuerrors.Validation("flag_invalid", "kind", "",
			"pass --kind compute.vm (only kind supported in m4.2)")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.kind != "compute.vm" {
		e := skuerrors.Validation("flag_invalid", "kind", f.kind,
			"only compute.vm is wired in m4.2; db.relational / storage.object / llm.text arrive in m4.3")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	switch f.sort {
	case "price", "vcpu", "memory":
	default:
		e := skuerrors.Validation("flag_invalid", "sort", f.sort,
			"allowed: price | vcpu | memory")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}

	var regionLiterals []string
	if f.regions != "" {
		raw := strings.Split(f.regions, ",")
		for i := range raw {
			raw[i] = strings.TrimSpace(raw[i])
		}
		lits, _, err := compare.Expand(raw)
		if err != nil {
			e := skuerrors.Validation("flag_invalid", "regions", f.regions, err.Error())
			skuerrors.Write(cmd.ErrOrStderr(), e)
			return e
		}
		regionLiterals = lits
	}

	if s.DryRun {
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command: "compare",
			ResolvedArgs: map[string]any{
				"kind":      f.kind,
				"vcpu":      f.vcpu,
				"memory_gb": f.memoryGB,
				"gpu_count": f.gpuCount,
				"max_price": f.maxPrice,
				"regions":   regionLiterals,
				"sort":      f.sort,
				"limit":     f.limit,
			},
			Shards: compareVMShards,
			Preset: s.Preset,
		})
	}

	installed, err := catalog.InstalledShards()
	if err != nil {
		e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	installedSet := map[string]bool{}
	for _, n := range installed {
		installedSet[n] = true
	}
	var targets []compare.ShardTarget
	var missing []string
	for _, name := range compareVMShards {
		if !installedSet[name] {
			missing = append(missing, name)
			continue
		}
		targets = append(targets, compare.ShardTarget{Name: name, Path: catalog.ShardPath(name)})
	}
	if len(targets) == 0 {
		e := &skuerrors.E{
			Code:       skuerrors.CodeNotFound,
			Message:    "no compute.vm shards installed",
			Suggestion: "Run: sku update aws-ec2 azure-vm gcp-gce",
			Details:    map[string]any{"shards_required": compareVMShards, "missing": missing},
		}
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if len(missing) > 0 && !s.StaleOK {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"warning: missing shards %v; compare is incomplete (run `sku update %s`)\n",
			missing, strings.Join(missing, " "))
	}

	for _, t := range targets {
		cat, err := catalog.Open(t.Path)
		if err != nil {
			e := &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
			skuerrors.Write(cmd.ErrOrStderr(), e)
			return e
		}
		stale := applyStaleGate(cmd, cat, t.Name, s)
		_ = cat.Close()
		if stale != nil {
			return stale
		}
	}

	rows, err := compare.Run(context.Background(), compare.Request{
		Kind:     f.kind,
		VCPU:     f.vcpu,
		MemoryGB: f.memoryGB,
		GPUCount: f.gpuCount,
		MaxPrice: f.maxPrice,
		Regions:  regionLiterals,
		Sort:     f.sort,
		Limit:    f.limit,
		Targets:  targets,
	})
	if err != nil {
		wrapped := fmt.Errorf("compare: %w", err)
		skuerrors.Write(cmd.ErrOrStderr(), wrapped)
		return wrapped
	}
	if len(rows) == 0 {
		e := skuerrors.NotFound("compare", f.kind,
			map[string]any{
				"vcpu": f.vcpu, "memory_gb": f.memoryGB, "gpu_count": f.gpuCount,
				"regions": regionLiterals, "max_price": f.maxPrice,
			},
			"Try relaxing --vcpu / --memory or widening --regions")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}

	if s.Preset == "" || s.Preset == "agent" {
		s.Preset = "compare"
	}
	return renderRows(cmd, rows, s)
}
