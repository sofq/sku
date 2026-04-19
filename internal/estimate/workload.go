package estimate

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const workloadSizeCap = 1 << 20 // 1 MiB

// WorkloadDoc is the structured shape accepted by --config and --stdin.
type WorkloadDoc struct {
	Items []WorkloadItem `json:"items" yaml:"items"`
}

// WorkloadItem mirrors the inline DSL fields in structured form.
type WorkloadItem struct {
	Provider string         `json:"provider"          yaml:"provider"`
	Service  string         `json:"service"           yaml:"service"`
	Resource string         `json:"resource"          yaml:"resource"`
	Params   map[string]any `json:"params,omitempty"  yaml:"params,omitempty"`
}

// DecodeWorkload parses a workload document from r and returns []Item ready
// for estimate.Run. format is "json" or "yaml".
func DecodeWorkload(r io.Reader, format string) ([]Item, error) {
	limited := io.LimitReader(r, workloadSizeCap+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("estimate/workload: read: %w", err)
	}
	if len(raw) > workloadSizeCap {
		return nil, fmt.Errorf("estimate/workload: input exceeds %d bytes", workloadSizeCap)
	}

	var doc WorkloadDoc
	switch format {
	case "json":
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&doc); err != nil {
			return nil, fmt.Errorf("estimate/workload: json: %w", err)
		}
	case "yaml":
		dec := yaml.NewDecoder(strings.NewReader(string(raw)))
		dec.KnownFields(true)
		if err := dec.Decode(&doc); err != nil {
			return nil, fmt.Errorf("estimate/workload: yaml: %w", err)
		}
	default:
		return nil, fmt.Errorf("estimate/workload: unknown format %q", format)
	}

	if len(doc.Items) == 0 {
		return nil, fmt.Errorf("estimate/workload: items must be non-empty")
	}

	out := make([]Item, 0, len(doc.Items))
	for i, wi := range doc.Items {
		it, err := workloadItemToItem(i, wi)
		if err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

func workloadItemToItem(i int, wi WorkloadItem) (Item, error) {
	if wi.Provider == "" {
		return Item{}, fmt.Errorf("estimate/workload: item %d: empty provider", i)
	}
	if wi.Service == "" {
		return Item{}, fmt.Errorf("estimate/workload: item %d: empty service", i)
	}
	if wi.Resource == "" {
		return Item{}, fmt.Errorf("estimate/workload: item %d: empty resource", i)
	}
	prov := strings.ToLower(wi.Provider)
	svc := strings.ToLower(wi.Service)
	kind, ok := providerServiceKind[prov+"/"+svc]
	if !ok {
		return Item{}, fmt.Errorf("estimate/workload: item %d: unsupported provider/service %q", i, prov+"/"+svc)
	}
	params := map[string]string{}
	for k, v := range wi.Params {
		lk := strings.ToLower(k)
		if _, dup := params[lk]; dup {
			return Item{}, fmt.Errorf("estimate/workload: item %d: duplicate param %q", i, lk)
		}
		sv, err := paramValueToString(v)
		if err != nil {
			return Item{}, fmt.Errorf("estimate/workload: item %d: param %q: %w", i, lk, err)
		}
		params[lk] = sv
	}
	raw := renderRaw(prov, svc, wi.Resource, params)
	return Item{
		Raw: raw, Provider: prov, Service: svc,
		Resource: wi.Resource, Kind: kind, Params: params,
	}, nil
}

func paramValueToString(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		if x {
			return "true", nil
		}
		return "false", nil
	case int:
		return fmt.Sprintf("%d", x), nil
	case int64:
		return fmt.Sprintf("%d", x), nil
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x)), nil
		}
		return fmt.Sprintf("%g", x), nil
	case nil:
		return "", fmt.Errorf("nil value")
	default:
		return "", fmt.Errorf("unsupported value type %T", v)
	}
}

func renderRaw(provider, service, resource string, params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(provider)
	b.WriteByte('/')
	b.WriteString(service)
	b.WriteByte(':')
	b.WriteString(resource)
	for _, k := range keys {
		b.WriteByte(':')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(params[k])
	}
	return b.String()
}
