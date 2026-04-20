package estimate

import (
	"context"
	"fmt"
	"strings"

	"github.com/sofq/sku/internal/catalog"
)

const llmShard = "openrouter"

var lookupLLM = func(ctx context.Context, shard string, f catalog.LLMFilter) ([]catalog.Row, error) {
	cat, err := catalog.Open(catalog.ShardPath(shard))
	if err != nil {
		return nil, err
	}
	defer func() { _ = cat.Close() }()
	return cat.LookupLLM(ctx, f)
}

var (
	llmPromptDims     = map[string]bool{"prompt": true, "input": true, "input_tokens": true}
	llmCompletionDims = map[string]bool{"completion": true, "output": true, "output_tokens": true}
)

type llmTextEstimator struct{}

func (llmTextEstimator) Kind() string { return "llm.text" }

func (llmTextEstimator) Estimate(ctx context.Context, it Item) (LineItem, error) {
	inTokens, err := paramFloatLLM(it.Params, "input", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	outTokens, err := paramFloatLLM(it.Params, "output", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	if inTokens == 0 && outTokens == 0 {
		return LineItem{}, fmt.Errorf("estimate/llm.text: %q requires :input=<tokens> and/or :output=<tokens>", it.Raw)
	}
	requests, err := paramFloatLLM(it.Params, "requests", 0, 0)
	if err != nil {
		return LineItem{}, err
	}
	serving := strings.ToLower(it.Params["serving_provider"])

	rows, err := lookupLLM(ctx, llmShard, catalog.LLMFilter{
		Model:             it.Resource,
		ServingProvider:   serving,
		IncludeAggregated: serving == "openrouter",
	})
	if err != nil {
		return LineItem{}, fmt.Errorf("estimate/llm.text: lookup: %w", err)
	}
	if len(rows) == 0 {
		return LineItem{}, fmt.Errorf("estimate/llm.text: no SKU for model %q (serving_provider=%q)", it.Resource, serving)
	}
	if len(rows) > 1 {
		return LineItem{}, fmt.Errorf("estimate/llm.text: %d rows for %q — add :serving_provider=<name>", len(rows), it.Resource)
	}
	r := rows[0]

	var promptPrice, completionPrice float64
	var havePrompt, haveCompletion bool
	for _, p := range r.Prices {
		switch {
		case llmPromptDims[p.Dimension]:
			promptPrice, havePrompt = p.Amount, true
		case llmCompletionDims[p.Dimension]:
			completionPrice, haveCompletion = p.Amount, true
		}
	}
	if inTokens > 0 && !havePrompt {
		return LineItem{}, fmt.Errorf("estimate/llm.text: no prompt price on SKU %s", r.SKUID)
	}
	if outTokens > 0 && !haveCompletion {
		return LineItem{}, fmt.Errorf("estimate/llm.text: no completion price on SKU %s", r.SKUID)
	}

	monthly := inTokens*promptPrice + outTokens*completionPrice

	notes := []string{fmt.Sprintf("serving_provider=%s", r.Provider)}
	if requests > 0 {
		notes = append(notes, fmt.Sprintf("requests=%g", requests))
	}

	return LineItem{
		Item: it, Kind: "llm.text", SKUID: r.SKUID,
		Provider: r.Provider, Service: r.Service, Resource: r.ResourceName,
		Region:       r.Region,
		Quantity:     inTokens + outTokens,
		QuantityUnit: "token",
		MonthlyUSD:   monthly,
		Notes:        notes,
	}, nil
}

func paramFloatLLM(p map[string]string, k string, def, minVal float64) (float64, error) {
	s, ok := p[k]
	if !ok {
		return def, nil
	}
	f, err := parseQuantity(s)
	if err != nil {
		return 0, fmt.Errorf("estimate/llm.text: %s=%q: %w", k, s, err)
	}
	if f < minVal {
		return 0, fmt.Errorf("estimate/llm.text: %s=%v below minimum %v", k, f, minVal)
	}
	return f, nil
}

func init() {
	Register(llmTextEstimator{})
}
