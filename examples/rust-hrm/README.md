# rust-hrm

> Multi-tenant HR Management in **Rust** (Actix-web · sqlx · pg-flux)

The most comprehensive pg-flux example. Every feature covered by the other examples
plus a set of capabilities that appear here for the first time:

| Feature | Where |
|---|---|
| `daterange` type | `positions.valid_during`, `leave_requests.during` |
| `tstzrange` type | `shifts.during` |
| `pg_trgm` extension + GIN trigram index | `idx_employees_name_trgm` on `full_name` |
| Window function `rank() OVER (...)` in materialized view | `department_stats` |
| Two EXCLUDE constraints in the same schema | `positions_no_overlap`, `shifts_no_overlap` |
| Deferrable FK | `leave_requests.approved_by DEFERRABLE INITIALLY DEFERRED` |
| Covering index (INCLUDE) | `idx_employees_email_cover INCLUDE (first_name, last_name, status)` |
| NULLS NOT DISTINCT unique index | `idx_employees_phone_unique` |
| Self-referential table | `departments.parent_id → departments.id` |
| Stored procedure (`CALL`) | `public.onboard_employee(...)` |
| SECURITY DEFINER audit function | `audit.log_change()` |
| Multiple schemas tracked | `public` + `audit` |
| pg-flux Rust codegen | `gen/` — tables · enums · views · types · functions |

## Stack

- **Actix-web 4** — HTTP server
- **sqlx 0.8** — async PostgreSQL, compile-time query verification
- **Tokio** — async runtime
- **serde** — JSON serialization
- **pg-flux** — declarative schema, migration generation, Rust codegen

## Schema overview

```
departments          (self-referential hierarchy, IDENTITY PK, JSONB)
   └─ employees      (UUID PK, domains, composite type, generated cols,
                       trigram GIN, tsvector GIN, NULLS NOT DISTINCT)
         ├─ positions   (daterange, EXCLUDE no-overlap)
         ├─ shifts      (tstzrange, EXCLUDE no-overlap)
         └─ leave_requests (daterange, deferrable FK)

attendance           (partitioned by RANGE(punch_in), BRIN index)
audit.change_log     (BRIN, cross-schema, SECURITY DEFINER trigger)

Views:
  employee_directory    (security_invoker = true)
  department_stats      (MATERIALIZED, window function rank() OVER ...)
```

## Quick start

```bash
# 1. Start a PostgreSQL 14-18 server

# 2. Create the database
createdb pgflux_rust_hrm
psql pgflux_rust_hrm -c "CREATE SCHEMA IF NOT EXISTS audit;"

# 3. Copy .env.example → .env and set DATABASE_URL

# 4. Apply migrations
pg-flux migrate apply

# 5. Run pg-flux gen to (re)generate Rust types
pg-flux gen --lang rust --functions --out gen/

# 6. Start the server
cargo run
```

## API

| Method | Path | Description |
|---|---|---|
| `GET`  | `/departments?org_id=1` | List departments (ordered by depth) |
| `POST` | `/departments` | Create department |
| `GET`  | `/departments/stats?org_id=1` | Department stats (materialized view, window function) |
| `GET`  | `/employees?org_id=1[&search=alice]` | List / full-text search employees |
| `POST` | `/employees` | Create employee |
| `POST` | `/employees/onboard` | Onboard via stored procedure (employee + first position) |
| `GET`  | `/employees/:id` | Get single employee |
| `GET`  | `/employees/:id/shifts` | List shifts (tstzrange) |
| `POST` | `/employees/:id/shifts` | Create shift (EXCLUDE guards overlap) |
| `GET`  | `/employees/:id/leave` | List leave requests (daterange) |

## Key pg-flux commands

```bash
pg-flux migrate generate   # diff schema/ vs live → write migrations/
pg-flux migrate apply      # apply pending migrations
pg-flux drift              # detect source ↔ live divergence
pg-flux verify             # detect undeclared live objects
pg-flux gen --lang rust --functions --out gen/  # regenerate Rust types
```
