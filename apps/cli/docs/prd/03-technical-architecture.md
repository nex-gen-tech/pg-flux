# 03 — Technical Architecture

**Document:** System Design & Technical Architecture
**Project:** pg-flux
**Version:** 1.0 | **Status:** Active Draft

---

## 3.1 Architecture Overview

pg-flux is a command-line tool structured as a **three-stage pipeline**. Each stage is implemented as a distinct Go module with a clean interface, allowing independent testing, replacement, and extension.

```
┌─────────────────────────────────────────────────────────────────┐
│                         pg-flux CLI                              │
│  (cobra commands: init | inspect | plan | apply | drift)        │
└──────────────────────┬──────────────────────────────────────────┘
                       │
         ┌─────────────┴────────────┐
         │                          │
         ▼                          ▼
┌─────────────────┐      ┌──────────────────────┐
│  SOURCE PARSER  │      │  STATE INSPECTOR      │
│  (Module: src)  │      │  (Module: inspector)  │
│                 │      │                       │
│ .sql files      │      │ PostgreSQL 18 system  │
│ → AST (Desired) │      │ catalogs → AST (Live) │
└────────┬────────┘      └──────────┬────────────┘
         │                          │
         └──────────┬───────────────┘
                    │
                    ▼
         ┌──────────────────────┐
         │   DIFFER ENGINE      │
         │   (Module: differ)   │
         │                      │
         │  Desired AST         │
         │  ─── diff ──►        │
         │  Current AST         │
         │                      │
         │  → Raw Delta         │
         └──────────┬───────────┘
                    │
         ┌──────────┴───────────┐
         │                      │
         ▼                      ▼
┌─────────────────┐  ┌──────────────────────────┐
│ HAZARD DETECTOR │  │  DAG DEPENDENCY SORTER    │
│ (Module: hazard)│  │  (Module: dag)            │
│                 │  │                           │
│ Intercepts lock │  │  Orders DDL statements    │
│ / data loss /   │  │  by topological dependency│
│ constraint scan │  │  (FK, triggers, policies) │
└────────┬────────┘  └──────────┬────────────────┘
         │                      │
         └──────────┬───────────┘
                    │
                    ▼
         ┌──────────────────────┐
         │  EXECUTION PLAN      │
         │  (plan.ExecutionPlan)│
         │                      │
         │  []Statement{        │
         │    DDL, Hazards[],   │
         │    IsConcurrent,     │
         │    Timeout           │
         │  }                   │
         └──────────┬───────────┘
                    │
         ┌──────────┴───────────┐
         │                      │
         ▼                      ▼
┌─────────────────┐  ┌──────────────────────────┐
│  EXECUTOR       │  │  RENDERER                 │
│  (Module: exec) │  │  (Module: render)         │
│                 │  │                           │
│  Applies plan   │  │  Outputs plan as:         │
│  via pgx inside │  │  - Terminal (colored)     │
│  advisory lock  │  │  - Plain SQL              │
│  + transaction  │  │  - JSON (for AI/CI)       │
└─────────────────┘  └──────────────────────────┘
```

---

## 3.2 Module Specifications

### Module: `src` — Source Parser

**Responsibility:** Parse `.sql` files into a normalized Desired State AST.

**Key Dependencies:**
- `github.com/pganalyze/pg_query_go/v6` — PostgreSQL C parser wrapper

**Processing Steps:**
1. Recursively walk the schema directory, collecting `.sql` files sorted by filename (deterministic ordering).
2. For each file, invoke `pg_query.Parse()` to obtain a Protobuf parse tree.
3. Walk the parse tree and map each `RawStmt` node to a typed Go struct in the internal schema model.
4. Extract hint comments (`-- @renamed`, `-- @deprecated`, etc.) from the `Comment` tokens adjacent to each DDL node.
5. Resolve cross-file references (e.g., a function referenced by a trigger defined in a different file).
6. Return a single `SchemaState` struct representing the complete desired schema.

**Error Handling:**
- Syntax errors from `pg_query` are surfaced with file path and line number.
- Undefined cross-references (e.g., a trigger referring to a function not present anywhere in the schema files) return a structured validation error before the diff phase runs.

**Internal Schema Model:**
```go
type SchemaState struct {
    Tables     map[string]*Table
    Indexes    map[string]*Index
    Functions  map[string]*Function
    Triggers   map[string]*Trigger
    Policies   map[string]*Policy
    Sequences  map[string]*Sequence
    Types      map[string]*Type
    Extensions map[string]*Extension
    Views      map[string]*View
}

type Table struct {
    Schema      string
    Name        string
    OldName     string // populated if @renamed hint is present
    Columns     []*Column
    Constraints []*Constraint
    Partitioning *PartitionSpec
    Comment     string
}

type Column struct {
    Name        string
    OldName     string // populated if @renamed hint is present
    DataType    *DataType
    Nullable    bool
    Default     *Expression
    Generated   *GeneratedSpec // virtual or stored
    Collation   string
}

type Policy struct {
    Name       string
    Table      string
    Command    PolicyCommand // ALL, SELECT, INSERT, UPDATE, DELETE
    Roles      []string
    Using      *Expression   // normalized expression tree
    WithCheck  *Expression   // normalized expression tree
}
```

---

### Module: `inspector` — State Inspector

**Responsibility:** Query live PostgreSQL 18 system catalogs and build the Current State AST using the same schema model as the Source Parser.

**Key Dependencies:**
- `github.com/jackc/pgx/v5` — PostgreSQL driver

**Why system catalogs, not `information_schema`?**

`information_schema` is a standards-compliance layer that deliberately abstracts away PostgreSQL-specific details. Using it would mean missing: partial index conditions, expression indexes, RLS policy expression trees, trigger timing details, function volatility markers, and more. Direct catalog queries provide the ground truth.

**Catalog Query Map:**

| Schema Object | Primary Catalogs | Notes |
|--------------|-----------------|-------|
| Tables | `pg_class`, `pg_namespace` | `relkind = 'r'` for regular tables |
| Columns | `pg_attribute`, `pg_type` | Filter `attnum > 0 AND NOT attisdropped` |
| NOT NULL constraints | `pg_constraint` (PG18) | `contype = 'n'` — new in PG18; previously only in `pg_attribute.attnotnull` |
| Check constraints | `pg_constraint` | `contype = 'c'`, includes expression text |
| Primary keys | `pg_constraint`, `pg_index` | `contype = 'p'` |
| Foreign keys | `pg_constraint` | `contype = 'f'`, includes `confupdtype`, `confdeltype` |
| Unique constraints | `pg_constraint`, `pg_index` | `contype = 'u'` |
| Temporal constraints | `pg_constraint` | `WITHOUT OVERLAPS` (PG18: `conperiod = true`) |
| Indexes | `pg_index`, `pg_class`, `pg_am` | Includes expression, partial condition, opclass |
| Functions | `pg_proc`, `pg_language` | Captures `prosrc`, `provolatile`, `proparallel`, `proisstrict` |
| Triggers | `pg_trigger`, `pg_proc` | Maps trigger → function dependency |
| RLS Policies | `pg_policy` | Captures `polcmd`, `polroles`, `polqual`, `polwithcheck` as deparse trees |
| Sequences | `pg_sequence`, `pg_class` | Owned-by column relationships |
| Views | `pg_class`, `pg_rewrite` | `relkind = 'v'` |
| Extensions | `pg_extension` | |
| Enum Types | `pg_type`, `pg_enum` | `typtype = 'e'` |
| Generated Columns | `pg_attribute` | `attgenerated = 'v'` (virtual, PG18 default) or `'s'` (stored) |

**Critical PG18 Catalog Changes to Handle:**

1. **`NOT NULL` in `pg_constraint`:** In PG18, `NOT NULL` constraints are stored as named entries in `pg_constraint` (type `'n'`). The inspector must query this table for NOT NULL constraints rather than relying solely on `pg_attribute.attnotnull`. Both sources must be reconciled.

2. **`pg_attribute.attcacheoff` removed:** PG18 removes this column. Any existing catalog query copying from older tools that references `attcacheoff` will fail. All queries must be verified against PG18 schema.

3. **Virtual generated columns:** `pg_attribute.attgenerated = 'v'` indicates a virtual generated column (PG18 default). The inspector must emit `GENERATED ALWAYS AS (...) VIRTUAL` in the AST.

4. **Temporal constraints:** `pg_constraint.conperiod = true` marks a temporal exclusion on `WITHOUT OVERLAPS`. The inspector must capture the `conkey` array to identify the period column.

**Concurrent Catalog Queries:**

The inspector uses Go goroutines to query all catalog tables concurrently, then merges results. A single connection pool is shared:

```go
func (i *Inspector) Inspect(ctx context.Context) (*SchemaState, error) {
    g, ctx := errgroup.WithContext(ctx)
    var tables, indexes, functions, triggers, policies, sequences atomic.Value

    g.Go(func() error { t, err := i.queryTables(ctx); tables.Store(t); return err })
    g.Go(func() error { t, err := i.queryIndexes(ctx); indexes.Store(t); return err })
    g.Go(func() error { t, err := i.queryFunctions(ctx); functions.Store(t); return err })
    g.Go(func() error { t, err := i.queryTriggers(ctx); triggers.Store(t); return err })
    g.Go(func() error { t, err := i.queryPolicies(ctx); policies.Store(t); return err })
    g.Go(func() error { t, err := i.querySequences(ctx); sequences.Store(t); return err })

    if err := g.Wait(); err != nil {
        return nil, fmt.Errorf("inspector: catalog query failed: %w", err)
    }
    // merge results...
}
```

---

### Module: `differ` — Differ Engine

**Responsibility:** Compare the Desired State AST and Current State AST, producing a list of typed `Change` objects.

**Comparison Strategy:**
1. All comparisons are performed on the internal schema model (Go structs), not on raw SQL strings.
2. Expression trees (`pg_policy.polqual`, function bodies, index expressions) are normalized using `pg_query.Fingerprint()` for semantic equivalence.
3. Rename detection runs **before** add/drop detection by checking for `@renamed` hints in the Desired State.

**Change Types:**
```go
type ChangeType string

const (
    ChangeType_CreateTable     ChangeType = "CREATE_TABLE"
    ChangeType_DropTable       ChangeType = "DROP_TABLE"
    ChangeType_RenameTable     ChangeType = "RENAME_TABLE"
    ChangeType_AddColumn       ChangeType = "ADD_COLUMN"
    ChangeType_DropColumn      ChangeType = "DROP_COLUMN"
    ChangeType_RenameColumn    ChangeType = "RENAME_COLUMN"
    ChangeType_AlterColumnType ChangeType = "ALTER_COLUMN_TYPE"
    ChangeType_SetNotNull      ChangeType = "SET_NOT_NULL"
    ChangeType_DropNotNull     ChangeType = "DROP_NOT_NULL"
    ChangeType_AddDefault      ChangeType = "ADD_DEFAULT"
    ChangeType_DropDefault     ChangeType = "DROP_DEFAULT"
    ChangeType_CreateIndex     ChangeType = "CREATE_INDEX"
    ChangeType_DropIndex       ChangeType = "DROP_INDEX"
    ChangeType_AlterIndex      ChangeType = "ALTER_INDEX"
    ChangeType_CreateConstraint ChangeType = "CREATE_CONSTRAINT"
    ChangeType_DropConstraint  ChangeType = "DROP_CONSTRAINT"
    ChangeType_CreateFunction  ChangeType = "CREATE_FUNCTION"
    ChangeType_DropFunction    ChangeType = "DROP_FUNCTION"
    ChangeType_AlterFunction   ChangeType = "ALTER_FUNCTION"
    ChangeType_CreateTrigger   ChangeType = "CREATE_TRIGGER"
    ChangeType_DropTrigger     ChangeType = "DROP_TRIGGER"
    ChangeType_CreatePolicy    ChangeType = "CREATE_POLICY"
    ChangeType_DropPolicy      ChangeType = "DROP_POLICY"
    ChangeType_AlterPolicy     ChangeType = "ALTER_POLICY"
    ChangeType_CreateSequence  ChangeType = "CREATE_SEQUENCE"
    ChangeType_DropSequence    ChangeType = "DROP_SEQUENCE"
    ChangeType_AlterSequence   ChangeType = "ALTER_SEQUENCE"
    ChangeType_CreateView      ChangeType = "CREATE_VIEW"
    ChangeType_DropView        ChangeType = "DROP_VIEW"
    ChangeType_CreateEnum      ChangeType = "CREATE_ENUM"
    ChangeType_AlterEnum       ChangeType = "ALTER_ENUM"
    ChangeType_DropEnum        ChangeType = "DROP_ENUM"
)
```

**Rename Resolution Algorithm:**
```
For each column C in desired_schema.tables[T].columns:
  if C has @renamed hint with source S:
    if live_schema.tables[T].columns contains S:
      emit Change{Type: RENAME_COLUMN, From: S, To: C.Name}
    else if live_schema.tables[T].columns contains C.Name:
      // column already renamed; no-op
      skip
    else:
      return error("rename source column '%s' not found in live schema", S)
  else if not in live_schema:
    emit Change{Type: ADD_COLUMN, Column: C}

For each column C in live_schema.tables[T].columns:
  if not in desired_schema (accounting for pending renames):
    emit Change{Type: DROP_COLUMN, Column: C}
```

---

### Module: `dag` — Dependency Sorter

**Responsibility:** Given a list of `Change` objects, produce a topologically sorted `ExecutionPlan` that respects object dependency ordering.

**Dependency Rules:**

**Creation order (topological sort forward):**
```
Extensions → Enum Types → Sequences → Tables → Indexes (concurrent)
→ Functions → Triggers → Foreign Keys → Check Constraints → NOT NULL Constraints
→ RLS Enable → RLS Policies → Views → Materialized Views
```

**Deletion order (topological sort reverse):**
```
Materialized Views → Views → RLS Policies → RLS Disable → NOT NULL Constraints
→ Check Constraints → Foreign Keys → Triggers → Functions
→ Indexes → Tables → Sequences → Enum Types → Extensions
```

**DAG Implementation:**
- Nodes: each schema object (identified by type + qualified name).
- Edges: dependency relationships derived from `pg_depend` and explicit parsing.
- Algorithm: Kahn's algorithm for topological sort (cycle detection included).
- Cycles: If a circular dependency is detected (which should not occur in valid PostgreSQL schema but could arise from misconfigured source files), the tool reports an error identifying the cycle.

---

### Module: `hazard` — Hazard Detector

**Responsibility:** Analyze each `Change` in the sorted execution plan and attach zero or more `Hazard` objects to statements that could harm production.

**Hazard Types:**

```go
type HazardType string

const (
    HazardType_DataLoss         HazardType = "DATA_LOSS"
    HazardType_TableLock        HazardType = "TABLE_LOCK"
    HazardType_ConstraintScan   HazardType = "CONSTRAINT_SCAN"
    HazardType_ColumnTypeChange HazardType = "COLUMN_TYPE_CHANGE"
    HazardType_IndexRebuild     HazardType = "INDEX_REBUILD"
    HazardType_SequenceReset    HazardType = "SEQUENCE_RESET"
    HazardType_FunctionSignatureChange HazardType = "FUNCTION_SIGNATURE_CHANGE"
    HazardType_EnumValueDrop    HazardType = "ENUM_VALUE_DROP"
    HazardType_NotReplicaSafe   HazardType = "NOT_REPLICA_SAFE"
)
```

**Automatic Rewrites:**

The hazard detector is not just a passive checker — it actively rewrites dangerous operations into safe equivalents where possible:

| Dangerous Operation | Safe Rewrite |
|--------------------|-------------|
| `CREATE INDEX` | `CREATE INDEX CONCURRENTLY` |
| `DROP INDEX` | `DROP INDEX CONCURRENTLY` |
| `ADD CONSTRAINT ... CHECK` on populated table | 3-step: ADD NOT VALID → VALIDATE → (done) |
| `ALTER COLUMN SET NOT NULL` on populated table | 4-step: ADD CHECK NOT VALID → VALIDATE → SET NOT NULL → DROP CHECK |
| `ADD FOREIGN KEY` | `ADD FOREIGN KEY NOT VALID` → `VALIDATE CONSTRAINT` |
| Online index replacement (changing index definition) | RENAME old → CREATE new CONCURRENTLY → DROP old CONCURRENTLY |

**Table Size Awareness:**
The hazard detector queries `pg_class.reltuples` and `pg_class.relpages` to estimate table size. For empty tables, some safe-rewrites (like the 4-step NOT NULL pattern) can be simplified to a single `ALTER COLUMN SET NOT NULL`.

---

### Module: `exec` — Executor

**Responsibility:** Execute an `ExecutionPlan` against the live database safely.

**Execution Strategy:**
1. Acquire `pg_try_advisory_lock(hash_of_connection_string)` to prevent concurrent migrations.
2. Begin transaction (`BEGIN`).
3. For each statement in the plan:
   - If `IsConcurrent == false`: execute inside the transaction with `statement_timeout` and `lock_timeout` set.
   - If `IsConcurrent == true`: execute outside the transaction (concurrent index operations cannot run inside a transaction).
4. Commit (`COMMIT`).
5. If any concurrent statement fails after the transaction committed: record the failure state, report the remaining steps, and exit with a non-zero code.
6. Inject `ANALYZE {affected_tables}` after all structural changes.
7. Release advisory lock.

---

## 3.3 Technology Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| Language | Go 1.22+ | Single-binary distribution, excellent concurrency, CGO for C parser |
| SQL Parser | `pg_query_go/v6` | The only 100%-accurate PostgreSQL parser for Go |
| DB Driver | `pgx/v5` | High-performance, low-allocation PostgreSQL driver |
| CLI Framework | `cobra` + `viper` | Industry standard Go CLI; env var support via viper |
| Protobuf | `google.golang.org/protobuf` | For pg_query_go AST structs |
| Error Handling | `errors` + `fmt.Errorf("%w")` | Standard Go error wrapping |
| Testing | `testing` + `testify` | Standard Go test framework |
| Integration Tests | Docker + `testcontainers-go` | Spin up real PostgreSQL 18 for tests |
| Build | `goreleaser` | Cross-platform binary release with checksums |

---

## 3.4 Data Flow: Complete Example

Given this change in `schema.sql`:
```sql
-- Adding an index and renaming a column

CREATE TABLE orders (
    id uuid DEFAULT uuidv7() PRIMARY KEY,
    -- @renamed from=customer_id
    buyer_id uuid NOT NULL,
    total numeric(10,2),
    created_at timestamptz DEFAULT now()
);

CREATE INDEX CONCURRENTLY idx_orders_buyer ON orders(buyer_id);
```

**Stage 1 (Source Parser):**
- Parses `CREATE TABLE`, extracts `@renamed from=customer_id` hint on `buyer_id`.
- Records `buyer_id` with `OldName: "customer_id"`.
- Records the index `idx_orders_buyer`.

**Stage 2 (State Inspector):**
- Queries live DB, finds `orders` table with column `customer_id` (not `buyer_id`).
- Finds no existing index `idx_orders_buyer`.

**Stage 3 (Differ):**
- Rename algorithm: `customer_id` → `buyer_id` detected via hint. Emits `RENAME_COLUMN`.
- Index diff: `idx_orders_buyer` absent in live. Emits `CREATE_INDEX`.

**Stage 4 (DAG Sort):**
- RENAME_COLUMN must precede CREATE_INDEX (index references the new name).
- Final order: [RENAME_COLUMN, CREATE_INDEX].

**Stage 5 (Hazard Detector):**
- `RENAME_COLUMN` — no hazard (rename is metadata-only in PG, no lock).
- `CREATE_INDEX` — rewrite to `CREATE INDEX CONCURRENTLY`. Attach `HazardType_IndexRebuild` advisory.

**Stage 6 (Execution Plan Output):**
```sql
-- Statement 1 (no hazards)
SET SESSION lock_timeout = '3s';
ALTER TABLE public.orders RENAME COLUMN customer_id TO buyer_id;

-- Statement 2 (advisory: INDEX_REBUILD — concurrent build may impact I/O)
SET SESSION statement_timeout = '20min';
SET SESSION lock_timeout = '3s';
CREATE INDEX CONCURRENTLY idx_orders_buyer ON public.orders USING btree (buyer_id);
```
