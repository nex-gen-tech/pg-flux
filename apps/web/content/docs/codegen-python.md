---
title: Python codegen
group: Codegen
order: 6
description: Generate Pydantic v2 models, enums, and helper types from your PostgreSQL schema for use with FastAPI, SQLAlchemy, psycopg, and more.
---

pg-flux generates **Pydantic v2 `BaseModel` classes** for every catalog object with a row shape: tables, views, composite types, domains, enums, and (optionally) function and procedure signatures. The output lands in a single `models.py` file that you import directly into your app.

## Quick start

```bash
pg-flux gen --lang python --out gen/
```

Or include it in a multi-output config:

```yaml
# .pg-flux-codegen.yml
outputs:
  - lang: python
    out: ./gen
    null_style: optional
    enum_style: strenum
    functions: true
    type_overrides:
      numeric: decimal.Decimal
```

The generated file requires **Python 3.11+** and **Pydantic v2**:

```bash
pip install pydantic>=2.0
```

## Configuration options

| Option | Type | Default | Description |
|---|---|---|---|
| `null_style` | `optional` \| `union` | `optional` | How nullable columns are expressed. `optional` → `Optional[T] = None`; `union` → `T \| None` (PEP 604, Python 3.10+). |
| `enum_style` | `strenum` \| `enum` | `strenum` | `strenum` → `class Foo(str, Enum)` (standard library); `enum` → plain `Enum` with string values. |
| `functions` | `bool` | `false` | Emit `TypedDict` params and result classes for every user-defined function and procedure. |
| `type_overrides` | `map[pgtype → python type]` | `{}` | Override the default PG-to-Python mapping for specific types. The value is a dotted import path, e.g. `decimal.Decimal`. |

## Type mapping

| PostgreSQL type | Python type | Notes |
|---|---|---|
| `int2` / `smallint` | `int` | |
| `int4` / `integer` | `int` | |
| `int8` / `bigint` | `int` | |
| `float4` / `real` | `float` | |
| `float8` / `double precision` | `float` | |
| `bool` / `boolean` | `bool` | |
| `text` / `varchar` / `char` | `str` | |
| `uuid` | `UUID` | from `uuid` |
| `timestamptz` | `datetime` | from `datetime`; always timezone-aware |
| `timestamp` | `datetime` | from `datetime`; naive |
| `date` | `date` | from `datetime` |
| `time` | `time` | from `datetime` |
| `interval` | `timedelta` | from `datetime` |
| `jsonb` / `json` | `dict[str, Any]` | |
| `bytea` | `bytes` | |
| `numeric` / `decimal` | `float` | override to `decimal.Decimal` via `type_overrides` |
| `inet` / `cidr` | `str` | |
| `macaddr` | `str` | |
| `_<type>` (any array) | `list[T]` | recursively resolved element type |

## Nullable columns

A column is nullable when it lacks `NOT NULL` or has an explicit `DEFAULT NULL`. pg-flux wraps the type according to `null_style`:

```python
# null_style: optional  (default)
class User(BaseModel):
    id: int
    email: str
    display_name: Optional[str] = None   # nullable

# null_style: union
class User(BaseModel):
    id: int
    email: str
    display_name: str | None = None      # nullable, PEP 604 syntax
```

The `= None` default allows constructing a model without the field. See [configuration options](#configuration-options) to switch between the two styles.

## Enums

PostgreSQL `ENUM` types become Python enums. With `enum_style: strenum` (default):

```python
class TodoPriority(str, Enum):
    low = "low"
    medium = "medium"
    high = "high"
```

With `enum_style: enum`:

```python
class TodoPriority(Enum):
    low = "low"
    medium = "medium"
    high = "high"
```

`str, Enum` is preferable for most use cases because Pydantic v2 and FastAPI serialise it correctly without extra configuration, and you can compare values against plain strings.

## Views

Views — including materialized views — are emitted as read-only `BaseModel` classes. Because the view's column nullability can't always be determined from the query plan, every field is `Optional`:

```python
class ActiveUserSummary(BaseModel):
    """Read-only view."""
    model_config = ConfigDict(frozen=True)

    user_id: Optional[int] = None
    email: Optional[str] = None
    post_count: Optional[int] = None
```

> [!NOTE]
> Views created with `SECURITY INVOKER` (PG 15+) enforce the calling user's privileges rather than the owner's. pg-flux emits a comment on the model noting this.

## Composite types

PostgreSQL composite types become nested `BaseModel` classes:

```sql
CREATE TYPE address AS (
  street text,
  city   text,
  zip    text
);
```

```python
class Address(BaseModel):
    street: Optional[str] = None
    city: Optional[str] = None
    zip: Optional[str] = None
```

When a table column references a composite type, the generated model uses the nested class directly:

```python
class Store(BaseModel):
    id: int
    name: str
    location: Optional[Address] = None
```

## Domains

PostgreSQL domains become `NewType` aliases:

```sql
CREATE DOMAIN email_address AS text
  CHECK (VALUE ~* '^[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}$');
```

```python
EmailAddress = NewType("EmailAddress", str)
```

Domain constraints are enforced at the database level; the Python alias exists for type-checking clarity. If you want runtime validation, use a `type_overrides` entry to point at a custom annotated type.

## Functions and procedures

Enable with `functions: true`. pg-flux emits a `TypedDict` for the parameters and a `BaseModel` for the result set:

```sql
CREATE FUNCTION search_users(query text, limit_n int DEFAULT 20)
  RETURNS TABLE(id bigint, email text, score float8) ...;

CREATE PROCEDURE archive_user(user_id bigint) ...;
```

```python
class SearchUsersParams(TypedDict, total=False):
    query: str          # required
    limit_n: int        # optional — has DEFAULT

class SearchUsersResult(BaseModel):
    id: int
    email: str
    score: float

class ArchiveUserParams(TypedDict):
    user_id: int
```

`TypedDict` with `total=False` makes parameters with `DEFAULT` values truly optional at the call site without requiring `None` values.

## Insert/Update helpers

pg-flux generates two companion models alongside every table model:

- **`<Table>Create`** — all columns except those that are server-managed (GENERATED ALWAYS, identity columns, columns with `DEFAULT` supplied by a sequence or `now()`). Use this to type POST request bodies.
- **`<Table>Update`** — same columns as `<Table>Create`, but every field is `Optional` so partial updates are expressible. Use this to type PATCH request bodies.

```python
class User(BaseModel):
    id: int
    email: str
    role: UserRole
    created_at: datetime

class UserCreate(BaseModel):
    email: str
    role: UserRole = UserRole.member   # has DEFAULT

class UserUpdate(BaseModel):
    email: Optional[str] = None
    role: Optional[UserRole] = None
```

`id` and `created_at` are excluded from both helpers because they are server-managed.

## ORM compatibility

To use the generated models with an ORM that yields row objects instead of dicts, enable `from_attributes`:

```python
class User(BaseModel):
    model_config = ConfigDict(from_attributes=True)

    id: int
    email: str
    role: UserRole
    created_at: datetime
```

### SQLAlchemy example

```python
from sqlalchemy.orm import Session
from gen.models import User

def get_user(db: Session, user_id: int) -> User:
    row = db.execute(
        text("SELECT * FROM users WHERE id = :id"), {"id": user_id}
    ).one()
    return User.model_validate(row._mapping)
```

### psycopg3 (row_factory) example

```python
import psycopg
from psycopg.rows import class_row
from gen.models import User

async with await psycopg.AsyncConnection.connect(dsn) as conn:
    async with conn.cursor(row_factory=class_row(User)) as cur:
        await cur.execute("SELECT * FROM users WHERE id = %s", (user_id,))
        user = await cur.fetchone()
```

## Type overrides

Override the default mapping for any PG type via config:

```yaml
outputs:
  - lang: python
    out: ./gen
    type_overrides:
      numeric: decimal.Decimal
      uuid: uuid.UUID          # already the default, shown for illustration
```

The value is a dotted module path. pg-flux resolves the import automatically and adds it to the `models.py` header:

```python
import decimal
import uuid

class Product(BaseModel):
    id: int
    price: decimal.Decimal    # was float before override
```

## Generated file structure

All output lands in a single `models.py`:

```python
# gen/models.py  (generated by pg-flux — do not edit)
from __future__ import annotations

from datetime import date, datetime, time, timedelta
from decimal import Decimal
from enum import Enum
from typing import Any, NewType, Optional, TypedDict
from uuid import UUID

from pydantic import BaseModel, ConfigDict

# ── Enums ──────────────────────────────────────────────────────────────────

class UserRole(str, Enum):
    admin = "admin"
    member = "member"
    guest = "guest"

# ── Composite types ────────────────────────────────────────────────────────

class Address(BaseModel):
    street: Optional[str] = None
    city: Optional[str] = None
    zip: Optional[str] = None

# ── Domains ────────────────────────────────────────────────────────────────

EmailAddress = NewType("EmailAddress", str)

# ── Tables ─────────────────────────────────────────────────────────────────

class User(BaseModel):
    id: int
    email: str
    role: UserRole
    created_at: datetime

class UserCreate(BaseModel):
    email: str
    role: UserRole = UserRole.member

class UserUpdate(BaseModel):
    email: Optional[str] = None
    role: Optional[UserRole] = None

# ── Views ──────────────────────────────────────────────────────────────────

class ActiveUserSummary(BaseModel):
    """Read-only view."""
    model_config = ConfigDict(frozen=True)
    user_id: Optional[int] = None
    email: Optional[str] = None

# ── Functions ──────────────────────────────────────────────────────────────

class SearchUsersParams(TypedDict, total=False):
    query: str

class SearchUsersResult(BaseModel):
    id: int
    email: str
    score: float
```

Import from the module in your application code:

```python
from gen.models import User, UserCreate, UserRole
```

## See also

- [Codegen overview →](/docs/codegen.html)
- [Function signatures →](/docs/codegen-functions.html)
- [Python codegen example (fastapi-todo) →](https://github.com/nex-gen-tech/pg-flux/tree/main/examples/fastapi-todo)
