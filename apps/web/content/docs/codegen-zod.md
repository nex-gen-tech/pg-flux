---
title: zod validators
group: Codegen
order: 3
---

# Runtime validation with zod

Set `validators: zod` to emit a parallel `validators.ts` with `z.object` schemas for every table, enum, composite type, and view:

```yaml
outputs:
  - lang: ts
    out: ./src/db
    validators: zod
```

Or via CLI:

```bash
pg-flux gen --lang ts --validators=zod
```

## What's emitted

```ts
// validators.ts (generated)
import { z } from "zod";

export const UserRoleSchema = z.enum(["admin", "member", "guest"]);

export const AddressSchema = z.object({
  street: z.string(),
  city: z.string(),
  zip: z.string(),
});

export const UserSchema = z.object({
  id: z.bigint(),
  email: z.string(),
  role: UserRoleSchema,        // ← references the enum schema
  created_at: z.date(),
  display_name: z.string().nullable(),
});
```

The schemas honour your other emit options:

| Option | Effect on zod schemas |
|---|---|
| `bigint_as: number` | `id: z.number()` instead of `z.bigint()` |
| `bigint_as: string` | `id: z.string()` |
| `date_as: string` | `created_at: z.string()` |
| `column_case: camel` | `display_name` becomes `displayName` (matches the type) |

## Using the schemas

Validate untrusted input at boundaries:

```ts
import { UserSchema, type User } from "./db";

app.post("/users", async (c) => {
  const body = await c.req.json();
  const result = UserSchema.parse(body);    // throws on invalid; returns User on success
  await db.insert("users", result);
  return c.json(result);
});
```

For partial validation (e.g. `PATCH /users/:id`):

```ts
const PartialUserSchema = UserSchema.partial();
const patch = PartialUserSchema.parse(body);
```

## Custom JSON shapes

When a `jsonb` column has a known shape, declare it via the column COMMENT hint:

```sql
COMMENT ON COLUMN posts.metadata IS 'pg-flux: tstype={ source: string; ip?: string }';
```

The TS interface uses the typed shape:

```ts
interface Post {
  metadata?: { source: string; ip?: string };
}
```

The zod schema can't auto-derive the shape from the comment, so it falls back to:

```ts
metadata: z.any() /* TODO: tstype hint, no schema generated */.nullable(),
```

Replace by hand or via a custom export in your project.

## Function & procedure schemas

Currently zod schemas are emitted for tables, enums, and composite types. Function `<Name>Params` / `<Name>Result` types are emitted as TS but not as zod schemas — most function calls receive validated input from API layers that already have their own schemas.

If you need them, file an issue with the use case.

## See also

- [Codegen overview →](/docs/codegen.html)
- [Function signatures →](/docs/codegen-functions.html)
