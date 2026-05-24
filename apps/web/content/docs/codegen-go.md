---
title: Go codegen
group: Codegen
order: 2
description: Generate database/sql-compatible structs, enum types, and function signatures from your PostgreSQL schema for use with sqlx, pgx, GORM, and more.
---

pg-flux generates **Go structs** for every catalog object with a row shape: tables, views, composite types, domains, enums, and (optionally) function and procedure signatures. The output is a small package with one file per object kind that you import directly into your app.

## Quick start

```bash
pg-flux gen --lang go --out ./internal/db
```

Or include it in a multi-output config:

```yaml
# .pg-flux-codegen.yml
outputs:
  - lang: go
    out: ./internal/db
    package: db
    orm_tags: sqlx
    omitempty: nullable
    functions: true
    type_overrides:
      numeric: github.com/shopspring/decimal.Decimal
```

The default package name is `dbgen`. Set `package` to match your project layout.

## Required dependencies

The generated code uses only Go's standard library by default:

```go
import (
    "encoding/json"   // json.RawMessage — used for jsonb / json columns
    "time"            // time.Time — used for all date/time columns
)
```

When you use ORM-specific tags (`orm_tags: sqlx`), the struct tags tell the ORM how to map columns — no extra generated code is required. The packages you need depend on which ORM you use:

| ORM | Go module |
|---|---|
| sqlx | `github.com/jmoiron/sqlx` |
| GORM | `gorm.io/gorm` |
| bun | `github.com/uptrace/bun` |
| ent | `entgo.io/ent` |
| pgx | `github.com/jackc/pgx/v5` |

To use the `github.com/google/uuid.UUID` type for `uuid` columns (rather than the default `string`), add it to `type_overrides`:

```yaml
type_overrides:
  uuid: github.com/google/uuid.UUID
```

And add the dependency:

```bash
go get github.com/google/uuid
```

## Configuration options

| Option | Type | Default | Description |
|---|---|---|---|
| `package` | `string` | `dbgen` | The Go package name written to every generated file. |
| `orm_tags` | `sqlx` \| `gorm` \| `bun` \| `ent` \| `""` | `""` | ORM tag flavor. Empty (default) emits `db` + `json` tags. `sqlx` emits `db` only. `gorm` adds `gorm:` tags with `primaryKey`, `not null`, `default:...`. `bun` adds `bun:` tags with `pk`, `nullzero`. When any ORM mode is set, enums also get `Scan`/`Value` interface implementations. |
| `omitempty` | `nullable` \| `defaults` \| `all` \| `""` | `""` | Controls which `json` struct tags include `,omitempty`. `nullable` — only nullable columns. `defaults` — nullable columns plus columns with a server-side `DEFAULT`. `all` — every column. |
| `readonly` | `none` \| `identity` \| `generated` \| `defaults` \| `all` | `none` | Emits a `// readonly (server-managed)` comment on matching fields. `identity` — identity columns only. `generated` — `GENERATED ALWAYS AS (...)` columns. `defaults` — identity, generated, and defaulted columns. `all` — every field. |
| `column_case` | `snake` \| `camel` \| `pascal` | `snake` | Controls the value of `db` and `json` struct tags (not the Go field name, which is always PascalCase). `snake` → `created_at`. `camel` → `createdAt`. `pascal` → `CreatedAt`. |
| `functions` | `bool` | `false` | Emit `Params` and `Result`/`Row` types for every user-defined function and procedure. |
| `type_overrides` | `map[pgtype → Go type]` | `{}` | Override the default PG-to-Go mapping. The value is a fully-qualified Go type. Import paths are resolved automatically. |

## Type mapping

| PostgreSQL type | Go type | Notes |
|---|---|---|
| `int2` / `smallint` | `int16` | |
| `int4` / `integer` | `int32` | |
| `int8` / `bigint` | `int64` | |
| `smallserial` / `serial2` | `int16` | |
| `serial` / `serial4` | `int32` | |
| `bigserial` / `serial8` | `int64` | |
| `float4` / `real` | `float32` | |
| `float8` / `double precision` | `float64` | |
| `bool` / `boolean` | `bool` | |
| `text` / `varchar` / `char` / `citext` | `string` | |
| `uuid` | `string` | override to `github.com/google/uuid.UUID` via `type_overrides` |
| `timestamptz` / `timestamp with time zone` | `time.Time` | from `time`; use `time.UTC` when reading |
| `timestamp` / `timestamp without time zone` | `time.Time` | from `time`; naive (no TZ) |
| `date` | `time.Time` | from `time` |
| `time` / `timetz` | `time.Time` | from `time` |
| `interval` | `time.Duration` | from `time`; approximate — PG intervals can exceed Duration range |
| `jsonb` / `json` | `json.RawMessage` | from `encoding/json`; use with `json.Unmarshal` |
| `bytea` | `[]byte` | |
| `numeric` / `decimal` | `string` | override to `github.com/shopspring/decimal.Decimal` via `type_overrides` |
| `inet` / `cidr` / `macaddr` | `string` | |
| `_<type>` (any array) | `[]T` | element type resolved recursively |

> [!NOTE]
> `uuid` defaults to `string` to avoid a mandatory third-party dependency. If your project already uses `github.com/google/uuid`, add `uuid: github.com/google/uuid.UUID` to `type_overrides` for a richer type.

> [!NOTE]
> `numeric` / `decimal` defaults to `string` to preserve precision without requiring `github.com/shopspring/decimal`. Override when you need arithmetic on the values.

## Nullable columns

A column is nullable when it lacks `NOT NULL` or has an explicit `DEFAULT NULL`. pg-flux wraps the type in a pointer:

```go
// NOT NULL columns → value type
type User struct {
    ID    int64     // NOT NULL
    Email string    // NOT NULL

    // nullable columns → pointer types
    DisplayName *string          // nullable
    DeletedAt   *time.Time       // nullable
    Metadata    *json.RawMessage // nullable
}
```

Pointer types integrate directly with `database/sql`'s `NULL` scanning — no extra configuration needed for sqlx, pgx, or plain `database/sql`.

## Struct tags

### Default (no `orm_tags`)

Both `db` and `json` tags are emitted. The `db` tag is used by sqlx and similar packages:

```go
type User struct {
    ID          int64     `db:"id" json:"id"`
    Email       string    `db:"email" json:"email"`
    DisplayName *string   `db:"display_name" json:"display_name"`
    CreatedAt   time.Time `db:"created_at" json:"created_at"`
}
```

### sqlx mode (`orm_tags: sqlx`)

Only the `db` tag is emitted (sqlx ignores `json`):

```go
type User struct {
    ID          int64     `db:"id"`
    Email       string    `db:"email"`
    DisplayName *string   `db:"display_name"`
    CreatedAt   time.Time `db:"created_at"`
}
```

### GORM mode (`orm_tags: gorm`)

Adds `gorm:` tags with column constraints. Also emits a `TableName()` method on every struct:

```go
type User struct {
    ID          int64     `db:"id" gorm:"column:id;primaryKey;not null" json:"id"`
    Email       string    `db:"email" gorm:"column:email;not null" json:"email"`
    DisplayName *string   `db:"display_name" gorm:"column:display_name" json:"display_name,omitempty"`
    CreatedAt   time.Time `db:"created_at" gorm:"column:created_at;not null;default:now()" json:"created_at"`
}

// TableName implements the gorm table-name interface.
func (User) TableName() string { return "public.users" }
```

### bun mode (`orm_tags: bun`)

Emits `bun:` tags with `pk` and `nullzero` markers. Also emits `TableName()`:

```go
type User struct {
    ID          int64     `bun:"id,pk" json:"id"`
    Email       string    `bun:"email" json:"email"`
    DisplayName *string   `bun:"display_name,nullzero" json:"display_name"`
    CreatedAt   time.Time `bun:"created_at" json:"created_at"`
}

// TableName implements the bun table-name interface.
func (User) TableName() string { return "public.users" }
```

### omitempty

`omitempty` controls which `json` struct tags include `,omitempty`. With `omitempty: nullable`:

```go
type User struct {
    ID          int64     `db:"id" json:"id"`                           // NOT NULL → no omitempty
    Email       string    `db:"email" json:"email"`                     // NOT NULL → no omitempty
    DisplayName *string   `db:"display_name" json:"display_name,omitempty"`  // nullable → omitempty
    DeletedAt   *time.Time `db:"deleted_at" json:"deleted_at,omitempty"` // nullable → omitempty
}
```

## Enums

PostgreSQL `ENUM` types become Go string types with a constant block:

```sql
CREATE TYPE user_role AS ENUM ('admin', 'member', 'guest');
```

```go
// UserRole mirrors PG enum public.user_role.
type UserRole string

const (
    UserRoleAdmin  UserRole = "admin"
    UserRoleMember UserRole = "member"
    UserRoleGuest  UserRole = "guest"
)
```

When any `orm_tags` value is set, pg-flux also emits `sql.Scanner` and `driver.Valuer` implementations so the enum round-trips through `database/sql` transparently:

```go
// Scan implements sql.Scanner for UserRole.
func (e *UserRole) Scan(src any) error {
    switch v := src.(type) {
    case nil:
        return nil
    case string:
        *e = UserRole(v)
        return nil
    case []byte:
        *e = UserRole(string(v))
        return nil
    }
    return fmt.Errorf("unsupported scan type for UserRole: %T", src)
}

// Value implements driver.Valuer for UserRole.
func (e UserRole) Value() (driver.Value, error) { return string(e), nil }
```

These methods allow sqlx, GORM, bun, and `database/sql` to scan directly into `UserRole` columns without any additional codec registration.

## Views

Views — including materialized views — are emitted as read-only structs. Because view column nullability cannot always be determined statically, every field is a pointer:

```go
// Read-only row from view public.active_user_summary.
type ActiveUserSummary struct {
    UserID    *int64  `db:"user_id" json:"user_id"`
    Email     *string `db:"email" json:"email"`
    PostCount *int32  `db:"post_count" json:"post_count"`
}
```

> [!NOTE]
> Views created with `SECURITY INVOKER` (PG 15+) enforce the calling user's privileges. pg-flux emits a doc comment noting this.

## Composite types

PostgreSQL composite types become nested structs:

```sql
CREATE TYPE address AS (
  street text,
  city   text,
  zip    text
);
```

```go
// Address mirrors composite type public.address.
type Address struct {
    Street string `db:"street" json:"street"`
    City   string `db:"city" json:"city"`
    Zip    string `db:"zip" json:"zip"`
}
```

When a table column references a composite type, the generated struct uses the nested type directly:

```go
// Store mirrors public.stores.
type Store struct {
    ID       int64    `db:"id" json:"id"`
    Name     string   `db:"name" json:"name"`
    Location *Address `db:"location" json:"location"`
}
```

## Domains

PostgreSQL domains become type aliases:

```sql
CREATE DOMAIN email_address AS text
  CHECK (VALUE ~* '^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$');
```

```go
// EmailAddress mirrors domain public.email_address over text.
type EmailAddress = string
```

The `=` makes this a true Go type alias (not a new defined type), so `EmailAddress` and `string` are fully interchangeable. Constraints are enforced at the database level.

## Functions and procedures

Enable with `functions: true`. pg-flux emits a `Params` struct for input parameters and either a `Result` struct (for `RETURNS TABLE` / `OUT` parameters) or a `Row` type alias (for scalar returns):

```sql
CREATE FUNCTION search_users(query text, limit_n int DEFAULT 20)
  RETURNS TABLE(id bigint, email text, score float8) ...;

CREATE FUNCTION string_length(s text) RETURNS integer ...;

CREATE PROCEDURE archive_user(user_id bigint) ...;
```

```go
// SearchUsersParams are the input parameters for public.search_users.
type SearchUsersParams struct {
    Query   string `db:"query" json:"query"`
    LimitN  int32  `db:"limit_n" json:"limit_n"`
}

// SearchUsersResult is one row returned by public.search_users.
type SearchUsersResult struct {
    ID    int64   `db:"id" json:"id"`
    Email string  `db:"email" json:"email"`
    Score float64 `db:"score" json:"score"`
}

// StringLengthParams are the input parameters for public.string_length.
type StringLengthParams struct {
    S string `db:"s" json:"s"`
}

// StringLengthRow is the scalar value returned by public.string_length.
type StringLengthRow = int32

// ArchiveUserParams are the input parameters for public.archive_user.
type ArchiveUserParams struct {
    UserID int64 `db:"user_id" json:"user_id"`
}
```

Parameters with `DEFAULT` values are not marked optional in Go (unlike the TS emitter), but the comment in the config hints at it. Procedures don't emit a result type since they return no rows.

## ORM integration examples

### sqlx

```go
import (
    "context"
    "github.com/jmoiron/sqlx"
    "your/project/internal/db"
)

func GetUser(ctx context.Context, pool *sqlx.DB, id int64) (*db.User, error) {
    var u db.User
    err := pool.GetContext(ctx, &u,
        "SELECT * FROM users WHERE id = $1", id)
    if err != nil {
        return nil, err
    }
    return &u, nil
}

func ListUsers(ctx context.Context, pool *sqlx.DB) ([]db.User, error) {
    var users []db.User
    err := pool.SelectContext(ctx, &users, "SELECT * FROM users ORDER BY id")
    return users, err
}
```

### pgx v5

```go
import (
    "context"
    "github.com/jackc/pgx/v5"
    "your/project/internal/db"
)

func GetUser(ctx context.Context, conn *pgx.Conn, id int64) (*db.User, error) {
    rows, err := conn.Query(ctx, "SELECT * FROM users WHERE id = $1", id)
    if err != nil {
        return nil, err
    }
    u, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[db.User])
    return &u, err
}
```

### GORM

```go
import (
    "gorm.io/gorm"
    "your/project/internal/db"
)

func GetUser(g *gorm.DB, id int64) (*db.User, error) {
    var u db.User
    result := g.First(&u, id)
    return &u, result.Error
}

func ListActiveUsers(g *gorm.DB) ([]db.User, error) {
    var users []db.User
    result := g.Where("deleted_at IS NULL").Find(&users)
    return users, result.Error
}
```

### bun

```go
import (
    "context"
    "github.com/uptrace/bun"
    "your/project/internal/db"
)

func GetUser(ctx context.Context, bundb *bun.DB, id int64) (*db.User, error) {
    u := new(db.User)
    err := bundb.NewSelect().Model(u).Where("id = ?", id).Scan(ctx)
    return u, err
}
```

## Type overrides

Override the default mapping for any PG type via `type_overrides`:

```yaml
outputs:
  - lang: go
    out: ./internal/db
    type_overrides:
      numeric: github.com/shopspring/decimal.Decimal
      uuid: github.com/google/uuid.UUID
```

The value is a fully-qualified Go type. pg-flux resolves the import path automatically and adds the necessary `import` statement to the generated file:

```go
import (
    "github.com/google/uuid"
    "github.com/shopspring/decimal"
)

// Product mirrors public.products.
type Product struct {
    ID    uuid.UUID       `db:"id" json:"id"`
    Price decimal.Decimal `db:"price" json:"price"`   // was string before override
}
```

You can also override per-column via a PG comment hint (without touching the config):

```sql
COMMENT ON COLUMN posts.metadata IS 'pg-flux: gotype=*postsmeta.Metadata';
```

## Generated file structure

pg-flux writes up to five files to the output directory:

| File | Contents |
|---|---|
| `tables.go` | One struct per table, with `db` / `json` struct tags |
| `enums.go` | One type + const block per PG enum; `Scan`/`Value` when `orm_tags` is set |
| `views.go` | One struct per view / matview; all fields are pointer types |
| `types.go` | Composite type structs and domain type aliases |
| `functions.go` | `Params` and `Result`/`Row` types (only when `functions: true`) |

Every file begins with the same header:

```go
// Code generated by pg-flux. DO NOT EDIT.
// To regenerate, run: pg-flux gen

package db

import (
    "encoding/json"
    "time"
)
```

Import from the package in your application code:

```go
import "your/project/internal/db"

func handler(w http.ResponseWriter, r *http.Request) {
    var u db.User
    // ...
    json.NewEncoder(w).Encode(u)
}
```

## See also

- [Codegen overview →](/docs/codegen.html)
- [Function signatures →](/docs/codegen-functions.html)
