package batch

import (
	"strings"
	"testing"
)

func FuzzBatchParse(f *testing.F) {
	seeds := []string{
		``,
		`[]`,
		`[{"command":"x"}]`,
		`{"command":"x"}` + "\n" + `{"command":"y"}`,
		`# comment` + "\n" + `{"command":"z"}`,
		`not-json`,
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		_, _, _ = Parse(strings.NewReader(in))
	})
}
