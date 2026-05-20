package hazard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnrichFromDDL_NotValidAndValidate(t *testing.T) {
	d1 := `ALTER TABLE a ADD CONSTRAINT c CHECK (x > 0) NOT VALID`
	d2 := `ALTER TABLE a VALIDATE CONSTRAINT c`
	h1 := EnrichFromDDL(d1)
	h2 := EnrichFromDDL(d2)
	require.NotEmpty(t, h1)
	require.Equal(t, DeferredConstraintValidation, h1[0].Type)
	require.NotEmpty(t, h2)
	require.Equal(t, ValidateConstraintScan, h2[0].Type)
	require.Empty(t, EnrichFromDDL(""))
}
