# Epic 5 — Codegen

**Priority:** P2 — ecosystem expansion; closes the gap for Python users.
**Spec ref:** [Spec 1 §Open Gaps G10](../spec.md)
**Roadmap ref:** [ROADMAP v0.3 — Python emitter](../../../ROADMAP.md)

---

## E5-T1 — Python type emitter (dataclasses + Pydantic v2)

**Summary:** `pg-flux gen` supports Go and TypeScript. The fastapi-todo example required hand-written Pydantic models, and any drift between `schema/*.sql` and `models.py` is invisible to the toolchain. A Python emitter closes this gap.

**Scope:** Tables (Pydantic v2 `BaseModel`) + enums (`python Enum`). No query generation (that's sqlc territory per ROADMAP "out of scope").

**Output layout:**
```
gen/
├── models.py        # one BaseModel per table + one Enum per type
```

Per-object layout (`--layout=per-object`) is a follow-on (see E5-T2).

**Acceptance criteria:**
- `pg-flux gen --lang python` emits `models.py` with:
  - One `class <TableName>(BaseModel)` per table, fields matching column names and PG→Python type mapping.
  - `Optional[T]` for nullable columns.
  - `datetime` for `timestamptz`/`timestamp`, `UUID` for `uuid`, `dict` for `jsonb`, `str` for `text`/`varchar`.
  - One `class <EnumName>(str, Enum)` per `CREATE TYPE ... AS ENUM`.
  - `GENERATED ALWAYS AS` columns included as read-only (no `default`, `frozen=True` or comment noting they are server-computed).
- Output is valid Python 3.11+ with standard library + `pydantic` imports only.
- fastapi-todo: `pg-flux gen --lang python` output is a drop-in replacement for `app/models.py` (manual model file can be deleted).
- Unit tests for the type mapping table (PG type → Python type).

**Dependencies:** None (but E4-T1 landing first means enums are structurally modeled, making the emitter more accurate).

---

## E5-T2 — Per-object file layout for Go and TypeScript codegen

**Summary:** `pg-flux gen` currently emits one `tables.go` / `tables.ts` file. For larger schemas this becomes unwieldy. Per-object layout emits `users.go`, `bookmarks.go`, etc.

**Acceptance criteria:**
- `pg-flux gen --layout=per-object` emits one file per table (named after the table) in the output directory.
- Enum types emitted into a shared `types.go` / `types.ts` file.
- Default layout (`--layout=single`) unchanged.
- Supported for Go, TypeScript, and Python (E5-T1) emitters.

**Dependencies:** E5-T1 (for Python support to be included from day one).
