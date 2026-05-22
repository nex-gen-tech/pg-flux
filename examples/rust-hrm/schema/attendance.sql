-- attendance: time-series punch-in/punch-out records.
-- Exercises: range-partitioned table with composite PK, BRIN index on the
-- partition key, and IDENTITY column inside a partitioned table.
-- Partitioned by RANGE on punch_in so each year's data lives in its own file.
CREATE TABLE public.attendance (
  id          bigint      GENERATED ALWAYS AS IDENTITY,
  org_id      bigint      NOT NULL,
  employee_id uuid        NOT NULL,
  shift_id    bigint,
  punch_in    timestamptz NOT NULL,
  punch_out   timestamptz,
  notes       text,
  created_at  timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id, punch_in),
  CONSTRAINT attendance_punch_order CHECK (punch_out IS NULL OR punch_out > punch_in)
) PARTITION BY RANGE (punch_in);

-- Year partitions — add more as needed; pg-flux tracks them as child objects.
CREATE TABLE public.attendance_2025 PARTITION OF public.attendance
  FOR VALUES FROM ('2025-01-01') TO ('2026-01-01');

CREATE TABLE public.attendance_2026 PARTITION OF public.attendance
  FOR VALUES FROM ('2026-01-01') TO ('2027-01-01');

-- BRIN is ideal here: punch_in is monotonically increasing (insert-only pattern)
-- so BRIN gives excellent range filtering at near-zero storage cost.
CREATE INDEX idx_attendance_brin     ON public.attendance USING BRIN (punch_in);
CREATE INDEX idx_attendance_employee ON public.attendance (employee_id, punch_in);
