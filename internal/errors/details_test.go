package errors_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	skuerrors "github.com/sofq/sku/internal/errors"
)

func TestCatalog_EveryCodeHasSchema(t *testing.T) {
	cat := skuerrors.ErrorCatalog()
	for _, code := range []skuerrors.Code{
		skuerrors.CodeGeneric, skuerrors.CodeAuth, skuerrors.CodeNotFound,
		skuerrors.CodeValidation, skuerrors.CodeRateLimited, skuerrors.CodeConflict,
		skuerrors.CodeServer, skuerrors.CodeStaleData,
	} {
		schema, ok := cat.Entries[string(code)]
		require.True(t, ok, "no schema for code %q", code)
		require.NotEmpty(t, schema.DetailsFields, "code %q has empty DetailsFields", code)
		require.NotZero(t, schema.ExitCode)
	}
}

func TestValidationDetailsShape_CoversReasons(t *testing.T) {
	reasons := skuerrors.ValidationReasons()
	require.Contains(t, reasons, "flag_invalid")
	require.Contains(t, reasons, "binary_too_old")
	require.Contains(t, reasons, "binary_too_new")
	require.Contains(t, reasons, "shard_too_old")
	require.Contains(t, reasons, "shard_too_new")
}
