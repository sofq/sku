package output

import (
	"strconv"
	"strings"
)

// ApplyFields projects doc down to the requested comma-separated dot paths.
// Paths are split on `.`; numeric segments index into `[]any`. Missing paths
// are silently dropped. When multiple paths share a prefix they merge into
// the same parent. An empty expression returns doc unchanged.
func ApplyFields(doc map[string]any, expr string) map[string]any {
	if expr == "" {
		return doc
	}
	out := map[string]any{}
	for _, raw := range strings.Split(expr, ",") {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		segs := strings.Split(path, ".")
		val, ok := walkPath(doc, segs)
		if !ok {
			continue
		}
		setAtPath(out, segs, val)
	}
	return out
}

// walkPath returns the value at segs within src, or (nil, false) if any
// segment fails to resolve.
func walkPath(src any, segs []string) (any, bool) {
	cur := src
	for _, seg := range segs {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(node) {
				return nil, false
			}
			cur = node[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// setAtPath writes value into out at the given path, creating intermediate
// maps or slices as needed. The next segment's kind (numeric vs string)
// decides whether to materialize a slice or a map at the current slot.
func setAtPath(out map[string]any, segs []string, value any) {
	if len(segs) == 0 {
		return
	}
	// Root is always a map; recurse from here.
	setMap(out, segs, value)
}

func setMap(m map[string]any, segs []string, value any) {
	head := segs[0]
	if len(segs) == 1 {
		m[head] = value
		return
	}
	rest := segs[1:]
	next := rest[0]
	if _, err := strconv.Atoi(next); err == nil {
		// Next segment is numeric → ensure a slice at m[head].
		slice, _ := m[head].([]any)
		m[head] = setSlice(slice, rest, value)
		return
	}
	// Next segment is a string → ensure a map at m[head].
	child, _ := m[head].(map[string]any)
	if child == nil {
		child = map[string]any{}
		m[head] = child
	}
	setMap(child, rest, value)
}

func setSlice(slice []any, segs []string, value any) []any {
	idx, _ := strconv.Atoi(segs[0])
	for len(slice) <= idx {
		slice = append(slice, nil)
	}
	if len(segs) == 1 {
		slice[idx] = value
		return slice
	}
	rest := segs[1:]
	next := rest[0]
	if _, err := strconv.Atoi(next); err == nil {
		child, _ := slice[idx].([]any)
		slice[idx] = setSlice(child, rest, value)
		return slice
	}
	child, _ := slice[idx].(map[string]any)
	if child == nil {
		child = map[string]any{}
		slice[idx] = child
	}
	setMap(child, rest, value)
	return slice
}
