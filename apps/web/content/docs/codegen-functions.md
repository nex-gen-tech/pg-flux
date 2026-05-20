---
title: Function signatures
group: Codegen
order: 2
---

# Function & procedure types

Enable with `functions: true` in `.pg-flux-codegen.yml` or `--functions` on the CLI. Default is off because large schemas often have many helper functions developers don't want flooding generated output.

```bash
pg-flux gen --lang go,ts --functions
```

## What's emitted

For every user-defined function (and procedure) pg-flux generates:

| Signature | Emit |
|---|---|
| `RETURNS scalar` | `<Name>Params` struct + `<Name>Row` type alias |
| `RETURNS TABLE(...)` | `<Name>Params` struct + `<Name>Result` struct (with the OUT columns) |
| `RETURNS SETOF scalar` | `<Name>Params` struct + `<Name>Row` type alias |
| `RETURNS trigger` / `event_trigger` | (skipped — no application caller) |
| `RETURNS void` | `<Name>Params` only |
| **Procedure** | `<Name>Params` only |
| **Procedure, no args** | (nothing emitted) |
| Aggregate / window | (skipped — call from SQL, no shape) |

## Example

```sql
CREATE FUNCTION calc_score(user_id bigint, weight numeric DEFAULT 1.0)
  RETURNS TABLE(score numeric, tier text) ...;

CREATE FUNCTION length_of(s text) RETURNS integer ...;

CREATE PROCEDURE bump_role(user_id bigint, new_role user_role) ...;
```

Produces (TS with `column_case: camel`):

```ts
export interface CalcScoreParams {
  userId: bigint;
  weight?: string;          // HasDefault → optional
}
export interface CalcScoreResult {
  score: string;
  tier: string;
}

export interface LengthOfParams {
  s: string;
}
export type LengthOfRow = number;

export interface BumpRoleParams {
  userId: bigint;
  newRole: UserRole;
}
```

And Go (with `orm_tags: sqlx`):

```go
type CalcScoreParams struct {
    UserID int64  `db:"user_id"`
    Weight string `db:"weight"`
}
type CalcScoreResult struct {
    Score string `db:"score"`
    Tier  string `db:"tier"`
}

type LengthOfParams struct {
    S string `db:"s"`
}
type LengthOfRow = int32

type BumpRoleParams struct {
    UserID  int64    `db:"user_id"`
    NewRole UserRole `db:"new_role"`
}
```

## Defaults

PG `DEFAULT` clauses on parameters become **optional** in TS (`weight?: string`). In Go they remain present in the struct (Go has no syntax to mark a field "optional"); your call code passes the zero value or sets the field explicitly.

## OUT / INOUT / VARIADIC

| Mode | TS / Go behaviour |
|---|---|
| `IN` (default) | Goes into `<Name>Params` |
| `INOUT` | Goes into `<Name>Params` with comment marker |
| `VARIADIC` | Goes into `<Name>Params` with comment marker |
| `OUT` | Goes into `<Name>Result` (synthesised row type) |
| `TABLE` (column) | Goes into `<Name>Result` |

## Custom type resolution

When a function parameter or result column uses a user-defined type, the generated identifier is used:

```sql
CREATE FUNCTION users_count(role_filter user_role) RETURNS bigint ...;
```

→

```ts
export interface UsersCountParams {
  roleFilter: UserRole;     // not "string"
}
```

## See also

- [Codegen overview →](/docs/codegen.html)
- [zod validators →](/docs/codegen-zod.html)
