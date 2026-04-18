package schema

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type goldenCase struct {
	Name      string            `json:"name"`
	Input     map[string]string `json:"input"`
	Canonical string            `json:"canonical"`
	TermsHash string            `json:"terms_hash"`
}

func loadGolden(t *testing.T) []goldenCase {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "terms_golden.jsonl"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	var out []goldenCase
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Bytes()
		if len(line) == 0 {
			continue
		}
		var c goldenCase
		require.NoError(t, json.Unmarshal(line, &c))
		out = append(out, c)
	}
	require.NoError(t, s.Err())
	require.NotEmpty(t, out)
	return out
}

func TestCanonicalizeTerms_MatchesGolden(t *testing.T) {
	for _, c := range loadGolden(t) {
		terms := Terms{
			Commitment:    c.Input["commitment"],
			Tenancy:       c.Input["tenancy"],
			OS:            c.Input["os"],
			SupportTier:   c.Input["support_tier"],
			Upfront:       c.Input["upfront"],
			PaymentOption: c.Input["payment_option"],
		}
		got := CanonicalizeTerms(terms)
		require.Equal(t, c.Canonical, got, "case %s", c.Name)
	}
}

func TestTermsHash_MatchesGolden(t *testing.T) {
	for _, c := range loadGolden(t) {
		terms := Terms{
			Commitment:    c.Input["commitment"],
			Tenancy:       c.Input["tenancy"],
			OS:            c.Input["os"],
			SupportTier:   c.Input["support_tier"],
			Upfront:       c.Input["upfront"],
			PaymentOption: c.Input["payment_option"],
		}
		got := TermsHash(terms)
		require.Equal(t, c.TermsHash, got, "case %s", c.Name)
		require.Len(t, got, 32, "128-bit hex = 32 chars")
	}
}
