package estimate

import (
	"context"
	"math"
	"testing"

	"github.com/sofq/sku/internal/catalog"
)

func TestLLMTextEstimator_promptAndCompletion(t *testing.T) {
	resetRegistry(t)
	prev := lookupLLM
	t.Cleanup(func() { lookupLLM = prev })
	lookupLLM = func(_ context.Context, shard string, f catalog.LLMFilter) ([]catalog.Row, error) {
		if shard != "openrouter" {
			t.Fatalf("shard = %q, want openrouter", shard)
		}
		if f.Model != "anthropic/claude-opus-4.6" {
			t.Fatalf("model = %q", f.Model)
		}
		if f.ServingProvider != "anthropic" {
			t.Fatalf("serving_provider = %q", f.ServingProvider)
		}
		if f.IncludeAggregated {
			t.Fatal("IncludeAggregated must default to false")
		}
		return []catalog.Row{{
			SKUID:    "anthropic/claude-opus-4.6::anthropic::default",
			Provider: "anthropic", Service: "llm",
			ResourceName: "anthropic/claude-opus-4.6",
			Prices: []catalog.Price{
				{Dimension: "prompt", Amount: 1.5e-5, Unit: "token"},
				{Dimension: "completion", Amount: 7.5e-5, Unit: "token"},
			},
		}}, nil
	}
	Register(llmTextEstimator{})

	it, err := ParseItem("llm:anthropic/claude-opus-4.6:input=1M:output=500K:serving_provider=anthropic")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	li, err := llmTextEstimator{}.Estimate(context.Background(), it)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	want := 1e6*1.5e-5 + 5e5*7.5e-5
	if math.Abs(li.MonthlyUSD-want) > 1e-9 {
		t.Fatalf("monthly = %v, want %v", li.MonthlyUSD, want)
	}
	if li.QuantityUnit != "token" || li.Quantity != 1_500_000 {
		t.Fatalf("bad line: %+v", li)
	}
}

func TestLLMTextEstimator_promptOnly(t *testing.T) {
	prev := lookupLLM
	t.Cleanup(func() { lookupLLM = prev })
	lookupLLM = func(_ context.Context, _ string, _ catalog.LLMFilter) ([]catalog.Row, error) {
		return []catalog.Row{{
			SKUID: "x", Provider: "anthropic",
			Prices: []catalog.Price{
				{Dimension: "prompt", Amount: 1e-5, Unit: "token"},
			},
		}}, nil
	}
	it, _ := ParseItem("llm:anthropic/claude-opus-4.6:input=2M:serving_provider=anthropic")
	li, err := llmTextEstimator{}.Estimate(context.Background(), it)
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	want := 2e6 * 1e-5
	if math.Abs(li.MonthlyUSD-want) > 1e-9 {
		t.Fatalf("monthly = %v, want %v", li.MonthlyUSD, want)
	}
}

func TestLLMTextEstimator_missingBothQuantities(t *testing.T) {
	it, _ := ParseItem("llm:anthropic/claude-opus-4.6:serving_provider=anthropic")
	if _, err := (llmTextEstimator{}).Estimate(context.Background(), it); err == nil {
		t.Fatal("expected error when input and output are both zero")
	}
}
