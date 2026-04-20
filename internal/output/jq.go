package output

import (
	"fmt"

	"github.com/itchyny/gojq"
)

// ApplyJQ runs a gojq expression against doc. Zero iterator outputs return
// nil; one returns that value directly; multiple return a `[]any` of all
// results. Parse/runtime errors are wrapped with a "jq:" prefix.
func ApplyJQ(doc any, expr string) (any, error) {
	query, err := gojq.Parse(expr)
	if err != nil {
		return nil, fmt.Errorf("jq: %w", err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return nil, fmt.Errorf("jq: %w", err)
	}
	iter := code.Run(doc)
	var results []any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if e, isErr := v.(error); isErr {
			return nil, fmt.Errorf("jq: %w", e)
		}
		results = append(results, v)
	}
	switch len(results) {
	case 0:
		return nil, nil
	case 1:
		return results[0], nil
	default:
		return results, nil
	}
}
