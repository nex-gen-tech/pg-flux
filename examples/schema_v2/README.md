# schema_v2 — feature stress fixtures

Each `.sql` file exercises one feature group added in the Phase 2–4 expansion.
Use this directory as the declarative source for stress tests against PG14–18.

| File | Feature group | Min PG |
|------|---------------|--------|
| `comments.sql` | COMMENT ON every supported object kind | 14 |
| `identity.sql` | GENERATED ALWAYS / BY DEFAULT AS IDENTITY columns | 14 |
| `function_metadata.sql` | VOLATILITY / SECURITY / PARALLEL / LEAKPROOF / COST / ROWS / SET search_path | 14 |
| `constraint_modes.sql` | DEFERRABLE INITIALLY DEFERRED FK, FK MATCH FULL | 14 |
| `column_attrs.sql` | COLLATE / STORAGE / COMPRESSION (LZ4) | 14 |
| `unlogged_and_reloptions.sql` | UNLOGGED tables + WITH (fillfactor = …) | 14 |
| `view_options.sql` | WITH (check_option, security_barrier, security_invoker) | 14 (15+ for security_invoker) |
| `sequence_advanced.sql` | CREATE SEQUENCE AS integer, OWNED BY column | 14 |
| `pg15_features.sql` | NULLS NOT DISTINCT UNIQUE | 15 |
| `pg18_features.sql` | virtual generated columns, NOT ENFORCED | 18 |

The differ fails loud when a fixture uses a feature unsupported on the target
server (e.g. running `pg18_features.sql` against PG17 errors out with a message
naming the feature and the minimum required version).
