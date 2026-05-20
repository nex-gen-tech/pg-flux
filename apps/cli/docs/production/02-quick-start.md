# 02 — Quick Start

---

## Step 1 — Initialise a project directory

```
myapp/
├── schema/          ← your .sql source files go here
├── migrations/      ← generated migration files (commit these)
└── .pg-flux.yml     ← project configuration
```

Create the config file:

```yaml
# .pg-flux.yml
db: postgres://pgflux:pgflux@localhost:5432/mydb
schema: ./schema
migrations_dir: ./migrations
schemas: public
tracking_schema: _pgflux
```

---

## Step 2 — Write your first schema file

```sql
-- schema/users.sql
CREATE TYPE public.user_role AS ENUM ('admin', 'member', 'guest');

CREATE TABLE public.users (
  id         bigserial    PRIMARY KEY,
  email      varchar(254) NOT NULL,
  role       user_role    NOT NULL DEFAULT 'member',
  created_at timestamptz  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_users_email ON public.users (email);
```

---

## Step 3 — Generate a migration

```bash
pg-flux migrate generate
```

Output:

```
Generated: migrations/20260428_120000.sql (3 statements)
```

Review the file:

```sql
BEGIN;

-- [1] CREATE_TYPE: public.user_role
DO $pgflux$ BEGIN
  CREATE TYPE public.user_role AS ENUM ('admin', 'member', 'guest');
EXCEPTION WHEN duplicate_object THEN NULL;
END $pgflux$;

-- [2] CREATE_TABLE: public.users
CREATE TABLE IF NOT EXISTS public.users (
  id         bigserial    PRIMARY KEY,
  email      varchar(254) NOT NULL,
  role       user_role    NOT NULL DEFAULT 'member',
  created_at timestamptz  NOT NULL DEFAULT now()
);

-- [3] CREATE_INDEX: public.idx_users_email
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON public.users (email);

COMMIT;
```

---

## Step 4 — Apply the migration

```bash
pg-flux migrate apply
```

Output:

```
apply 20260428_120000.sql ...
      ok

Applied 1 migration(s), 0 already up to date.
```

---

## Step 5 — Make a schema change

Add a column to `users.sql`:

```sql
  role       user_role    NOT NULL DEFAULT 'member',
  bio        text,                             ← add this
  created_at timestamptz  NOT NULL DEFAULT now(),
```

Generate again:

```bash
pg-flux migrate generate --label add_bio
# Generated: migrations/20260428_120100_add_bio.sql (1 statements)
```

The generated migration contains only the delta:

```sql
BEGIN;

-- [1] ADD_COLUMN: public.users.bio
ALTER TABLE public.users ADD COLUMN bio text;

COMMIT;
```

---

## Step 6 — Verify no drift

After applying, re-running generate should produce nothing:

```bash
pg-flux migrate generate
# No changes detected — no migration generated.
```

---

## Handling Hazardous Changes

If you change a column type, pg-flux blocks the generation by default:

```
ERROR: plan contains blocking hazard COLUMN_TYPE_CHANGE on public.users.bio
  Use --allow-hazards COLUMN_TYPE_CHANGE to proceed
```

Allow it explicitly when you have reviewed the migration:

```bash
pg-flux migrate generate --allow-hazards COLUMN_TYPE_CHANGE
```

For incompatible type changes (e.g. `boolean` → `enum`), annotate the column with a USING expression:

```sql
-- @using CASE is_verified WHEN TRUE THEN 'verified'::public.verification_status
--             ELSE 'unverified'::public.verification_status END
is_verified public.verification_status NOT NULL DEFAULT 'unverified',
```

pg-flux will emit:

```sql
ALTER TABLE public.users
  ALTER COLUMN is_verified DROP DEFAULT;

ALTER TABLE public.users
  ALTER COLUMN is_verified
  SET DATA TYPE public.verification_status
  USING CASE is_verified
    WHEN TRUE THEN 'verified'::public.verification_status
    ELSE 'unverified'::public.verification_status
  END;

ALTER TABLE public.users
  ALTER COLUMN is_verified SET DEFAULT 'unverified';
```
