---
title: TypeScript codegen
group: Codegen
order: 3
description: Generate TypeScript interfaces, enum constants, Zod validators, and branded ID types from your PostgreSQL schema for use with postgres.js, Drizzle, tRPC, and more.
---

pg-flux generates **TypeScript interfaces and type aliases** for every catalog object with a row shape: tables, views, composite types, domains, enums, and (optionally) function and procedure signatures. The output is a small module with one file per object kind and a barrel `index.ts` you import from directly.

## Quick start

```bash
pg-flux gen --lang ts --out ./src/db
```

Or include it in a multi-output config:

```yaml
# .pg-flux-codegen.yml
outputs:
  - lang: ts
    out: ./src/db
    column_case: camel
    null_style: optional
    enum_style: const-object
    branded_ids: true
    insert_update_helpers: true
    validators: zod
    functions: true
```

Then import from the barrel:

```ts
import type { User, InsertUser, UserRole } from './src/db';
```

## Configuration options

| Option | Type | Default | Description |
|---|---|---|---|
| `column_case` | `snake` \| `camel` \| `pascal` | `snake` | How PG column names map to interface property names. `camel` is idiomatic for TypeScript projects. |
| `bigint_as` | `bigint` \| `number` \| `string` | `bigint` | How `bigint` / `int8` / `bigserial` columns are typed. `bigint` — native JS bigint (cannot be serialised to JSON directly). `number` — JS number (loses precision above 2^53). `string` — string (safe for JSON and IDs). |
| `date_as` | `Date` \| `string` \| `temporal` | `Date` | How timestamp / date / time columns are typed. `Date` — JS Date object. `string` — ISO 8601 string (best for JSON APIs). `temporal` — TC39 Temporal API (`Temporal.Instant`; requires the `temporal-polyfill` package). |
| `null_style` | `union` \| `undefined` \| `optional` | `union` | How nullable columns are expressed. `union` → `field: T \| null`. `undefined` → `field: T \| undefined`. `optional` → `field?: T`. |
| `enum_style` | `union` \| `const-object` \| `ts-enum` | `union` | How PG enums are emitted. `union` — string literal union. `const-object` — `as const` object + derived type (tree-shakeable, no runtime overhead). `ts-enum` — TypeScript `enum` keyword (runtime object). |
| `branded_ids` | `bool` | `false` | Emit branded types for primary key columns to prevent accidentally passing a `PostId` where a `UserId` is expected. No-op when the table has a composite PK. |
| `insert_update_helpers` | `bool` | `false` | Emit `Insert<T>` and `Update<T>` utility types alongside every table interface for ergonomic write paths. |
| `validators` | `zod` \| `""` | `""` | Emit a `validators.ts` module with `z.object(...)` schemas for every table, enum, and composite type. |
| `functions` | `bool` | `false` | Emit `Params` and `Result`/`Row` interfaces for every user-defined function and procedure. |
| `json_shapes` | `map["schema.table.column" → TS type]` | `{}` | Replace `unknown` with a concrete TS type for specific `jsonb` columns whose runtime shape is known. |
| `type_overrides` | `map[pgtype → TS type]` | `{}` | Override the default PG-to-TS mapping for a specific type. The value is a verbatim TS type expression. |
| `readonly` | `none` \| `identity` \| `generated` \| `defaults` \| `all` | `none` | Prefix matching fields with the `readonly` keyword. `identity` — identity columns. `generated` — `GENERATED ALWAYS AS (...)` columns. `defaults` — identity, generated, and defaulted columns. `all` — every field. |

## Type mapping

| PostgreSQL type | TypeScript type | Notes |
|---|---|---|
| `int2` / `smallint` | `number` | |
| `int4` / `integer` | `number` | |
| `float4` / `real` | `number` | |
| `float8` / `double precision` | `number` | |
| `smallserial` / `serial` / `serial4` | `number` | |
| `int8` / `bigint` | `bigint` | configurable via `bigint_as` |
| `bigserial` / `serial8` | `bigint` | configurable via `bigint_as` |
| `bool` / `boolean` | `boolean` | |
| `text` / `varchar` / `char` / `citext` | `string` | |
| `uuid` | `string` | |
| `bytea` | `Uint8Array` | |
| `timestamptz` / `timestamp with time zone` | `Date` | configurable via `date_as` |
| `timestamp` / `timestamp without time zone` | `Date` | configurable via `date_as` |
| `date` | `Date` | configurable via `date_as` |
| `time` / `timetz` | `Date` | configurable via `date_as` |
| `interval` | `string` | ISO 8601 duration string |
| `jsonb` / `json` | `unknown` | use `json_shapes` to narrow specific columns |
| `numeric` / `decimal` | `string` | preserves precision; override via `type_overrides` |
| `inet` / `cidr` / `macaddr` | `string` | |
| `_<type>` (any array) | `T[]` | element type resolved recursively |

> [!NOTE]
> `jsonb` columns default to `unknown` rather than `any` so TypeScript forces you to narrow the type before use. Use `json_shapes` to declare the runtime shape for specific columns.

> [!NOTE]
> `bigint_as: bigint` (the default) cannot be serialised to JSON with `JSON.stringify` without a replacer. For REST APIs, `bigint_as: string` or `bigint_as: number` is usually more practical.

## Nullable columns

pg-flux supports three styles for nullable columns, controlled by `null_style`:

```ts
// null_style: union  (default)
export interface User {
  id: bigint;
  email: string;
  display_name: string | null;   // nullable
  deleted_at: Date | null;       // nullable
}

// null_style: undefined
export interface User {
  id: bigint;
  email: string;
  display_name: string | undefined;  // nullable
  deleted_at: Date | undefined;      // nullable
}

// null_style: optional
export interface User {
  id: bigint;
  email: string;
  display_name?: string;    // nullable — property is optional
  deleted_at?: Date;        // nullable — property is optional
}
```

`optional` (`?:`) is the most ergonomic for application code because you can omit the key rather than explicitly passing `null`. Use `union` if you need to distinguish between a missing key and a `null` value, or if you're serialising to a format where `null` is meaningful.

## Enums

PostgreSQL `ENUM` types can be emitted in three styles:

### union (default)

A string literal union — zero runtime overhead, fully tree-shakeable:

```sql
CREATE TYPE user_role AS ENUM ('admin', 'member', 'guest');
```

```ts
/** PG enum public.user_role */
export type UserRole =
  | "admin"
  | "member"
  | "guest";
```

### const-object

An `as const` object provides a values map and a derived type. Useful when you need to iterate over the values at runtime:

```ts
/** PG enum public.user_role */
export const UserRole = {
  Admin: "admin",
  Member: "member",
  Guest: "guest",
} as const;
export type UserRole = (typeof UserRole)[keyof typeof UserRole];
```

Usage:

```ts
function isAdmin(role: UserRole) {
  return role === UserRole.Admin;
}
```

### ts-enum

A TypeScript `enum` — runtime object with string values, familiar to Java/C# developers:

```ts
/** PG enum public.user_role */
export enum UserRole {
  Admin = "admin",
  Member = "member",
  Guest = "guest",
}
```

> [!NOTE]
> `ts-enum` creates a runtime object that is not tree-shaken as aggressively as the `union` or `const-object` styles. Prefer `union` for libraries and `const-object` when you need runtime value enumeration.

## Branded IDs

Enable `branded_ids: true` to get nominal ID types that prevent accidentally passing a `PostId` where a `UserId` is expected. pg-flux generates a separate `brands.ts` file:

```ts
// brands.ts  (generated)
export type PostId = number & { readonly __brand: "PostId" };
export type UserId = number & { readonly __brand: "UserId" };
```

The primary key field in the table interface then uses the branded type:

```ts
export interface Post {
  id: PostId;   // was: number
  title: string;
}

export interface User {
  id: UserId;   // was: number
  email: string;
}
```

TypeScript's structural type system means `PostId` and `UserId` are not assignable to each other, even though both are `number & { ... }`:

```ts
function getPost(id: PostId): Promise<Post> { ... }

const userId: UserId = 42 as UserId;
getPost(userId);  // Error: Argument of type 'UserId' is not assignable to parameter of type 'PostId'
```

> [!NOTE]
> Branded IDs require the consumer to cast raw values: `const id = 42 as PostId`. This is intentional — it forces explicit acknowledgement of the ID source.

## Insert/Update helpers

Enable `insert_update_helpers: true` to get `Insert<T>` and `Update<T>` utility types alongside every table interface. These are generated using `Omit` and `Partial` over the full interface:

```ts
export interface User {
  id: UserId;
  email: string;
  display_name?: string;   // nullable (optional)
  role: UserRole;
  readonly created_at: Date;  // server-managed
}

// Server-managed and nullable columns become optional in the Insert helper.
export type InsertUser = Omit<User, "display_name" | "created_at"> & Partial<Pick<User, "display_name" | "created_at">>;

// Every field is optional in the Update helper (for PATCH semantics).
export type UpdateUser = Partial<User>;
```

These types are especially useful as request body types in API handlers:

```ts
// POST /users
async function createUser(body: InsertUser): Promise<User> { ... }

// PATCH /users/:id
async function updateUser(id: UserId, body: UpdateUser): Promise<User> { ... }
```

## Views

Views — including materialized views — are emitted as read-only interfaces. Because view column nullability cannot always be determined statically, every property follows the configured `null_style` in the nullable form:

```ts
/** Read-only row from view public.active_user_summary. */
export interface ActiveUserSummary {
  user_id: bigint | null;
  email: string | null;
  post_count: number | null;
}
```

With `null_style: optional`:

```ts
export interface ActiveUserSummary {
  user_id?: bigint;
  email?: string;
  post_count?: number;
}
```

## Composite types

PostgreSQL composite types become interfaces:

```sql
CREATE TYPE address AS (
  street text,
  city   text,
  zip    text
);
```

```ts
/** Composite type public.address. */
export interface Address {
  street: string;
  city: string;
  zip: string;
}
```

When a table column references a composite type, the generated interface uses the nested type directly:

```ts
export interface Store {
  id: number;
  name: string;
  location: Address | null;
}
```

## Domains

PostgreSQL domains become type aliases:

```sql
CREATE DOMAIN email_address AS text
  CHECK (VALUE ~* '^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$');
```

```ts
/** Domain public.email_address over text. */
export type EmailAddress = string;
```

Domain constraints are enforced at the database level; the TypeScript alias exists for readability. Columns of this domain type use `EmailAddress` instead of `string` in the generated interface.

## Zod validators

Enable with `validators: zod`. pg-flux emits a `validators.ts` module with `z.object(...)` schemas for every table, enum, and composite type. The schemas honour `bigint_as` and `date_as` so runtime shapes match the static types:

```ts
// validators.ts  (generated)
import { z } from "zod";

export const UserRoleSchema = z.enum(["admin", "member", "guest"]);

export const AddressSchema = z.object({
  street: z.string(),
  city: z.string(),
  zip: z.string(),
});

export const UserSchema = z.object({
  id: z.number(),
  email: z.string(),
  displayName: z.string().nullable(),
  role: UserRoleSchema,
  createdAt: z.string(),   // date_as: string
  deletedAt: z.string().nullable(),
  metadata: z.unknown().nullable(),
});
```

Use the schemas in API route validation:

```ts
import { UserSchema } from './db/validators';

// Validate an incoming request body
app.post('/users', async (req, res) => {
  const result = UserSchema.safeParse(req.body);
  if (!result.success) {
    return res.status(400).json(result.error.format());
  }
  const user = result.data;
  // user is fully typed as User
});
```

Enum columns reference the generated enum schema automatically — e.g. `role: UserRoleSchema`. For `jsonb` columns with a `tstype=` comment hint, the validator emits `z.any() /* TODO */` as a placeholder for you to fill in.

> [!NOTE]
> The `zod` package must be installed separately: `npm install zod`

## Functions and procedures

Enable with `functions: true`. pg-flux emits a `Params` interface for input parameters and either a `Result` interface (for `RETURNS TABLE` / `OUT` parameters) or a `Row` type alias (for scalar returns):

```sql
CREATE FUNCTION search_users(query text, limit_n int DEFAULT 20)
  RETURNS TABLE(id bigint, email text, score float8) ...;

CREATE FUNCTION string_length(s text) RETURNS integer ...;

CREATE PROCEDURE archive_user(user_id bigint) ...;
```

```ts
/** Input parameters for public.search_users. */
export interface SearchUsersParams {
  query: string;
  limit_n?: number;   // has DEFAULT → optional
}

/** One row returned by public.search_users. */
export interface SearchUsersResult {
  id: bigint;
  email: string;
  score: number;
}

/** Input parameters for public.string_length. */
export interface StringLengthParams {
  s: string;
}

/** Scalar value returned by public.string_length. */
export type StringLengthRow = number;

/** Input parameters for public.archive_user. */
export interface ArchiveUserParams {
  user_id: bigint;
}
```

Parameters with `DEFAULT` values are marked `?:` optional. Procedures don't emit a result type.

## JSON shapes

`jsonb` columns default to `unknown`. Use `json_shapes` to declare the runtime shape of a specific column without modifying the database schema:

```yaml
outputs:
  - lang: ts
    out: ./src/db
    json_shapes:
      public.posts.metadata: "{ source: string; ip?: string }"
      public.users.settings: "UserSettings"
```

The generated interface uses the declared type instead of `unknown`:

```ts
export interface Post {
  id: bigint;
  title: string;
  metadata: { source: string; ip?: string } | null;   // was: unknown | null
}
```

You can also set this per-column via a PG comment hint (without touching the config):

```sql
COMMENT ON COLUMN posts.metadata IS 'pg-flux: tstype={ source: string; ip?: string }';
```

## Type overrides

Override the default mapping for any PG type via `type_overrides`:

```yaml
outputs:
  - lang: ts
    out: ./src/db
    type_overrides:
      numeric: string   # already the default; shown for illustration
      interval: number  # treat interval as milliseconds
```

The value is a verbatim TypeScript type expression:

```ts
export interface Job {
  id: bigint;
  name: string;
  duration: number;   // was: string before override; maps interval → number
}
```

## Generated file structure

pg-flux writes up to eight files to the output directory:

| File | Contents |
|---|---|
| `tables.ts` | One interface per table; `Insert<T>` / `Update<T>` helpers when `insert_update_helpers: true` |
| `enums.ts` | One type alias / const-object / enum per PG enum type |
| `views.ts` | One interface per view / matview; all fields nullable |
| `types.ts` | Composite type interfaces and domain type aliases |
| `functions.ts` | `Params` and `Result` / `Row` interfaces (when `functions: true`) |
| `validators.ts` | Zod `z.object(...)` schemas (when `validators: zod`) |
| `brands.ts` | Branded ID types (when `branded_ids: true`) |
| `index.ts` | Barrel re-exporting everything from all of the above |

The `index.ts` barrel always re-exports in dependency order (brands before tables, enums before tables):

```ts
// index.ts  (generated)
export * from './brands';
export * from './enums';
export * from './types';
export * from './tables';
export * from './views';
export * from './functions';
export * from './validators';
```

You only need to import from the barrel:

```ts
import type { User, InsertUser, UserRole, UserSchema } from './src/db';
```

## See also

- [Codegen overview →](/docs/codegen.html)
- [Zod validators →](/docs/codegen-zod.html)
- [Function signatures →](/docs/codegen-functions.html)
