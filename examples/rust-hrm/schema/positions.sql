-- positions: tracks an employee's job title and level over a date range.
-- Exercises: IDENTITY PK, daterange type, EXCLUDE constraint (no overlapping
-- positions for the same employee), GIST index on the range column, BRIN.
CREATE TABLE public.positions (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  org_id        bigint      NOT NULL,
  employee_id   uuid        NOT NULL REFERENCES public.employees (id) ON DELETE CASCADE,
  title         text        NOT NULL,
  level         public.position_level NOT NULL,
  department_id bigint      REFERENCES public.departments (id) ON DELETE SET NULL,
  salary        numeric(12,2),
  valid_during  daterange   NOT NULL,
  created_at    timestamptz NOT NULL DEFAULT now(),
  CONSTRAINT positions_salary_positive CHECK (salary IS NULL OR salary > 0),
  -- An employee can only hold one position at a time (date ranges must not overlap).
  CONSTRAINT positions_no_overlap EXCLUDE USING gist (
    employee_id WITH =,
    valid_during WITH &&
  )
);

CREATE INDEX idx_positions_employee     ON public.positions (employee_id);
CREATE INDEX idx_positions_department   ON public.positions (department_id);
CREATE INDEX idx_positions_valid_during ON public.positions USING GIST (valid_during);
-- BRIN is efficient for the append-mostly created_at column.
CREATE INDEX idx_positions_created_brin ON public.positions USING BRIN (created_at);
