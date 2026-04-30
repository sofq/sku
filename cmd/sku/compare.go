package sku

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/sofq/sku/internal/batch"
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

	// container.orchestration
	tier string
	mode string

	// paas.app
	planOS string

	// warehouse.query
	edition     string
	storageTier string
}

var (
	compareVMShards                     = []string{"aws-ec2", "azure-vm", "gcp-gce"}
	compareStorageObjectShards          = []string{"aws-s3", "azure-blob", "gcp-gcs"}
	compareDBRelationalShards           = []string{"aws-rds", "aws-aurora", "azure-sql", "azure-postgres", "azure-mysql", "azure-mariadb", "gcp-cloud-sql", "gcp-spanner"}
	compareCacheKVShards                = []string{"aws-elasticache", "azure-redis", "gcp-memorystore"}
	compareContainerOrchestrationShards = []string{"aws-eks", "azure-aks", "gcp-gke"}
	compareSearchEngineShards           = []string{"aws-opensearch"}
	comparePaasAppShards                = []string{"azure-appservice"}
	compareWarehouseQueryShards         = []string{"gcp-bigquery"}
)

func shardsForKind(kind string) []string {
	switch kind {
	case "compute.vm":
		return compareVMShards
	case "storage.object":
		return compareStorageObjectShards
	case "db.relational":
		return compareDBRelationalShards
	case "cache.kv":
		return compareCacheKVShards
	case "container.orchestration":
		return compareContainerOrchestrationShards
	case "search.engine":
		return compareSearchEngineShards
	case "paas.app":
		return comparePaasAppShards
	case "warehouse.query":
		return compareWarehouseQueryShards
	}
	return nil
}

func newCompareCmd() *cobra.Command {
	var f compareFlags
	c := &cobra.Command{
		Use:   "compare",
		Short: "Cross-provider equivalence compare (compute.vm, storage.object, db.relational, cache.kv, search.engine, paas.app, warehouse.query)",
		RunE:  func(cmd *cobra.Command, _ []string) error { return runCompare(cmd, &f) },
	}
	c.Flags().StringVar(&f.kind, "kind", "", "equivalence kind (compute.vm | storage.object | db.relational | cache.kv | container.orchestration | search.engine | paas.app | warehouse.query)")
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
	c.Flags().StringVar(&f.engine, "engine", "", "db.relational engine (postgres | mysql | ...) or cache.kv engine (redis | memcached)")
	c.Flags().StringVar(&f.deploymentOption, "deployment-option", "", "db.relational deployment option (single-az | multi-az | zonal | regional)")
	c.Flags().Float64Var(&f.storageGB, "storage-gb", 0, "db.relational minimum storage (GB)")
	c.Flags().StringVar(&f.tier, "tier", "", "container.orchestration/paas.app tier (free|standard|premium|... — container.orchestration or paas.app only)")
	c.Flags().StringVar(&f.mode, "mode", "", "mode filter (container.orchestration: control-plane|fargate|...; search.engine: managed-cluster|serverless; warehouse.query: on-demand|capacity|storage)")
	c.Flags().StringVar(&f.planOS, "os", "", "paas.app OS filter (linux|windows — paas.app only)")
	c.Flags().StringVar(&f.edition, "edition", "", "warehouse.query capacity edition (enterprise|enterprise-plus — warehouse.query only)")
	c.Flags().StringVar(&f.storageTier, "storage-tier", "", "warehouse.query storage tier (active|long-term — warehouse.query only)")
	return c
}

// compareValidate validates the requested compareFlags and returns a
// *skuerrors.E envelope (nil if valid) plus the expanded region literals.
func compareValidate(f compareFlags) (regionLiterals []string, err *skuerrors.E) {
	if f.kind == "" {
		return nil, skuerrors.Validation("flag_invalid", "kind", "",
			"pass --kind compute.vm | storage.object | db.relational | cache.kv | container.orchestration | search.engine | paas.app | warehouse.query")
	}
	supportedKinds := map[string]bool{
		"compute.vm":              true,
		"storage.object":          true,
		"db.relational":           true,
		"cache.kv":                true,
		"container.orchestration": true,
		"search.engine":           true,
		"paas.app":                true,
		"warehouse.query":         true,
	}
	if !supportedKinds[f.kind] {
		return nil, skuerrors.Validation("flag_invalid", "kind", f.kind,
			"supported kinds: compute.vm, storage.object, db.relational, cache.kv, container.orchestration, search.engine, paas.app, warehouse.query")
	}
	switch f.kind {
	case "compute.vm":
		if f.storageClass != "" || f.durabilityNines != 0 || f.availabilityTier != "" || f.storageGB != 0 || f.tier != "" || f.mode != "" {
			return nil, skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"compute.vm does not accept --storage-class / --durability-nines / --availability-tier / --storage-gb / --tier / --mode")
		}
	case "storage.object":
		if f.vcpu != 0 || f.memoryGB != 0 || f.gpuCount != 0 || f.storageGB != 0 || f.tier != "" || f.mode != "" {
			return nil, skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"storage.object does not accept --vcpu / --memory / --gpu-count / --storage-gb / --tier / --mode")
		}
	case "db.relational":
		if f.gpuCount != 0 || f.storageClass != "" || f.durabilityNines != 0 || f.availabilityTier != "" || f.tier != "" || f.mode != "" {
			return nil, skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"db.relational does not accept --gpu-count / --storage-class / --durability-nines / --availability-tier / --tier / --mode")
		}
	case "cache.kv":
		if f.vcpu != 0 || f.gpuCount != 0 || f.storageClass != "" || f.durabilityNines != 0 ||
			f.availabilityTier != "" || f.storageGB != 0 || f.deploymentOption != "" || f.tier != "" || f.mode != "" {
			return nil, skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"cache.kv does not accept --vcpu / --gpu-count / --storage-class / --durability-nines / --availability-tier / --storage-gb / --deployment-option / --tier / --mode")
		}
	case "container.orchestration":
		if f.vcpu != 0 || f.memoryGB != 0 || f.gpuCount != 0 || f.storageClass != "" ||
			f.durabilityNines != 0 || f.availabilityTier != "" || f.storageGB != 0 ||
			f.engine != "" || f.deploymentOption != "" || f.planOS != "" || f.edition != "" || f.storageTier != "" {
			return nil, skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"container.orchestration accepts --tier / --mode / --regions / --max-price")
		}
	case "search.engine":
		if f.gpuCount != 0 || f.storageClass != "" ||
			f.durabilityNines != 0 || f.availabilityTier != "" || f.storageGB != 0 ||
			f.engine != "" || f.deploymentOption != "" || f.tier != "" || f.planOS != "" ||
			f.edition != "" || f.storageTier != "" {
			return nil, skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"search.engine accepts --vcpu / --memory / --mode / --regions / --max-price")
		}
	case "paas.app":
		if f.gpuCount != 0 || f.storageClass != "" || f.durabilityNines != 0 ||
			f.availabilityTier != "" || f.storageGB != 0 || f.engine != "" ||
			f.deploymentOption != "" || f.mode != "" || f.edition != "" || f.storageTier != "" {
			return nil, skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"paas.app accepts --os / --tier / --vcpu / --memory / --regions / --max-price")
		}
	case "warehouse.query":
		if f.vcpu != 0 || f.memoryGB != 0 || f.gpuCount != 0 || f.storageClass != "" ||
			f.durabilityNines != 0 || f.availabilityTier != "" || f.storageGB != 0 ||
			f.engine != "" || f.deploymentOption != "" || f.tier != "" || f.planOS != "" {
			return nil, skuerrors.Validation("flag_invalid", "kind-flag-mismatch", f.kind,
				"warehouse.query accepts --mode / --edition / --storage-tier / --regions / --max-price")
		}
	}
	switch f.sort {
	case "price", "vcpu", "memory":
	default:
		return nil, skuerrors.Validation("flag_invalid", "sort", f.sort, "allowed: price | vcpu | memory")
	}
	if f.kind == "storage.object" && (f.sort == "vcpu" || f.sort == "memory") {
		return nil, skuerrors.Validation("flag_invalid", "sort", f.sort, "storage.object supports --sort price only")
	}
	if f.regions != "" {
		raw := strings.Split(f.regions, ",")
		for i := range raw {
			raw[i] = strings.TrimSpace(raw[i])
		}
		lits, _, expErr := compare.Expand(raw)
		if expErr != nil {
			return nil, skuerrors.Validation("flag_invalid", "regions", f.regions, expErr.Error())
		}
		regionLiterals = lits
	}
	return regionLiterals, nil
}

// compareLookup is the shared body used by the standalone Cobra command and
// the batch handler. Returns []catalog.Row on success, a *skuerrors.E envelope
// on failure.
func compareLookup(ctx context.Context, f compareFlags, s *batch.Settings) ([]catalog.Row, error) {
	regionLiterals, vErr := compareValidate(f)
	if vErr != nil {
		return nil, vErr
	}
	kindShards := shardsForKind(f.kind)

	installed, err := catalog.InstalledShards()
	if err != nil {
		return nil, &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
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
		return nil, &skuerrors.E{
			Code:       skuerrors.CodeNotFound,
			Message:    fmt.Sprintf("no %s shards installed", f.kind),
			Suggestion: "Run: sku update " + strings.Join(kindShards, " "),
			Details:    map[string]any{"shards_required": kindShards, "missing": missing},
		}
	}

	// Stale error gate (no stderr writes here).
	for _, t := range targets {
		cat, err := catalog.Open(t.Path)
		if err != nil {
			return nil, &skuerrors.E{Code: skuerrors.CodeServer, Message: err.Error()}
		}
		if s != nil && s.StaleErrorDays > 0 && !s.StaleOK {
			age := cat.Age(time.Now().UTC())
			if age >= s.StaleErrorDays {
				_ = cat.Close()
				return nil, &skuerrors.E{
					Code:       skuerrors.CodeStaleData,
					Message:    fmt.Sprintf("catalog %d days old exceeds threshold %d", age, s.StaleErrorDays),
					Suggestion: "Run: sku update " + t.Name,
					Details:    map[string]any{"shard": t.Name, "age_days": age, "threshold_days": s.StaleErrorDays},
				}
			}
		}
		_ = cat.Close()
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
		Tier:             f.tier,
		Mode:             f.mode,
		PlanOS:           f.planOS,
		Edition:          f.edition,
		StorageTier:      f.storageTier,
		Regions:          regionLiterals,
		Sort:             f.sort,
		Limit:            f.limit,
		Targets:          targets,
	}
	rows, err := compare.Run(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("compare: %w", err)
	}
	if len(rows) == 0 {
		applied := map[string]any{
			"vcpu": f.vcpu, "memory_gb": f.memoryGB, "gpu_count": f.gpuCount,
			"regions": regionLiterals,
		}
		// Only echo --max-price when the user actually set a ceiling; a
		// zero value here means "unset", not "free only", so shipping
		// `max_price: 0` in applied_filters would be a lie about what
		// the comparator actually filtered on.
		if f.maxPrice > 0 {
			applied["max_price"] = f.maxPrice
		}
		return nil, skuerrors.NotFound("compare", f.kind, applied,
			"Try relaxing --vcpu / --memory or widening --regions")
	}
	return rows, nil
}

func compareFlagsFromArgs(args map[string]any) compareFlags {
	vcpu, _ := argFloat(args, "vcpu")
	mem, _ := argFloat(args, "memory")
	gpu, _ := argFloat(args, "gpu_count")
	maxPrice, _ := argFloat(args, "max_price")
	durNines, _ := argFloat(args, "durability_nines")
	storageGB, _ := argFloat(args, "storage_gb")
	limit, ok := argFloat(args, "limit")
	if !ok {
		limit = 20
	}
	regions := argString(args, "regions")
	if regions == "" {
		if regs := argStringSlice(args, "regions"); len(regs) > 0 {
			regions = strings.Join(regs, ",")
		}
	}
	sort := argString(args, "sort")
	if sort == "" {
		sort = "price"
	}
	engine := argString(args, "engine")
	if engine == "" && argString(args, "kind") == "db.relational" {
		engine = "postgres"
	}
	deployment := argString(args, "deployment_option")
	if deployment == "" && argString(args, "kind") == "db.relational" {
		deployment = "single-az"
	}
	return compareFlags{
		kind:             argString(args, "kind"),
		vcpu:             int64(vcpu),
		memoryGB:         mem,
		gpuCount:         int64(gpu),
		maxPrice:         maxPrice,
		regions:          regions,
		sort:             sort,
		limit:            int(limit),
		storageClass:     argString(args, "storage_class"),
		durabilityNines:  int64(durNines),
		availabilityTier: argString(args, "availability_tier"),
		engine:           engine,
		deploymentOption: deployment,
		storageGB:        storageGB,
		tier:             argString(args, "tier"),
		mode:             argString(args, "mode"),
		planOS:           argString(args, "os"),
		edition:          argString(args, "edition"),
		storageTier:      argString(args, "storage_tier"),
	}
}

func handleCompare(ctx context.Context, args map[string]any, env batch.Env) (any, error) {
	return compareLookup(ctx, compareFlagsFromArgs(args), env.Settings)
}

func runCompare(cmd *cobra.Command, f *compareFlags) error {
	s := globalSettings(cmd)
	if f.kind == "db.relational" {
		if f.engine == "" {
			f.engine = "postgres"
		}
		if f.deploymentOption == "" {
			f.deploymentOption = "single-az"
		}
	}

	regionLiterals, vErr := compareValidate(*f)
	if vErr != nil {
		skuerrors.Write(cmd.ErrOrStderr(), vErr)
		return vErr
	}

	kindShards := shardsForKind(f.kind)

	if s.DryRun {
		args := map[string]any{
			"kind":    f.kind,
			"regions": regionLiterals,
			"sort":    f.sort,
			"limit":   f.limit,
		}
		// Mirror the applied-filters echo: only include max_price when set.
		if f.maxPrice > 0 {
			args["max_price"] = f.maxPrice
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
		case "cache.kv":
			args["memory_gb"] = f.memoryGB
			args["engine"] = f.engine
		case "container.orchestration":
			args["tier"] = f.tier
			args["mode"] = f.mode
		case "search.engine":
			args["mode"] = f.mode
			args["vcpu"] = f.vcpu
			args["memory_gb"] = f.memoryGB
		case "paas.app":
			args["os"] = f.planOS
			args["tier"] = f.tier
		case "warehouse.query":
			args["mode"] = f.mode
			args["edition"] = f.edition
			args["storage_tier"] = f.storageTier
		}
		return output.EmitDryRun(cmd.OutOrStdout(), output.DryRunPlan{
			Command:      "compare",
			ResolvedArgs: args,
			Shards:       kindShards,
			Preset:       s.Preset,
		})
	}

	// Emit the "missing shards" stderr warning + stale warnings here; the
	// batch handler skips them.
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
	var missing []string
	for _, name := range kindShards {
		if !installedSet[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 && !s.StaleOK && len(missing) != len(kindShards) {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(),
			"warning: missing shards %v; compare is incomplete (run `sku update %s`)\n",
			missing, strings.Join(missing, " "))
	}
	for _, name := range kindShards {
		if !installedSet[name] {
			continue
		}
		cat, err := catalog.Open(catalog.ShardPath(name))
		if err != nil {
			continue
		}
		_ = applyStaleGate(cmd, cat, name, s) // emits stale-warning lines only (error-gate is re-checked in compareLookup)
		_ = cat.Close()
	}

	bs := ToBatchSettings(s)
	rows, err := compareLookup(context.Background(), *f, &bs)
	if err != nil {
		skuerrors.Write(cmd.ErrOrStderr(), err)
		return err
	}

	if s.Preset == "" || s.Preset == "agent" {
		s.Preset = "compare"
	}
	return renderRows(cmd, rows, s)
}
