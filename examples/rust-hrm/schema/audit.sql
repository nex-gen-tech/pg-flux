-- audit.change_log: immutable event log of all DML on employees.
-- Exercises: cross-schema table (tracked via target_schemas: [public, audit]),
-- IDENTITY PK, JSONB old/new snapshots, BRIN on the append-only timestamp.
CREATE TABLE audit.change_log (
  id          bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  table_name  text        NOT NULL,
  operation   text        NOT NULL,
  row_id      text,
  old_data    jsonb,
  new_data    jsonb,
  changed_by  text,
  changed_at  timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT change_log_op_check CHECK (operation IN ('INSERT', 'UPDATE', 'DELETE'))
);

-- Composite index for querying a table's history sorted by time.
CREATE INDEX idx_change_log_table ON audit.change_log (table_name, changed_at DESC);
-- BRIN on changed_at: append-only time-series column, perfect BRIN candidate.
CREATE INDEX idx_change_log_brin  ON audit.change_log USING BRIN (changed_at);
