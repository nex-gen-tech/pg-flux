package differ

import (
	"strings"

	"github.com/nexg/pg-flux/pkg/pgver"
	"github.com/nexg/pg-flux/pkg/schema"
)

// checkServerCompat walks the desired schema and returns an error when it references a
// PostgreSQL feature that is not supported on the connected server. This is the "fail
// loud" path: rather than letting a migration apply and crash at the DB, we surface a
// clear error during diff generation that names the feature and the minimum required
// version.
//
// Detection is best-effort and conservative: when the live PGVersion is zero (e.g. unit
// tests with no DB) the check is skipped. As new features get added to the parser and
// schema model, register them here with their pgver.Feature constant.
func checkServerCompat(desired *schema.SchemaState, srv pgver.Version) error {
	if desired == nil || srv == (pgver.Version{}) {
		return nil
	}
	// Builtin checkers; additional features can register via registerCompat (init blocks).
	checks := []struct {
		feat  pgver.Feature
		inUse func(*schema.SchemaState) bool
	}{
		{pgver.FeatureVirtualGenerated, hasVirtualGeneratedColumn},
		{pgver.FeatureNotEnforced, hasNotEnforcedConstraint},
		{pgver.FeatureNullsNotDistinct, hasNullsNotDistinctUnique},
		{pgver.FeatureLZ4Compression, hasLZ4CompressionColumn},
	}
	checks = append(checks, dynamicCompatChecks...)
	for _, c := range checks {
		if c.inUse != nil && c.inUse(desired) {
			if err := srv.Require(c.feat); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasVirtualGeneratedColumn(s *schema.SchemaState) bool {
	if s == nil {
		return false
	}
	for _, t := range s.Tables {
		if t == nil {
			continue
		}
		for _, c := range t.Columns {
			if c != nil && c.GeneratedKind == "virtual" {
				return true
			}
		}
	}
	return false
}

func hasNotEnforcedConstraint(s *schema.SchemaState) bool {
	if s == nil {
		return false
	}
	for _, t := range s.Tables {
		if t == nil {
			continue
		}
		for _, c := range t.Checks {
			if c != nil && c.NotEnforced {
				return true
			}
		}
		for _, fk := range t.ForeignKeys {
			if fk != nil && fk.NotEnforced {
				return true
			}
		}
	}
	return false
}

// dynamicCompatChecks holds checkers registered from init() blocks in other files
// (e.g. view security_invoker registered from view_attrs.go).
var dynamicCompatChecks []struct {
	feat  pgver.Feature
	inUse func(*schema.SchemaState) bool
}

func registerCompat(feat pgver.Feature, inUse func(*schema.SchemaState) bool) {
	dynamicCompatChecks = append(dynamicCompatChecks, struct {
		feat  pgver.Feature
		inUse func(*schema.SchemaState) bool
	}{feat, inUse})
}

func hasLZ4CompressionColumn(s *schema.SchemaState) bool {
	if s == nil {
		return false
	}
	for _, t := range s.Tables {
		if t == nil {
			continue
		}
		for _, c := range t.Columns {
			if c != nil && strings.EqualFold(c.Compression, "lz4") {
				return true
			}
		}
	}
	return false
}

func hasNullsNotDistinctUnique(s *schema.SchemaState) bool {
	if s == nil {
		return false
	}
	for _, t := range s.Tables {
		if t == nil {
			continue
		}
		for _, u := range t.Uniques {
			if u != nil && u.NullsNotDistinct {
				return true
			}
		}
	}
	return false
}
