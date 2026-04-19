package estimate

import (
	"fmt"
	"strings"
)

// Item is one parsed --item entry.
type Item struct {
	Raw      string
	Provider string
	Service  string
	Resource string
	Kind     string
	Params   map[string]string
}

// ParseItem parses one --item value. Format: provider/service:resource[:k=v...].
func ParseItem(raw string) (Item, error) {
	if raw == "" {
		return Item{}, fmt.Errorf("estimate/item: empty value")
	}
	segs := strings.Split(raw, ":")
	if len(segs) < 2 {
		return Item{}, fmt.Errorf("estimate/item: %q missing ':resource'", raw)
	}
	ps := segs[0]
	slash := strings.IndexByte(ps, '/')
	if slash <= 0 || slash == len(ps)-1 {
		return Item{}, fmt.Errorf("estimate/item: %q: first segment must be provider/service", raw)
	}
	provider := strings.ToLower(ps[:slash])
	service := strings.ToLower(ps[slash+1:])
	resource := segs[1]
	if resource == "" {
		return Item{}, fmt.Errorf("estimate/item: %q: empty resource", raw)
	}
	kind, ok := providerServiceKind[provider+"/"+service]
	if !ok {
		return Item{}, fmt.Errorf("estimate/item: unsupported provider/service %q", provider+"/"+service)
	}
	params := map[string]string{}
	for _, kv := range segs[2:] {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			return Item{}, fmt.Errorf("estimate/item: %q: param %q must be key=value", raw, kv)
		}
		k := strings.ToLower(kv[:eq])
		v := kv[eq+1:]
		if _, exists := params[k]; exists {
			return Item{}, fmt.Errorf("estimate/item: %q: duplicate param %q", raw, k)
		}
		params[k] = v
	}
	return Item{
		Raw: raw, Provider: provider, Service: service,
		Resource: resource, Kind: kind, Params: params,
	}, nil
}
