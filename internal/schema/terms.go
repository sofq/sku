// Package schema holds shared data-schema primitives used by both the catalog
// reader and (eventually) code-generated clients. The canonical terms encoding
// in this file MUST remain byte-identical to pipeline/normalize/terms.py —
// their equality is asserted by testdata/terms_golden.jsonl which is consumed
// by both test suites.
package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// Terms is the six-tuple that participates in terms_hash.
// Field order in the JSON array is fixed (see CanonicalizeTerms) and a
// schema_version bump is required to change it.
type Terms struct {
	Commitment    string
	Tenancy       string
	OS            string
	SupportTier   string
	Upfront       string
	PaymentOption string
}

// CanonicalizeTerms returns the canonical JSON array encoding of Terms.
// Output: `["commitment","tenancy","os","support_tier","upfront","payment_option"]`
// with no whitespace, matching json.dumps(..., separators=(",", ":")) on the Python side.
func CanonicalizeTerms(t Terms) string {
	// Use json.Marshal on a []string so Go's encoder emits the exact same
	// bytes as Python's json.dumps with compact separators: no spaces, fields
	// escaped identically.
	arr := [6]string{t.Commitment, t.Tenancy, t.OS, t.SupportTier, t.Upfront, t.PaymentOption}
	b, err := json.Marshal(arr)
	if err != nil {
		// []string marshalling cannot fail; panic preserves the invariant.
		panic("schema.CanonicalizeTerms: unreachable marshal error: " + err.Error())
	}
	return string(b)
}

// TermsHash returns the 128-bit hex digest (32 chars) of CanonicalizeTerms(t).
func TermsHash(t Terms) string {
	sum := sha256.Sum256([]byte(CanonicalizeTerms(t)))
	return hex.EncodeToString(sum[:16])
}
