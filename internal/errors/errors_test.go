package errors_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	skuerrors "github.com/sofq/sku/internal/errors"
)

func TestWrite_ShapeMatchesSpec(t *testing.T) {
	var buf bytes.Buffer
	err := &skuerrors.E{
		Code:       skuerrors.CodeNotFound,
		Message:    "No SKU matches filters",
		Suggestion: "Try `sku schema openrouter llm` to see valid filters",
		Details: map[string]any{
			"provider": "openrouter",
			"service":  "llm",
			"applied_filters": map[string]any{
				"model": "anthropic/nope",
			},
		},
	}
	got := skuerrors.Write(&buf, err)
	require.Equal(t, 3, got, "not_found -> exit 3")

	var env map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	body, ok := env["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "not_found", body["code"])
	require.Equal(t, "No SKU matches filters", body["message"])
	require.Contains(t, body, "suggestion")
	require.Contains(t, body, "details")
	require.Equal(t, byte('\n'), buf.Bytes()[buf.Len()-1])
}

func TestWrite_UnknownErrorMapsToGeneric(t *testing.T) {
	var buf bytes.Buffer
	got := skuerrors.Write(&buf, errors.New("boom"))
	require.Equal(t, 1, got)

	var env map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &env))
	body := env["error"].(map[string]any)
	require.Equal(t, "generic_error", body["code"])
	require.Equal(t, "boom", body["message"])
}

func TestCode_ExitMapping(t *testing.T) {
	cases := map[skuerrors.Code]int{
		skuerrors.CodeOK:          0,
		skuerrors.CodeGeneric:     1,
		skuerrors.CodeAuth:        2,
		skuerrors.CodeNotFound:    3,
		skuerrors.CodeValidation:  4,
		skuerrors.CodeRateLimited: 5,
		skuerrors.CodeConflict:    6,
		skuerrors.CodeServer:      7,
		skuerrors.CodeStaleData:   8,
	}
	for c, want := range cases {
		require.Equal(t, want, c.ExitCode(), "code %s", c)
	}
}

func TestWrite_Nil_Returns0(t *testing.T) {
	var buf bytes.Buffer
	require.Equal(t, 0, skuerrors.Write(&buf, nil))
	require.Zero(t, buf.Len())
}
