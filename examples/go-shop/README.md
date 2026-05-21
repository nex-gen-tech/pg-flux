# go-shop — pg-flux comprehensive example

A multi-schema e-commerce backend that exercises every DDL feature pg-flux supports. Two schemas: `public` (storefront) and `audit` (change log).

## What this example covers

- `CREATE EXTENSION` (`btree_gist`, `pgcrypto`)
- `CREATE SCHEMA` (`audit`)
- `CREATE DOMAIN` (`email_address`, `positive_amount`)
- `CREATE TYPE AS (...)` composite type (`address`)
- `CREATE TYPE AS ENUM` (3 enums) + `ALTER TYPE ADD VALUE` (enum evolution)
- `GENERATED ALWAYS AS IDENTITY` primary keys
- `bigserial` primary key (categories)
- Composite type column (`shipping_addr public.address`)
- Domain-typed columns (`email public.email_address`, `price public.positive_amount`)
- `tsvector GENERATED ALWAYS AS ... STORED` column
- `PARTITION BY RANGE` table with 3 child partitions
- `EXCLUDE USING gist` constraint (requires btree_gist)
- Cross-schema references (`audit.change_log`, `audit.log_change()`)
- `SECURITY DEFINER` function (`audit.log_change`, `calculate_order_total`)
- `CREATE OR REPLACE PROCEDURE` (`process_order`)
- `CREATE OR REPLACE VIEW WITH (security_invoker = true)`
- `CREATE MATERIALIZED VIEW`
- `CREATE TRIGGER` (3 triggers: 2 set_updated_at, 1 audit)
- `ALTER TABLE ... ENABLE ROW LEVEL SECURITY` + `CREATE POLICY`
- `GRANT ... TO PUBLIC`
- BRIN, GIN, GIST, INCLUDE, partial, and NULLS NOT DISTINCT indexes
- Column add (`phone`, `cost`)
- Column rename (`display_name` → `full_name` via `@renamed from=display_name`)
- Enum value add (`order_status` gains `'refunded'`)

## Stack

- Go + [chi](https://github.com/go-chi/chi) HTTP router
- [pgx/v5](https://github.com/jackc/pgx) PostgreSQL driver
- Port 8014
- pg-flux for schema management and Go type generation

## Setup

```bash
# Install pg-flux
curl -sSfL https://raw.githubusercontent.com/nex-gen-tech/pg-flux/main/install.sh | sh

# Create database
createdb pgflux_go_shop
export DATABASE_URL=postgres://localhost/pgflux_go_shop?sslmode=disable

# Apply all migrations
pg-flux migrate apply

# Verify schema
pg-flux drift
pg-flux verify

# Generate Go types
pg-flux gen --lang go --out gen/
```

## Routes

| Method | Path | Description |
|--------|------|-------------|
| POST | `/customers` | Create customer |
| GET | `/customers/:id` | Get customer |
| POST | `/products` | Create product |
| GET | `/products/search?q=` | Full-text search via `search_vector` |
| GET | `/products` | List active products (uses `active_products` view) |
| GET | `/products/:id/price` | Get current active price from `price_rules` |
| POST | `/orders` | Create order |
| POST | `/orders/:id/items` | Add item to order |
| POST | `/orders/:id/process` | `CALL public.process_order($1)` |
| GET | `/orders/:id` | Get order with computed total from `calculate_order_total()` |

## Generated types

`pg-flux gen --lang go --out gen/` generates four files:

- `gen/tables.go` — one struct per table (includes `audit.ChangeLog`)
- `gen/enums.go` — typed string constants for all three enums
- `gen/views.go` — `ActiveProduct` (regular view) and `ProductCatalog` (materialized view)
- `gen/types.go` — `Address` struct (composite type), `EmailAddress` and `PositiveAmount` type aliases (domains)

## Schema layout

```
schema/
├── extensions.sql     -- CREATE EXTENSION btree_gist, pgcrypto
├── schemas.sql        -- CREATE SCHEMA audit
├── domains.sql        -- email_address, positive_amount
├── types.sql          -- address composite + 3 enums
├── customers.sql      -- customers table + 4 indexes
├── categories.sql     -- categories (self-referential)
├── products.sql       -- products + 7 indexes (GIN, partial, INCLUDE, NULLS NOT DISTINCT)
├── price_rules.sql    -- price_rules + EXCLUDE USING gist constraint
├── orders.sql         -- partitioned orders + 3 child partitions + 3 indexes
├── order_items.sql    -- order_items with composite FK to orders
├── audit.sql          -- audit schema tables + SECURITY DEFINER trigger function
├── triggers.sql       -- set_updated_at + products audit trigger
├── views.sql          -- active_products (SECURITY INVOKER) + product_catalog (materialized)
├── functions.sql      -- calculate_order_total (SECURITY DEFINER) + process_order procedure
├── rls.sql            -- RLS + policies on products and orders
└── grants.sql         -- GRANT SELECT TO PUBLIC on products/views
```

## Known issues

See [JOURNEY.md](JOURNEY.md) for the full log. Key limitations:
- **B1**: FKs to partitioned tables generate per-partition inherited constraints that drift checker tries (and fails) to drop on every run.
- **B2**: Stored procedures are re-emitted in every `migrate generate` even when already applied.
- **B3**: `CREATE SCHEMA` in RAW_DDL re-appears in every `drift` and `migrate generate` run.
- **B4**: Inline (unnamed) column CHECK constraints are silently dropped by the source parser.
- **B5**: `GRANT ... TO PUBLIC` is never emitted in migrations — grants declared in source are parsed but discarded by the differ.
- **B6**: Partial indexes on enum columns require explicit `::type_name` casts in source to avoid perpetual drift.
