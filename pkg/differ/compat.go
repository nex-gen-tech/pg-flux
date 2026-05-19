package differ

import (
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
	// Each registered checker returns true if the desired state uses the feature.
	checks := []struct {
		feat   pgver.Feature
		inUse  func(*schema.SchemaState) bool
	}{
		// Registrations land here as features become tracked in the schema model.
		// Example shape (added in P3.2):
		//   {pgver.FeatureVirtualGenerated, anyVirtualGeneratedColumn},
		//   {pgver.FeatureNullsNotDistinct,  anyNullsNotDistinctConstraint},
		//   {pgver.FeatureNotEnforced,       anyNotEnforcedConstraint},
		//   {pgver.FeatureWithoutOverlaps,   anyTemporalConstraint},
	}
	for _, c := range checks {
		if c.inUse != nil && c.inUse(desired) {
			if err := srv.Require(c.feat); err != nil {
				return err
			}
		}
	}
	return nil
}
