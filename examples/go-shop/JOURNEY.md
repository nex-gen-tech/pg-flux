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

All three enums (`order_status`, `product_status`, `customer_tier`) generated correctly. The Wave 14 enum evolution (adding `'refunded'` to `order_status`) was handled correctly:

```sql
-- [9] RAW_DDL: raw
ALTER TYPE public.order_status ADD VALUE IF NOT EXISTS 'refunded' AFTER 'cancelled';
```

The `IF NOT EXISTS` guard is generated automatically — making the migration safe to re-run. The new value appeared in `gen/enums.go` after re-running `pg-flux gen`:

```go
OrderStatusRefunded OrderStatus = "refunded"
```

### EXCLUDE constraint (`EXCLUDE USING gist`)

`price_rules_no_overlap` was declared:
```sql
CONSTRAINT price_rules_no_overlap
  EXCLUDE USING gist (product_id WITH =, valid_during WITH &&)
```

The migration correctly included this constraint in the `CREATE TABLE` body:
```sql
CONSTRAINT price_rules_no_overlap EXCLUDE USING gist (product_id WITH =, valid_during WITH &&),
```

After apply, the constraint is confirmed in the catalog:
```
price_rules_no_overlap  | x | EXCLUDE USING gist (product_id WITH =, valid_during WITH &&)
```

`drift` did not flag this constraint after initial apply — EXCLUDE constraints round-trip cleanly.

### BRIN indexes

Three BRIN indexes were declared:
- `idx_customers_created ON public.customers USING BRIN (created_at)`
- `idx_orders_created_brin ON public.orders USING BRIN (created_at)` — on a partitioned table root
- `idx_change_log_date ON audit.change_log USING BRIN (changed_at)` — in `audit` schema

All three were emitted correctly as:
```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_customers_created ON public.customers USING brin (created_at);
```

The BRIN on the partitioned root creates BRIN indexes on each partition automatically. `drift` showed no false positives for BRIN indexes.

### INCLUDE columns in index

```sql
CREATE INDEX idx_products_sku_name ON public.products (sku) INCLUDE (name, price);
```

Generated:
```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_sku_name ON public.products USING btree (sku) INCLUDE (name, price);
```

Both `name` and `price` (a domain-typed column) are preserved in INCLUDE. `drift` showed no false positives.

### GIN indexes

Four GIN indexes across three tables generated and applied cleanly:
```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_customers_metadata ON public.customers USING gin (metadata);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_tags ON public.products USING gin (tags);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_attributes ON public.products USING gin (attributes);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_products_search ON public.products USING gin (search_vector);
```

GIN on `tsvector` (generated column), `jsonb`, and `text[]` all applied without error and drifted cleanly.

### Partitioned table (`PARTITION BY RANGE`)

`public.orders` was declared with `PARTITION BY RANGE (created_at)` and three partitions. pg-flux generated the parent table with the correct syntax and emitted the partition creation as RAW_DDL:

```sql
CREATE TABLE IF NOT EXISTS public.orders (...) PARTITION BY RANGE (created_at);
...
CREATE TABLE IF NOT EXISTS public.orders_2024 PARTITION OF public.orders
  FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
```

Partition children are correctly captured as `ExtraDDL` (not in `Tables`), which is the correct internal model. The `verify` command recognized partition children as declared and did not report them as undeclared live objects. `drift` did not flag the partition children as drift.

Indexes on the partitioned root (e.g., `idx_orders_status ON public.orders`) automatically propagate to all partitions; the auto-created child indexes (`orders_2024_status_created_at_idx`, etc.) are not flagged as drift.

### `tsvector GENERATED ALWAYS AS ... STORED` column

```sql
search_vector tsvector GENERATED ALWAYS AS (
  to_tsvector('english', coalesce(name,'') || ' ' || coalesce(description,''))
) STORED,
```

Generated correctly in the migration:
```sql
search_vector tsvector GENERATED ALWAYS AS (to_tsvector('english', (COALESCE(name, '') || ' ') || COALESCE(description, ''))) STORED,
```

The generated column populated automatically on insert and is queryable via `search_vector @@ plainto_tsquery(...)`.

### Cross-schema (`target_schemas: [public, audit]`)

With `target_schemas: [public, audit]` in `.pg-flux.yml`, pg-flux tracked objects in both schemas:
- `audit.change_log` table and its indexes appeared in the initial migration
- `audit.log_change()` function appeared as `CREATE_FUNCTION`
- `verify` returned clean for both schemas — no undeclared objects in either

### `SECURITY DEFINER` function round-trip

`audit.log_change()` and `public.calculate_order_total()` were declared with `SECURITY DEFINER`. The migrations preserved this attribute:

```sql
CREATE OR REPLACE FUNCTION audit.log_change() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER AS ...
CREATE OR REPLACE FUNCTION public.calculate_order_total(p_order_id bigint) RETURNS numeric LANGUAGE sql STABLE SECURITY DEFINER AS ...
```

After apply, `prosecdef = t` is confirmed for both functions. `drift` did not report `SECURITY DEFINER` as drifted.

### `CREATE PROCEDURE` — stored procedure (not function)

`public.process_order` was declared with `CREATE OR REPLACE PROCEDURE`. The migration correctly emitted `CREATE OR REPLACE PROCEDURE`:

```sql
-- [18] CREATE_FUNCTION: public.process_order(bigint)
CREATE OR REPLACE PROCEDURE public.process_order(p_order_id bigint) LANGUAGE plpgsql AS ...
```

Note: the migration comment says `CREATE_FUNCTION` even though the DDL is `CREATE OR REPLACE PROCEDURE`. The operation type label is wrong but the DDL is correct. After apply, `prokind = 'p'` confirms this is a procedure.

### `CALL` in Go (pgx)

pgx calls procedures via `CALL` using `Exec` — no special handling needed:
```go
_, err = h.pool.Exec(r.Context(),
    `CALL public.process_order($1)`,
    orderID,
)
```

### `SECURITY INVOKER` view option (PG15+)

```sql
CREATE OR REPLACE VIEW public.active_products
  WITH (security_invoker = true)
AS ...
```

Generated:
```sql
CREATE OR REPLACE VIEW public.active_products WITH (security_invoker=true) AS ...
```

The `security_invoker=true` option is preserved in the migration and confirmed in `pg_class.reloptions = {security_invoker=true}`.

### Materialized view — `pg-flux gen --lang go` integration

`public.product_catalog` was declared as a materialized view. The gen output placed it in `gen/views.go`:

```go
// ProductCatalog — Read-only row from materialized view public.product_catalog.
type ProductCatalog struct {
    ID           *int64          `db:"id" json:"id"`
    Sku          *string         `db:"sku" json:"sku"`
    ...
}
```

The materialized view struct correctly uses pointer types (nullable columns from the LEFT JOIN). `drift` did not flag the materialized view body — consistent with go-events findings.

### `UNIQUE NULLS NOT DISTINCT` (PG15+)

Declared on PG17:
```sql
CREATE UNIQUE INDEX idx_products_sku_nulls_nd ON public.products (sku) NULLS NOT DISTINCT;
```

Generated:
```sql
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_products_sku_nulls_nd ON public.products USING btree (sku) NULLS NOT DISTINCT;
```

`NULLS NOT DISTINCT` is preserved correctly and does not drift.

### Column rename with `@renamed`

```sql
-- @renamed from=display_name
full_name text NOT NULL,
```

Generated:
```sql
ALTER TABLE public.customers RENAME COLUMN display_name TO full_name;
```

Data preserved. No `DROP + ADD` pattern. Consistent with the go-events and express-bookmarks examples (after the B1 fix).

### View false drift — B3 is fixed

In fastapi-todo and express-bookmarks, `pg-flux drift` reported every regular `CREATE VIEW` as drifted on every run (because PostgreSQL normalizes view bodies in the catalog). This build confirms that fix is in place: `active_products` with `security_invoker=true` does NOT appear in `drift` output despite having a different body text in the catalog vs source. No workaround needed.

### `pg-flux gen --lang go` — enum names

All three enums generated correct names (no truncation):
- `CustomerTier` / `CustomerTierStandard`, `CustomerTierSilver`, `CustomerTierGold`, `CustomerTierPlatinum`
- `OrderStatus` / `OrderStatusPending` ... `OrderStatusRefunded`
- `ProductStatus` / `ProductStatusDraft`, `ProductStatusActive`, `ProductStatusArchived`

The `EventStatu` / `AttendeeStatu` truncation bug from go-events was fixed by `f47d1ce fix(codegen): exception list prevents singularizer mangling -status suffixes`. All three `-status` enums in this example are confirmed correct.

## Real bugs hit

### B1 — FK to partitioned table generates per-partition ghost constraints (critical)

`public.order_items` has a FK to the partitioned `public.orders`:
```sql
CONSTRAINT order_items_order_fkey FOREIGN KEY (order_id, order_date) REFERENCES public.orders (id, created_at)
```

PostgreSQL 17 automatically creates one FK constraint per partition:
```
order_items_order_id_order_date_fkey  → REFERENCES orders_2024(id, created_at)
order_items_order_id_order_date_fkey1 → REFERENCES orders_2025(id, created_at)
order_items_order_id_order_date_fkey2 → REFERENCES orders_2026(id, created_at)
```

These are **inherited constraints** (`coninhcount=1`, `conparentid=order_items_order_fkey`). They are not declared in source. pg-flux's drift checker sees them as undeclared constraints and generates:

```sql
ALTER TABLE public.order_items DROP CONSTRAINT IF EXISTS order_items_order_id_order_date_fkey;
ALTER TABLE public.order_items DROP CONSTRAINT IF EXISTS order_items_order_id_order_date_fkey1;
ALTER TABLE public.order_items DROP CONSTRAINT IF EXISTS order_items_order_id_order_date_fkey2;
```

When applied, PostgreSQL raises:
```
ERROR: cannot drop inherited constraint "order_items_order_id_order_date_fkey"
  of relation "order_items" (SQLSTATE 42P16)
```

This makes `migrate generate` produce broken migrations on every run after the initial schema. There is no workaround other than manually deleting these three statements from every generated migration.

**Proper fix in pg-flux:** The inspector or differ should filter out constraints where `coninhcount > 0` (inherited from parent constraint) and not include them in the "live but undeclared" set.

**Workaround in this example:** The three DROP CONSTRAINT lines were removed from migrations `20260521_224047_334_fix_partial_index_casts.sql` and `20260521_224221_122_schema_evolution.sql` and replaced with WORKAROUND comments. This makes `drift` permanently noisy (these three DROP_TABLE_CONSTRAINT lines appear on every `drift` run) but `migrate apply` works.

### B2 — Stored procedure re-emitted on every `migrate generate`

After `process_order` (a `CREATE OR REPLACE PROCEDURE`) is applied, `pg-flux drift` always reports:

```
CREATE_FUNCTION public.process_order(bigint): CREATE OR REPLACE PROCEDURE public.process_order(p_order_id bigint) ...
```

And `migrate generate` always includes the PROCEDURE re-creation in every subsequent migration. The procedure body is identical to what's in the DB — this is not a content change. The differ appears to treat procedures (`prokind = 'p'`) differently from functions (`prokind = 'f'`) in a way that prevents it from marking procedures as "already applied".

This means every `migrate generate` after the first `process_order` apply will include a spurious `CREATE OR REPLACE PROCEDURE` statement. It's idempotent (no harm) but noisy and confusing.

**Workaround in this example:** The spurious `CREATE OR REPLACE PROCEDURE` was removed from `20260521_224221_122_schema_evolution.sql` with a WORKAROUND comment.

**Proper fix in pg-flux:** The function/procedure differ should compare `prokind` and ensure procedures are fingerprinted distinctly. The `CREATE_FUNCTION` label on `CREATE OR REPLACE PROCEDURE` output (visible in migration comments) is also a symptom of treating both as the same object type.

### B3 — `CREATE SCHEMA` in RAW_DDL re-emitted on every drift/generate run

`schemas.sql` contains `CREATE SCHEMA IF NOT EXISTS audit;`. pg-flux does not track schemas as first-class objects — they end up as `ExtraDDL`. Because the ExtraDDL for `CREATE SCHEMA IF NOT EXISTS audit` is always emitted on generate, every subsequent `migrate generate` includes:

```sql
-- [N] RAW_DDL: raw
CREATE SCHEMA IF NOT EXISTS audit;
```

And `pg-flux drift` always shows:
```
RAW_DDL raw: CREATE SCHEMA IF NOT EXISTS audit
```

The `CREATE SCHEMA IF NOT EXISTS` is idempotent (it's safe to re-run) but it generates perpetual false drift and makes every migration file noisier than it should be.

**Workaround in this example:** The `CREATE SCHEMA IF NOT EXISTS audit` statement was left in the migrations as-is (it's idempotent). It appears on every drift run and in every generated migration.

**Proper fix in pg-flux:** pg-flux should track `CREATE SCHEMA` as a first-class object, inspect whether the schema exists in the live DB, and not emit it if already present.

### B4 — Inline (unnamed) column CHECK constraints are silently dropped

`audit.change_log.operation` was declared with an inline unnamed CHECK:
```sql
operation text NOT NULL CHECK (operation IN ('INSERT','UPDATE','DELETE')),
```

And `order_items.qty` was declared:
```sql
qty int NOT NULL CHECK (qty > 0),
```

Both CHECK constraints were silently dropped from the generated migration. The `CREATE TABLE` in the migration contains neither constraint. After apply, `\d audit.change_log` shows no check constraint on `operation`.

The error message when using an unnamed table-level constraint is helpful: *"named table constraints (CHECK, FOREIGN KEY, UNIQUE, EXCLUDE) require CONSTRAINT name"* — but this applies to table-level constraints, not inline column checks. Inline column-level CHECK constraints (without `CONSTRAINT name`) appear to be silently discarded by the parser.

**Workaround:** Add explicit `CONSTRAINT name CHECK (...)` to all CHECK constraints, both table-level and inline.

**Proper fix in pg-flux:** The source parser should capture inline column-level CHECK constraints and either warn when they are unnamed or synthesize a name.

### B5 — `GRANT ... TO PUBLIC` never emitted in migrations

`grants.sql` declares:
```sql
GRANT SELECT ON public.products TO PUBLIC;
GRANT SELECT ON public.active_products TO PUBLIC;
GRANT SELECT ON public.product_catalog TO PUBLIC;
```

pg-flux parses these into `Table.Privileges` internally (confirmed by `TestLoadDesiredState_GrantRevoke`). However, the differ never emits `GRANT` DDL in the generated migrations. After all three migrations were applied, `relacl` on all three objects is `NULL` — the grants were never applied.

```sql
SELECT relname, relacl FROM pg_class
WHERE relname IN ('products','active_products','product_catalog');
-- relacl is NULL for all three
```

Additionally, `drift` never reports missing grants as drift. The gap is silent: grants declared in source are parsed but then discarded by the differ.

**Workaround in this example:** Apply grants manually after `migrate apply`. They are not enforced by pg-flux.

**Proper fix in pg-flux:** The differ should compare `Table.Privileges` against live `pg_class.relacl` and emit `GRANT` or `REVOKE` DDL when they diverge.

### B6 — Enum cast in partial index predicate causes perpetual drift on first run

Declaring a partial index on an enum column with a bare string literal:
```sql
CREATE INDEX idx_products_status ON public.products (status) WHERE status = 'active';
```

PostgreSQL stores the predicate as:
```sql
WHERE (status = 'active'::product_status)
```

pg-flux's differ compares the source predicate against the catalog form and sees a difference, generating DROP + CREATE on every `drift` run. The fix (carried over from the `6f43a28` commit) requires writing the source predicate in the canonical PG form:

```sql
CREATE INDEX idx_products_status ON public.products (status) WHERE status = 'active'::product_status;
```

Note: using `'active'::public.product_status` (with schema prefix) still causes drift — only the unqualified form `'active'::product_status` matches what PostgreSQL stores.

A second migration (`20260521_224047_334_fix_partial_index_casts.sql`) was generated to rebuild the two affected indexes. After this fix, the indexes drift cleanly.

**Workaround in this example (B6):** Schema files use `status = 'active'::product_status` (unqualified cast). This is the canonical form required by the differ.

## Drift and verify — final state

### `pg-flux drift`

```
DROP_TABLE_CONSTRAINT public.order_items/order_items_order_id_order_date_fkey: ...
DROP_TABLE_CONSTRAINT public.order_items/order_items_order_id_order_date_fkey1: ...
DROP_TABLE_CONSTRAINT public.order_items/order_items_order_id_order_date_fkey2: ...
CREATE_FUNCTION public.process_order(bigint): CREATE OR REPLACE PROCEDURE ...
RAW_DDL raw: CREATE SCHEMA IF NOT EXISTS audit
exit: 1
```

Three persistent sources of drift, all documented above (B1, B2, B3). The project cannot use `drift` as a CI gate without `|| true` while B1 exists — the partition FK ghosts appear on every run.

The following all drift **cleanly** (no false positives):
- BRIN, GIN, INCLUDE indexes
- NULLS NOT DISTINCT unique index
- EXCLUDE constraint
- Partial indexes (after B5 fix)
- Triggers (all three: customers_set_updated_at, products_set_updated_at, products_audit)
- SECURITY DEFINER functions
- RLS and policies
- Composite type column
- Domain-typed columns
- tsvector generated column
- Materialized view (product_catalog)
- Regular view with security_invoker (active_products) — B3 from previous examples is fixed
- Enum additions (refunded value)
- Column additions and rename

### `pg-flux verify`

```
verify: clean — every live object is declared in source.
exit: 0
```

`verify` is fully clean. All live objects — including both schemas, all tables, the materialized view, the regular view, enum types, domain types, composite type, functions, procedure, triggers, RLS policies, and partition children — are correctly recognized as declared in source. `verify` can be used as a CI gate.

## Polish-level issues hit

- Migration comment says `CREATE_FUNCTION public.process_order(bigint)` even when the DDL is `CREATE OR REPLACE PROCEDURE`. The operation label is wrong — it should be `CREATE_PROCEDURE`.
- `ADVISORY COLUMN_REORDER` for `public.customers` and `public.products` appears in every migration generated after columns were added in schema evolution. The advisory is technically correct (column order diverges after `ADD COLUMN` adds to the end) but repeats on every migration for the rest of the build. This was documented as B3 in go-events; still not fixed.
- The initial migration placed `CREATE SCHEMA IF NOT EXISTS audit` at step [55] — after the `audit.change_log` table at step [9]. Because the `audit` schema didn't exist yet, `migrate apply` failed with `ERROR: schema "audit" does not exist`. Workaround: pre-create the schema manually before the first `migrate apply`. This happens because `CREATE SCHEMA` goes through as `RAW_DDL` and RAW_DDL is always sorted to the end of the migration. Any project with a custom schema in `target_schemas` that doesn't already exist must pre-create it manually before the first apply.

## Bottom line

This is the most feature-rich schema tested against pg-flux so far: 2 schemas, 7 tables (including a 3-partition partitioned table), 3 enums, 2 domains, 1 composite type, 2 views (regular + materialized), 2 functions + 1 procedure, 3 triggers, 2 RLS policies, and 23 indexes across every PG index type (btree, GIN, BRIN, GIST, partial, INCLUDE, NULLS NOT DISTINCT).

The generate/apply pipeline handled nearly everything correctly. The 6 bugs above represent the gaps:
- B1 (partition FK ghosts) is blocking for schemas with FKs to partitioned tables — every `migrate generate` produces broken statements that can't be applied.
- B2 (procedure perpetual re-creation) is noisy but idempotent.
- B3 (CREATE SCHEMA RAW_DDL) is noisy but idempotent.
- B4 (inline CHECK silently dropped) is a correctness gap — constraints declared in source don't end up in the DB.
- B5 (grants never emitted) is a correctness gap — grants declared in source are never applied.
- B6 (enum cast in partial index) requires a non-obvious workaround (explicit `::type_name` cast in source).

`verify` exits 0 cleanly — it can be used as a CI gate. `drift` exits 1 due to B1/B2/B3 until those are fixed.

This example was built in one sitting against pg-flux built from the current main branch on PostgreSQL 17, acting as a first-time user with no insider knowledge of the tool.
