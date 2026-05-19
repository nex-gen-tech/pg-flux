// Package pgver carries the connected PostgreSQL server's version and exposes
// feature-gate helpers so the inspector / parser / differ can branch on whether
// the target server supports a given DDL feature.
//
// pg-flux supports PG 14+. Anything older fails loud at connect time.
//
// Server version is queried once via SHOW server_version_num and parsed into a
// Version{Major, Minor}. Code paths check feature availability via the typed
// Feature constants below; see Supports() and Require().
package pgver

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MinSupportedMajor is the oldest major version pg-flux explicitly supports.
const MinSupportedMajor = 14

// Version represents a parsed PostgreSQL server version.
type Version struct {
	Major int
	Minor int
}

// Zero is the zero-value version; ZeroVersion.AtLeast(N) is false for any N>0.
var Zero = Version{}

// ParseServerVersionNum converts PostgreSQL's server_version_num integer into a
// Version. The format is MMmmpp on PG 10+ (e.g. 140005 → 14.5, 170002 → 17.2)
// and MMmm00ppp on PG 9.x (legacy). We accept both for safety but pg-flux
// requires PG14+ at runtime.
func ParseServerVersionNum(num int) Version {
	if num <= 0 {
		return Zero
	}
	if num >= 100000 {
		return Version{Major: num / 10000, Minor: num % 10000}
	}
	// Legacy PG 9.x: 90608 = 9.6.8 — encode major as 9.x
	return Version{Major: num / 10000, Minor: num % 10000}
}

// String renders as "14.5" or "14" when minor is unknown.
func (v Version) String() string {
	if v.Minor == 0 {
		return fmt.Sprintf("%d", v.Major)
	}
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// AtLeast returns true when the server's major version is >= major.
func (v Version) AtLeast(major int) bool {
	return v.Major >= major
}

// Feature enumerates DDL/runtime features that vary across supported PG versions.
// Adding a new constant requires updating featureMinVersion.
type Feature string

const (
	FeatureLZ4Compression      Feature = "LZ4 column compression"
	FeatureMultirangeTypes     Feature = "multirange types"
	FeatureNullsNotDistinct    Feature = "NULLS NOT DISTINCT unique constraints"
	FeatureMergeStatement      Feature = "MERGE statement"
	FeatureSecurityInvokerView Feature = "security_invoker view option"
	FeatureWithoutOverlaps     Feature = "WITHOUT OVERLAPS temporal PK/UNIQUE"
	FeaturePeriodFK            Feature = "PERIOD foreign key"
	FeatureNamedNotNullValid   Feature = "named NOT NULL constraint with NOT VALID"
	FeatureVirtualGenerated    Feature = "GENERATED ALWAYS AS ... VIRTUAL columns"
	FeatureNotEnforced         Feature = "NOT ENFORCED constraints"
)

// featureMinVersion maps a Feature to the earliest major version that supports it.
// Source: PostgreSQL release notes (verify on https://www.postgresql.org/docs/release).
var featureMinVersion = map[Feature]int{
	FeatureLZ4Compression:      14,
	FeatureMultirangeTypes:     14,
	FeatureNullsNotDistinct:    15,
	FeatureMergeStatement:      15,
	FeatureSecurityInvokerView: 15,
	FeatureWithoutOverlaps:     17,
	FeaturePeriodFK:            17,
	FeatureNamedNotNullValid:   18,
	FeatureVirtualGenerated:    18,
	FeatureNotEnforced:         18,
}

// Supports returns true when the server can use the named feature.
func (v Version) Supports(f Feature) bool {
	min, ok := featureMinVersion[f]
	if !ok {
		// Unknown feature: be conservative — assume not supported.
		return false
	}
	return v.Major >= min
}

// Require returns a descriptive error when the server does not support f.
// Use this in the differ to fail loud rather than emit DDL the server will reject.
func (v Version) Require(f Feature) error {
	if v.Supports(f) {
		return nil
	}
	min := featureMinVersion[f]
	return fmt.Errorf("pg-flux: %s requires PostgreSQL %d+; connected server is %s", f, min, v)
}

// Detect queries the connected server for its version using SHOW server_version_num.
// Returns Zero{} and an error when the query fails (caller may choose to proceed
// in offline / unit-test contexts). Enforces MinSupportedMajor.
func Detect(ctx context.Context, pool *pgxpool.Pool) (Version, error) {
	if pool == nil {
		return Zero, fmt.Errorf("pgver.Detect: nil pool")
	}
	var raw int
	// server_version_num is exposed as a text GUC; cast to int via the server.
	if err := pool.QueryRow(ctx, "SELECT current_setting('server_version_num')::int").Scan(&raw); err != nil {
		return Zero, fmt.Errorf("pgver.Detect: %w", err)
	}
	v := ParseServerVersionNum(raw)
	if v.Major > 0 && v.Major < MinSupportedMajor {
		return v, fmt.Errorf("pg-flux: unsupported PostgreSQL %s; minimum is %d", v, MinSupportedMajor)
	}
	return v, nil
}
