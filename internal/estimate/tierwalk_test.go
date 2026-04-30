package estimate

import (
	"math"
	"testing"
)

func TestWalkTiers_ZeroVolume(t *testing.T) {
	entries := []TierEntry{
		{Lower: 0, Upper: math.MaxFloat64, Amount: 0.01},
	}
	got := WalkTiers(entries, 0)
	if got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
}

func TestWalkTiers_SingleTierWithinBounds(t *testing.T) {
	// Single tier: price $0.01 per unit, unbounded
	entries := []TierEntry{
		{Lower: 0, Upper: math.MaxFloat64, Amount: 0.01},
	}
	got := WalkTiers(entries, 100)
	want := 1.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestWalkTiers_MultiTierWithinFirstTier(t *testing.T) {
	// Two tiers: first 1000 @ $0.01, rest @ $0.005
	entries := []TierEntry{
		{Lower: 0, Upper: 1000, Amount: 0.01},
		{Lower: 1000, Upper: math.MaxFloat64, Amount: 0.005},
	}
	// Volume 500 is entirely within first tier
	got := WalkTiers(entries, 500)
	want := 5.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestWalkTiers_MultiTierCrossingBoundary(t *testing.T) {
	// Two tiers: first 1000 @ $0.01, rest @ $0.005
	entries := []TierEntry{
		{Lower: 0, Upper: 1000, Amount: 0.01},
		{Lower: 1000, Upper: math.MaxFloat64, Amount: 0.005},
	}
	// Volume 1500: 1000 @ $0.01 + 500 @ $0.005 = 10 + 2.5 = 12.5
	got := WalkTiers(entries, 1500)
	want := 12.5
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestWalkTiers_ExhaustAllTiers(t *testing.T) {
	// Three tiers: 0-100 @ $0.10, 100-500 @ $0.08, 500+ @ $0.05
	entries := []TierEntry{
		{Lower: 0, Upper: 100, Amount: 0.10},
		{Lower: 100, Upper: 500, Amount: 0.08},
		{Lower: 500, Upper: math.MaxFloat64, Amount: 0.05},
	}
	// Volume 1000: 100 @ 0.10 + 400 @ 0.08 + 500 @ 0.05
	//            = 10 + 32 + 25 = 67
	got := WalkTiers(entries, 1000)
	want := 67.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestWalkTiers_UntieriedSingleUnbounded(t *testing.T) {
	// Single untiered entry (like most current rows): Upper = math.MaxFloat64
	entries := []TierEntry{
		{Lower: 0, Upper: math.MaxFloat64, Amount: 0.023},
	}
	// 500 GB @ $0.023/GB = $11.5
	got := WalkTiers(entries, 500)
	want := 11.5
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestWalkTiers_SortsByLower(t *testing.T) {
	// Pass entries in reverse order — should still compute correctly
	entries := []TierEntry{
		{Lower: 1000, Upper: math.MaxFloat64, Amount: 0.005},
		{Lower: 0, Upper: 1000, Amount: 0.01},
	}
	got := WalkTiers(entries, 1500)
	want := 12.5
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestWalkTiers_EmptyEntries(t *testing.T) {
	got := WalkTiers(nil, 100)
	if got != 0 {
		t.Fatalf("expected 0 for empty entries, got %v", got)
	}
}
