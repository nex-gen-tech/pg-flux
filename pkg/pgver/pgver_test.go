package pgver

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseServerVersionNum(t *testing.T) {
	cases := []struct {
		in       int
		major    int
		expected string
	}{
		{140005, 14, "14.5"},
		{170002, 17, "17.2"},
		{180000, 18, "18"},
		{0, 0, "0"},
	}
	for _, c := range cases {
		v := ParseServerVersionNum(c.in)
		assert.Equal(t, c.major, v.Major, "input %d", c.in)
		assert.Equal(t, c.expected, v.String(), "input %d", c.in)
	}
}

func TestAtLeast(t *testing.T) {
	v := Version{Major: 16, Minor: 3}
	assert.True(t, v.AtLeast(14))
	assert.True(t, v.AtLeast(16))
	assert.False(t, v.AtLeast(17))
}

func TestSupportsAndRequire(t *testing.T) {
	pg14 := Version{Major: 14}
	pg15 := Version{Major: 15}
	pg17 := Version{Major: 17}
	pg18 := Version{Major: 18}

	// LZ4 is PG14+
	assert.True(t, pg14.Supports(FeatureLZ4Compression))
	assert.NoError(t, pg14.Require(FeatureLZ4Compression))

	// NULLS NOT DISTINCT is PG15+
	assert.False(t, pg14.Supports(FeatureNullsNotDistinct))
	err := pg14.Require(FeatureNullsNotDistinct)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "NULLS NOT DISTINCT")
	assert.Contains(t, err.Error(), "PostgreSQL 15+")
	assert.True(t, pg15.Supports(FeatureNullsNotDistinct))

	// WITHOUT OVERLAPS is PG17+
	assert.False(t, pg15.Supports(FeatureWithoutOverlaps))
	assert.True(t, pg17.Supports(FeatureWithoutOverlaps))

	// Virtual generated columns are PG18+
	assert.False(t, pg17.Supports(FeatureVirtualGenerated))
	assert.True(t, pg18.Supports(FeatureVirtualGenerated))
}

func TestRequire_messageMentionsVersion(t *testing.T) {
	pg14 := Version{Major: 14, Minor: 5}
	err := pg14.Require(FeatureVirtualGenerated)
	assert.Error(t, err)
	// Should name the feature, the required version, and the connected version.
	msg := err.Error()
	assert.True(t, strings.Contains(msg, "14.5"), "expected connected version 14.5 in message: %q", msg)
	assert.True(t, strings.Contains(msg, "18"), "expected required version 18 in message: %q", msg)
}

// Unknown feature constants must be conservative — return not-supported, not panic.
func TestSupports_unknownFeature(t *testing.T) {
	v := Version{Major: 99}
	assert.False(t, v.Supports(Feature("imaginary feature")))
}
