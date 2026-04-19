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

	// storage.object
	storageClass     string
	durabilityNines  int64
	availabilityTier string

	// db.relational
	engine           string
	deploymentOption string
	storageGB        float64
}

var (
	compareVMShards            = []string{"aws-ec2", "azure-vm", "gcp-gce"}
	compareStorageObjectShards = []string{"aws-s3", "azure-blob", "gcp-gcs"}
	compareDBRelationalShards  = []string{"aws-rds", "azure-sql", "gcp-cloud-sql"}
)

func shardsForKind(kind string) []string {
	switch kind {
	case "compute.vm":
		return compareVMShards
	case "storage.object":
		return compareStorageObjectShards
	case "db.relational":
		return compareDBRelationalShards
	}
	return nil
}

func newCompareCmd() *cobra.Command {
	var f compareFlags
	c := &cobra.Command{
		Use:   "compare",
		Short: "Cross-provider equivalence compare (compute.vm, storage.object, db.relational)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runCompare(cmd, &f) },
	}
	c.Flags().StringVar(&f.kind, "kind", "", "equivalence kind (compute.vm | storage.object | db.relational)")
	c.Flags().Int64Var(&f.vcpu, "vcpu", 0, "minimum vCPU count")
	c.Flags().Float64Var(&f.memoryGB, "memory", 0, "minimum memory in GB")
	c.Flags().Int64Var(&f.gpuCount, "gpu-count", 0, "minimum GPU count (0 excludes GPU SKUs)")
	c.Flags().Float64Var(&f.maxPrice, "max-price", 0, "maximum per-dimension price")
	c.Flags().StringVar(&f.regions, "regions", "", "comma-separated region literals or groups (us-east, us-west, ...)")
	c.Flags().StringVar(&f.sort, "sort", "price", "sort column: price | vcpu | memory")
	c.Flags().IntVar(&f.limit, "limit", 20, "maximum rows across all providers")
	c.Flags().StringVar(&f.storageClass, "storage-class", "", "storage.object resource_name (e.g. standard, standard-ia, hot)")
	c.Flags().Int64Var(&f.durabilityNines, "durability-nines", 0, "storage.object minimum durability (e.g. 11)")
	c.Flags().StringVar(&f.availabilityTier, "availability-tier", "", "storage.object availability tier (e.g. standard, infrequent, archive)")
	c.Flags().StringVar(&f.engine, "engine", "postgres", "db.relational engine (postgres | mysql | ...)")
	c.Flags().StringVar(&f.deploymentOption, "deployment-option", "single-az", "db.relational deployment option (single-az | multi-az | zonal | regional)")
	c.Flags().Float64Var(&f.storageGB, "storage-gb", 0, "db.relational minimum storage (GB)")
	return c
}

func runCompare(cmd *cobra.Command, f *compareFlags) error {
	s := globalSettings(cmd)

	if f.kind == "" {
		e := skuerrors.Validation("flag_invalid", "kind", "",
			"pass --kind compute.vm | storage.object | db.relational")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	supportedKinds := map[string]bool{
		"compute.vm":     true,
		"storage.object": true,
		"db.relational":  true,
	}
	if !supportedKinds[f.kind] {
		e := skuerrors.Validation("flag_invalid", "kind", f.kind,
			"supported kinds in m4.3: compute.vm, storage.object, db.relational (llm.text arrives in m4.4)")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}

	switch f.kind {
	case "compute.vm":
		if f.storageClass != "" || f.durabilityNines != 0 || f.availabilityTier != "" || f.storageGB != 0 {
			e := skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"compute.vm does not accept --storage-class / --durability-nines / --availability-tier / --storage-gb")
			skuerrors.Write(cmd.ErrOrStderr(), e)
			return e
		}
	case "storage.object":
		if f.vcpu != 0 || f.memoryGB != 0 || f.gpuCount != 0 || f.storageGB != 0 {
			e := skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"storage.object does not accept --vcpu / --memory / --gpu-count / --storage-gb")
			skuerrors.Write(cmd.ErrOrStderr(), e)
			return e
		}
	case "db.relational":
		if f.gpuCount != 0 || f.storageClass != "" || f.durabilityNines != 0 || f.availabilityTier != "" {
			e := skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"db.relational does not accept --gpu-count / --storage-class / --durability-nines / --availability-tier")
			skuerrors.Write(cmd.ErrOrStderr(), e)
			return e
		}
	}

	switch f.sort {
	case "price", "vcpu", "memory":
	default:
		e := skuerrors.Validation("flag_invalid", "sort", f.sort,
			"allowed: price | vcpu | memory")
		skuerrors.Write(cmd.ErrOrStderr(), e)
		return e
	}
	if f.kind == "storage.object" && (f.sort == "vcpu" || f.sort == "memory") {
		e := skuerrors.Validation("flag_invalid", "sort", f.sort, "storage.object supports --sort price only")
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

	kindShards := shardsForKind(f.kind)

	if s.DryRun {
		args := map[string]any{
			"kind":      f.kind,
			"regions":   regionLiterals,
			"sort":      f.sort,
			"limit":     f.limit,
			"max_price": f.maxPrice,
		}
		switch f.kind {
		case "compute.vm":
			args["vcpu"] = f.vcpu
			args["memory_gb"] = f.memoryGB
			args["gpu_count"] = f.gpuCount
		case "storage.object":
			args["storage_class"] = f.storageClass
			args["durability_nines"] = f.durabilityNines
			args["availability_tier"] = f.availabilityTier
		case "db.relational":
			args["vcpu"] = f.vcpu
			args["memory_gb"] = f.memoryGB
			args["storage_gb"] = f.storageGB
			args["engine"] = f.engine
			args["deployment_option"] = f.deploymentOption
		}
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command:      "compare",
			ResolvedArgs: args,
			Shards:       kindShards,
			Preset:       s.Preset,
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
	for _, name := range kindShards {
		if !installedSet[name] {
			missing = append(missing, name)
			continue
		}
		targets = append(targets, compare.ShardTarget{Name: name, Path: catalog.ShardPath(name)})
	}
	if len(targets) == 0 {
		e := &skuerrors.E{
			Code:       skuerrors.CodeNotFound,
			Message:    fmt.Sprintf("no %s shards installed", f.kind),
			Suggestion: "Run: sku update " + strings.Join(kindShards, " "),
			Details:    map[string]any{"shards_required": kindShards, "missing": missing},
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

	req := compare.Request{
		Kind:             f.kind,
		VCPU:             f.vcpu,
		MemoryGB:         f.memoryGB,
		GPUCount:         f.gpuCount,
		MaxPrice:         f.maxPrice,
		StorageClass:     f.storageClass,
		DurabilityNines:  f.durabilityNines,
		AvailabilityTier: f.availabilityTier,
		StorageGB:        f.storageGB,
		Engine:           f.engine,
		DeploymentOption: f.deploymentOption,
		Regions:          regionLiterals,
		Sort:             f.sort,
		Limit:            f.limit,
		Targets:          targets,
	}
	rows, err := compare.Run(context.Background(), req)
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
