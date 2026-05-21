---
title: Integration recipes
group: Codegen
order: 5
description: Plug pg-flux-generated types into sqlx, pgx, Drizzle, Hono, Express, and more.
---

pg-flux gives you typed structs and interfaces. This page shows how to actually *use* them in the major Go and TypeScript stacks. Every recipe is real code we'd ship in production.

## Go

### sqlx — typed scans

[`jmoiron/sqlx`](https://github.com/jmoiron/sqlx) reads `db:"..."` tags into struct fields. pg-flux's default Go output is sqlx-ready out of the box.

`.pg-flux-codegen.yml`:

```yaml
outputs:
  - lang: go
    out: ./internal/db
    package: db
    orm_tags: sqlx
    omitempty: nullable
    functions: true
```

Generated code:

```go
type User struct {
  ID    int64  `db:"id"`
  Email string `db:"email"`
  Role  Role   `db:"role"`
}

type Role string
const (
  RoleAdmin  Role = "admin"
  RoleMember Role = "member"
)
func (e *Role) Scan(src any) error { /* ... */ }
func (e Role)  Value() (driver.Value, error) { return string(e), nil }
```

Usage:

```go
package main

import (
  "context"
  "github.com/jmoiron/sqlx"
  _ "github.com/jackc/pgx/v5/stdlib"
  "myapp/internal/db"
)

func GetUser(ctx context.Context, conn *sqlx.DB, id int64) (db.User, error) {
  var u db.User
  err := conn.GetContext(ctx, &u, "SELECT id, email, role FROM users WHERE id = $1", id)
  return u, err
}

func ListAdmins(ctx context.Context, conn *sqlx.DB) ([]db.User, error) {
  var users []db.User
  err := conn.SelectContext(ctx, &users,
    "SELECT id, email, role FROM users WHERE role = $1", db.RoleAdmin)
  return users, err
}
```

The enum type implements `sql.Scanner` and `driver.Valuer` so it round-trips through `database/sql` transparently. You can `WHERE role = $1` with `db.RoleAdmin` and it works.

### pgx — typed scans

[`jackc/pgx`](https://github.com/jackc/pgx) doesn't read `db:` tags by default. Use the standard scanning patterns:

```go
package main

import (
  "context"
  "github.com/jackc/pgx/v5/pgxpool"
  "myapp/internal/db"
)

func GetUser(ctx context.Context, pool *pgxpool.Pool, id int64) (db.User, error) {
  var u db.User
  err := pool.QueryRow(ctx,
    "SELECT id, email, role FROM users WHERE id = $1", id,
  ).Scan(&u.ID, &u.Email, &u.Role)
  return u, err
}
```

For row-into-struct, use [`pgxscan`](https://github.com/georgysavva/scany) — it reads `db:` tags the same way sqlx does:

```go
import "github.com/georgysavva/scany/v2/pgxscan"

var users []db.User
err := pgxscan.Select(ctx, pool, &users, "SELECT * FROM users")
```

### gorm — auto-migrate alternative

If you've inherited a gorm codebase and want to switch to pg-flux for migrations but keep gorm for queries:

```yaml
outputs:
  - lang: go
    out: ./internal/models
    package: models
    orm_tags: gorm
    omitempty: defaults
```

Generated code:

```go
type User struct {
  ID    int64  `db:"id" gorm:"column:id;primaryKey;not null" json:"id"`
  Email string `db:"email" gorm:"column:email;not null" json:"email"`
  Role  Role   `db:"role" gorm:"column:role;not null" json:"role"`
}

func (User) TableName() string { return "public.users" }
```

Usage:

```go
import "gorm.io/gorm"

func GetUser(db *gorm.DB, id int64) (models.User, error) {
  var u models.User
  err := db.First(&u, id).Error
  return u, err
}
```

> [!IMPORTANT]
> If you use gorm tags, **don't** also use gorm's `AutoMigrate`. pg-flux owns
> the schema; gorm should only query against it. Disable gorm's migration
> behavior entirely.

### bun — full ORM with codegen-friendly tags

```yaml
outputs:
  - lang: go
    out: ./internal/bunmodels
    package: bunmodels
    orm_tags: bun
```

Generated code:

```go
type User struct {
  ID    int64  `bun:"id,pk,autoincrement" json:"id"`
  Email string `bun:"email" json:"email"`
  Role  Role   `bun:"role" json:"role"`
}

func (User) TableName() string { return "public.users" }
```

Plays well with bun's relations and query builder.

## TypeScript

### postgres.js + zod — minimal, modern

[`porsager/postgres`](https://github.com/porsager/postgres) is the cleanest TS PG client. Pair with pg-flux's zod schemas for validation at API boundaries.

`.pg-flux-codegen.yml`:

```yaml
outputs:
  - lang: ts
    out: ./src/db
    column_case: camel
    bigint_as: number
    date_as: string
    null_style: optional
    enum_style: const-object
    branded_ids: true
    insert_update_helpers: true
    validators: zod
```

Server code (Hono example):

```ts
import postgres from "postgres";
import { Hono } from "hono";
import { UserSchema, type User, type UserId } from "./db";

const sql = postgres(process.env.DATABASE_URL!);
const app = new Hono();

app.get("/users/:id", async (c) => {
  const id = c.req.param("id") as unknown as UserId;
  const rows = await sql<User[]>`
    SELECT id, email, role FROM users WHERE id = ${id}
  `;
  if (rows.length === 0) return c.notFound();
  return c.json(rows[0]);
});

app.post("/users", async (c) => {
  const body = await c.req.json();
  // Validate at the API boundary
  const parsed = UserSchema.parse(body);
  const result = await sql<User[]>`
    INSERT INTO users ${sql(parsed)} RETURNING *
  `;
  return c.json(result[0]);
});

export default app;
```

The `UserSchema` from `validators.ts` parses raw JSON into the typed `User` shape. Mismatched fields throw at the boundary instead of silently corrupting your DB.

### Drizzle — as the query layer (not the schema)

If you want Drizzle's nice query syntax but pg-flux for schema + migrations:

```yaml
# .pg-flux-codegen.yml — separate output for Drizzle-friendly types
outputs:
  - lang: ts
    out: ./src/db
    column_case: snake     # Drizzle expects snake_case to match DB columns
    bigint_as: number
    null_style: union      # Drizzle uses `T | null` not optional
    branded_ids: false     # incompatible with Drizzle's strict typing
```

You define a Drizzle schema file that mirrors what pg-flux knows:

```ts
// src/db/schema-drizzle.ts (hand-written or generated by a small adapter script)
import { bigserial, pgEnum, pgTable, text, timestamp } from "drizzle-orm/pg-core";

export const userRole = pgEnum("user_role", ["admin", "member"]);

export const users = pgTable("users", {
  id: bigserial({ mode: "number" }).primaryKey(),
  email: text().notNull(),
  role: userRole().notNull().default("member"),
  createdAt: timestamp("created_at", { withTimezone: true }).notNull().defaultNow(),
});

export type User = typeof users.$inferSelect;
```

> [!NOTE]
> Drizzle has its own schema. Maintaining both pg-flux SQL and Drizzle TS is
> two sources of truth, which we generally argue against. If you do this,
> add a CI check that compares pg-flux's types vs Drizzle's `$inferSelect`
> shape and fails if they diverge.

### Express + zod validators

```ts
import express from "express";
import { UserSchema, InsertUserSchema, type User } from "./db";
import { db } from "./db-client";

const app = express();
app.use(express.json());

app.post("/users", async (req, res) => {
  const parsed = InsertUserSchema.safeParse(req.body);
  if (!parsed.success) {
    return res.status(400).json({ errors: parsed.error.errors });
  }
  const user = await db.users.insert(parsed.data);
  res.json(user);
});

app.get("/users/:id", async (req, res) => {
  const user = await db.users.findById(req.params.id);
  if (!user) return res.status(404).end();
  // No need to revalidate on response — types prove the shape
  const validated: User = UserSchema.parse(user);
  res.json(validated);
});
```

### Next.js — server actions

```tsx
// app/users/actions.ts
"use server";

import { sql } from "@/lib/db";
import { InsertUserSchema, type User } from "@/db";

export async function createUser(formData: FormData) {
  const parsed = InsertUserSchema.parse({
    email: formData.get("email"),
    role: formData.get("role"),
  });

  const [user] = await sql<User[]>`
    INSERT INTO users (email, role) VALUES (${parsed.email}, ${parsed.role})
    RETURNING *
  `;
  return user;
}

// app/users/page.tsx
import { createUser } from "./actions";

export default function NewUserPage() {
  return (
    <form action={createUser}>
      <input name="email" type="email" required />
      <select name="role" defaultValue="member">
        <option value="admin">Admin</option>
        <option value="member">Member</option>
      </select>
      <button type="submit">Create</button>
    </form>
  );
}
```

### tRPC

```ts
// server/routers/users.ts
import { z } from "zod";
import { router, procedure } from "../trpc";
import { UserSchema, InsertUserSchema } from "@/db";
import { db } from "@/db-client";

export const usersRouter = router({
  byId: procedure
    .input(z.object({ id: z.number() }))
    .output(UserSchema.nullable())
    .query(async ({ input }) => {
      return db.users.findById(input.id);
    }),

  create: procedure
    .input(InsertUserSchema)
    .output(UserSchema)
    .mutation(async ({ input }) => {
      return db.users.insert(input);
    }),
});
```

The pg-flux schemas plug directly into tRPC's input/output validation. No duplicated schema definitions.

## Working with views

Generated view types are emitted in `views.go` / `views.ts`. They look identical to table types but represent read-only rows:

```go
// views.go
type UserStat struct {
  ID        *int64  `db:"id"`
  PostCount *int64  `db:"post_count"`
  Display   *string `db:"display"`
}
```

```ts
// views.ts
export interface UserStat {
  id: number | null;
  post_count: number | null;
  display: string | null;
}
```

> [!NOTE]
> View columns are conservatively typed as nullable because `pg_attribute`
> can't tell us which join branches might emit NULL. If your view's query
> guarantees a column is never null, use a comment hint to force the type:
> `COMMENT ON COLUMN user_stats.id IS 'pg-flux: tstype=number gotype=int64';`

## Working with functions

When `functions: true` is set in the codegen config, pg-flux emits `Params` and `Result` types for every user-defined function and procedure:

```go
// functions.go
type CalcScoreParams struct {
  UserID int64  `db:"user_id"`
  Weight string `db:"weight"`
}
type CalcScoreResult struct {
  Score string `db:"score"`
  Tier  string `db:"tier"`
}
```

Calling the function from Go:

```go
var result db.CalcScoreResult
err := pool.QueryRow(ctx,
  "SELECT * FROM calc_score($1, $2)",
  params.UserID, params.Weight,
).Scan(&result.Score, &result.Tier)
```

The types ensure your arguments and result columns line up with the function's signature. If the function changes, regeneration catches the mismatch at compile time, not runtime.

## Mixing typed and untyped queries

For one-off queries that don't fit the generated types, fall back to the driver's native API:

```go
// Typed (most queries)
var u db.User
err := pool.QueryRow(ctx, "SELECT * FROM users WHERE id = $1", id).Scan(&u.ID, &u.Email, &u.Role)

// Untyped (ad-hoc aggregate)
var totalUsers int64
err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE role = 'admin'").Scan(&totalUsers)
```

pg-flux doesn't try to make every possible query typed — that's sqlc territory. We give you the table-row shapes; you use whatever scanner / query library you want.

## What pg-flux doesn't help with

Some integration questions are outside the typed-schema scope:

- **Connection pooling strategy** — use pgx pool / sqlx open with reasonable max connections
- **Transactional patterns** — your query library's transaction API
- **Bulk operations** — pgx's COPY or your library's batch insert
- **Migration deployment automation** — see [CI/CD integration](/docs/ci-cd.html)
- **Schema versioning across services** — use the version field in your schema's metadata table

We give you the types and trust you with how you wire the rest.
