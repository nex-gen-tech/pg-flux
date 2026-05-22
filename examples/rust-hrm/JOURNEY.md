# rust-hrm — Build journey

The goal was a single example that exercises every feature covered by the four
existing examples *plus* the capabilities unique to a Rust/pg-flux workflow.

## Schema decisions

### `daterange` (new in this example)

`positions.valid_during` and `leave_requests.during` use **`daterange`** rather
than individual `start_date`/`end_date` columns. Benefits:
- `&&` operator detects overlap in one expression
- `EXCLUDE USING gist (employee_id WITH =, valid_during WITH &&)` prevents two
  active positions for the same employee without application-layer checks
- `isempty(during)` check prevents zero-length leave requests at the DB level

The go-shop uses `tstzrange` for price rules; `daterange` was unused across all
prior examples.

### `pg_trgm` trigram index (new)

HR apps commonly need fuzzy search: "find employees named 'jon'" matching
"John", "Jonathan", etc. The trigram GIN index

```sql
CREATE INDEX idx_employees_name_trgm ON public.employees
  USING GIN (full_name gin_trgm_ops);
```

makes `full_name % 'jon'` (similarity operator) and `full_name ILIKE '%jon%'`
fast via the index rather than a sequential scan. Combined with the existing
`search_vector` GIN for ranked full-text search, this gives two complementary
search paths.

### Window function in materialized view (new)

```sql
rank() OVER (PARTITION BY org_id ORDER BY employee_count DESC) AS size_rank
```

The `department_stats` materialized view uses a window function to rank each
department by headcount within its organisation. This is the first example to
use window functions; the aggregate `count(...) FILTER (WHERE ...)` pattern was
already in `go-events/event_stats`.

### Multiple EXCLUDE constraints (new)

Both `positions` (daterange) and `shifts` (tstzrange) carry their own EXCLUDE
constraint. The go-shop had one EXCLUDE on `price_rules`. Having two in the same
schema confirms pg-flux tracks them independently and neither causes perpetual
drift.

### Deferrable FK

`leave_requests.approved_by DEFERRABLE INITIALLY DEFERRED` lets a caller who is
bulk-importing employees insert both the requestor and the approver in the same
transaction and set the FK check to happen at COMMIT time. The same pattern was
in `go-events/attendees`; here it appears in a different semantic context.

### `pg_trgm` operator class in index definition

pg-flux's differ reads indexes via `pg_get_indexdef()` and compares against the
parsed source. The operator class `gin_trgm_ops` is preserved in
`pg_get_indexdef()` output, so the comparison is stable. No drift observed after
initial apply.

### Functions with user-defined type parameters

`pg_get_functiondef()` strips schema qualification from type names in both
parameter lists and `RETURNS TABLE` columns — `public.employee_status` becomes
`employee_status`. The source files must match: using unqualified names keeps
drift clean without any special handling in the differ.

### Self-referential table

`departments.parent_id REFERENCES public.departments(id)` is a simple
self-reference. The `depth` column is app-managed (incremented on insert via
CTE). No recursive CTE is stored in the schema itself, keeping pg-flux's job
straightforward (just a regular FK).

### Composite type as column

`employees.address public.mailing_address` stores a structured address. sqlx
doesn't automatically decode PG composite types, so handlers that need the
address extract it as JSON: `row_to_json(address)`. The generated `types.rs`
documents the struct shape for out-of-band deserialization.

## Rust codegen

Running:
```
pg-flux gen --lang rust --functions --out gen/
```
produced six files:

| File | Contents |
|---|---|
| `enums.rs` | `EmployeeStatus`, `PositionLevel`, `LeaveType`, `ShiftStatus` with `sqlx::Type` derives and per-variant `#[sqlx(rename)]` |
| `tables.rs` | `Department`, `Employee`, `Position`, `Shift`, `LeaveRequest`, `Attendance`, `ChangeLog` structs |
| `views.rs` | `EmployeeDirectory`, `DepartmentStat` structs |
| `types.rs` | `MailingAddress` composite struct; `EmailAddress`, `PhoneNumber` newtypes |
| `functions.rs` | `GetDepartmentEmployeesResult`, `CountActiveEmployeesRow`, procedure param structs |
| `mod.rs` | `pub mod` barrel |

The generated types are included into `src/main.rs` via `include!()` so they
live in the same crate. In a production project you would put them in a
dedicated `db_types` crate.
