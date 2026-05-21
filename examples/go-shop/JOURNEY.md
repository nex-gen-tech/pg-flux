# Journey log

This file is the running log of the build of this example. It exists so contributors can read the friction a real user hits — not the polished marketing story.

The schema in this example was designed to exercise every DDL feature pg-flux supports: two schemas (`public` and `audit`), domains, composite types, enums, partitioned tables, EXCLUDE constraints, BRIN/GIN/INCLUDE/NULLS NOT DISTINCT indexes, SECURITY DEFINER functions and triggers, stored procedures, materialized and regular views with `security_invoker`, RLS, and grants. Every issue below was hit live during that build.

## What worked, with no friction

### Domains (`CREATE DOMAIN`)

`public.email_address` and `public.positive_amount` were declared in `schema/domains.sql`. The migration emitted them correctly as typed domains with named constraints:

```sql
DO $pgflux$ BEGIN
  CREATE DOMAIN public.email_address AS text CONSTRAINT email_format CHECK (value ~ '^[^@]+@[^@]+$');
EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;

DO $pgflux$ BEGIN
  CREATE DOMAIN public.positive_amount AS numeric(12, 2) CONSTRAINT positive CHECK (value > 0);
EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;
```

Domain columns (e.g., `email public.email_address`, `price public.positive_amount`) round-tripped through the differ cleanly. `verify` and `drift` both treat domain-typed columns correctly — no false positives.

`pg-flux gen --lang go` generated:
```go
// EmailAddress mirrors domain public.email_address over text.
type EmailAddress = string

// PositiveAmount mirrors domain public.positive_amount over numeric(12,2).
type PositiveAmount = string
```

Type aliases (not distinct types) — correct for domain types that are transparent wrappers.

### Composite type (`CREATE TYPE ... AS (...)`)

`public.address` was declared as a composite type with 5 fields. The migration emitted it correctly:

```sql
DO $pgflux$ BEGIN
  CREATE TYPE public.address AS (line1 text, city text, state text, zip text, country text);
EXCEPTION WHEN duplicate_object THEN NULL; END $pgflux$;
```

The `shipping_addr public.address` column in `customers` was generated as `shipping_addr public.address` in the migration and applied cleanly.

`pg-flux gen --lang go` generated the composite type as a struct in `gen/types.go`:
```go
// Address mirrors composite type public.address.
type Address struct {
    Line1   string `db:"line1" json:"line1"`
    City    string `db:"city" json:"city"`
    State   string `db:"state" json:"state"`
    Zip     string `db:"zip" json:"zip"`
    Country string `db:"country" json:"country"`
}
```

The `Customer` struct then uses `*Address` for the nullable `shipping_addr` column:
```go
ShippingAddr *Address `db:"shipping_addr" json:"shipping_addr"`
```

Composite type columns round-trip cleanly through `drift`.

### Enums

All three enums (`order_status`, `product_status`, `customer_tier`) generated correctly. Enum additions are handled correctly:

```sql
ALTER TYPE public.order_status ADD VALUE IF NOT EXISTS 'refunded' AFTER 'cancelled';
```

The `IF NOT EXISTS` guard is generated automatically — making the migration safe to re-run.

### EXCLUDE constraint (`EXCLUDE USING gist`)

`price_rules_no_overlap` was declared:
```sql
CONSTRAINT price_rules_no_overlap
  EXCLUDE USING gist (product_id WITH =, valid_during WITH &&)
```

The migration correctly included this constraint in the `CREATE TABLE` body. After apply, the constraint is confirmed in the catalog and `drift` did not flag this constraint — EXCLUDE constraints round-trip cleanly.

### BRIN indexes

Three BRIN indexes were declared across `public.customers`, `public.orders` (partitioned table root), and `audit.change_log`. All three were emitted correctly with `CONCURRENTLY IF NOT EXISTS`. `drift` showed no false positives.

### INCLUDE columns in index

```sql
CREATE INDEX idx_products_sku_name ON public.products (sku) INCLUDE (name, price);
```

Both `name` and `price` (a domain-typed column) are preserved in INCLUDE. `drift` showed no false positives.

### GIN indexes

Four GIN indexes across three tables generated and applied cleanly on `jsonb`, `text[]`, and `tsvector` (generated column) columns. No false positives.

### Partitioned table (`PARTITION BY RANGE`)

`public.orders` was declared with `PARTITION BY RANGE (created_at)` and three partitions. pg-flux generated the parent table and emitted the partition creation as RAW_DDL:

```sql
CREATE TABLE IF NOT EXISTS public.orders (...) PARTITION BY RANGE (created_at);
CREATE TABLE IF NOT EXISTS public.orders_2024 PARTITION OF public.orders
  FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
```

Partition children are correctly captured as `ExtraDDL`. The `verify` command recognized partition children as declared and did not report them as undeclared live objects. `drift` did not flag partition children as drift.

Indexes on the partitioned root automatically propagate to all partitions; the auto-created child indexes are not flagged as drift.

### `tsvector GENERATED ALWAYS AS ... STORED` column

Generated correctly in the migration and populated automatically on insert.

### Cross-schema (`target_schemas: [public, audit]`)

With `target_schemas: [public, audit]` in `.pg-flux.yml`, pg-flux tracked objects in both schemas:
- `audit.change_log` table and its indexes appeared in the initial migration
- `audit.log_change()` function appeared as `CREATE_FUNCTION`
- `verify` returned clean for both schemas — no undeclared objects in either

### `SECURITY DEFINER` function round-trip

`audit.log_change()` and `public.calculate_order_total()` were declared with `SECURITY DEFINER`. The migrations preserved this attribute. After apply, `prosecdef = t` is confirmed for both functions. `drift` did not report `SECURITY DEFINER` as drifted.

### `CREATE PROCEDURE` — stored procedure (not function)

`public.process_order` was declared with `CREATE OR REPLACE PROCEDURE`. The migration correctly emitted `CREATE OR REPLACE PROCEDURE`:

```sql
CREATE OR REPLACE PROCEDURE public.process_order(p_order_id bigint) LANGUAGE plpgsql AS ...
```

Note: the migration comment says `CREATE_FUNCTION` even though the DDL is `CREATE OR REPLACE PROCEDURE`. The operation type label is wrong but the DDL is correct. After apply, `prokind = 'p'` confirms this is a procedure.

### `SECURITY INVOKER` view option (PG15+)

```sql
CREATE OR REPLACE VIEW public.active_products
  WITH (security_invoker = true)
AS ...
```

The `security_invoker=true` option is preserved in the migration and confirmed in `pg_class.reloptions`.

### Materialized view — `pg-flux gen --lang go` integration

`public.product_catalog` was declared as a materialized view. The gen output placed it in `gen/views.go` with pointer types (nullable columns from the LEFT JOIN). `drift` did not flag the materialized view body.

### `UNIQUE NULLS NOT DISTINCT` (PG15+)

`NULLS NOT DISTINCT` is preserved correctly and does not drift.

### Column rename with `@renamed`

```sql
-- @renamed from=display_name
full_name text NOT NULL,
```

Generated `ALTER TABLE public.customers RENAME COLUMN display_name TO full_name;`. Data preserved. No `DROP + ADD` pattern.

### View false drift — fixed

`active_products` with `security_invoker=true` does NOT appear in `drift` output despite PostgreSQL normalizing view bodies in the catalog. No workaround needed.

### Inline unnamed CHECK constraints — fixed (B4)

```sql
qty int NOT NULL CHECK (qty > 0),
```

The migration correctly includes the inline CHECK constraint with an auto-generated name:
```sql
qty int NOT NULL,
CONSTRAINT order_items_qty_check CHECK (qty > 0)
```

### GRANT to PUBLIC — fixed (B5)

```sql
GRANT SELECT ON public.products TO PUBLIC;
GRANT SELECT ON public.active_products TO PUBLIC;
GRANT SELECT ON public.product_catalog TO PUBLIC;
```

All three GRANT statements appear in the generated migration and are applied correctly. After apply, `relacl = {pgflux=arwdDxtm/pgflux,=r/pgflux}` confirms the PUBLIC SELECT grant.

## Real bugs hit (all fixed)

### B1 — FK to partitioned table generates per-partition ghost constraints ✅ FIXED

PostgreSQL 17 automatically creates per-partition FK clones (`conparentid != 0`) for FKs to partitioned tables. These are not user-declared. The inspector was loading them, causing pg-flux to generate spurious `DROP CONSTRAINT` statements on every run.

**Fix:** Added `AND c.conparentid = 0` to the `pg_constraint` query in `mergeTableConstraints`. Inspector now only loads root (non-inherited) constraints.

### B2 — Stored procedure re-emitted on every `migrate generate` ✅ FIXED

`pg_get_functiondef()` returns `IN param_name type` but source omits the `IN` mode keyword, causing fingerprint mismatch.

**Fix:** `reParamModeIn` regex strips `IN ` prefix from procedure parameter modes in the ExtraDDL fingerprint.

### B3 — `CREATE SCHEMA` in RAW_DDL re-emitted on every drift/generate run ✅ FIXED

pg-flux now tracks `CREATE SCHEMA` as first-class objects, inspects whether the schema exists in the live DB via `pg_namespace`, and does not emit the statement if already present.

### B4 — Inline (unnamed) column CHECK constraints silently dropped ✅ FIXED

The source parser now captures inline column-level CHECK constraints and auto-generates names following PostgreSQL's `<table>_<col>_check` convention.

### B5 — `GRANT ... TO PUBLIC` never emitted in migrations ✅ FIXED

`grants.sql` sorts alphabetically before `products.sql` and `views.sql`. When processed, the target objects didn't exist yet in the schema state, so grants were silently discarded.

**Fix:** Added `PendingGrants` second-pass in `LoadDesiredState`, matching the existing `PendingRLS`/`PendingAlterPolicy` patterns. Grants that fail to resolve on first pass are buffered and applied after all schema files are loaded.

### B6 — Enum cast in partial index predicate causes perpetual drift ✅ FIXED

PostgreSQL stores `WHERE status = 'active'::product_status` while source has `WHERE status = 'active'`. The differ now strips user-defined type casts from index predicates before comparing, so bare string literals match the catalog form.

## Drift and verify — final state

```
No drift.
verify: clean — every live object is declared in source.
```

Both `drift` and `verify` exit 0 cleanly. All six bugs are fixed. The project uses `drift` as a hard CI gate.

The following all drift cleanly:
- BRIN, GIN, INCLUDE indexes
- NULLS NOT DISTINCT unique index
- EXCLUDE constraint
- Partial indexes with bare string literals
- All three triggers (customers_set_updated_at, products_set_updated_at, products_audit)
- SECURITY DEFINER functions
- RLS and policies
- Composite type column
- Domain-typed columns
- tsvector generated column
- Materialized view (product_catalog)
- Regular view with security_invoker (active_products)
- Enum additions
- Column additions and rename
- GRANT to PUBLIC on tables and views
- Stored procedure (no perpetual re-emit)
- CREATE SCHEMA (no perpetual re-emit)
- FK to partitioned table (no ghost inherited constraints)

## Polish-level issues remaining

- Migration comment says `CREATE_FUNCTION public.process_order(bigint)` even when the DDL is `CREATE OR REPLACE PROCEDURE`. The operation label is wrong — it should be `CREATE_PROCEDURE`. (Cosmetic only — DDL is correct.)
- `ADVISORY COLUMN_REORDER` for `public.customers` and `public.products` appears in migrations generated after columns are added (since `ADD COLUMN` appends to the end but source declares a specific order). This is documented as G7 in `docs/spec/1/spec.md`.
- The initial migration must be generated against an already-created `audit` schema. This happens because `CREATE SCHEMA` goes through as `RAW_DDL` and RAW_DDL is sorted to the end of the migration — after the `audit.change_log` table. Any project with a custom schema in `target_schemas` that doesn't already exist must pre-create it manually before the first `migrate generate`.

## Bottom line

This is the most feature-rich schema tested against pg-flux: 2 schemas, 7 tables (including a 3-partition partitioned table), 3 enums, 2 domains, 1 composite type, 2 views (regular + materialized), 2 functions + 1 procedure, 3 triggers, 2 RLS policies, 23 indexes, and 3 grants.

The generate/apply pipeline handled everything correctly after the six bugs documented above were fixed. Single clean migration (58 statements, no workarounds). `drift` and `verify` both exit 0.

This example was built against pg-flux built from the current main branch on PostgreSQL 17.
