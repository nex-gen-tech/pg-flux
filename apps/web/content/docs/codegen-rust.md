---
title: Rust codegen
group: Codegen
order: 7
description: Generate sqlx-compatible structs, enums, and helper types from your PostgreSQL schema for use with Actix-web, Axum, and any tokio-based stack.
---

pg-flux generates **`sqlx`-compatible Rust structs and enums** for every catalog object with a row shape. The output is a small module (`gen/`) with one file per object kind, wired together by a `mod.rs` you include in your project with a single line.

## Quick start

```bash
pg-flux gen --lang rust --functions --out gen/
```

Or via config:

```yaml
# .pg-flux-codegen.yml
outputs:
  - lang: rust
    out: ./gen
    functions: true
    type_overrides:
      numeric: rust_decimal::Decimal
```

## Required Cargo.toml dependencies

```toml
[dependencies]
sqlx = { version = "0.8", features = ["runtime-tokio-rustls", "postgres", "uuid", "chrono", "json"] }
serde = { version = "1", features = ["derive"] }
serde_json = "1"
uuid = { version = "1", features = ["v4", "serde"] }
chrono = { version = "0.4", features = ["serde"] }
tokio = { version = "1", features = ["full"] }
```

Optional, for `numeric` override:

```toml
rust_decimal = { version = "1", features = ["db-postgres"] }
```

## Configuration options

| Option | Type | Default | Description |
|---|---|---|---|
| `functions` | `bool` | `false` | Emit `Params` and `Result` structs for every user-defined function and procedure. |
| `type_overrides` | `map[pgtype → rust type]` | `{}` | Override the default PG-to-Rust mapping. Value is a fully-qualified Rust path, e.g. `rust_decimal::Decimal`. |

## Type mapping

| PostgreSQL type | Rust type | Notes |
|---|---|---|
| `int2` / `smallint` | `i16` | |
| `int4` / `integer` | `i32` | |
| `int8` / `bigint` | `i64` | |
| `float4` / `real` | `f32` | |
| `float8` / `double precision` | `f64` | |
| `bool` / `boolean` | `bool` | |
| `text` / `varchar` / `char` | `String` | |
| `uuid` | `uuid::Uuid` | requires `uuid` feature in sqlx |
| `timestamptz` | `chrono::DateTime<chrono::Utc>` | requires `chrono` feature |
| `timestamp` | `chrono::NaiveDateTime` | |
| `date` | `chrono::NaiveDate` | |
| `time` | `chrono::NaiveTime` | |
| `jsonb` / `json` | `serde_json::Value` | requires `json` feature |
| `bytea` | `Vec<u8>` | |
| `numeric` / `decimal` | `f64` | override to `rust_decimal::Decimal` via `type_overrides` |
| `inet` / `cidr` | `String` | |
| `interval` | `i64` | microseconds; wrap in `chrono::Duration` as needed |
| `_<type>` (any array) | `Vec<T>` | recursively resolved element type |

## Nullable columns

Nullable columns are wrapped in `Option<T>`:

```rust
pub struct User {
    pub id: i64,
    pub email: String,
    pub display_name: Option<String>,   // nullable
    pub role: UserRole,
}
```

`Option<T>` maps directly to sqlx's `NULL` handling — no extra configuration needed.

## Enums

PostgreSQL enums become Rust enums with the `sqlx::Type` derive and a `type_name` attribute that matches the PG type name. Per-variant renames handle PG's case-sensitive variant names:

```sql
CREATE TYPE todo_priority AS ENUM ('low', 'medium', 'high');
```

```rust
#[derive(Debug, Clone, PartialEq, sqlx::Type, serde::Serialize, serde::Deserialize)]
#[sqlx(type_name = "todo_priority", rename_all = "snake_case")]
pub enum TodoPriority {
    #[sqlx(rename = "low")]
    #[serde(rename = "low")]
    Low,
    #[sqlx(rename = "medium")]
    #[serde(rename = "medium")]
    Medium,
    #[sqlx(rename = "high")]
    #[serde(rename = "high")]
    High,
}
```

The `#[sqlx(type_name = "...")]` attribute is required for sqlx to serialise the value to the correct PG type OID. Without it, sqlx falls back to treating the value as `TEXT`.

## Views

View structs derive `sqlx::FromRow` and wrap every field in `Option<T>` because view column nullability cannot always be determined statically:

```rust
#[derive(Debug, Clone, sqlx::FromRow, serde::Serialize, serde::Deserialize)]
pub struct ActiveUserSummary {
    pub user_id: Option<i64>,
    pub email: Option<String>,
    pub post_count: Option<i32>,
}
```

## Composite types

PostgreSQL composite types become plain serde structs. They do not derive `sqlx::FromRow` because they're embedded inside rows, not queried directly:

```sql
CREATE TYPE address AS (
  street text,
  city   text,
  zip    text
);
```

```rust
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct Address {
    pub street: Option<String>,
    pub city: Option<String>,
    pub zip: Option<String>,
}
```

When a table column is of composite type, sqlx deserialises it from the PG composite literal via the `serde` impl.

## Domains

PostgreSQL domains become newtypes. The inner field is `pub` so you can construct and destructure freely:

```sql
CREATE DOMAIN email_address AS text;
```

```rust
#[derive(Debug, Clone, PartialEq, sqlx::Type, serde::Serialize, serde::Deserialize)]
#[sqlx(transparent)]
pub struct EmailAddress(pub String);
```

`#[sqlx(transparent)]` tells sqlx to treat the newtype identically to its inner type — no custom codec needed.

## Functions and procedures

Enable with `functions: true`. pg-flux emits `Params` and `Result` structs for every function and a `Params`-only struct for procedures:

```sql
CREATE FUNCTION search_users(query text, limit_n int DEFAULT 20)
  RETURNS TABLE(id bigint, email text, score float8) ...;

CREATE PROCEDURE archive_user(user_id bigint) ...;
```

```rust
pub struct SearchUsersParams {
    pub query: String,
    pub limit_n: Option<i32>,   // has DEFAULT → Option
}

#[derive(Debug, Clone, sqlx::FromRow, serde::Serialize, serde::Deserialize)]
pub struct SearchUsersResult {
    pub id: i64,
    pub email: String,
    pub score: f64,
}

pub struct ArchiveUserParams {
    pub user_id: i64,
}
```

Call the function with sqlx:

```rust
let rows = sqlx::query_as::<_, SearchUsersResult>(
    "SELECT * FROM search_users($1, $2)"
)
.bind(&params.query)
.bind(params.limit_n)
.fetch_all(&pool)
.await?;
```

## Module structure

pg-flux writes six files to the output directory:

| File | Contents |
|---|---|
| `tables.rs` | One struct per table, plus `FromRow`, `Serialize`, `Deserialize` derives |
| `enums.rs` | One enum per PG enum type, with `sqlx::Type`, `Serialize`, `Deserialize` |
| `views.rs` | One struct per view/matview, all fields `Option<T>`, `FromRow` derive |
| `types.rs` | Composite types (plain serde structs) and domain newtypes |
| `functions.rs` | Params and result structs for functions/procedures (when `functions: true`) |
| `mod.rs` | Re-exports from all of the above; the only file you need to touch |

To include the generated module in your project, add one line to `main.rs` (or `lib.rs`):

```rust
include!(concat!(env!("CARGO_MANIFEST_DIR"), "/gen/mod.rs"));
```

Or declare it as a module if the directory is under `src/`:

```rust
// src/main.rs
mod gen;
use gen::tables::User;
```

The `mod.rs` re-exports everything:

```rust
// gen/mod.rs  (generated)
pub mod tables;
pub mod enums;
pub mod views;
pub mod types;
pub mod functions;

pub use tables::*;
pub use enums::*;
pub use views::*;
pub use types::*;
pub use functions::*;
```

## Type overrides

Map a PG type to a custom Rust type via `type_overrides`:

```yaml
outputs:
  - lang: rust
    out: ./gen
    type_overrides:
      numeric: rust_decimal::Decimal
```

The value is a fully-qualified Rust path. pg-flux adds the necessary `use` statement at the top of the relevant file:

```rust
use rust_decimal::Decimal;

pub struct Product {
    pub id: i64,
    pub price: Decimal,    // was f64 before override
}
```

Make sure the crate is in your `Cargo.toml` and the feature flags match what sqlx expects (e.g. `rust_decimal = { version = "1", features = ["db-postgres"] }`).

## Fully-qualified paths

The generated code uses fully-qualified paths like `chrono::DateTime<chrono::Utc>` and `uuid::Uuid` rather than bare names with `use` statements at the file top. This is intentional: it avoids import conflicts when the generated code is included into an existing module that may already have its own `use chrono::DateTime` or `use uuid::Uuid` in scope.

If your project uses `clippy::pedantic` with the `use_self` lint, you can suppress it with `#[allow(clippy::use_self)]` on the generated structs, or simply re-export through a wrapper module that normalises the import style.

## See also

- [Codegen overview →](/docs/codegen.html)
- [Function signatures →](/docs/codegen-functions.html)
- [Rust codegen example (rust-hrm) →](https://github.com/nex-gen-tech/pg-flux/tree/main/examples/rust-hrm)
